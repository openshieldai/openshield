package models

import (
	"github.com/google/uuid"
)

type ApiKeys struct {
	Base      `gorm:"embedded"`
	ProductID uuid.UUID `gorm:"product_id;<-:false;not null"`
	ApiKey    string    `gorm:"api_key;<-:false;not null;uniqueIndex;index:idx_api_keys_status,unique"`
	Status    string    `gorm:"status;<-:false;not null;index:idx_api_keys_status,unique"`
	Tags      string    `gorm:"tags;<-:false"`
	CreatedBy string    `gorm:"created_by;<-:false;not null"`
}
