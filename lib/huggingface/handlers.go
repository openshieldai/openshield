package huggingface

import (
	"log"
	"net/http"

	"github.com/openshieldai/openshield/lib/openai"
	"github.com/openshieldai/openshield/lib/provider"

	"github.com/openshieldai/openshield/lib"
)

const OSCacheStatusHeader = "OS-Cache-Status"

var HuggingFaceProvider provider.Provider

func InitHuggingFaceProvider() {
	config := lib.GetConfig()
	HuggingFaceProvider = openai.NewOpenAIProvider(
		config.Secrets.HuggingFaceApiKey,
		config.Providers.HuggingFace.BaseUrl,
	)
}

func ChatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting Huggingface ChatCompletionHandler")

	if HuggingFaceProvider == nil {
		InitHuggingFaceProvider()
	}

	req, ctx, productID, ok := provider.HandleCommonRequestLogic(w, r, "nvidia")
	if !ok {
		log.Println("Request blocked or error occurred, skipping API call")
		return
	}

	provider.HandleAPICallAndResponse(w, r, ctx, req, productID, HuggingFaceProvider)

	log.Println("Huggingface ChatCompletionHandler completed successfully")
}
