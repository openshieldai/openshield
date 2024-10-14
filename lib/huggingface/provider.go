package huggingface

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib/provider"
	"github.com/openshieldai/openshield/lib/types"
)

type HuggingFaceProvider struct {
	ApiKey  string
	BaseURL string
}

func NewHuggingFaceProvider(apiKey, baseURL string) provider.Provider {
	return &HuggingFaceProvider{
		ApiKey:  apiKey,
		BaseURL: baseURL,
	}
}

func (h *HuggingFaceProvider) CreateChatCompletion(ctx context.Context, req provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
	cachedResp, err := provider.HandleChatCompletionRequest(ctx, req)
	if err != nil {
		log.Printf("Error handling chat completion request: %v", err)
		return nil, err
	}
	if cachedResp != nil {
		log.Println("Returning cached response")
		return cachedResp, nil
	}

	productID, ok := ctx.Value("productID").(uuid.UUID)
	if !ok {
		log.Println("productID not found in context")
		return nil, fmt.Errorf("productID not found in context")
	}

	url := fmt.Sprintf("%s/models/%s", h.BaseURL, req.Model)
	//log.Printf("Making request to URL: %s", url)

	// Convert messages to a single string prompt
	prompt := convertMessages(req.Messages)

	requestBody := map[string]interface{}{
		"inputs": prompt,
		"parameters": map[string]interface{}{
			"max_new_tokens":   req.MaxTokens,
			"return_full_text": false,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Error marshaling request body: %v", err)
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.ApiKey)

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Error sending request: %v", err)
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)

	log.Printf("Response status: %d", httpResp.StatusCode)

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", httpResp.StatusCode, string(body))
	}

	var hfResp []map[string]interface{}
	if err := json.Unmarshal(body, &hfResp); err != nil {
		log.Printf("Error unmarshaling response: %v", err)
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	providerResp := convertHuggingFaceResponse(hfResp, req.Model)

	err = provider.SetContextCacheResponse(ctx, req, providerResp, productID)
	if err != nil {
		log.Printf("Error setting context cache: %v", err)
	}

	return providerResp, nil
}

func (h *HuggingFaceProvider) CreateChatCompletionStream(ctx context.Context, req provider.ChatCompletionRequest) (provider.Stream, error) {
	url := fmt.Sprintf("%s/models/%s", h.BaseURL, req.Model)
	log.Printf("Making streaming request to URL: %s", url)

	prompt := convertMessages(req.Messages)

	requestBody := map[string]interface{}{
		"inputs": prompt,
		"parameters": map[string]interface{}{
			"max_new_tokens":   req.MaxTokens,
			"return_full_text": false,
			"stream":           true,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Error marshaling request body: %v", err)
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	log.Printf("Streaming request body: %s", string(jsonBody))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Printf("Error creating streaming request: %v", err)
		return nil, fmt.Errorf("error creating streaming request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+h.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Error sending streaming request: %v", err)
		return nil, fmt.Errorf("error sending streaming request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		log.Printf("Unexpected status code for streaming request: %d", resp.StatusCode)
		return nil, fmt.Errorf("unexpected status code for streaming request: %d", resp.StatusCode)
	}

	return &HuggingFaceStream{
		reader: bufio.NewReader(resp.Body),
		model:  req.Model,
	}, nil
}

type HuggingFaceStream struct {
	reader *bufio.Reader
	model  string
	id     string
}

func (s *HuggingFaceStream) Close() {
}

func (s *HuggingFaceStream) Recv() (provider.StreamResponse, error) {
	line, err := s.reader.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("error reading stream: %v", err)
	}

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "data: ") {
		return s.Recv() // Skip non-data lines and try again
	}

	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return nil, io.EOF
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	if s.id == "" {
		s.id = uuid.New().String()
	}

	token, ok := resp["token"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	text, ok := token["text"].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected token format")
	}

	return &HuggingFaceStreamResponse{
		ID:      s.id,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   s.model,
		Choices: []provider.Choice{
			{
				Index: 0,
				Message: types.Message{
					Role:    "assistant",
					Content: text,
				},
				FinishReason: "",
			},
		},
	}, nil
}

type HuggingFaceStreamResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []provider.Choice `json:"choices"`
}

func (r *HuggingFaceStreamResponse) GetContent() string {
	if len(r.Choices) > 0 {
		return r.Choices[0].Message.Content
	}
	return ""
}

func (r *HuggingFaceStreamResponse) GetFinishReason() string {
	if len(r.Choices) > 0 {
		return r.Choices[0].FinishReason
	}
	return ""
}

func (r *HuggingFaceStreamResponse) GetID() string {
	return r.ID
}

func (r *HuggingFaceStreamResponse) GetCreated() int64 {
	return r.Created
}

func (r *HuggingFaceStreamResponse) GetModel() string {
	return r.Model
}

func convertMessages(messages []types.Message) string {
	var conversation strings.Builder
	for _, msg := range messages {
		conversation.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	return conversation.String()
}

func convertHuggingFaceResponse(hfResp []map[string]interface{}, model string) *provider.ChatCompletionResponse {
	content := ""
	if len(hfResp) > 0 && hfResp[0]["generated_text"] != nil {
		content = hfResp[0]["generated_text"].(string)
	}

	return &provider.ChatCompletionResponse{
		ID:      "hf_" + uuid.New().String(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []provider.Choice{
			{
				Index: 0,
				Message: types.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
			},
		},
		Usage: provider.Usage{
			PromptTokens:     0, // Hugging Face doesn't provide token usage
			CompletionTokens: 0,
			TotalTokens:      0,
		},
	}
}
