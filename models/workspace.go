package models

type Workspaces struct {
	Base      Base   `gorm:"embedded"`
	Status    Status `faker:"status" sql:"status;not null;type:enum('active', 'inactive', 'archived');default:'active'"`
	Name      string `gorm:"name;not null"`
	Tags      string `faker:"tags" gorm:"tags;<-:false"`
	CreatedBy string `faker:"name" gorm:"created_by;not null"`
}
