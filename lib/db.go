package lib

import (
	"log"

	"github.com/openshieldai/openshield/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db *gorm.DB
)

func SetDB(customDB *gorm.DB) {
	db = customDB
}

func DB() *gorm.DB {
	if db == nil {
		config := GetConfig()
		connection, err := gorm.Open(postgres.Open(config.Settings.Database.URI), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		if config.Settings.Database.AutoMigration {
			err = connection.AutoMigrate(
				&models.Tags{},
				&models.AiModels{},
				&models.ApiKeys{},
				&models.AuditLogs{},
				&models.Products{},
				&models.Usage{},
				&models.Workspaces{},
			)
		}
		if err != nil {
			log.Panic(err)
		}
		db = connection
	}
	return db
}
