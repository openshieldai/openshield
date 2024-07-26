package cmd

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"

	"github.com/go-faker/faker/v4"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/models"
	"gorm.io/gorm"
)

var generatedTags []string

type StatusProvider struct {
	mu              sync.Mutex
	activeGenerated map[reflect.Type]bool
}

func NewStatusProvider() *StatusProvider {
	return &StatusProvider{
		activeGenerated: make(map[reflect.Type]bool),
	}
}

func (sp *StatusProvider) Status(v reflect.Value) (interface{}, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	modelType := v.Type()
	if !sp.activeGenerated[modelType] {
		sp.activeGenerated[modelType] = true
		return string(models.Active), nil
	}

	statuses := []string{string(models.Active), string(models.Inactive), string(models.Archived)}
	return statuses[rand.Intn(len(statuses))], nil
}

func init() {

	faker.AddProvider("status", func(v reflect.Value) (interface{}, error) {
		statuses := []string{string(models.Active), string(models.Inactive), string(models.Archived)}
		return statuses[rand.Intn(len(statuses))], nil
	})

	faker.AddProvider("aifamily", func(v reflect.Value) (interface{}, error) {
		return models.OpenAI, nil
	})

	faker.AddProvider("finishreason", func(v reflect.Value) (interface{}, error) {
		reasons := []models.FinishReason{models.Stop, models.Length, models.Null, models.FunctionCall, models.ContentFilter}
		return reasons[rand.Intn(len(reasons))], nil
	})

	faker.AddProvider("tags", func(v reflect.Value) (interface{}, error) {
		return getRandomTags(), nil
	})
}

func createMockData() {
	db := lib.DB()
	createMockTags(db, 10)
	createMockRecords(db, &models.AiModels{}, 0)
	createMockRecords(db, &models.ApiKeys{}, 1)
	createMockRecords(db, &models.AuditLogs{}, 1)
	createMockRecords(db, &models.Products{}, 1)
	createMockRecords(db, &models.Usage{}, 1)
	createMockRecords(db, &models.Workspaces{}, 1)
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
			fmt.Printf("error inserting fake data for %T: %v\n", newModel, result.Error)
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
