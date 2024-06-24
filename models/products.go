package models

import (
	"github.com/google/uuid"
)

type Products struct {
	Base        Base      `gorm:"embedded"`
	Status      Status    `sql:"status;<-:false;not null;type:enum('active', 'inactive', 'archived');default:'active';default:'active'"`
	Name        string    `gorm:"name;<-:false;not null"`
	WorkspaceID uuid.UUID `gorm:"workspace_id;type:uuid;<-:false;not null"`
	Tags        string    `gorm:"tags;<-:false"`
	CreatedBy   string    `gorm:"created_by;<-:false;not null"`
}
