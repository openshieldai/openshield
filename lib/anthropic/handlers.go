package anthropic

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/provider"
	"github.com/openshieldai/openshield/rules"
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

	err = provider.HandleChatCompletion(w, r, anthropicProvider, req)
	if err != nil {
		log.Printf("Error handling chat completion: %v", err)
		handleError(w, fmt.Errorf("error handling chat completion: %v", err), http.StatusInternalServerError)
		return
	}

	log.Println("CreateMessageHandler completed successfully")
}

func handleError(w http.ResponseWriter, err error, statusCode int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), statusCode)
}
