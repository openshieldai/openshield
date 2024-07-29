package lib

import (
	"log"
	"sync"

	"github.com/openshieldai/openshield/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db   *gorm.DB
	once sync.Once
)

func SetDB(customDB *gorm.DB) {
	db = customDB
}

func DB() *gorm.DB {
	once.Do(func() {
		if db == nil {
			config := GetConfig()
			connection, err := gorm.Open(postgres.Open(config.Settings.Database.URI), &gorm.Config{})
			if err != nil {
				panic("failed to connect database")
			}

			if config.Settings.Database.AutoMigration {
				err := connection.AutoMigrate(
					&models.Tags{},
					&models.AiModels{},
					&models.ApiKeys{},
					&models.AuditLogs{},
					&models.Products{},
					&models.Usage{},
					&models.Workspaces{},
				)
				if err != nil {
					log.Panic(err)
				}
			}
			db = connection
		}
	})
	return db
}
