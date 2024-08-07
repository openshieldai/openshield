package models

import (
	"github.com/google/uuid"
)

type ApiKeys struct {
	Base      `gorm:"embedded"`
	ProductID uuid.UUID `gorm:"product_id;not null"`
	ApiKey    string    `faker:"uuid_hyphenated" gorm:"api_key;not null;uniqueIndex;index:idx_api_keys_status,unique"`
	Status    Status    `faker:"status" sql:"status;not null;index:idx_api_keys_status,unique;type:enum('active', 'inactive', 'archived')"`
	Tags      string    `faker:"tags" gorm:"tags;<-:false"`
	CreatedBy string    `faker:"uuid_hyphenated" gorm:"created_by;not null"`
}
