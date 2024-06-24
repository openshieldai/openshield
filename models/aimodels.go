package models

type AiFamily string

// CREATE TYPE aifamily AS ENUM ('openai');

const (
	OpenAI AiFamily = "openai"
)

type AiModels struct {
	Base      `gorm:"embedded"`
	Family    AiFamily `sql:"family;<-:false;not null;type:enum('openai')"`
	ModelType string   `gorm:"model_type;<-:false;not null"`
	Model     string   `gorm:"model;<-:false;not null"`
	Encoding  string   `gorm:"encoding;<-:false;not null"`
	Size      string   `gorm:"size;<-:false"`
	Quality   string   `gorm:"quality;<-:false"`
	Status    Status   `sql:"status;<-:false;not null;type:enum('active', 'inactive', 'archived');default:'active'"`
}
