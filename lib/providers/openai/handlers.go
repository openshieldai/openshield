package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/openshieldai/go-openai"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/rules"
)

const OSCacheStatusHeader = "OS-Cache-Status"

type Handler struct {
	config lib.Configuration
	client *openai.Client
}

func NewHandler(config lib.Configuration) *Handler {
	c := openai.DefaultConfig(config.Secrets.OpenAIApiKey)
	c.BaseURL = config.Providers.OpenAI.BaseUrl
	return &Handler{
		config: config,
		client: openai.NewClientWithConfig(c),
	}
}

func (h *Handler) ListModelsHandler(w http.ResponseWriter, r *http.Request) {
	h.handleCachedRequest(w, r, func(ctx context.Context) (interface{}, error) {
		return h.client.ListModels(ctx)
	})
}

func (h *Handler) GetModelHandler(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model")
	h.handleCachedRequest(w, r, func(ctx context.Context) (interface{}, error) {
		return h.client.GetModel(ctx, modelName)
	})
}

func (h *Handler) CreateThreadHandler(w http.ResponseWriter, r *http.Request) {
	var req openai.ThreadRequest
	if err := h.decodeJSONBody(r, &req); err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	h.performAuditLogging(r, "openai_create_thread", "input", req)

	if filtered, msg := h.applyInputRules(r, req); filtered {
		h.handleError(w, errors.New(msg), http.StatusBadRequest)
		return
	}

	resp, err := h.client.CreateThread(r.Context(), req)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to create thread: %v", err), http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, resp, "BYPASS")
	h.performThreadAuditLogging(r, resp)
}

func (h *Handler) GetThreadHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	resp, err := h.client.RetrieveThread(r.Context(), threadID)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to retrieve thread: %v", err), http.StatusInternalServerError)
		return
	}
	h.respondWithJSON(w, resp, "")
}

func (h *Handler) ModifyThreadHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	var req openai.ModifyThreadRequest
	if err := h.decodeJSONBody(r, &req); err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	resp, err := h.client.ModifyThread(r.Context(), threadID, req)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to modify thread: %v", err), http.StatusInternalServerError)
		return
	}
	h.respondWithJSON(w, resp, "")
}

func (h *Handler) DeleteThreadHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	resp, err := h.client.DeleteThread(r.Context(), threadID)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to delete thread: %v", err), http.StatusInternalServerError)
		return
	}
	h.respondWithJSON(w, resp, "")
}

func (h *Handler) CreateMessageHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	var req openai.MessageRequest
	if err := h.decodeJSONBody(r, &req); err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	if filtered, msg := h.applyInputRules(r, req); filtered {
		h.handleError(w, errors.New(msg), http.StatusBadRequest)
		return
	}

	resp, err := h.client.CreateMessage(r.Context(), threadID, req)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to create message: %v", err), http.StatusInternalServerError)
		return
	}
	h.respondWithJSON(w, resp, "")
}

func (h *Handler) ListMessagesHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	pagination := h.getPaginationFromRequest(r)
	resp, err := h.client.ListMessage(r.Context(), threadID, pagination.Limit, pagination.Order, pagination.After, pagination.Before, nil)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to list messages: %v", err), http.StatusInternalServerError)
		return
	}
	h.respondWithJSON(w, resp, "")
}

func (h *Handler) RetrieveMessageHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	messageID := chi.URLParam(r, "message_id")
	resp, err := h.client.RetrieveMessage(r.Context(), threadID, messageID)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to retrieve message: %v", err), http.StatusInternalServerError)
		return
	}
	h.respondWithJSON(w, resp, "")
}

func (h *Handler) ModifyMessageHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	messageID := chi.URLParam(r, "message_id")
	var req map[string]string
	if err := h.decodeJSONBody(r, &req); err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	if filtered, msg := h.applyInputRules(r, req); filtered {
		h.handleError(w, errors.New(msg), http.StatusBadRequest)
		return
	}

	resp, err := h.client.ModifyMessage(r.Context(), threadID, messageID, req)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to modify message: %v", err), http.StatusInternalServerError)
		return
	}
	h.respondWithJSON(w, resp, "")
}

func (h *Handler) DeleteMessageHandler(w http.ResponseWriter, r *http.Request) {
	threadID := chi.URLParam(r, "thread_id")
	messageID := chi.URLParam(r, "message_id")
	resp, err := h.client.DeleteMessage(r.Context(), threadID, messageID)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to delete message: %v", err), http.StatusInternalServerError)
		return
	}
	h.respondWithJSON(w, resp, "")
}

