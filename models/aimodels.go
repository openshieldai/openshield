package models

type AiFamily string

// CREATE TYPE aifamily AS ENUM ('openai');

const (
	OpenAI AiFamily = "openai"
)

type AiModels struct {
	Base      `gorm:"embedded"`
	Family    AiFamily `sql:"family;not null;type:enum('openai')"`
	ModelType string   `gorm:"model_type;not null"`
	Model     string   `gorm:"model;not null"`
	Encoding  string   `gorm:"encoding;not null"`
	Size      string   `gorm:"size;"`
	Quality   string   `gorm:"quality;"`
	Status    Status   `sql:"status;not null;type:enum('active', 'inactive', 'archived');default:'active'"`
}
