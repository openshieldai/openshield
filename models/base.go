package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Base struct {
	Id        uuid.UUID       `gorm:"id;<-:false;type:uuid;default:gen_random_uuid();primaryKey;not null"`
	CreatedAt time.Time       `gorm:"created_at;<-:false;default:now();not null"`
	UpdatedAt time.Time       `gorm:"updated_at;<-:false;default:now();not null"`
	DeletedAt *gorm.DeletedAt `gorm:"deleted_at;index;<-:false"`
}

type Status string

// CREATE TYPE status AS ENUM ('active', 'inactive', 'archived');

const (
	Active   Status = "active"
	Inactive Status = "inactive"
	Archived Status = "archived"
)
