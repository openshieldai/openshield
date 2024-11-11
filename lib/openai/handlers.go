package openai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/openshieldai/go-openai"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/provider"
	"github.com/openshieldai/openshield/lib/rules"
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

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.CreateThread(r.Context(), req)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create thread: %v", err), http.StatusInternalServerError)
		return
	}

	if config.Settings.ContextCache.Enabled {
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
		hashedApiKeyId := sha256.Sum256([]byte(apiKeyId.String()))
		log.Printf("Failed to retrieve ProductID for apiKeyId %s: %v", hex.EncodeToString(hashedApiKeyId[:]), err)
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

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	resp, err := client.CreateMessage(r.Context(), threadID, req)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create message: %v", err), http.StatusInternalServerError)
		return
	}

	if config.Settings.ContextCache.Enabled {
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

	resp, err := client.ListMessage(r.Context(), threadID, limitInt, orderStr, afterStr, beforeStr, nil)
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
func CreateAssistantHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantRequest(w, r, "create")
}

func ListAssistantsHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantRequest(w, r, "list")
}

func RetrieveAssistantHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantRequest(w, r, "retrieve")
}

func ModifyAssistantHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantRequest(w, r, "modify")
}

func DeleteAssistantHandler(w http.ResponseWriter, r *http.Request) {
	handleAssistantRequest(w, r, "delete")
}

// New handler functions for Runs
func CreateRunHandler(w http.ResponseWriter, r *http.Request) {
	handleRunRequest(w, r, "create")
}

func ListRunsHandler(w http.ResponseWriter, r *http.Request) {
	handleRunRequest(w, r, "list")
}

func RetrieveRunHandler(w http.ResponseWriter, r *http.Request) {
	handleRunRequest(w, r, "retrieve")
}

func ModifyRunHandler(w http.ResponseWriter, r *http.Request) {
	handleRunRequest(w, r, "modify")
}

func CancelRunHandler(w http.ResponseWriter, r *http.Request) {
	handleRunRequest(w, r, "cancel")
}

func SubmitToolOutputsHandler(w http.ResponseWriter, r *http.Request) {
	handleRunRequest(w, r, "submit_tool_outputs")
}

