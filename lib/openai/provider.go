package openai

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/openshieldai/go-openai"
	"github.com/openshieldai/openshield/lib/provider"
	"github.com/openshieldai/openshield/lib/types"
	"log"
)

type OpenAIProvider struct {
	client *openai.Client
}

func NewOpenAIProvider(apiKey, baseURL string) provider.Provider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL
	client := openai.NewClientWithConfig(config)
	return &OpenAIProvider{client: client}
}

func (o *OpenAIProvider) CreateChatCompletion(ctx context.Context, req provider.ChatCompletionRequest) (*provider.ChatCompletionResponse, error) {
	resp, err := provider.HandleChatCompletionRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		return resp, nil
	}

	openAIReq := openai.ChatCompletionRequest{
		Model:     req.Model,
		Messages:  convertMessages(req.Messages),
		MaxTokens: req.MaxTokens,
	}

	openAIResp, err := o.client.CreateChatCompletion(ctx, openAIReq)
	if err != nil {
		return nil, fmt.Errorf("error creating chat completion: %v", err)
	}

	providerResp := convertResponse(openAIResp)

	productID := ctx.Value("productID").(uuid.UUID)
	err = provider.SetContextCacheResponse(ctx, req, providerResp, productID)
	if err != nil {
		log.Printf("Error setting context cache: %v", err)
	}

	return providerResp, nil
}

func (o *OpenAIProvider) CreateChatCompletionStream(ctx context.Context, req provider.ChatCompletionRequest) (provider.Stream, error) {
	log.Println("Creating chat completion stream")
	openAIReq := openai.ChatCompletionRequest{
		Model:     req.Model,
		Messages:  convertMessages(req.Messages),
		MaxTokens: req.MaxTokens,
		Stream:    true,
	}

	stream, err := o.client.CreateChatCompletionStream(ctx, openAIReq)
	if err != nil {
		log.Printf("Error creating stream: %v", err)
		return nil, err
	}

	return &OpenAIStream{stream: stream}, nil
}

type OpenAIStream struct {
	stream *openai.ChatCompletionStream
}

func (s *OpenAIStream) Recv() (provider.StreamResponse, error) {
	resp, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	return &OpenAIStreamResponse{resp: resp}, nil
}

func (s *OpenAIStream) Close() {
	s.stream.Close()
}

type OpenAIStreamResponse struct {
	resp openai.ChatCompletionStreamResponse
}

func (r *OpenAIStreamResponse) GetContent() string {
	if len(r.resp.Choices) > 0 {
		content := r.resp.Choices[0].Delta.Content
		return content
	}
	log.Println("GetContent: empty")
	return ""
}

func (r *OpenAIStreamResponse) GetFinishReason() string {
	if len(r.resp.Choices) > 0 {
		reason := string(r.resp.Choices[0].FinishReason)
		return reason
	}
	return ""
}

func (r *OpenAIStreamResponse) GetID() string {
	id := r.resp.ID
	log.Printf("GetID: %s", id)
	return id
}

func (r *OpenAIStreamResponse) GetCreated() int64 {
	created := r.resp.Created
	return created
}

func (r *OpenAIStreamResponse) GetModel() string {
	model := r.resp.Model
	return model
}

func convertMessages(messages []types.Message) []openai.ChatCompletionMessage {
	openAIMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openAIMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return openAIMessages
}

func convertResponse(resp openai.ChatCompletionResponse) *provider.ChatCompletionResponse {
	choices := make([]provider.Choice, len(resp.Choices))
	for i, choice := range resp.Choices {
		choices[i] = provider.Choice{
			Index: choice.Index,
			Message: types.Message{
				Role:    choice.Message.Role,
				Content: choice.Message.Content,
			},
			FinishReason: string(choice.FinishReason),
		}
	}

	return &provider.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  resp.Object,
		Created: resp.Created,
		Model:   resp.Model,
		Choices: choices,
		Usage: provider.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}
