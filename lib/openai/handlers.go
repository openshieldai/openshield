package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/rules"
	"github.com/sashabaranov/go-openai"
)

const OSCacheStatusHeader = "OS-Cache-Status"

func initializeOpenAIClient() *openai.Client {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl
	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	return openai.NewClientWithConfig(c)
}

func ListModelsHandler(w http.ResponseWriter, r *http.Request) {
	client := initializeOpenAIClient()

	getCache, cacheStatus, err := lib.GetCache(r.URL.Path)
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		w.Header().Set(OSCacheStatusHeader, "HIT")
		w.Write(getCache)
		return
	}

	log.Printf("Cache miss for %v", cacheStatus)
	res, err := client.ListModels(r.Context())
	if err != nil {
		lib.ErrorResponse(w, err)
		return
	}
	handleModelResponse(w, r, res, err)
}

func GetModelHandler(w http.ResponseWriter, r *http.Request) {
	client := initializeOpenAIClient()

	getCache, cacheStatus, err := lib.GetCache(r.URL.Path)
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		w.Header().Set(OSCacheStatusHeader, "HIT")
		w.Write(getCache)
		return
	}

	log.Printf("Cache miss for %v", cacheStatus)
	modelName := chi.URLParam(r, "model")
	res, err := client.GetModel(r.Context(), modelName)
	if err != nil {
		lib.ErrorResponse(w, err)
		return
	}
	handleModelResponse(w, r, res, err)
}

