package cmd

import (
	"bufio"
	"fmt"
	"github.com/bxcodec/faker/v3"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/models"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"math/rand"
	"os"
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
	rootCmd.AddCommand(configWizardCmd)
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
		printConfig(v.AllSettings(), 0)

		fmt.Println("\nEnter the path of the setting you want to change, or 'q' to quit:")
		var path string
		fmt.Scanln(&path)

		if path == "q" {
			break
		}

		fmt.Println("Enter the new value (YAML format):")
		var value string
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			value = scanner.Text()
		}

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

func printConfig(value interface{}, indent int) {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, val := range v {
			fmt.Printf("%s%s:\n", strings.Repeat("  ", indent), key)
			printConfig(val, indent+1)
		}
	case []interface{}:
		for i, val := range v {
			fmt.Printf("%s- [%d]\n", strings.Repeat("  ", indent), i)
			printConfig(val, indent+1)
		}
	default:
		fmt.Printf("%s%v\n", strings.Repeat("  ", indent), v)
	}
}

func updateConfig(v *viper.Viper, path string, value string) error {
	var parsedValue interface{}
	err := yaml.Unmarshal([]byte(value), &parsedValue)
	if err != nil {
		return fmt.Errorf("invalid YAML: %v", err)
	}

	currentValue := v.Get(path)
	if currentValue == nil {
		return fmt.Errorf("invalid configuration path: %s", path)
	}

	// Type checking and conversion
	switch reflect.TypeOf(currentValue).Kind() {
	case reflect.Int:
		if i, ok := parsedValue.(int); ok {
			v.Set(path, i)
		} else {
			return fmt.Errorf("invalid type for %s: expected int", path)
		}
	case reflect.Bool:
		if b, ok := parsedValue.(bool); ok {
			v.Set(path, b)
		} else {
			return fmt.Errorf("invalid type for %s: expected bool", path)
		}
	case reflect.Float64:
		if f, ok := parsedValue.(float64); ok {
			v.Set(path, f)
		} else {
			return fmt.Errorf("invalid type for %s: expected float64", path)
		}
	case reflect.String:
		if s, ok := parsedValue.(string); ok {
			v.Set(path, s)
		} else {
			return fmt.Errorf("invalid type for %s: expected string", path)
		}
	case reflect.Slice:
		if slice, ok := parsedValue.([]interface{}); ok {
			v.Set(path, slice)
		} else {
			return fmt.Errorf("invalid type for %s: expected slice", path)
		}
	case reflect.Map:
		if m, ok := parsedValue.(map[string]interface{}); ok {
			v.Set(path, m)
		} else {
			return fmt.Errorf("invalid type for %s: expected map", path)
		}
	default:
		v.Set(path, parsedValue)
	}

	return nil
}

var configWizardCmd = &cobra.Command{
	Use:   "config-wizard",
	Short: "Interactive wizard to create or update config.yaml",
	Run: func(cmd *cobra.Command, args []string) {
		runConfigWizard()
	},
}

func runConfigWizard() {
	config := lib.Configuration{}
	v := reflect.ValueOf(&config).Elem()

	fmt.Println("Do you want to change default values? (y/n):")
	changeDefaults := confirmInput()

	fmt.Println("Please provide values for the following settings:")

	fillStructure(v, "", changeDefaults)

	yamlData, err := yaml.Marshal(config)
	if err != nil {
		fmt.Printf("Error marshaling config to YAML: %v\n", err)
		return
	}

	err = os.WriteFile("config.yaml", yamlData, 0644)
	if err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Println("Configuration file 'config.yaml' has been created successfully!")
}

func fillStructure(v reflect.Value, prefix string, changeDefaults bool) {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		fieldName := fieldType.Name
		fullName := prefix + fieldName

		switch field.Kind() {
		case reflect.Struct:
			fillStructure(field, fullName+".", changeDefaults)
		case reflect.Ptr:
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			fillStructure(field.Elem(), fullName+".", changeDefaults)
		case reflect.Slice:
			handleSlice(field, fullName, changeDefaults)
		default:
			handleField(field, fieldType, fullName, changeDefaults)
		}
	}
}

func handleSlice(field reflect.Value, fullName string, changeDefaults bool) {
	fmt.Printf("Enter the number of elements for %s: ", fullName)
	countStr := getInput()
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 0 {
		fmt.Println("Invalid input. Using 0 elements.")
		return
	}

	sliceType := field.Type().Elem()
	newSlice := reflect.MakeSlice(field.Type(), count, count)

	for i := 0; i < count; i++ {
		fmt.Printf("Element %d of %s:\n", i+1, fullName)
		elem := reflect.New(sliceType).Elem()
		fillStructure(elem, fmt.Sprintf("%s[%d].", fullName, i), changeDefaults)
		newSlice.Index(i).Set(elem)
	}

	field.Set(newSlice)
}

func handleField(field reflect.Value, fieldType reflect.StructField, fullName string, changeDefaults bool) {
	tag := fieldType.Tag.Get("mapstructure")
	defaultValue := getDefaultValue(tag)

	if !changeDefaults && defaultValue != "" {
		setValue(field, defaultValue)
		return
	}

	if strings.Contains(tag, "omitempty") && !changeDefaults {
		return
	}

	prompt := fmt.Sprintf("Enter value for %s (%v)", fullName, fieldType.Type)
	if defaultValue != "" {
		prompt += fmt.Sprintf(" [default: %s]", defaultValue)
	}
	prompt += ": "

	var value string
	for {
		fmt.Print(prompt)
		value = getInput()

		if value == "" && defaultValue != "" {
			value = defaultValue
		}

		if setValue(field, value) {
			break
		}
		fmt.Println("Invalid input. Please try again.")
	}
}

func getDefaultValue(tag string) string {
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		if strings.HasPrefix(part, "default=") {
			return strings.TrimPrefix(part, "default=")
		}
	}
	return ""
}

func setValue(field reflect.Value, value string) bool {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int:
		if intValue, err := strconv.Atoi(value); err == nil {
			field.SetInt(int64(intValue))
		} else {
			return false
		}
	case reflect.Bool:
		if boolValue, err := strconv.ParseBool(value); err == nil {
			field.SetBool(boolValue)
		} else {
			return false
		}
	default:
		return false
	}
	return true
}

func getInput() string {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func confirmInput() bool {
	input := getInput()
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}
