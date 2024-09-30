package lib

import (
	"context"
	"io"
)

type Provider interface {
	CreateChatCompletion(ctx context.Context, messages []Message, model string, maxTokens int) (*ChatCompletionResponse, error)
	StreamChatCompletion(ctx context.Context, messages []Message, model string, maxTokens int) (io.ReadCloser, error)
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
