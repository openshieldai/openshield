package openai

import (
	"bytes"
	"encoding/json"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/rules"
	"github.com/sashabaranov/go-openai"
)

var client *openai.Client

func ListModelsHandler(c *fiber.Ctx) error {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	client = openai.NewClient(openAIAPIKey)

	getCache, cacheStatus, err := lib.GetCache(c.Path())
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		c.Set("OS-Cache-Status", "HIT")
		return c.Send(getCache)
	} else {
		log.Printf("Cache miss for %v", cacheStatus)
		res, err := client.ListModels(c.Context())
		if err != nil {
			return lib.ErrorResponse(c, err)
		}

		if config.Settings.Cache.Enabled {
			c.Set("OS-Cache-Status", "MISS")
			resJson, err := json.Marshal(res)
			if err != nil {
				log.Printf("Error marshalling response to JSON: %v", err)
				return c.Status(500).JSON(fiber.Map{
					"error": fiber.Map{
						"message": "Internal server error",
						"type":    "server_error",
					},
				})
			}

			_, err = lib.SetCache(c.Path(), resJson)
			if err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		} else {
			c.Set("OS-Cache-Status", "BYPASS")
		}

		return c.JSON(res)
	}
}

func GetModelHandler(c *fiber.Ctx) error {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey

	client = openai.NewClient(openAIAPIKey)
	getCache, cacheStatus, err := lib.GetCache(c.Path())
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		c.Set("OS-Cache-Status", "HIT")
		return c.Send(getCache)
	} else {
		log.Printf("Cache miss for %v", cacheStatus)
		res, err := client.GetModel(c.Context(), c.Params("model"))
		if err != nil {
			return lib.ErrorResponse(c, err)
		}

		if config.Settings.Cache.Enabled {
			c.Set("OS-Cache-Status", "MISS")
			resJson, err := json.Marshal(res)
			if err != nil {
				log.Printf("Error marshalling response to JSON: %v", err)
				return c.Status(500).JSON(fiber.Map{
					"error": fiber.Map{
						"message": "Internal server error",
						"type":    "server_error",
					},
				})
			}

			_, err = lib.SetCache(c.Path(), resJson)
			if err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		} else {
			c.Set("OS-Cache-Status", "BYPASS")
		}

		return c.JSON(res)
	}
}

func ChatCompletionHandler(c *fiber.Ctx) error {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	var inputJson bytes.Buffer
	err := json.Compact(&inputJson, c.BodyRaw())
	if err != nil {
		log.Printf("Error compacting JSON: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Invalid request body",
				"type":    "invalid_request_error",
			}),
		})
	}

	lib.AuditLogs(inputJson.String(), "openai_chat_completion", c.Locals("apiKeyId").(uuid.UUID), "input", c)

	var openAIData []byte
	openAIData = c.BodyRaw()

	client = openai.NewClient(openAIAPIKey)
	var req openai.ChatCompletionRequest
	err = json.Unmarshal(openAIData, &req)
	if err != nil {
		log.Printf("Error unmarshalling openAIData: %v", err)
		return c.Status(400).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": "Invalid request body",
				"type":    "invalid_request_error",
			}),
		})
	}

	filteredResp, errorMessage, err := rules.Input(c, req)
	//if err != nil {
	//	log.Printf("Error filtering input: %v", err)
	//	return c.Status(500).JSON(fiber.Map{
	//		"error": interface{}(fiber.Map{
	//			"message": "Internal server error",
	//			"type":    "server_error",
	//		}),
	//	})
	//}

	if filteredResp {
		return c.Status(400).JSON(fiber.Map{
			"error": interface{}(fiber.Map{
				"message": errorMessage,
				"type":    "rule_block",
			}),
		})
	}

	getCache, cacheStatus, err := lib.GetCache(string(openAIData))
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		c.Set("OS-Cache-Status", "HIT")
		return c.Send(getCache)
	} else {
		log.Printf("Cache miss for %v", cacheStatus)
		res, err := client.CreateChatCompletion(c.Context(), req)
		if err != nil {
			log.Printf("Error creating chat completion: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"error": interface{}(fiber.Map{
					"message": "Internal server error",
					"type":    "server_error",
				}),
			})
		}

		if err != nil {
			return lib.ErrorResponse(c, err)
		}

		if config.Settings.Cache.Enabled {
			c.Set("OS-Cache-Status", "MISS")
			resJson, err := json.Marshal(res)
			if err != nil {
				log.Printf("Error marshalling response to JSON: %v", err)
				return c.Status(500).JSON(fiber.Map{
					"error": fiber.Map{
						"message": "Internal server error",
						"type":    "server_error",
					},
				})
			}

			_, err = lib.SetCache(string(openAIData), resJson)
			if err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		} else {
			c.Set("OS-Cache-Status", "BYPASS")
		}

		outputJsonData, err := json.Marshal(res)
		lib.AuditLogs(string(outputJsonData), "openai_chat_completion", c.Locals("apiKeyId").(uuid.UUID), "output", c)
		// Fix me: predictedTokens is hard coded to 0, finish reason is hard coded to the first choice finish reason
		lib.Usage(res.Model, 0, res.Usage.TotalTokens, res.Usage.CompletionTokens, res.Usage.TotalTokens, string(res.Choices[0].FinishReason), "chat_completion")
		return c.JSON(res)
	}
}
