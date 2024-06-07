package openai

import (
	"encoding/json"
	"errors"
	"log"
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
	authToken, err := lib.AuthHeaderParser(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Unauthorized",
				"type":    "invalid_request_error",
				"param":   "Authorization",
			}),
		})
	}
	client = openai.NewClient(authToken)

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
	authToken, err := lib.AuthHeaderParser(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Unauthorized",
				"type":    "invalid_request_error",
				"param":   "Authorization",
			}),
		})
	}

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

	client = openai.NewClient(authToken)
	res, err := client.GetModel(c.Context(), c.Params("model"))
	if err != nil {
		return lib.ErrorResponse(c, err)
	}
	return c.JSON(res)
}

func ChatCompletionHandler(c *fiber.Ctx) error {
	authToken, err := lib.AuthHeaderParser(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Unauthorized",
				"type":    "invalid_request_error",
				"param":   "Authorization",
			}),
		})
	}

	client = openai.NewClient(authToken)
	req := new(openai.ChatCompletionRequest)
	if err := c.BodyParser(req); err != nil {
		log.Printf("Error parsing request body: %v", err)
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
	return c.JSON(res)
}
