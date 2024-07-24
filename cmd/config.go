package cmd

import (
	"bufio"
	"fmt"
	"github.com/openshieldai/openshield/lib"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"os"
	"reflect"
	"strconv"
	"strings"
)

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
