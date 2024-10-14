package anthropic

import (
	"log"
	"net/http"

	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/provider"
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

	req, ctx, productID, ok := provider.HandleCommonRequestLogic(w, r, "anthropic")
	if !ok {
		log.Println("Request blocked or error occurred, skipping API call")
		return
	}

	provider.HandleAPICallAndResponse(w, r, ctx, req, productID, anthropicProvider)
}
