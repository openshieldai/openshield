package models

type Workspaces struct {
	Base      Base   `gorm:"embedded"`
	Status    Status `sql:"status;<-:false;not null;type:enum('active', 'inactive', 'archived');default:'active'"`
	Name      string `gorm:"name;<-:false;not null"`
	Tags      string `gorm:"tags;<-:false"`
	CreatedBy string `gorm:"created_by;<-:false;not null"`
}
