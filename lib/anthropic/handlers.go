package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/provider"
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
		provider.HandleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var req provider.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		provider.HandleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("Received request: %+v", req)

	provider.PerformAuditLogging(r, "anthropic_create_message", "input", body)

	filtered, err := provider.ProcessInput(w, r, req)
	if err != nil {
		return
	}
	if filtered {
		return
	}

	apiKeyID, ok := r.Context().Value("apiKeyId").(uuid.UUID)
	if !ok {
		provider.HandleError(w, fmt.Errorf("apiKeyId not found in context"), http.StatusInternalServerError)
		return
	}

	productID, err := provider.GetProductIDFromAPIKey(r.Context(), apiKeyID)
	if err != nil {
		provider.HandleError(w, fmt.Errorf("error getting productID: %v", err), http.StatusInternalServerError)
		return
	}

	ctx := context.WithValue(r.Context(), "productID", productID)

	cachedResponse, cacheHit, err := provider.HandleContextCache(ctx, req, productID)
	if err != nil {
		log.Printf("Error handling context cache: %v", err)
	}

	var resp *provider.ChatCompletionResponse

	if cacheHit {
		log.Println("Cache hit, using cached response")
		resp, _ = provider.CreateChatCompletionResponseFromCache(cachedResponse, req.Model)
	} else {
		log.Println("Cache miss, making API call to Anthropic")
		resp, err = anthropicProvider.CreateChatCompletion(ctx, req)
		if err != nil {
			provider.HandleError(w, fmt.Errorf("error creating chat completion: %v", err), http.StatusInternalServerError)
			return
		}

		if err := provider.SetContextCacheResponse(ctx, req, resp, productID); err != nil {
			log.Printf("Error setting context cache: %v", err)
		}

		provider.PerformResponseAuditLogging(r, resp)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
