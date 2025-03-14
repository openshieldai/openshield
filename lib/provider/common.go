package provider

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/openshieldai/openshield/lib/rules"
	"github.com/openshieldai/openshield/lib/types"

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

func performProviderAuditLog(r *http.Request, logPrefix string, messageType string, content interface{}) {
	apiKeyID := r.Context().Value("apiKeyId").(uuid.UUID)

	productID, err := getProductIDFromAPIKey(apiKeyID)
	if err != nil {
		hashedApiKeyId := sha256.Sum256([]byte(apiKeyID.String()))
		log.Printf("Failed to retrieve ProductID for apiKeyId %x: %v", hashedApiKeyId, err)
		return
	}

	var messageStr string
	switch v := content.(type) {
	case string:
		messageStr = v
	case []byte:
		messageStr = string(v)
	default:
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			log.Printf("Failed to marshal content: %v", err)
			return
		}
		messageStr = string(jsonBytes)
	}

	auditLog := lib.AuditLogs(
		messageStr,
		logPrefix+"_"+messageType,
		apiKeyID,
		messageType,
		productID,
		r,
	)

	if auditLog == nil {
		log.Printf("Failed to create audit log for %s", logPrefix)
	}
}

func LogProviderInput(r *http.Request, provider string, content interface{}) {
	performProviderAuditLog(r, provider, "input", content)
}

func LogProviderOutput(r *http.Request, provider string, content interface{}) {
	performProviderAuditLog(r, provider, "output", content)
}

func LogProviderError(r *http.Request, provider string, err error) {
	performProviderAuditLog(r, provider, "error", err.Error())
}

// getProductIDFromAPIKey centralizes the product ID lookup logic
func getProductIDFromAPIKey(apiKeyID uuid.UUID) (uuid.UUID, error) {
	var productIDStr string
	err := lib.DB().Table("api_keys").Where("id = ?", apiKeyID).Pluck("product_id", &productIDStr).Error
	if err != nil {
		return uuid.Nil, err
	}

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse product_id as UUID")
	}

	return productID, nil
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

type InputRequest struct {
	Model     string          `json:"model"`
	Messages  []types.Message `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
	Stream    bool            `json:"stream"`
}

func ProcessInput(w http.ResponseWriter, r *http.Request, req ChatCompletionRequest) (bool, error) {
	inputRequest := struct {
		Model     string          `json:"model"`
		Messages  []types.Message `json:"messages"`
		MaxTokens int             `json:"max_tokens"`
		Stream    bool            `json:"stream"`
	}{
		Model:     req.Model,
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}

	filtered, message, errorMessage := rules.Input(r, inputRequest)
	if errorMessage != nil {
		HandleError(w, fmt.Errorf("error processing input: %v", errorMessage), http.StatusBadRequest)
		return false, errorMessage
	}

	if filtered {
		PerformAuditLogging(r, "rule", "filtered", []byte(message))
		HandleError(w, fmt.Errorf("%v", message), http.StatusBadRequest)
		return true, nil
	}

	log.Println("Input processing completed successfully")
	return false, nil
}

func HandleError(w http.ResponseWriter, err error, statusCode int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), statusCode)
}
func HandleCommonRequestLogic(w http.ResponseWriter, r *http.Request, providerName string) (ChatCompletionRequest, context.Context, uuid.UUID, bool) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		HandleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return ChatCompletionRequest{}, nil, uuid.Nil, false
	}

	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		HandleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return ChatCompletionRequest{}, nil, uuid.Nil, false
	}

	log.Printf("Received request: %+v", req)

	apiKeyID, ok := r.Context().Value("apiKeyId").(uuid.UUID)
	if !ok {
		HandleError(w, fmt.Errorf("apiKeyId not found in context"), http.StatusInternalServerError)
		return ChatCompletionRequest{}, nil, uuid.Nil, false
	}

	productID, err := GetProductIDFromAPIKey(r.Context(), apiKeyID)
	if err != nil {
		HandleError(w, fmt.Errorf("error getting productID: %v", err), http.StatusInternalServerError)
		return ChatCompletionRequest{}, nil, uuid.Nil, false
	}

	ctx := context.WithValue(r.Context(), "productID", productID)

	// Check cache before running rules
	if !req.Stream {
		cachedResponse, cacheHit, err := HandleContextCache(ctx, req, productID)
		if err != nil {
			log.Printf("Error handling context cache: %v", err)
		}

		if cacheHit {
			log.Println("Cache hit, using cached response")
			resp, err := CreateChatCompletionResponseFromCache(cachedResponse, req.Model)
			if err != nil {
				log.Printf("Error creating response from cache: %v", err)
			} else {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					log.Printf("Error encoding cached response: %v", err)
					http.Error(w, "Error encoding response", http.StatusInternalServerError)
				}
				return ChatCompletionRequest{}, nil, uuid.Nil, true
			}
		}
	}

	PerformAuditLogging(r, providerName+"_create_message", "input", body)

	filtered, err := ProcessInput(w, r, req)
	if err != nil || filtered {
		log.Printf("Request filtered or error occurred: %v", err)
		return ChatCompletionRequest{}, nil, uuid.Nil, false
	}

	return req, ctx, productID, true
}

func HandleCacheLogic(ctx context.Context, req ChatCompletionRequest, productID uuid.UUID) (*ChatCompletionResponse, bool, error) {
	cachedResponse, cacheHit, err := HandleContextCache(ctx, req, productID)
	if err != nil {
		log.Printf("Error handling context cache: %v", err)
	}

	if cacheHit {
		log.Println("Cache hit, using cached response")
		resp, err := CreateChatCompletionResponseFromCache(cachedResponse, req.Model)
		return resp, true, err
	}

	return nil, false, nil
}

func HandleAPICallAndResponse(w http.ResponseWriter, r *http.Request, ctx context.Context, req ChatCompletionRequest, productID uuid.UUID, provider Provider) {
	if req.Stream {
		if err := handleStreamingRequest(w, r, provider, req); err != nil {
			HandleError(w, err, http.StatusInternalServerError)
		}
		return
	}

	resp, err := provider.CreateChatCompletion(ctx, req)
	if err != nil {
		HandleError(w, fmt.Errorf("error creating chat completion: %v", err), http.StatusInternalServerError)
		return
	}

	if err := SetContextCacheResponse(ctx, req, resp, productID); err != nil {
		log.Printf("Error setting context cache: %v", err)
	}

	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)
	responseJSON, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		HandleError(w, fmt.Errorf("error encoding response: %v", err), http.StatusInternalServerError)
		return
	}

	auditLog := lib.AuditLogs(
		string(responseJSON),
		"chat_completion",
		apiKeyId,
		"output",
		productID,
		r,
	)

	if auditLog == nil {
		log.Printf("Failed to create audit log")
		HandleError(w, fmt.Errorf("failed to create audit log"), http.StatusInternalServerError)
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
