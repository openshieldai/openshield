package cmd

import (
	"fmt"
	"github.com/bxcodec/faker/v3"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/models"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
	"math/rand"
	"reflect"
	"strings"
)

func init() {
	faker.AddProvider("aifamily", func(v reflect.Value) (interface{}, error) {
		return string(models.OpenAI), nil
	})

	faker.AddProvider("status", func(v reflect.Value) (interface{}, error) {
		statuses := []string{string(models.Active), string(models.Inactive), string(models.Archived)}
		return statuses[rand.Intn(len(statuses))], nil
	})
	faker.AddProvider("finishreason", func(v reflect.Value) (interface{}, error) {
		statuses := []string{string(models.Stop), string(models.Length), string(models.Null), string(models.FunctionCall), string(models.ContentFilter)}
		return statuses[rand.Intn(len(statuses))], nil
	})
	faker.AddProvider("tags", func(v reflect.Value) (interface{}, error) {
		return getRandomTags(), nil
	})
}

var generatedTags []string
var rootCmd = &cobra.Command{
	Use:   "openshield",
	Short: "OpenShield CLI",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(createTablesCmd)
	rootCmd.AddCommand(createMockDataCmd)
}

var createTablesCmd = &cobra.Command{
	Use:   "create-tables",
	Short: "Create database tables from models",
	Run: func(cmd *cobra.Command, args []string) {
		createTables()
	},
}

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

var createMockDataCmd = &cobra.Command{
	Use:   "create-mock-data",
	Short: "Create mock data in the database",
	Run: func(cmd *cobra.Command, args []string) {
		createMockData()
	},
}

func createMockData() {
	db := lib.DB()
	createMockTags(db, 10)
	createMockRecords(db, &models.AiModels{}, 2)
	createMockRecords(db, &models.ApiKeys{}, 2)
	createMockRecords(db, &models.AuditLogs{}, 2)
	createMockRecords(db, &models.Products{}, 2)
	createMockRecords(db, &models.Usage{}, 2)
	createMockRecords(db, &models.Workspaces{}, 2)
}

func createMockTags(db *gorm.DB, count int) {
	for i := 0; i < count; i++ {
		tag := &models.Tags{}
		if err := faker.FakeData(tag); err != nil {
			fmt.Printf("error generating fake data for Tag: %v\n", err)
			continue

		}
		tag.Name = fmt.Sprintf("Tag%d", i+1) // Ensure unique names
		generatedTags = append(generatedTags, tag.Name)

		fmt.Printf("Generated data for Tag:\n")
		fmt.Printf("%+v\n\n", tag)

		// result := db.Create(tag)
		// if result.Error != nil {
		//     fmt.Printf("error inserting fake data for Tag: %v\n", result.Error)
		// }
	}
}

func getRandomTags() string {
	numTags := rand.Intn(3) + 1
	//prevent duplicatte entries
	tagsCopy := make([]string, len(generatedTags))
	copy(tagsCopy, generatedTags)

	rand.Shuffle(len(tagsCopy), func(i, j int) {
		tagsCopy[i], tagsCopy[j] = tagsCopy[j], tagsCopy[i]
	})

	selectedTags := tagsCopy[:numTags]

	return strings.Join(selectedTags, ",")
}

func createMockRecords(db *gorm.DB, model interface{}, count int) {
	for i := 0; i < count; i++ {
		if err := faker.FakeData(model); err != nil {
			fmt.Printf("error generating fake data for %T: %v\n", model, err)
			continue
		}
		fmt.Printf("Generated data for %T:\n", model)
		fmt.Printf("%+v\n\n", model)
		//	result := db.Create(model)
		//	if result.Error != nil {
		//		fmt.Errorf("error inserting fake data for %T: %v", model, result.Error)
		//	}
	}

}