func CreateThreadHandler(w http.ResponseWriter, r *http.Request) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var req openai.ThreadRequest
	if err := json.Unmarshal(body, &req); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	performAuditLogging(r, "openai_create_thread", "input", body)

	filtered, message, errorMessage := rules.Input(r, req)
	if errorMessage != nil {
		handleError(w, fmt.Errorf("error processing input: %v", errorMessage), http.StatusBadRequest)
		return
	}

	logMessage, err := json.Marshal(message)
	if err != nil {
		handleError(w, fmt.Errorf("error marshalling message: %v", err), http.StatusBadRequest)
		return
	}

	if filtered {
		performAuditLogging(r, "rule", "filtered", logMessage)
		handleError(w, fmt.Errorf(message), http.StatusBadRequest)
		return
	}

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.CreateThread(r.Context(), req)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create thread: %v", err), http.StatusInternalServerError)
		return
	}

	if config.Settings.Cache.Enabled {
		w.Header().Set(OSCacheStatusHeader, "MISS")
		resJson, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshalling response to JSON: %v", err)
		} else {
			err = lib.SetCache(string(body), resJson)
			if err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		}
	} else {
		w.Header().Set(OSCacheStatusHeader, "BYPASS")
	}

	performThreadAuditLogging(r, resp)
	json.NewEncoder(w).Encode(resp)
}
func GetThreadHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")

	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.RetrieveThread(r.Context(), threadID)
	if err != nil {
		handleError(w, fmt.Errorf("failed to retrieve thread: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
func ModifyThreadHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")

	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var req openai.ModifyThreadRequest
	if err := json.Unmarshal(body, &req); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.ModifyThread(r.Context(), threadID, req)
	if err != nil {
		handleError(w, fmt.Errorf("failed to modify thread: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func performThreadAuditLogging(r *http.Request, resp openai.Thread) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)
	productID, err := getProductIDFromAPIKey(apiKeyId)
	responseJSON, _ := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to retrieve ProductID for apiKeyId %s: %v", apiKeyId, err)
		return
	}

	lib.AuditLogs(string(responseJSON), "openai_create_thread", apiKeyId, "output", productID, r)
}
func DeleteThreadHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")

	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.DeleteThread(r.Context(), threadID)
	if err != nil {
		handleError(w, fmt.Errorf("failed to delete thread: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
func CreateMessageHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")

	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var req openai.MessageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	// Perform input validation and filtering
	filtered, message, errorMessage := rules.Input(r, req)
	if errorMessage != nil {
		handleError(w, fmt.Errorf("error processing input: %v", errorMessage), http.StatusBadRequest)
		return
	}

	logMessage, err := json.Marshal(message)
	if err != nil {
		handleError(w, fmt.Errorf("error marshalling message: %v", err), http.StatusBadRequest)
		return
	}

	if filtered {
		performAuditLogging(r, "rule", "filtered", logMessage)
		handleError(w, fmt.Errorf(message), http.StatusBadRequest)
		return
	}

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.CreateMessage(r.Context(), threadID, req)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create message: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
func ListMessagesHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")

	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	limit := r.URL.Query().Get("limit")
	order := r.URL.Query().Get("order")
	after := r.URL.Query().Get("after")
	before := r.URL.Query().Get("before")

	var limitInt *int
	if limit != "" {
		limitIntVal, err := strconv.Atoi(limit)
		if err != nil {
			handleError(w, fmt.Errorf("invalid limit parameter: %v", err), http.StatusBadRequest)
			return
		}
		limitInt = &limitIntVal
	}

	var orderStr *string
	if order != "" {
		orderStr = &order
	}

	var afterStr *string
	if after != "" {
		afterStr = &after
	}

	var beforeStr *string
	if before != "" {
		beforeStr = &before
	}

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.ListMessage(r.Context(), threadID, limitInt, orderStr, afterStr, beforeStr)
	if err != nil {
		handleError(w, fmt.Errorf("failed to list messages: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
func RetrieveMessageHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	messageID := chi.URLParam(r, "message_id")

	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.RetrieveMessage(r.Context(), threadID, messageID)
	if err != nil {
		handleError(w, fmt.Errorf("failed to retrieve message: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
func ModifyMessageHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	messageID := chi.URLParam(r, "message_id")

	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var req map[string]string
	if err := json.Unmarshal(body, &req); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	// Perform input validation and filtering
	filtered, message, errorMessage := rules.Input(r, req)
	if errorMessage != nil {
		handleError(w, fmt.Errorf("error processing input: %v", errorMessage), http.StatusBadRequest)
		return
	}

	logMessage, err := json.Marshal(message)
	if err != nil {
		handleError(w, fmt.Errorf("error marshalling message: %v", err), http.StatusBadRequest)
		return
	}

	if filtered {
		performAuditLogging(r, "rule", "filtered", logMessage)
		handleError(w, fmt.Errorf(message), http.StatusBadRequest)
		return
	}

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.ModifyMessage(r.Context(), threadID, messageID, req)
	if err != nil {
		handleError(w, fmt.Errorf("failed to modify message: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
func DeleteMessageHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	messageID := chi.URLParam(r, "message_id")

	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.DeleteMessage(r.Context(), threadID, messageID)
	if err != nil {
		handleError(w, fmt.Errorf("failed to delete message: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
func ChatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var req openai.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	performAuditLogging(r, "openai_chat_completion", "input", body)

	filtered, message, errorMessage := rules.Input(r, req)
	if errorMessage != nil {
		handleError(w, fmt.Errorf("error processing input: %v", errorMessage), http.StatusBadRequest)
		return
	}

	logMessage, err := json.Marshal(message)
	if err != nil {
		handleError(w, fmt.Errorf("error marshalling message: %v", err), http.StatusBadRequest)
		return
	}

	if filtered {
		performAuditLogging(r, "rule", "filtered", logMessage)
		handleError(w, fmt.Errorf(message), http.StatusBadRequest)
		return
	}

	if req.Stream {
		handleStreamingRequest(w, r, req, openAIAPIKey, openAIBaseURL)
	} else {
		handleNonStreamingRequest(w, r, body, req, config, openAIAPIKey, openAIBaseURL)
	}
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
func getProductIDFromAPIKey(apiKeyId uuid.UUID) (uuid.UUID, error) {
	var productIDStr string
	err := lib.DB().Table("api_keys").Where("id = ?", apiKeyId).Pluck("product_id", &productIDStr).Error
	if err != nil {
		return uuid.Nil, err
	}

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		return uuid.Nil, errors.New("failed to parse product_id as UUID")
	}

	return productID, nil
}

func handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, body []byte, req openai.ChatCompletionRequest, config lib.Configuration, openAIAPIKey string, openAIBaseURL string) {
	getCache, cacheStatus, err := lib.GetCache(string(body))
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		w.Header().Set(OSCacheStatusHeader, "HIT")
		w.Write(getCache)
		return
	}

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)
	resp, err := client.CreateChatCompletion(r.Context(), req)
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
			err = lib.SetCache(string(body), resJson)
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

func handleStreamingRequest(w http.ResponseWriter, r *http.Request, req openai.ChatCompletionRequest, openAIAPIKey string, openAIBaseURL string) {
	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)
	stream, err := client.CreateChatCompletionStream(r.Context(), req)
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

	var buffer bytes.Buffer

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			buffer.WriteString("data: [DONE]\n\n")
			flusher.Flush()
			break
		}
		if err != nil {
			log.Printf("Error receiving stream: %v", err)
			buffer.WriteString(fmt.Sprintf("data: {\"error\": \"%v\"}\n\n", err))
			flusher.Flush()
			break
		}
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			data, err := json.Marshal(response)
			if err != nil {
				log.Printf("Error marshaling response: %v", err)
				continue
			}
			buffer.WriteString(fmt.Sprintf("data: %s\n\n", string(data)))
			flusher.Flush()
		}
	}

	// Write the full output to the response
	fmt.Fprint(w, buffer.String())
	flusher.Flush()

	// Perform response audit logging
	var resp openai.ChatCompletionResponse
	if err := json.Unmarshal(buffer.Bytes(), &resp); err == nil {
		performResponseAuditLogging(r, resp)
	}
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

	auditLog := lib.AuditLogs(string(responseJSON), "openai_chat_completion", apiKeyId, "input", productID, r)

	if auditLog == nil {
		log.Printf("Failed to create audit log")
		return
	}

	lib.Usage(
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

func handleError(w http.ResponseWriter, err error, statusCode int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), statusCode)
}

func handleModelResponse(w http.ResponseWriter, r *http.Request, res interface{}, err error) {
	if err != nil {
		lib.ErrorResponse(w, err)
		return
	}

	config := lib.GetConfig()
	if config.Settings.Cache.Enabled {
		w.Header().Set(OSCacheStatusHeader, "MISS")
		resJson, err := json.Marshal(res)
		if err != nil {
			log.Printf("Error marshalling response to JSON: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		err = lib.SetCache(r.URL.Path, resJson)
		if err != nil {
			log.Printf("Error setting cache: %v", err)
		}
	} else {
		w.Header().Set(OSCacheStatusHeader, "BYPASS")
	}

	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		return
	}
}
