package anthropic

import (
	"encoding/json"
	"fmt"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/provider"
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

	req, ctx, productID, err := provider.HandleCommonRequestLogic(w, r, "anthropic")
	if err != nil {
		return
	}

	resp, cacheHit, err := provider.HandleCacheLogic(ctx, req, productID)
	if err != nil {
		log.Printf("Error handling cache logic: %v", err)
	}

	if !cacheHit {
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
