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

		_, err = lib.SetCache(r.URL.Path, resJson)
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

		_, err = lib.SetCache(r.URL.Path, resJson)
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

	// Read the entire request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// Parse the request
	var req openai.ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Perform audit logging
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)
	lib.AuditLogs(string(body), "openai_chat_completion", apiKeyId, "input", r)

	// Apply input rules
	filteredResp, errorMessage, err := rules.Input(r, req)
	if filteredResp {
		http.Error(w, errorMessage, http.StatusBadRequest)
		return
	}

	// Check cache for non-streaming requests
	if !req.Stream {
		getCache, cacheStatus, err := lib.GetCache(string(body))
		if err != nil {
			log.Printf("Error getting cache: %v", err)
		}
		if cacheStatus {
			w.Header().Set(OSCacheStatusHeader, "HIT")
			w.Write(getCache)
			return
		}
		log.Printf("Cache miss for %v", cacheStatus)
	}

	client := openai.NewClient(openAIAPIKey)

	if req.Stream {
		// Streaming logic
		stream, err := client.CreateChatCompletionStream(r.Context(), req)
		if err != nil {
			log.Printf("Failed to create chat completion stream: %v", err)
			http.Error(w, fmt.Sprintf("Failed to create chat completion stream: %v", err), http.StatusInternalServerError)
			return
		}
		defer stream.Close()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			log.Println("Streaming unsupported")
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
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
	} else {
		// Non-streaming logic
		resp, err := client.CreateChatCompletion(r.Context(), req)
		if err != nil {
			log.Printf("Error creating chat completion: %v", err)
			http.Error(w, "Failed to create chat completion", http.StatusInternalServerError)
			return
		}

		// Cache the response if caching is enabled
		if config.Settings.Cache.Enabled {
			w.Header().Set(OSCacheStatusHeader, "MISS")
			resJson, err := json.Marshal(resp)
			if err != nil {
				log.Printf("Error marshalling response to JSON: %v", err)
			} else {
				_, err = lib.SetCache(string(body), resJson)
				if err != nil {
					log.Printf("Error setting cache: %v", err)
				}
			}
		} else {
			w.Header().Set(OSCacheStatusHeader, "BYPASS")
		}

		// Perform audit logging for the response
		responseJSON, _ := json.Marshal(resp)
		lib.AuditLogs(string(responseJSON), "openai_chat_completion", apiKeyId, "output", r)
		lib.Usage(resp.Model, 0, resp.Usage.TotalTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens, string(resp.Choices[0].FinishReason), "chat_completion")
		json.NewEncoder(w).Encode(resp)
	}
}
