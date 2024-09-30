package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type AnthropicProvider struct {
	ApiKey  string
	BaseURL string
}

func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	return &AnthropicProvider{
		ApiKey:  apiKey,
		BaseURL: baseURL,
	}
}

func (a *AnthropicProvider) CreateChatCompletion(ctx context.Context, messages []Message, model string, maxTokens int) (*ChatCompletionResponse, error) {
	url := fmt.Sprintf("%s/v1/messages", a.BaseURL)

	requestBody := map[string]interface{}{
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.ApiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var anthropicResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, err
	}

	return &ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: Message{
					Role:    "assistant",
					Content: anthropicResp.Content[0].Text,
				},
			},
		},
	}, nil
}

func (a *AnthropicProvider) StreamChatCompletion(ctx context.Context, messages []Message, model string, maxTokens int) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/v1/messages", a.BaseURL)

	requestBody := map[string]interface{}{
		"model":      model,
		"messages":   messages,
		"max_tokens": maxTokens,
		"stream":     true,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.ApiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}
