package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openshieldai/openshield/lib/rules"
	"github.com/openshieldai/openshield/lib/types"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/openshieldai/openshield/lib"

	"github.com/google/uuid"
)

type ChatCompletionRequest struct {
	Model     string          `json:"model"`
	Messages  []types.Message `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
	Stream    bool            `json:"stream"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int           `json:"index"`
	Message      types.Message `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Provider interface {
	CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error)
	CreateChatCompletionStream(ctx context.Context, req ChatCompletionRequest) (Stream, error)
}

type Stream interface {
	Recv() (StreamResponse, error)
	Close()
}
type StreamResponse interface {
	GetContent() string
	GetFinishReason() string
	GetID() string
	GetCreated() int64
	GetModel() string
}

func HandleChatCompletion(w http.ResponseWriter, r *http.Request, provider Provider, req ChatCompletionRequest) error {
	if req.Stream {
		return handleStreamingRequest(w, r, provider, req)
	} else {
		return handleNonStreamingRequest(w, r, provider, req)
	}
}

func handleStreamingRequest(w http.ResponseWriter, r *http.Request, provider Provider, req ChatCompletionRequest) error {
	stream, err := provider.CreateChatCompletionStream(r.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to create chat completion stream: %v", err)
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming unsupported")
	}

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return nil
		}
		if err != nil {
			return fmt.Errorf("error receiving stream: %v", err)
		}

		chunk := struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Index        int    `json:"index"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}{
			ID:      response.GetID(),
			Object:  "chat.completion.chunk",
			Created: response.GetCreated(),
			Model:   response.GetModel(),
			Choices: []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Index        int    `json:"index"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Delta: struct {
						Content string `json:"content"`
					}{
						Content: response.GetContent(),
					},
					Index:        0,
					FinishReason: response.GetFinishReason(),
				},
			},
		}

		data, err := json.Marshal(chunk)
		if err != nil {
			return fmt.Errorf("error marshaling response: %v", err)
		}

		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, provider Provider, req ChatCompletionRequest) error {
	resp, err := provider.CreateChatCompletion(r.Context(), req)
	if err != nil {
		return fmt.Errorf("failed to create chat completion: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return fmt.Errorf("error encoding response: %v", err)
	}

	// Perform response audit logging
	PerformResponseAuditLogging(r, resp)

	return nil
}

func GetProductIDFromAPIKey(ctx context.Context, apiKeyId uuid.UUID) (uuid.UUID, error) {
	var productIDStr string
	err := lib.DB().Table("api_keys").Where("id = ?", apiKeyId).Pluck("product_id", &productIDStr).Error
	if err != nil {
		return uuid.Nil, err
	}

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse product_id as UUID: %v", err)
	}

	return productID, nil
}

func PerformAuditLogging(r *http.Request, logType string, messageType string, body []byte) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)

	productID, err := GetProductIDFromAPIKey(r.Context(), apiKeyId)
	if err != nil {

	}

	lib.AuditLogs(string(body), logType, apiKeyId, messageType, productID, r)
}

func PerformResponseAuditLogging(r *http.Request, resp *ChatCompletionResponse) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)
	productID, err := GetProductIDFromAPIKey(r.Context(), apiKeyId)
	if err != nil {
		return
	}

	responseJSON, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	auditLog := lib.AuditLogs(string(responseJSON), "chat_completion", apiKeyId, "output", productID, r)

	if auditLog == nil {
		log.Printf("Failed to create audit log")
		return
	}

	lib.LogUsage(
		resp.Model,
		0,
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens,
		resp.Choices[0].FinishReason,
		"chat_completion",
		productID,
		auditLog.Id,
	)
}
func HandleContextCache(ctx context.Context, req ChatCompletionRequest, productID uuid.UUID) (string, bool, error) {
	config := lib.GetConfig()
	if !config.Settings.ContextCache.Enabled {
		return "", false, nil
	}

	prompt := lastUserMessage(req.Messages)
	sessionID := fmt.Sprintf("%s-%s", productID.String(), req.Model)

	cachedResponse, err := lib.GetContextCache(prompt, sessionID)
	if err != nil {
		if err.Error() == "cache hit" {
			return cachedResponse, true, nil
		}
		if err.Error() == "cache miss" {
			return cachedResponse, false, err
		}
		// Log the error for debugging purposes
		log.Printf("Error getting context cache: %v", err)
		return "", false, nil
	}

	return "", false, nil
}

func SetContextCacheResponse(ctx context.Context, req ChatCompletionRequest, resp *ChatCompletionResponse, productID uuid.UUID) error {
	config := lib.GetConfig()
	if !config.Settings.ContextCache.Enabled {
		return nil
	}

	prompt := lastUserMessage(req.Messages)
	sessionID := fmt.Sprintf("%s-%s", productID.String(), req.Model)
	answer := resp.Choices[0].Message.Content

	return lib.SetContextCache(prompt, answer, sessionID)
}

func lastUserMessage(messages []types.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}
func CreateChatCompletionResponseFromCache(cachedResponse string, model string) (*ChatCompletionResponse, error) {
	var cachedResp struct {
		Prompt    string `json:"prompt"`
		Answer    string `json:"answer"`
		ProductID string `json:"product_id"`
	}
	err := json.Unmarshal([]byte(cachedResponse), &cachedResp)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling cached response: %v", err)
	}

	return &ChatCompletionResponse{
		ID:      "cached_" + cachedResp.ProductID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []Choice{
			{
				Index: 0,
				Message: types.Message{
					Role:    "assistant",
					Content: cachedResp.Answer,
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}, nil
}
func HandleChatCompletionRequest(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	productID, ok := ctx.Value("productID").(uuid.UUID)
	if !ok {
		return nil, fmt.Errorf("productID not found in context")
	}

	cachedResponse, cacheHit, err := HandleContextCache(ctx, req, productID)
	if err != nil {
		log.Printf("Error handling context cache: %v", err)
	}
	if cacheHit {
		var resp ChatCompletionResponse
		err = json.Unmarshal([]byte(cachedResponse), &resp)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling cached response: %v", err)
		}
		return &resp, nil
	}

	return nil, nil
}

func ValidateAndFilterRequest(r *http.Request, req interface{}) (bool, string, error) {
	filtered, message, errorMessage := rules.Input(r, req)
	if errorMessage != nil {
		return false, "", fmt.Errorf("error processing input: %v", errorMessage)
	}

	if filtered {
		logMessage, err := json.Marshal(message)
		if err != nil {
			return false, "", fmt.Errorf("error marshalling message: %v", err)
		}
		PerformAuditLogging(r, "rule", "filtered", logMessage)
		return true, message, nil
	}

	return false, "", nil
}
