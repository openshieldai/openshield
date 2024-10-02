package nvidia

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/openshieldai/go-openai"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/rules"
)

const OSCacheStatusHeader = "OS-Cache-Status"

var nvidiaProvider *lib.OpenAIProvider

func InitNVIDIAProvider() {
	config := lib.GetConfig()
	nvidiaProvider = lib.NewOpenAIProvider(
		config.Secrets.NvidiaApiKey,
		config.Providers.Nvidia.BaseUrl,
	)
}

func ChatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	if nvidiaProvider == nil {
		InitNVIDIAProvider()
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var nvidiaReq struct {
		Model     string                         `json:"model"`
		Messages  []openai.ChatCompletionMessage `json:"messages"`
		MaxTokens int                            `json:"max_tokens"`
		Stream    bool                           `json:"stream"`
	}

	if err := json.Unmarshal(body, &nvidiaReq); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	// Convert openai.ChatCompletionMessage to lib.Message
	libMessages := make([]lib.Message, len(nvidiaReq.Messages))
	for i, msg := range nvidiaReq.Messages {
		libMessages[i] = lib.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Create a new request struct with lib.Message
	req := struct {
		Model     string        `json:"model"`
		Messages  []lib.Message `json:"messages"`
		MaxTokens int           `json:"max_tokens"`
		Stream    bool          `json:"stream"`
	}{
		Model:     nvidiaReq.Model,
		Messages:  libMessages,
		MaxTokens: nvidiaReq.MaxTokens,
		Stream:    nvidiaReq.Stream,
	}

	performAuditLogging(r, "nvidia_chat_completion", "input", body)

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
		handleStreamingRequest(w, r, nvidiaReq.Messages, req.Model, req.MaxTokens)
	} else {
		handleNonStreamingRequest(w, r, nvidiaReq.Messages, req.Model, req.MaxTokens)
	}
}

func handleStreamingRequest(w http.ResponseWriter, r *http.Request, messages []openai.ChatCompletionMessage, model string, maxTokens int) {
	stream, err := nvidiaProvider.CreateChatCompletionStream(r.Context(), messages, model, maxTokens)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create chat completion stream: %v", err), http.StatusInternalServerError)
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

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		if err != nil {
			log.Printf("Error receiving stream: %v", err)
			fmt.Fprintf(w, "data: {\"error\": \"%v\"}\n\n", err)
			flusher.Flush()
			return
		}

		data, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
			continue
		}

		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, messages []openai.ChatCompletionMessage, model string, maxTokens int) {
	config := lib.GetConfig()

	cacheKey := fmt.Sprintf("%s-%s-%d", model, messages, maxTokens)
	getCache, cacheStatus, err := lib.GetCache(cacheKey)
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		w.Header().Set(OSCacheStatusHeader, "HIT")
		w.Write(getCache)
		return
	}

	resp, err := nvidiaProvider.CreateChatCompletion(r.Context(), messages, model, maxTokens)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create chat completion: %v", err), http.StatusInternalServerError)
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

	performResponseAuditLogging(r, *resp)
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

func performResponseAuditLogging(r *http.Request, resp openai.ChatCompletionResponse) {
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

	auditLog := lib.AuditLogs(string(responseJSON), "nvidia_chat_completion", apiKeyId, "output", productID, r)

	if auditLog == nil {
		log.Printf("Failed to create audit log")
		return
	}

	lib.LogUsage(
		resp.Model,
		0,
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		resp.Usage.TotalTokens,
		string(resp.Choices[0].FinishReason),
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
