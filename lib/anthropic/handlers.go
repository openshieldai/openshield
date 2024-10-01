package anthropic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/rules"
	"io"
	"log"
	"net/http"
	"strings"
)

const OSCacheStatusHeader = "OS-Cache-Status"

var anthropicProvider *lib.AnthropicProvider

func InitAnthropicProvider(config lib.Configuration) {
	anthropicProvider = lib.NewAnthropicProvider(
		config.Secrets.AnthropicApiKey,
		config.Providers.Anthropic.BaseUrl,
	)

	log.Printf("Anthropic Provider initialized")
}

func CreateMessageHandler(w http.ResponseWriter, r *http.Request) {
	if anthropicProvider == nil {
		config := lib.GetConfig()
		InitAnthropicProvider(config)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var anthropicReq struct {
		Model     string        `json:"model"`
		System    string        `json:"system"`
		Messages  []lib.Message `json:"messages"`
		MaxTokens int           `json:"max_tokens"`
		Stream    bool          `json:"stream"`
	}

	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	// Create a new request struct with lib.Message
	req := struct {
		Model     string        `json:"model"`
		Messages  []lib.Message `json:"messages"`
		MaxTokens int           `json:"max_tokens"`
		Stream    bool          `json:"stream"`
	}{
		Model:     anthropicReq.Model,
		Messages:  anthropicReq.Messages,
		MaxTokens: anthropicReq.MaxTokens,
		Stream:    anthropicReq.Stream,
	}

	performAuditLogging(r, "anthropic_create_message", "input", body)

	filtered, message, errorMessage := rules.Input(r, req)
	if errorMessage != nil {
		handleError(w, fmt.Errorf("error processing input: %v", errorMessage), http.StatusBadRequest)
		return
	}

	if filtered {
		performAuditLogging(r, "rule", "filtered", []byte(message))
		handleError(w, fmt.Errorf("%v", message), http.StatusBadRequest)
		return
	}

	if req.Stream {
		handleStreamingRequest(w, r, anthropicReq.System, anthropicReq.Messages, req.Model, req.MaxTokens)
	} else {
		handleNonStreamingRequest(w, r, anthropicReq.System, anthropicReq.Messages, req.Model, req.MaxTokens)
	}
}

func handleStreamingRequest(w http.ResponseWriter, r *http.Request, system string, messages []lib.Message, model string, maxTokens int) {
	stream, err := anthropicProvider.StreamChatCompletion(r.Context(), system, messages, model, maxTokens)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create message stream: %v", err), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		handleError(w, fmt.Errorf("streaming unsupported"), http.StatusInternalServerError)
		return
	}

	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			fmt.Fprintf(w, "%s\n\n", line)
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stream: %v", err)
		fmt.Fprintf(w, "data: {\"error\": \"%v\"}\n\n", err)
		flusher.Flush()
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, system string, messages []lib.Message, model string, maxTokens int) {
	config := lib.GetConfig()

	cacheKey := fmt.Sprintf("%s-%s-%s-%d", model, system, messages, maxTokens)
	getCache, cacheStatus, err := lib.GetCache(cacheKey)
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		w.Header().Set(OSCacheStatusHeader, "HIT")
		w.Write(getCache)
		return
	}

	resp, err := anthropicProvider.CreateChatCompletion(r.Context(), system, messages, model, maxTokens)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create message: %v", err), http.StatusInternalServerError)
		return
	}

	if config.Settings.Cache.Enabled {
		w.Header().Set(OSCacheStatusHeader, "MISS")
		resJson, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshalling response to JSON: %v", err)
		} else {
			err = lib.SetCache(cacheKey, resJson)
			if err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		}
	} else {
		w.Header().Set(OSCacheStatusHeader, "BYPASS")
	}

	performResponseAuditLogging(r, resp)
	json.NewEncoder(w).Encode(resp)
}

func performAuditLogging(r *http.Request, logType string, messageType string, body []byte) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)

	productID, err := getProductIDFromAPIKey(apiKeyId)
	if err != nil {
		log.Printf("Failed to retrieve ProductID for apiKeyId %s: %v", apiKeyId, err)
		return
	}
	lib.AuditLogs(string(body), logType, apiKeyId, messageType, productID, r)
}

func performResponseAuditLogging(r *http.Request, resp *lib.AnthropicResponse) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)
	productID, err := getProductIDFromAPIKey(apiKeyId)
	if err != nil {
		log.Printf("Failed to retrieve ProductID for apiKeyId %s: %v", apiKeyId, err)
		return
	}

	responseJSON, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	auditLog := lib.AuditLogs(string(responseJSON), "anthropic_chat_completion", apiKeyId, "output", productID, r)

	if auditLog == nil {
		log.Printf("Failed to create audit log")
		return
	}

	lib.LogUsage(
		resp.Model,
		0, // This field is not present in the Anthropic response
		resp.Usage.InputTokens,
		resp.Usage.OutputTokens,
		resp.Usage.InputTokens+resp.Usage.OutputTokens,
		resp.StopReason,
		"chat_completion",
		productID,
		auditLog.Id,
	)
}

func getProductIDFromAPIKey(apiKeyId uuid.UUID) (uuid.UUID, error) {
	var productIDStr string
	err := lib.DB().Table("api_keys").Where("id = ?", apiKeyId).Pluck("product_id", &productIDStr).Error
	if err != nil {
		return uuid.Nil, err
	}

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse product_id as UUID: %v", err)
	}

	return productID, nil
}

func handleError(w http.ResponseWriter, err error, statusCode int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), statusCode)
}