func (h *Handler) CreateAssistantHandler(w http.ResponseWriter, r *http.Request) {
	h.handleAssistantRequest(w, r, "create")
}

func (h *Handler) ListAssistantsHandler(w http.ResponseWriter, r *http.Request) {
	h.handleAssistantRequest(w, r, "list")
}

func (h *Handler) RetrieveAssistantHandler(w http.ResponseWriter, r *http.Request) {
	h.handleAssistantRequest(w, r, "retrieve")
}

func (h *Handler) ModifyAssistantHandler(w http.ResponseWriter, r *http.Request) {
	h.handleAssistantRequest(w, r, "modify")
}

func (h *Handler) DeleteAssistantHandler(w http.ResponseWriter, r *http.Request) {
	h.handleAssistantRequest(w, r, "delete")
}

func (h *Handler) CreateRunHandler(w http.ResponseWriter, r *http.Request) {
	h.handleRunRequest(w, r, "create")
}

func (h *Handler) ListRunsHandler(w http.ResponseWriter, r *http.Request) {
	h.handleRunRequest(w, r, "list")
}

func (h *Handler) RetrieveRunHandler(w http.ResponseWriter, r *http.Request) {
	h.handleRunRequest(w, r, "retrieve")
}

func (h *Handler) ModifyRunHandler(w http.ResponseWriter, r *http.Request) {
	h.handleRunRequest(w, r, "modify")
}

func (h *Handler) CancelRunHandler(w http.ResponseWriter, r *http.Request) {
	h.handleRunRequest(w, r, "cancel")
}

func (h *Handler) SubmitToolOutputsHandler(w http.ResponseWriter, r *http.Request) {
	h.handleRunRequest(w, r, "submit_tool_outputs")
}

func (h *Handler) CreateThreadAndRunHandler(w http.ResponseWriter, r *http.Request) {
	var req openai.CreateThreadAndRunRequest
	if err := h.decodeJSONBody(r, &req); err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	if filtered, msg := h.applyInputRules(r, req); filtered {
		h.handleError(w, errors.New(msg), http.StatusBadRequest)
		return
	}

	resp, err := h.client.CreateThreadAndRun(r.Context(), req)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to create thread and run: %v", err), http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, resp, "")
}

func (h *Handler) ListRunStepsHandler(w http.ResponseWriter, r *http.Request) {
	h.handleRunStepRequest(w, r, "list")
}

func (h *Handler) RetrieveRunStepHandler(w http.ResponseWriter, r *http.Request) {
	h.handleRunStepRequest(w, r, "retrieve")
}

type FilteredResponse struct {
	Status   string `json:"status"`
	RuleType string `json:"rule_type"`
}

func (h *Handler) ChatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	var req openai.ChatCompletionRequest
	if err := h.decodeJSONBody(r, &req); err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	if err := h.validateChatCompletionRequest(&req); err != nil {
		h.handleError(w, err, http.StatusBadRequest)
		return
	}

	h.performAuditLoggingAsync(r, "openai_chat_completion", "input", req)

	if h.config.Settings.ContextCache.Enabled {
		cached, err := h.handleCachedChatCompletion(w, r, req)
		if err != nil {
			h.handleError(w, err, http.StatusInternalServerError)
			return
		}

		if cached {
			return
		} else {
			w.Header().Set(OSCacheStatusHeader, "MISS")
		}
	} else {
		w.Header().Set(OSCacheStatusHeader, "BYPASS")
	}

	if filtered, msg := h.applyInputRules(r, req); filtered {
		if h.config.Settings.ContextCache.Enabled {
			h.setContextCacheFiltered(r, req, msg)
		}

		response, _ := json.Marshal(h.openaiResponse(msg, h.getSessionId(r, req)))
		h.performAuditLoggingAsync(r, "openai_chat_completion", "output", req)
		h.handleError(w, errors.New(string(response)), http.StatusBadRequest)
		return
	}

	if req.Stream {
		h.handleStreamingChatCompletion(w, r, req)
	} else {
		h.handleNonStreamingChatCompletion(w, r, req)
	}
}

// Helper methods

func (h *Handler) handleCachedRequest(w http.ResponseWriter, r *http.Request, action func(context.Context) (interface{}, error)) {
	if cache, hit := h.getCache(r.URL.Path); hit {
		h.respondWithJSON(w, cache, "HIT")
		return
	}

	res, err := action(r.Context())
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}

	h.setCacheAndRespond(w, r, res)
}

