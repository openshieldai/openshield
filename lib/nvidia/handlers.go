package nvidia

import (
	"log"
	"net/http"

	"github.com/openshieldai/openshield/lib/openai"
	"github.com/openshieldai/openshield/lib/provider"

	"github.com/openshieldai/openshield/lib"
)

const OSCacheStatusHeader = "OS-Cache-Status"

var nvidiaProvider provider.Provider

func InitNVIDIAProvider() {
	config := lib.GetConfig()
	nvidiaProvider = openai.NewOpenAIProvider(
		config.Secrets.NvidiaApiKey,
		config.Providers.Nvidia.BaseUrl,
	)
}

func ChatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting Nvidia ChatCompletionHandler")

	if nvidiaProvider == nil {
		InitNVIDIAProvider()
	}

	req, ctx, productID, ok := provider.HandleCommonRequestLogic(w, r, "nvidia")
	if !ok {
		log.Println("Request blocked or error occurred, skipping API call")
		return
	}

	provider.HandleAPICallAndResponse(w, r, ctx, req, productID, nvidiaProvider)

	log.Println("Nvidia ChatCompletionHandler completed successfully")
}
