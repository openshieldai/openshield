package models

import "github.com/google/uuid"

type FinishReason string

const (
	Stop          FinishReason = "stop"
	Length        FinishReason = "length"
	Null          FinishReason = "null"
	FunctionCall  FinishReason = "function_call"
	ContentFilter FinishReason = "content_filter"
)

type Usage struct {
	Base                 `gorm:"embedded"`
	ModelID              uuid.UUID    `gorm:"model_id;<-:create;not null"`
	PredictedTokensCount int          `gorm:"predicted_tokens_count;<-:create"`
	PromptTokensCount    int          `gorm:"prompt_tokens_count;<-:create;not null"`
	CompletionTokens     int          `gorm:"completion_tokens;<-:create;not null"`
	TotalTokens          int          `gorm:"total_tokens;<-:create;not null"`
	FinishReason         FinishReason `faker:"finishreason" gorm:"finish_reason;<-:create;not null"`
	RequestType          string       `gorm:"request_type;<-:create;not null"`
}
