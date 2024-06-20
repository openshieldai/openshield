package models

type Workspaces struct {
	Base      Base   `gorm:"embedded"`
	Status    string `gorm:"status;<-:false;not null;default:'active'"`
	Name      string `gorm:"name;<-:false;not null"`
	Tags      string `gorm:"tags;<-:false"`
	CreatedBy string `gorm:"created_by;<-:false;not null"`
}
