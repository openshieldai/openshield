package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
	"github.com/sashabaranov/go-openai"
)

var client *openai.Client

func ListModelsHandler(c *fiber.Ctx) error {
	settings := lib.NewSettings()
	openAIAPIKey := settings.OpenAI.APIKey
	client = openai.NewClient(openAIAPIKey)

	res, err := client.ListModels(c.Context())
	if err != nil {
		return lib.ErrorResponse(c, err)
	}

	return c.JSON(res)
}

func GetModelHandler(c *fiber.Ctx) error {
	settings := lib.NewSettings()
	openAIAPIKey := settings.OpenAI.APIKey

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

	var inputJson bytes.Buffer
	err = json.Compact(&inputJson, c.BodyRaw())
	if err != nil {
		log.Printf("Error compacting JSON: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Invalid request body",
				"type":    "invalid_request_error",
			}),
		})
	}

	lib.AuditLogs(inputJson.String(), "openai_chat_completion", apiKey.Id, "input", c)

	var openAIData []byte

	if settings.OpenShield.PIIService.Status {
		bodyReader := bytes.NewReader(c.BodyRaw())
		// TODO: Use Fiber Client here
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
	lib.AuditLogs(string(outputJsonData), "openai_chat_completion", apiKey.Id, "output", c)
	// Fix me: predictedTokens is hard coded to 0, finish reason is hard coded to the first choice finish reason
	lib.Usage(res.Model, 0, res.Usage.TotalTokens, res.Usage.CompletionTokens, res.Usage.TotalTokens, string(res.Choices[0].FinishReason), "chat_completion")
	return c.JSON(res)
}
