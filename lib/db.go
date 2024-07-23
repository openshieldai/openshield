package lib

import (
	"log"

	"github.com/openshieldai/openshield/models"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"
)

func DB() *gorm.DB {
	config := GetConfig()
	connection, err := gorm.Open(postgres.Open(config.Settings.Database.URI), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	if config.Settings.Database.AutoMigration {
		err := connection.AutoMigrate(&models.ApiKeys{},
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
