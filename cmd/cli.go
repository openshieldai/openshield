package cmd

import (
	"fmt"
	"github.com/bxcodec/faker/v3"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"math/rand"
	"reflect"
	"strconv"
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
	rootCmd.AddCommand(editConfigCmd)
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

var editConfigCmd = &cobra.Command{
	Use:   "edit-config",
	Short: "Edit the config.yaml file",
	Run: func(cmd *cobra.Command, args []string) {
		editConfig()
	},
}

func editConfig() {
	v := viper.New()
	v.SetConfigFile("config.yaml")
	err := v.ReadInConfig()
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}

	for {
		fmt.Println("\nCurrent configuration:")
		printConfig(v.AllSettings())

		fmt.Println("\nEnter the path of the setting you want to change, or 'q' to quit:")
		var path string
		fmt.Scanln(&path)

		if path == "q" {
			break
		}

		fmt.Println("Enter the new value:")
		var value string
		fmt.Scanln(&value)

		if err := updateConfig(v, path, value); err != nil {
			fmt.Printf("Error updating config: %v\n", err)
		} else {
			fmt.Println("Configuration updated successfully.")
			if err := v.WriteConfig(); err != nil {
				fmt.Printf("Error writing config file: %v\n", err)
			}
		}
	}
}

func printConfig(settings map[string]interface{}, prefix ...string) {
	for key, value := range settings {
		fullKey := strings.Join(append(prefix, key), ".")
		if subMap, ok := value.(map[string]interface{}); ok {
			printConfig(subMap, append(prefix, key)...)
		} else {
			fmt.Printf("%s: %v\n", fullKey, value)
		}
	}
}

func updateConfig(v *viper.Viper, path string, value string) error {
	currentValue := v.Get(path)
	if currentValue == nil {
		return fmt.Errorf("invalid configuration path: %s", path)
	}

	var newValue interface{}
	var err error

	switch reflect.TypeOf(currentValue).Kind() {
	case reflect.Int:
		newValue, err = strconv.Atoi(value)
	case reflect.Bool:
		newValue, err = strconv.ParseBool(value)
	case reflect.Float64:
		newValue, err = strconv.ParseFloat(value, 64)
	default:
		newValue = value
	}

	if err != nil {
		return fmt.Errorf("invalid value for %s: %v", path, err)
	}

	v.Set(path, newValue)
	return nil
}
