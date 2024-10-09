package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/provider"
	"github.com/openshieldai/openshield/lib/rules"
	"io"
	"log"
	"net/http"
)

var anthropicProvider provider.Provider

func InitAnthropicProvider() {
	config := lib.GetConfig()
	anthropicProvider = NewAnthropicProvider(
		config.Secrets.AnthropicApiKey,
		config.Providers.Anthropic.BaseUrl,
	)
	log.Printf("Anthropic Provider initialized")
}

func CreateMessageHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting CreateMessageHandler")

	if anthropicProvider == nil {
		InitAnthropicProvider()
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var req provider.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Received request: %+v", req)

	provider.PerformAuditLogging(r, "anthropic_create_message", "input", body)

	inputRequest := struct {
		Model     string             `json:"model"`
		Messages  []provider.Message `json:"messages"`
		MaxTokens int                `json:"max_tokens"`
		Stream    bool               `json:"stream"`
	}{
		Model:     req.Model,
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}

	filtered, message, errorMessage := rules.Input(r, inputRequest)
	if errorMessage != nil {
		handleError(w, fmt.Errorf("error processing input: %v", errorMessage), http.StatusBadRequest)
		return
	}

	if filtered {
		provider.PerformAuditLogging(r, "rule", "filtered", []byte(message))
		handleError(w, fmt.Errorf("%v", message), http.StatusBadRequest)
		return
	}

	log.Println("Input processing completed successfully")

	apiKeyID, ok := r.Context().Value("apiKeyId").(uuid.UUID)
	if !ok {
		handleError(w, fmt.Errorf("apiKeyId not found in context"), http.StatusInternalServerError)
		return
	}

	productID, err := provider.GetProductIDFromAPIKey(r.Context(), apiKeyID)
	if err != nil {
		handleError(w, fmt.Errorf("error getting productID: %v", err), http.StatusInternalServerError)
		return
	}

	// Create a new context with the productID
	ctx := context.WithValue(r.Context(), "productID", productID)

	// Check context cache
	cachedResponse, cacheHit, err := provider.HandleContextCache(ctx, req, productID)
	if err != nil {
		log.Printf("Error handling context cache: %v", err)
	}

	var resp *provider.ChatCompletionResponse

	if cacheHit {
		log.Println("Cache hit, using cached response")

		var cachedResp struct {
			Prompt    string `json:"prompt"`
			Answer    string `json:"answer"`
			ProductID string `json:"product_id"`
		}
		err = json.Unmarshal([]byte(cachedResponse), &cachedResp)
		if err != nil {
			log.Printf("Error unmarshaling cached response: %v", err)
		} else {
			// Create a ChatCompletionResponse from the cached data
			resp, _ = provider.CreateChatCompletionResponseFromCache(cachedResponse, req.Model)
		}

		// Send the response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("Error encoding response: %v", err)
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}

	} else {
		log.Println("Cache miss, making API call to Anthropic")
		// Make the API call using the context with productID
		resp, err = anthropicProvider.CreateChatCompletion(ctx, req)
		if err != nil {
			handleError(w, fmt.Errorf("error creating chat completion: %v", err), http.StatusInternalServerError)
			return
		}

		// Send the API response to the client
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("Error encoding API response: %v", err)
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}

		// Cache the response only if it wasn't a cache hit
		if err := provider.SetContextCacheResponse(ctx, req, resp, productID); err != nil {
			log.Printf("Error setting context cache: %v", err)
		}

		// Perform response audit logging only for direct API calls
		provider.PerformResponseAuditLogging(r, resp)
	}

}

func handleError(w http.ResponseWriter, err error, statusCode int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), statusCode)
}