func (h *Handler) getCache(key string) ([]byte, bool) {
	cache, err, _ := lib.GetCache(key)
	if !err {
		return nil, false
	}
	return cache, true
}

func (h *Handler) setCacheAndRespond(w http.ResponseWriter, r *http.Request, res interface{}) {
	resJSON, err := json.Marshal(res)
	if err != nil {
		h.handleError(w, err, http.StatusInternalServerError)
		return
	}

	if h.config.Settings.Cache.Enabled {
		// Asynchronously set cache
		go func() {
			if err := lib.SetCache(r.URL.Path, resJSON); err != nil {
				log.Printf("Error setting cache: %v", err)
			}
		}()
		h.respondWithJSON(w, res, "MISS")
	} else {
		h.respondWithJSON(w, res, "BYPASS")
	}
}

func (h *Handler) decodeJSONBody(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *Handler) validateChatCompletionRequest(req *openai.ChatCompletionRequest) error {
	if len(req.Messages) == 0 {
		return errors.New("messages array is empty")
	}
	if req.Model == "" {
		return errors.New("model is required")
	}
	return nil
}

func (h *Handler) applyInputRules(r *http.Request, req interface{}) (bool, string) {
	filtered, message, err := rules.Input(r, req)
	if err != nil {
		return true, fmt.Sprintf("error processing input: %v", err)
	}
	if filtered {
		h.performAuditLogging(r, "rule", "filtered", message)
		return true, message
	}
	return false, ""
}

func (h *Handler) handleError(w http.ResponseWriter, err error, statusCode int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), statusCode)
}

func (h *Handler) respondWithJSON(w http.ResponseWriter, data interface{}, cacheStatus string) {
	w.Header().Set("Content-Type", "application/json")
	if cacheStatus != "" {
		w.Header().Set(OSCacheStatusHeader, cacheStatus)
	}
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) performAuditLogging(r *http.Request, logType, messageType string, data interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered in performAuditLogging: %v", r)
			// Optionally log the stack trace:
			// debug.PrintStack()
		}
	}()

	apiKeyID := r.Context().Value("apiKeyId").(uuid.UUID)
	productID, err := h.getProductIDFromAPIKey(apiKeyID)
	if err != nil {
		log.Printf("Failed to retrieve ProductID for apiKeyId %s: %v", apiKeyID, err)
		return
	}

	body, _ := json.Marshal(data)
	lib.AuditLogs(string(body), logType, apiKeyID, messageType, productID, r)
}

func (h *Handler) performAuditLoggingAsync(r *http.Request, logType, messageType string, data interface{}) {
	go h.performAuditLogging(r, logType, messageType, data)
}

func (h *Handler) getProductIDFromAPIKey(apiKeyID uuid.UUID) (uuid.UUID, error) {
	var productIDStr string
	err := lib.DB().Table("api_keys").Where("id = ?", apiKeyID).Pluck("product_id", &productIDStr).Error
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(productIDStr)
}

func (h *Handler) getPaginationFromRequest(r *http.Request) openai.Pagination {
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

func (h *Handler) getSessionId(r *http.Request, req openai.ChatCompletionRequest) string {
	apiKeyID := r.Context().Value("apiKeyId").(uuid.UUID)
	productID, err := h.getProductIDFromAPIKey(apiKeyID)
	if err != nil {
		log.Printf("Failed to retrieve ProductID for apiKeyId %s: %v", apiKeyID, err)
		return ""
	}
	return productID.String() + "-" + req.Model
}

func (h *Handler) openaiResponse(answer string, productID string) openai.ChatCompletionResponse {
	response := openai.ChatCompletionResponse{
		ID:      "cached_" + productID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "cached_model",
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: answer,
				},
				FinishReason: "stop",
			},
		},
		Usage: openai.Usage{},
	}
	return response
}

func (h *Handler) handleCachedChatCompletion(w http.ResponseWriter, r *http.Request, req openai.ChatCompletionRequest) (bool, error) {
	prompt := h.lastUserMessage(req)

	sessionId := h.getSessionId(r, req)
	cache, err := lib.GetContextCache(prompt, sessionId)
	if err != nil {
		var cachedData struct {
			Prompt    string `json:"prompt"`
			Answer    string `json:"answer"`
			ProductID string `json:"product_id"`
		}
		if err := json.Unmarshal([]byte(cache), &cachedData); err != nil {
			return false, fmt.Errorf("error unmarshaling cached response: %v", err)
		}

		h.respondWithJSON(w, h.openaiResponse(cachedData.Answer, cachedData.ProductID), "HIT")
		return true, nil
	}
	return false, nil
}

