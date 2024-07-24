package models

type Tags struct {
	Base      Base   `gorm:"embedded"`
	Status    Status `sql:"status;not null;type:enum('active', 'inactive', 'archived');default:'active'"`
	Name      string `gorm:"name;not null"`
	CreatedBy string `gorm:"created_by;not null"`
}
