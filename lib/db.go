package lib

import (
	"log"

	"github.com/openshieldai/openshield/models"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"
)

func DB() *gorm.DB {
	settings := NewSettings()
	connection, err := gorm.Open(postgres.Open(settings.Database.URL), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	if settings.Database.AutoMigration {
		err := connection.AutoMigrate(&models.ApiKeys{},
			&models.Products{},
			&models.Workspaces{},
			&models.Tags{},
			&models.AuditLogs{},
			&models.Usage{},
			&models.AiModels{})
		if err != nil {
			log.Panic(err)
		}
	}
	return connection
}
