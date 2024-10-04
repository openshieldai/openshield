package nvidia

import (
	"encoding/json"
	"fmt"
	"github.com/openshieldai/openshield/lib/openai"
	"github.com/openshieldai/openshield/lib/provider"
	"io"
	"log"
	"net/http"

	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/rules"
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
	log.Println("Starting ChatCompletionHandler")

	if nvidiaProvider == nil {
		InitNVIDIAProvider()
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

	provider.PerformAuditLogging(r, "openai_chat_completion", "input", body)

	// Create the exact struct type expected by the Input function
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

	// Pass the parsed request to HandleChatCompletion
	err = provider.HandleChatCompletion(w, r, nvidiaProvider, req)
	if err != nil {
		log.Printf("Error handling chat completion: %v", err)
		handleError(w, fmt.Errorf("error handling chat completion: %v", err), http.StatusInternalServerError)
		return
	}

	log.Println("ChatCompletionHandler completed successfully")
}
func handleError(w http.ResponseWriter, err error, statusCode int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), statusCode)
}
