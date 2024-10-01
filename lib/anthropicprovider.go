package lib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

func (a *AnthropicProvider) CreateChatCompletion(ctx context.Context, system string, messages []Message, model string, maxTokens int) (*AnthropicResponse, error) {
	url := fmt.Sprintf("%s/messages", a.BaseURL)

	requestBody := map[string]interface{}{
		"model":      model,
		"system":     system,
		"messages":   messages,
		"max_tokens": maxTokens,
	}

	if maxTokens > 0 {
		requestBody["max_tokens"] = maxTokens
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", a.ApiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}

	return &anthropicResp, nil
}

func (a *AnthropicProvider) StreamChatCompletion(ctx context.Context, system string, messages []Message, model string, maxTokens int) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/messages", a.BaseURL)

	requestBody := map[string]interface{}{
		"model":      model,
		"system":     system,
		"messages":   messages,
		"stream":     true,
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

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}
