package huggingface

import (
	"log"
	"net/http"

	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/provider"
)

var huggingFaceProvider provider.Provider

func InitHuggingFaceProvider() {
	config := lib.GetConfig()
	huggingFaceProvider = NewHuggingFaceProvider(
		config.Secrets.HuggingFaceApiKey,
		config.Providers.HuggingFace.BaseUrl,
	)
	log.Printf("HuggingFace Provider initialized")
}

func ChatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting HuggingFace ChatCompletionHandler")

	if huggingFaceProvider == nil {
		InitHuggingFaceProvider()
	}

	req, ctx, productID, ok := provider.HandleCommonRequestLogic(w, r, "huggingface")
	if !ok {
		log.Println("Request blocked or error occurred, skipping API call")
		return
	}

	log.Printf("HuggingFace request stream parameter: %v", req.Stream)

	provider.HandleAPICallAndResponse(w, r, ctx, req, productID, huggingFaceProvider)

	log.Println("HuggingFace ChatCompletionHandler completed successfully")
}
