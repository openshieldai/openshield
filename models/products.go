package models

import (
	"github.com/google/uuid"
)

type Products struct {
	Base        Base      `gorm:"embedded"`
	Status      string    `gorm:"status;<-:false;not null;default:'active'"`
	Name        string    `gorm:"name;<-:false;not null"`
	WorkspaceID uuid.UUID `gorm:"workspace_id;type:uuid;<-:false;not null"`
	Tags        string    `gorm:"tags;<-:false"`
	CreatedBy   string    `gorm:"created_by;<-:false;not null"`
}
