package cmd

import (
	"fmt"
	"github.com/go-faker/faker/v4"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/models"
	"gorm.io/gorm"
	"math/rand"
	"reflect"
	"strings"
)

var generatedTags []string

func init() {
	// Faker providers setup
	{
		err := faker.AddProvider("aifamily", func(v reflect.Value) (interface{}, error) {
			return string(models.OpenAI), nil
		})
		if err != nil {
			return
		}
	}
	{
		err := faker.AddProvider("status", func(v reflect.Value) (interface{}, error) {
			statuses := []string{string(models.Active), string(models.Inactive), string(models.Archived)}
			return statuses[rand.Intn(len(statuses))], nil
		})
		{
			if err != nil {
				return
			}
		}
	}
	{
		err := faker.AddProvider("finishreason", func(v reflect.Value) (interface{}, error) {
			statuses := []string{string(models.Stop), string(models.Length), string(models.Null), string(models.FunctionCall), string(models.ContentFilter)}
			return statuses[rand.Intn(len(statuses))], nil
		})
		if err != nil {
			return
		}
	}
	{
		err := faker.AddProvider("tags", func(v reflect.Value) (interface{}, error) {
			return getRandomTags(), nil
		})
		if err != nil {
			return
		}
	}
}

func createMockData(db ...*gorm.DB) {
	database := lib.DB()
	if len(db) > 0 && db[0] != nil {
		database = db[0]
	}
	createMockTags(database, 10)
	createMockRecords(database, &models.AiModels{}, 2)
	createMockRecords(database, &models.ApiKeys{}, 2)
	createMockRecords(database, &models.AuditLogs{}, 2)
	createMockRecords(database, &models.Products{}, 2)
	createMockRecords(database, &models.Usage{}, 2)
	createMockRecords(database, &models.Workspaces{}, 2)
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

		result := db.Create(tag)
		if result.Error != nil {
			fmt.Printf("error inserting fake data for Tag: %v\n", result.Error)
		}
	}
}

func getRandomTags() string {
	numTags := rand.Intn(3) + 1
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
		newModel := reflect.New(reflect.TypeOf(model).Elem()).Interface()
		if err := faker.FakeData(newModel); err != nil {
			fmt.Printf("error generating fake data for %T: %v\n", newModel, err)
			continue
		}

		// Ensure the first record is always active
		if i == 0 {
			setValueOfObject(newModel, "Status", models.Active)
		}

		fmt.Printf("Generated data for %T:\n", newModel)
		fmt.Printf("%+v\n\n", newModel)
		result := db.Create(newModel)
		if result.Error != nil {
			_ = fmt.Errorf("error inserting fake data for %T: %v", model, result.Error)
		}
	}
}
func setValueOfObject(obj interface{}, fieldName string, value interface{}) {
	field := reflect.ValueOf(obj).Elem().FieldByName(fieldName)
	if field.IsValid() && field.CanSet() {
		field.Set(reflect.ValueOf(value))
	} else {
		fmt.Printf("Warning: Unable to set field %s\n", fieldName)
	}
}
