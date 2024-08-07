package openai

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"io"
	"log"
	"net/http"

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

	// Read the entire request body
	body, err := io.ReadAll(c.Request().BodyStream())
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "Error reading request body",
				"type":    "invalid_request_error",
			},
		})
	}

	// Parse the request
	var req openai.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "Invalid request body",
				"type":    "invalid_request_error",
			},
		})
	}

	// Perform audit logging
	lib.AuditLogs(string(body), "openai_chat_completion", c.Locals("apiKeyId").(uuid.UUID), "input", c)

	// Apply input rules
	filteredResp, errorMessage, err := rules.Input(c, req)
	if filteredResp {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fiber.Map{
				"message": errorMessage,
				"type":    "rule_block",
			},
		})
	}

	// Check cache
	getCache, cacheStatus, err := lib.GetCache(string(body))
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		c.Set("OS-Cache-Status", "HIT")
		return c.Send(getCache)
	}

	log.Printf("Cache miss for %v", cacheStatus)

	// If streaming is requested, use the HTTP handler
	if req.Stream {
		// Create a closure that captures the necessary variables
		handler := func(w http.ResponseWriter, r *http.Request) {
			// Create a new request body
			newBody, err := json.Marshal(req)
			if err != nil {
				log.Printf("Error marshaling request: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			HTTPStreamHandler(w, r, newBody, openAIAPIKey)
		}

		return adaptor.HTTPHandler(http.HandlerFunc(handler))(c)
	}

	// Non-streaming logic
	client := openai.NewClient(openAIAPIKey)
	resp, err := client.CreateChatCompletion(c.Context(), req)
	if err != nil {
		log.Printf("Error creating chat completion: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fiber.Map{
				"message": "Failed to create chat completion",
				"type":    "server_error",
			},
		})
	}

	// Cache the response if caching is enabled
	if config.Settings.Cache.Enabled {
		c.Set("OS-Cache-Status", "MISS")
		resJson, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshalling response to JSON: %v", err)
		} else {
			_, err = lib.SetCache(string(body), resJson)
			if err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		}
	} else {
		c.Set("OS-Cache-Status", "BYPASS")
	}

	// Perform audit logging for the response
	responseJSON, _ := json.Marshal(resp)
	lib.AuditLogs(string(responseJSON), "openai_chat_completion", c.Locals("apiKeyId").(uuid.UUID), "output", c)
	lib.Usage(resp.Model, 0, resp.Usage.TotalTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, string(resp.Choices[0].FinishReason), "chat_completion")
	return c.JSON(resp)
}

func HTTPStreamHandler(w http.ResponseWriter, r *http.Request, body []byte, openAIAPIKey string) {
	// Log the received request body for debugging
	log.Printf("Received request body in HTTPStreamHandler: %s", string(body))

	var req openai.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Ensure the stream flag is set to true
	req.Stream = true

	// Apply input rules
	filteredResp, errorMessage, err := rules.Input(nil, req)
	if filteredResp {
		log.Printf("Request blocked by input rules: %s", errorMessage)
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}

	client := openai.NewClient(openAIAPIKey)
	stream, err := client.CreateChatCompletionStream(r.Context(), req)
	if err != nil {
		log.Printf("Failed to create chat completion stream: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create chat completion stream: %v", err), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Println("Streaming unsupported")
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}

		if err != nil {
			log.Printf("Error receiving stream: %v", err)
			fmt.Fprintf(w, "data: {\"error\": \"%v\"}\n\n", err)
			flusher.Flush()
			return
		}

		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			data, err := json.Marshal(response)
			if err != nil {
				log.Printf("Error marshaling response: %v", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}
	}
}
