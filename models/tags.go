package models

type Tags struct {
	Base      Base   `gorm:"embedded"`
	Status    Status `faker:"status" sql:"status;not null;type:enum('active', 'inactive', 'archived');default:'active'"`
	Name      string `gorm:"name;not null"`
	CreatedBy string `faker:"uuid_hyphenated" gorm:"created_by;not null"`
}
