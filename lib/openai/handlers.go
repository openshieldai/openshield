package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/rules"
	"github.com/sashabaranov/go-openai"
)

var client *openai.Client

const OSCacheStatusHeader = "OS-Cache-Status"

func ListModelsHandler(w http.ResponseWriter, r *http.Request) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey
	client = openai.NewClient(openAIAPIKey)

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

	json.NewEncoder(w).Encode(res)
}

func GetModelHandler(w http.ResponseWriter, r *http.Request) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey

	client = openai.NewClient(openAIAPIKey)
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

	json.NewEncoder(w).Encode(res)
}

func ChatCompletionHandler(w http.ResponseWriter, r *http.Request) {
	config := lib.GetConfig()
	openAIAPIKey := config.Secrets.OpenAIApiKey

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

	performAuditLogging(r, body)

	if filtered, errorMessage, _ := rules.Input(r, req); filtered {
		handleError(w, fmt.Errorf(errorMessage), http.StatusBadRequest)
		return
	}

	if req.Stream {
		handleStreamingRequest(w, r, req, openAIAPIKey)
	} else {
		handleNonStreamingRequest(w, r, body, req, config, openAIAPIKey)
	}
}

func performAuditLogging(r *http.Request, body []byte) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)
	lib.AuditLogs(string(body), "openai_chat_completion", apiKeyId, "input", r)
}

func handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, body []byte, req openai.ChatCompletionRequest, config lib.Configuration, openAIAPIKey string) {
	getCache, cacheStatus, err := lib.GetCache(string(body))
	if err != nil {
		log.Printf("Error getting cache: %v", err)
	}
	if cacheStatus {
		w.Header().Set(OSCacheStatusHeader, "HIT")
		w.Write(getCache)
		return
	}

	client := openai.NewClient(openAIAPIKey)
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

func handleStreamingRequest(w http.ResponseWriter, r *http.Request, req openai.ChatCompletionRequest, openAIAPIKey string) {
	client := openai.NewClient(openAIAPIKey)
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
		if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
			data, err := json.Marshal(response)
			if err != nil {
				log.Printf("Error marshaling response: %v", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}
	}
}

func performResponseAuditLogging(r *http.Request, resp openai.ChatCompletionResponse) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)
	responseJSON, _ := json.Marshal(resp)
	lib.AuditLogs(string(responseJSON), "openai_chat_completion", apiKeyId, "output", r)
	lib.Usage(resp.Model, 0, resp.Usage.TotalTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, string(resp.Choices[0].FinishReason), "chat_completion")
}

func handleError(w http.ResponseWriter, err error, statusCode int) {
	log.Printf("Error: %v", err)
	http.Error(w, err.Error(), statusCode)
}