func (h *Handler) lastUserMessage(req openai.ChatCompletionRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}

func (h *Handler) handleStreamingChatCompletion(w http.ResponseWriter, r *http.Request, req openai.ChatCompletionRequest) {
	stream, err := h.client.CreateChatCompletionStream(r.Context(), req)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to create chat completion stream: %v", err), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.handleError(w, fmt.Errorf("streaming unsupported"), http.StatusInternalServerError)
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

	fmt.Fprint(w, buffer.String())
	flusher.Flush()

	var resp openai.ChatCompletionResponse
	if err := json.Unmarshal(buffer.Bytes(), &resp); err == nil {
		go h.performResponseAuditLogging(r, resp)
		if h.config.Settings.ContextCache.Enabled {
			go func() {
				if err := h.setContextCache(r, req, resp); err != nil {
					log.Printf("Error setting context cache: %v", err)
				}
			}()
		}
	}
}

func (h *Handler) handleNonStreamingChatCompletion(w http.ResponseWriter, r *http.Request, req openai.ChatCompletionRequest) {
	resp, err := h.client.CreateChatCompletion(r.Context(), req)
	if err != nil {
		h.handleError(w, fmt.Errorf("failed to create chat completion: %v", err), http.StatusInternalServerError)
		return
	}

	// Set context cache if enabled
	if h.config.Settings.ContextCache.Enabled {
		go func() {
			if err := h.setContextCache(r, req, resp); err != nil {
				log.Printf("Error setting context cache: %v", err)
			}
		}()
	}

	h.respondWithJSON(w, resp, "")
	h.performResponseAuditLogging(r, resp)
}

func (h *Handler) setContextCache(r *http.Request, req openai.ChatCompletionRequest, resp openai.ChatCompletionResponse) error {
	prompt := h.lastUserMessage(req)
	sessionId := h.getSessionId(r, req)

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no choices in response")
	}

	if err := lib.SetContextCache(prompt, resp.Choices[0].Message.Content, sessionId); err != nil {
		return fmt.Errorf("error setting context cache: %v", err)
	}

	return nil
}

func (h *Handler) setContextCacheFiltered(r *http.Request, req openai.ChatCompletionRequest, msg string) error {
	prompt := h.lastUserMessage(req)
	sessionId := h.getSessionId(r, req)
	if err := lib.SetContextCache(prompt, msg, sessionId); err != nil {
		return fmt.Errorf("error setting context cache: %v", err)
	}

	return nil
}