func CreateThreadAndRunHandler(w http.ResponseWriter, r *http.Request) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	var createThreadAndRunRequest openai.CreateThreadAndRunRequest
	if err := json.Unmarshal(body, &createThreadAndRunRequest); err != nil {
		handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
		return
	}

	// Perform input validation and filtering
	filtered, message, errorMessage := rules.Input(r, createThreadAndRunRequest)
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

	resp, err := client.CreateThreadAndRun(r.Context(), createThreadAndRunRequest)
	if err != nil {
		handleError(w, fmt.Errorf("failed to create thread and run: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

// New handler functions for Run Steps
func ListRunStepsHandler(w http.ResponseWriter, r *http.Request) {
	handleRunStepRequest(w, r, "list")
}

func RetrieveRunStepHandler(w http.ResponseWriter, r *http.Request) {
	handleRunStepRequest(w, r, "retrieve")
}

// Helper functions to handle requests
func handleAssistantRequest(w http.ResponseWriter, r *http.Request, action string) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	var resp interface{}
	var apiErr error

	switch action {
	case "create":
		var req openai.AssistantRequest
		if err := json.Unmarshal(body, &req); err != nil {
			handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, apiErr = client.CreateAssistant(r.Context(), req)
	case "list":
		resp, apiErr = client.ListAssistants(r.Context(), nil, nil, nil, nil)
	case "retrieve":
		assistantID := chi.URLParam(r, "assistant_id")
		resp, apiErr = client.RetrieveAssistant(r.Context(), assistantID)
	case "modify":
		assistantID := chi.URLParam(r, "assistant_id")
		var req openai.AssistantRequest
		if err := json.Unmarshal(body, &req); err != nil {
			handleError(w, fmt.Errorf("error decoding request body: %v", err), http.StatusBadRequest)
			return
		}
		resp, apiErr = client.ModifyAssistant(r.Context(), assistantID, req)
	case "delete":
		assistantID := chi.URLParam(r, "assistant_id")
		resp, apiErr = client.DeleteAssistant(r.Context(), assistantID)
	}

	if apiErr != nil {
		handleError(w, fmt.Errorf("API error: %v", apiErr), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func handleRunRequest(w http.ResponseWriter, r *http.Request, action string) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl
	threadID := chi.URLParam(r, "thread_id")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		handleError(w, fmt.Errorf("error reading request body: %v", err), http.StatusBadRequest)
		return
	}

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	var runRequest openai.RunRequest
	err = json.Unmarshal(body, &runRequest)
	if err != nil {
		handleError(w, fmt.Errorf("error parsing request body: %v", err), http.StatusBadRequest)
		return
	}

	if runRequest.Stream {
		handleStreamingRun(w, r.Context(), client, threadID, runRequest)
	} else {
		handleNonStreamingRun(w, r.Context(), client, action, threadID, runRequest, r, body)
	}
}

func handleStreamingRun(w http.ResponseWriter, ctx context.Context, client *openai.Client, threadID string, runRequest openai.RunRequest) {
	stream, err := client.CreateRunStream(ctx, threadID, runRequest)
	if err != nil {
		handleError(w, fmt.Errorf("error creating stream: %v", err), http.StatusInternalServerError)
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
		if err != nil {
			if err == io.EOF {
				fmt.Fprint(w, "data: [DONE]\n\n")
			} else {
				fmt.Fprintf(w, "data: {\"error\": \"%v\"}\n\n", err)
			}
			flusher.Flush()
			return
		}

		data, err := json.Marshal(response)
		if err != nil {
			handleError(w, fmt.Errorf("error marshaling response: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	}
}

func handleNonStreamingRun(w http.ResponseWriter, ctx context.Context, client *openai.Client, action, threadID string, runRequest openai.RunRequest, r *http.Request, body []byte) {
	var resp interface{}
	var err error

	switch action {
	case "create":
		resp, err = client.CreateRun(ctx, threadID, runRequest)
	case "list":
		pagination := getPaginationFromRequest(r)
		resp, err = client.ListRuns(ctx, threadID, pagination)
	case "retrieve":
		runID := chi.URLParam(r, "run_id")
		resp, err = client.RetrieveRun(ctx, threadID, runID)
	case "modify":
		runID := chi.URLParam(r, "run_id")
		var modifyRequest openai.RunModifyRequest
		err = json.Unmarshal(body, &modifyRequest)
		if err != nil {
			handleError(w, fmt.Errorf("error parsing modify request: %v", err), http.StatusBadRequest)
			return
		}
		resp, err = client.ModifyRun(ctx, threadID, runID, modifyRequest)
	case "cancel":
		runID := chi.URLParam(r, "run_id")
		resp, err = client.CancelRun(ctx, threadID, runID)
	case "submit_tool_outputs":
		runID := chi.URLParam(r, "run_id")
		var submitRequest openai.SubmitToolOutputsRequest
		err = json.Unmarshal(body, &submitRequest)
		if err != nil {
			handleError(w, fmt.Errorf("error parsing submit tool outputs request: %v", err), http.StatusBadRequest)
			return
		}
		resp, err = client.SubmitToolOutputs(ctx, threadID, runID, submitRequest)
	case "create_thread_and_run":
		var createThreadAndRunRequest openai.CreateThreadAndRunRequest
		err = json.Unmarshal(body, &createThreadAndRunRequest)
		if err != nil {
			handleError(w, fmt.Errorf("error parsing create thread and run request: %v", err), http.StatusBadRequest)
			return
		}
		resp, err = client.CreateThreadAndRun(ctx, createThreadAndRunRequest)
	default:
		handleError(w, fmt.Errorf("unsupported action: %s", action), http.StatusBadRequest)
		return
	}

	if err != nil {
		handleError(w, fmt.Errorf("API error: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func handleRunStepRequest(w http.ResponseWriter, r *http.Request, action string) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	openAIBaseURL := config.Providers.OpenAI.BaseUrl

	c := openai.DefaultConfig(openAIAPIKey)
	c.BaseURL = openAIBaseURL
	client := openai.NewClientWithConfig(c)

	var resp interface{}
	var apiErr error

	threadID := chi.URLParam(r, "thread_id")
	runID := chi.URLParam(r, "run_id")

	switch action {
	case "list":
		pagination := getPaginationFromRequest(r)
		resp, apiErr = client.ListRunSteps(r.Context(), threadID, runID, pagination)
	case "retrieve":
		stepID := chi.URLParam(r, "step_id")
		resp, apiErr = client.RetrieveRunStep(r.Context(), threadID, runID, stepID)
	}

	if apiErr != nil {
		handleError(w, fmt.Errorf("API error: %v", apiErr), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func getPaginationFromRequest(r *http.Request) openai.Pagination {
	pagination := openai.Pagination{}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			pagination.Limit = &limit
		}
	}

	if order := r.URL.Query().Get("order"); order != "" {
		pagination.Order = &order
	}

	if after := r.URL.Query().Get("after"); after != "" {
		pagination.After = &after
	}

	if before := r.URL.Query().Get("before"); before != "" {
		pagination.Before = &before
	}

	return pagination
}

var openaiProvider provider.Provider

func InitOpenAIProvider() {
	config := lib.GetConfig()
	openaiProvider = NewOpenAIProvider(
		config.Secrets.OpenAIApiKey,
		config.Providers.OpenAI.BaseUrl,
	)
}

func ChatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting ChatCompletionHandler")

	if openaiProvider == nil {
		InitOpenAIProvider()
	}

	req, ctx, productID, ok := provider.HandleCommonRequestLogic(w, r, "openai")
	if !ok {
		log.Println("Request blocked or error occurred, skipping API call")
		return
	}

	provider.HandleAPICallAndResponse(w, r, ctx, req, productID, openaiProvider)

	log.Println("ChatCompletionHandler completed successfully")
}

func performAuditLogging(r *http.Request, logType string, messageType string, body []byte) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)

	productID, err := getProductIDFromAPIKey(apiKeyId)
	if err != nil {
		hashedApiKeyId := sha256.Sum256([]byte(apiKeyId.String()))
		log.Printf("Failed to retrieve ProductID for apiKeyId %x: %v", hashedApiKeyId, err)
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

func performResponseAuditLogging(r *http.Request, resp openai.ChatCompletionResponse) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)
	productID, err := getProductIDFromAPIKey(apiKeyId)
	if err != nil {
		hashedApiKeyId := sha256.Sum256([]byte(apiKeyId.String()))
		log.Printf("Failed to retrieve ProductID for apiKeyId %x: %v", hashedApiKeyId, err)
		return
	}

	responseJSON, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	auditLog := lib.AuditLogs(string(responseJSON), "openai_chat_completion", apiKeyId, "output", productID, r)

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
