package cmd

import (
	"fmt"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/models"
)

func createTables() {
	db := lib.DB()

	err := db.AutoMigrate(
		&models.ApiKeys{},
		&models.Products{},
		&models.Workspaces{},
		&models.Tags{},
		&models.AuditLogs{},
		&models.Usage{},
		&models.AiModels{},
	)
	if err != nil {
		fmt.Printf("Error creating tables: %v\n", err)
		return
	}

	fmt.Println("All tables created successfully")
}
