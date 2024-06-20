package models

type Tags struct {
	Base      Base   `gorm:"embedded"`
	Status    string `gorm:"status;<-:false;not null;default:'active'"`
	Name      string `gorm:"name;<-:false;not null"`
	CreatedBy string `gorm:"created_by;<-:false;not null"`
}
