package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib/types"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/openshieldai/openshield/lib/provider"
)

type AnthropicProvider struct {
	ApiKey  string
	BaseURL string
}
type AnthropicResponse struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Role         string    `json:"role"`
	Model        string    `json:"model"`
	Content      []Content `json:"content"`
	StopReason   string    `json:"stop_reason"`
	StopSequence string    `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func NewAnthropicProvider(apiKey, baseURL string) provider.Provider {
	return &AnthropicProvider{
		ApiKey:  apiKey,
		BaseURL: baseURL,
	}
}

func (a *AnthropicProvider) CreateChatCompletion(ctx context.Context, req provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
	cachedResp, err := provider.HandleChatCompletionRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if cachedResp != nil {
		return cachedResp, nil
	}

	productID, ok := ctx.Value("productID").(uuid.UUID)
	if !ok {
		return nil, fmt.Errorf("productID not found in context")
	}

	url := fmt.Sprintf("%s/messages", a.BaseURL)

	requestBody := map[string]interface{}{
		"model":      req.Model,
		"messages":   req.Messages,
		"max_tokens": req.MaxTokens,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", a.ApiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", httpResp.StatusCode, string(body))
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	providerResp := convertAnthropicResponse(&anthropicResp)

	// Set the context cache
	err = provider.SetContextCacheResponse(ctx, req, providerResp, productID)
	if err != nil {
		log.Printf("Error setting context cache: %v", err)
	}

	return providerResp, nil
}

func (a *AnthropicProvider) CreateChatCompletionStream(ctx context.Context, req provider.ChatCompletionRequest) (provider.Stream, error) {
	url := fmt.Sprintf("%s/messages", a.BaseURL)

	requestBody := map[string]interface{}{
		"model":      req.Model,
		"messages":   req.Messages,
		"stream":     true,
		"max_tokens": req.MaxTokens,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.ApiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return &AnthropicStream{reader: bufio.NewReader(resp.Body)}, nil
}

type AnthropicStream struct {
	reader *bufio.Reader
	id     string
	model  string
}

func (s *AnthropicStream) Close() {
}

func (s *AnthropicStream) Recv() (provider.StreamResponse, error) {
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("error reading stream: %v", err)
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return nil, io.EOF
		}

		var event struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return nil, fmt.Errorf("error unmarshaling event type: %v", err)
		}

		switch event.Type {
		case "message_start":
			var resp struct {
				Type    string `json:"type"`
				Message struct {
					ID    string `json:"id"`
					Model string `json:"model"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &resp); err != nil {
				return nil, fmt.Errorf("error unmarshaling message_start: %v", err)
			}
			s.id = resp.Message.ID
			s.model = resp.Message.Model
		case "content_block_delta":
			var resp struct {
				Type  string `json:"type"`
				Index int    `json:"index"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &resp); err != nil {
				return nil, fmt.Errorf("error unmarshaling content_block_delta: %v", err)
			}
			return &AnthropicStreamResponse{
				ID:      s.id,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   s.model,
				Choices: []provider.Choice{
					{
						Index: resp.Index,
						Message: types.Message{
							Content: resp.Delta.Text,
						},
						FinishReason: "",
					},
				},
			}, nil
		case "message_delta":
			var resp struct {
				Type  string `json:"type"`
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &resp); err != nil {
				return nil, fmt.Errorf("error unmarshaling message_delta: %v", err)
			}
			return &AnthropicStreamResponse{
				ID:      s.id,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   s.model,
				Choices: []provider.Choice{
					{
						Index:        0,
						Message:      types.Message{},
						FinishReason: resp.Delta.StopReason,
					},
				},
			}, nil
		case "message_stop":
			return &AnthropicStreamResponse{
				ID:      s.id,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   s.model,
				Choices: []provider.Choice{
					{
						Index:        0,
						Message:      types.Message{},
						FinishReason: "stop",
					},
				},
			}, nil
		default:
			continue
		}
	}
}

type AnthropicStreamResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []provider.Choice `json:"choices"`
}

func (r *AnthropicStreamResponse) GetContent() string {
	if len(r.Choices) > 0 {
		return r.Choices[0].Message.Content
	}
	return ""
}

func (r *AnthropicStreamResponse) GetFinishReason() string {
	if len(r.Choices) > 0 {
		return r.Choices[0].FinishReason
	}
	return ""
}

func (r *AnthropicStreamResponse) GetID() string {
	return r.ID
}

func (r *AnthropicStreamResponse) GetCreated() int64 {
	return r.Created
}

func (r *AnthropicStreamResponse) GetModel() string {
	return r.Model
}

func convertAnthropicResponse(resp *AnthropicResponse) *provider.ChatCompletionResponse {
	content := ""
	for _, c := range resp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &provider.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  resp.Type,
		Created: 0, // Anthropic doesn't provide this
		Model:   resp.Model,
		Choices: []provider.Choice{
			{
				Index: 0,
				Message: types.Message{
					Role:    resp.Role,
					Content: content,
				},
				FinishReason: resp.StopReason,
			},
		},
		Usage: provider.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

func readLine(r io.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk := make([]byte, 1)
		n, err := r.Read(chunk)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return line, nil
		}
		if chunk[0] == '\n' {
			return line, nil
		}
		line = append(line, chunk[0])
	}
}
