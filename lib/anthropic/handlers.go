package anthropic

import (
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

	provider.HandleAPICallAndResponse(w, r, ctx, req, productID, anthropicProvider)
}
