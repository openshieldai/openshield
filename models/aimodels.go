package models

type AiFamily string

// CREATE TYPE aifamily AS ENUM ('openai');

const (
	OpenAI AiFamily = "openai"
)

type AiModels struct {
	Base      `gorm:"embedded"`
	Family    AiFamily `faker:"aifamily" sql:"family;not null;type:enum('openai')"`
	ModelType string   `faker:"oneof: LLM,imagegen" gorm:"model_type;not null"`
	Model     string   `faker:"oneof: gpt3.5,gpt4" gorm:"model;not null"`
	Encoding  string   `faker:"oneof: SHA,MD5" gorm:"encoding;not null"`
	Size      string   `faker:"oneof: small,medium,large" gorm:"size;"`
	Quality   string   `faker:"oneof: low,medium,high" gorm:"quality;"`
	Status    Status   `faker:"status" sql:"status;not null;type:enum('active', 'inactive', 'archived');default:'active'"`
}
