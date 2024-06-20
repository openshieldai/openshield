package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
	"github.com/sashabaranov/go-openai"
)

var client *openai.Client

type model struct {
	ID        string `json:"id"`
	ModelType string `json:"modelType"`
	Enabled   bool   `json:"enabled"`
}

type group struct {
	Enabled bool    `json:"enabled"`
	Models  []model `json:"models"`
}

type listModelsResponse struct {
	Models []openai.Model `json:"models"`
}

func getMergedGroups(basicModelFilePath, customModelFilePath string) (map[string]group, error) {
	basicModels, err := os.ReadFile(basicModelFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]group), nil
		}
		return nil, err
	}

	customModel, err := os.ReadFile(customModelFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]group), nil
		}
		return nil, err
	}

	var groups map[string]group
	err = json.Unmarshal(basicModels, &groups)
	if err != nil {
		return nil, err
	}

	var customGroups map[string]group
	err = json.Unmarshal(customModel, &customGroups)
	if err != nil {
		return nil, err
	}

	// Merge groups and customGroups
	for k, v := range customGroups {
		groups[k] = v
	}

	return groups, nil
}

func ListModelsHandler(c *fiber.Ctx) error {
	settings := lib.NewSettings()
	openAIAPIKey := settings.OpenAI.APIKey
	client = openai.NewClient(openAIAPIKey)

	models, err := client.ListModels(c.Context())
	if err != nil {
		return lib.ErrorResponse(c, err)
	}

	groups, err := getMergedGroups("./db/models.json", "./db/customModels.json")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var enabledModels []openai.Model
	for _, model := range models.Models {
		for _, group := range groups {
			for _, groupModel := range group.Models {
				if groupModel.ID == model.ID && groupModel.Enabled && group.Enabled {
					enabledModels = append(enabledModels, model)
					break
				}
			}
			if len(enabledModels) > 0 && enabledModels[len(enabledModels)-1].ID == model.ID {
				break
			}
		}
	}
	return c.JSON(listModelsResponse{Models: enabledModels})
}

func GetModelHandler(c *fiber.Ctx) error {
	settings := lib.NewSettings()
	openAIAPIKey := settings.OpenAI.APIKey

	groups, err := getMergedGroups("./db/models.json", "./db/customModels.json")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	modelID := c.Params("model")

	var modelEnabled bool
	var groupEnabled bool
	for _, group := range groups {
		for _, model := range group.Models {
			if model.ID == modelID {
				modelEnabled = model.Enabled
				groupEnabled = group.Enabled
				break
			}
		}
		if modelEnabled {
			break
		}
	}

	if !modelEnabled || !groupEnabled {
		return c.Status(403).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "This model is disabled. Please contact your administrator to enable",
				"type":    "invalid_request_error",
				"param":   "null",
				"code":    "null",
			}),
		})
	}

	client = openai.NewClient(openAIAPIKey)
	res, err := client.GetModel(c.Context(), c.Params("model"))
	if err != nil {
		return lib.ErrorResponse(c, err)
	}
	return c.JSON(res)
}

func ChatCompletionHandler(c *fiber.Ctx) error {
	settings := lib.NewSettings()
	openAIAPIKey := settings.OpenAI.APIKey
	token, err := lib.AuthHeaderParser(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Unauthorized",
				"type":    "authentication_error",
			}),
		})
	}

	apiKey, err := lib.GetApiKey(token)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Unauthorized",
				"type":    "authentication_error",
			}),
		})

	}

	lib.AuditLogs(string(c.BodyRaw()), "openai_chat_completion", apiKey.Id, "input")

	var openAIData []byte

	if settings.OpenShield.PIIService.Status {
		bodyReader := bytes.NewReader(c.BodyRaw())
		resp, err := http.Post(settings.OpenShield.PIIService.URL, "application/json", bodyReader)
		if err != nil {
			log.Printf("Error uploading data to PIIService: %v", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				log.Printf("Error closing response body: %v", err)
			}
		}(resp.Body)

		body, err := io.ReadAll(resp.Body)
		openAIData = body
	} else {
		openAIData = c.BodyRaw()
	}

	client = openai.NewClient(openAIAPIKey)
	req := new(openai.ChatCompletionRequest)
	err = json.Unmarshal(openAIData, req)
	if err != nil {
		log.Printf("Error unmarshalling openAIData: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Invalid request body",
				"type":    "invalid_request_error",
			}),
		})
	}

	res, err := client.CreateChatCompletion(c.Context(), *req)
	if err != nil {
		log.Printf("Error creating chat completion: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Internal server error",
				"type":    "server_error",
			}),
		})
	}
	outputJsonData, err := json.Marshal(res)
	lib.AuditLogs(string(outputJsonData), "openai_chat_completion", apiKey.Id, "output")
	return c.JSON(res)
}
