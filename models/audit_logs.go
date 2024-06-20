package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLogs struct {
	Id          uuid.UUID       `gorm:"id;<-:create;type:uuid;default:gen_random_uuid();primaryKey;not null"`
	CreatedAt   time.Time       `gorm:"created_at;<-:create;default:now();not null"`
	UpdatedAt   time.Time       `gorm:"updated_at;<-:create;default:now();not null"`
	DeletedAt   *gorm.DeletedAt `gorm:"deleted_at;index"`
	ApiKeyID    uuid.UUID       `gorm:"api_key_id;type:uuid;<-:create;not null"`
	Message     string          `gorm:"message;<-:create;not null"`
	MessageType string          `gorm:"message_type;<-:create;not null"`
	Type        string          `gorm:"log_type;<-:create;not null"`
	Metadata    string          `gorm:"metadata;<-:create;not null"`
}
