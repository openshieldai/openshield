package lib

import (
	"context"
	"fmt"
	goopenai "github.com/openshieldai/go-openai"
)

type OpenAIProvider struct {
	ApiKey  string
	BaseURL string
	Client  *goopenai.Client
}

func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	c := goopenai.DefaultConfig(apiKey)
	c.BaseURL = baseURL
	client := goopenai.NewClientWithConfig(c)

	return &OpenAIProvider{
		ApiKey:  apiKey,
		BaseURL: baseURL,
		Client:  client,
	}
}

func (o *OpenAIProvider) CreateChatCompletion(ctx context.Context, messages []goopenai.ChatCompletionMessage, model string, maxTokens int) (*goopenai.ChatCompletionResponse, error) {
	req := goopenai.ChatCompletionRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	resp, err := o.Client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error creating chat completion: %v", err)
	}

	return &resp, nil
}

func (o *OpenAIProvider) CreateChatCompletionStream(ctx context.Context, messages []goopenai.ChatCompletionMessage, model string, maxTokens int) (*goopenai.ChatCompletionStream, error) {
	req := goopenai.ChatCompletionRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	stream, err := o.Client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error creating chat completion stream: %v", err)
	}

	return stream, nil
}