func (h *Handler) performResponseAuditLogging(r *http.Request, resp openai.ChatCompletionResponse) {
	apiKeyID := r.Context().Value("apiKeyId").(uuid.UUID)
	productID, err := h.getProductIDFromAPIKey(apiKeyID)
	if err != nil {
		log.Printf("Failed to retrieve ProductID for apiKeyId %s: %v", apiKeyID, err)
		return
	}

	responseJSON, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	auditLog := lib.AuditLogs(string(responseJSON), "openai_chat_completion", apiKeyID, "output", productID, r)

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

func (h *Handler) handleAssistantRequest(w http.ResponseWriter, r *http.Request, action string) {
	var resp interface{}
	var err error

	switch action {
	case "create":
		var req openai.AssistantRequest
		if err := h.decodeJSONBody(r, &req); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
		resp, err = h.client.CreateAssistant(r.Context(), req)
	case "list":
		pagination := h.getPaginationFromRequest(r)
		resp, err = h.client.ListAssistants(r.Context(), pagination.Limit, pagination.Order, pagination.After, pagination.Before)
	case "retrieve":
		assistantID := chi.URLParam(r, "assistant_id")
		resp, err = h.client.RetrieveAssistant(r.Context(), assistantID)
	case "modify":
		assistantID := chi.URLParam(r, "assistant_id")
		var req openai.AssistantRequest
		if err := h.decodeJSONBody(r, &req); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
		resp, err = h.client.ModifyAssistant(r.Context(), assistantID, req)
	case "delete":
		assistantID := chi.URLParam(r, "assistant_id")
		resp, err = h.client.DeleteAssistant(r.Context(), assistantID)
	default:
		h.handleError(w, fmt.Errorf("unsupported action: %s", action), http.StatusBadRequest)
		return
	}

	if err != nil {
		h.handleError(w, fmt.Errorf("API error: %v", err), http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, resp, "")
}

func (h *Handler) handleRunRequest(w http.ResponseWriter, r *http.Request, action string) {
	threadID := chi.URLParam(r, "thread_id")

	var resp interface{}
	var err error

	switch action {
	case "create":
		var runRequest openai.RunRequest
		if err := h.decodeJSONBody(r, &runRequest); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
		if runRequest.Stream {
			h.handleStreamingRun(w, r.Context(), threadID, runRequest)
			return
		}
		resp, err = h.client.CreateRun(r.Context(), threadID, runRequest)
	case "list":
		pagination := h.getPaginationFromRequest(r)
		resp, err = h.client.ListRuns(r.Context(), threadID, pagination)
	case "retrieve":
		runID := chi.URLParam(r, "run_id")
		resp, err = h.client.RetrieveRun(r.Context(), threadID, runID)
	case "modify":
		runID := chi.URLParam(r, "run_id")
		var modifyRequest openai.RunModifyRequest
		if err := h.decodeJSONBody(r, &modifyRequest); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
		resp, err = h.client.ModifyRun(r.Context(), threadID, runID, modifyRequest)
	case "cancel":
		runID := chi.URLParam(r, "run_id")
		resp, err = h.client.CancelRun(r.Context(), threadID, runID)
	case "submit_tool_outputs":
		runID := chi.URLParam(r, "run_id")
		var submitRequest openai.SubmitToolOutputsRequest
		if err := h.decodeJSONBody(r, &submitRequest); err != nil {
			h.handleError(w, err, http.StatusBadRequest)
			return
		}
		resp, err = h.client.SubmitToolOutputs(r.Context(), threadID, runID, submitRequest)
	default:
		h.handleError(w, fmt.Errorf("unsupported action: %s", action), http.StatusBadRequest)
		return
	}

	if err != nil {
		h.handleError(w, fmt.Errorf("API error: %v", err), http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, resp, "")
}

func (h *Handler) handleStreamingRun(w http.ResponseWriter, ctx context.Context, threadID string, runRequest openai.RunRequest) {
	stream, err := h.client.CreateRunStream(ctx, threadID, runRequest)
	if err != nil {
		h.handleError(w, fmt.Errorf("error creating stream: %v", err), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("OSCacheStatus", "MISS")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.handleError(w, fmt.Errorf("streaming unsupported"), http.StatusInternalServerError)
		return
	}

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		if err != nil {
			fmt.Fprintf(w, "data: {\"error\": \"%v\"}\n\n", err)
			flusher.Flush()
			return
		}

		data, err := json.Marshal(response)
		if err != nil {
			h.handleError(w, fmt.Errorf("error marshaling response: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()
	}
}

func (h *Handler) handleRunStepRequest(w http.ResponseWriter, r *http.Request, action string) {
	threadID := chi.URLParam(r, "thread_id")
	runID := chi.URLParam(r, "run_id")

	var resp interface{}
	var err error

	switch action {
	case "list":
		pagination := h.getPaginationFromRequest(r)
		resp, err = h.client.ListRunSteps(r.Context(), threadID, runID, pagination)
	case "retrieve":
		stepID := chi.URLParam(r, "step_id")
		resp, err = h.client.RetrieveRunStep(r.Context(), threadID, runID, stepID)
	default:
		h.handleError(w, fmt.Errorf("unsupported action: %s", action), http.StatusBadRequest)
		return
	}

	if err != nil {
		h.handleError(w, fmt.Errorf("API error: %v", err), http.StatusInternalServerError)
		return
	}

	h.respondWithJSON(w, resp, "")
}

func (h *Handler) performThreadAuditLogging(r *http.Request, resp openai.Thread) {
	apiKeyID := r.Context().Value("apiKeyId").(uuid.UUID)
	productID, err := h.getProductIDFromAPIKey(apiKeyID)
	if err != nil {
		log.Printf("Failed to retrieve ProductID for apiKeyId %s: %v", apiKeyID, err)
		return
	}

	responseJSON, _ := json.Marshal(resp)
	lib.AuditLogs(string(responseJSON), "openai_create_thread", apiKeyID, "output", productID, r)
}
