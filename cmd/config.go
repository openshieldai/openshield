package cmd

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/openshieldai/openshield/lib"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	configOptions []configOption
	optionCounter int
)

type configOption struct {
	number int
	path   string
	value  interface{}
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
		configOptions = []configOption{} // Clear the configOptions slice
		optionCounter = 1
		generateConfigOptions(v.AllSettings(), "")

		fmt.Println("\nCurrent configuration:")
		for _, option := range configOptions {
			fmt.Printf("%d. %s: %v\n", option.number, option.path, option.value)
		}

		fmt.Println("\nEnter the number of the setting you want to change, or 'q' to quit:")
		var input string
		fmt.Scanln(&input)

		if input == "q" {
			break
		}

		number, err := strconv.Atoi(input)
		if err != nil || number < 1 || number > len(configOptions) {
			fmt.Println("Invalid input. Please enter a valid number.")
			continue
		}

		option := configOptions[number-1]
		fmt.Printf("Enter new value for %s: ", option.path)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			newValue := scanner.Text()
			if err := updateConfig(v, option.path, newValue); err != nil {
				fmt.Printf("Error updating config: %v\n", err)
			} else {
				fmt.Println("Configuration updated successfully.")
				if err := v.WriteConfig(); err != nil {
					fmt.Printf("Error writing config file: %v\n", err)
				} else {
					fmt.Println("Configuration file updated.")
				}
			}
		}

		// Reload the configuration
		err = v.ReadInConfig()
		if err != nil {
			fmt.Printf("Error reading updated config file: %v\n", err)
			return
		}
	}
}

func generateConfigOptions(value interface{}, prefix string) {
	v := reflect.ValueOf(value)

	switch {
	case value == nil:
		configOptions = append(configOptions, configOption{
			number: optionCounter,
			path:   prefix,
			value:  nil,
		})
		optionCounter++
	case v.Kind() == reflect.Map:
		for _, key := range v.MapKeys() {
			keyStr := key.String()
			newPrefix := prefix
			if newPrefix != "" {
				newPrefix += "."
			}
			newPrefix += keyStr
			generateConfigOptions(v.MapIndex(key).Interface(), newPrefix)
		}
	case v.Kind() == reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			newPrefix := fmt.Sprintf("%s[%d]", prefix, i)
			generateConfigOptions(v.Index(i).Interface(), newPrefix)
		}
	default:
		configOptions = append(configOptions, configOption{
			number: optionCounter,
			path:   prefix,
			value:  v.Interface(),
		})
		optionCounter++
	}
}

func updateConfig(v *viper.Viper, path string, value string) error {
	var parsedValue interface{}
	err := yaml.Unmarshal([]byte(value), &parsedValue)
	if err != nil {
		return fmt.Errorf("invalid YAML: %v", err)
	}

	parts := strings.Split(path, ".")

	// Create a function to recursively update nested maps
	var updateNestedMap func(m map[string]interface{}, parts []string, value interface{}) error
	updateNestedMap = func(m map[string]interface{}, parts []string, value interface{}) error {
		if len(parts) == 1 {
			m[parts[0]] = value
			return nil
		}

		key := parts[0]
		if strings.HasSuffix(key, "]") {
			arrayName := strings.Split(key, "[")[0]
			indexStr := strings.TrimSuffix(strings.Split(key, "[")[1], "]")
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return fmt.Errorf("invalid array index: %v", err)
			}

			array, ok := m[arrayName].([]interface{})
			if !ok {
				return fmt.Errorf("invalid array at %s", arrayName)
			}

			if index < 0 || index >= len(array) {
				return fmt.Errorf("array index out of bounds: %d", index)
			}

			element, ok := array[index].(map[string]interface{})
			if !ok {
				element = make(map[string]interface{})
			}

			err = updateNestedMap(element, parts[1:], value)
			if err != nil {
				return err
			}

			array[index] = element
			m[arrayName] = array
		} else {
			nextMap, ok := m[key].(map[string]interface{})
			if !ok {
				nextMap = make(map[string]interface{})
				m[key] = nextMap
			}
			return updateNestedMap(nextMap, parts[1:], value)
		}

		return nil
	}

	// Get the current configuration
	config := v.AllSettings()

	// Update the nested structure
	err = updateNestedMap(config, parts, parsedValue)
	if err != nil {
		return err
	}

	// Update the Viper instance with the modified configuration
	for key, val := range config {
		v.Set(key, val)
	}

	return nil
}
func addRule() {
	v := viper.New()
	v.SetConfigFile("config.yaml")
	err := v.ReadInConfig()
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}

	var ruleType string
	fmt.Print("Enter rule type (input/output): ")
	fmt.Scanln(&ruleType)

	if ruleType != "input" && ruleType != "output" {
		fmt.Println("Invalid rule type. Please enter 'input' or 'output'.")
		return
	}

	newRule := createRuleWizard()

	rules := v.Get(fmt.Sprintf("filters.%s", ruleType))
	var ruleSlice []interface{}

	if rules != nil {
		var ok bool
		ruleSlice, ok = rules.([]interface{})
		if !ok {
			fmt.Printf("Error: unexpected format for %s rules.\n", ruleType)
			return
		}
	}

	ruleSlice = append(ruleSlice, newRule)
	v.Set(fmt.Sprintf("filters.%s", ruleType), ruleSlice)

	if err := v.WriteConfig(); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Println("Rule added successfully.")
}

func createRuleWizard() map[string]interface{} {
	rule := make(map[string]interface{})
	rule["enabled"] = true

	fmt.Print("Enter rule name: ")
	rule["name"] = getInput()

	fmt.Print("Enter rule type (e.g., pii_filter): ")
	rule["type"] = getInput()

	action := make(map[string]interface{})
	fmt.Print("Enter action type: ")
	action["type"] = getInput()
	rule["action"] = action

	config := make(map[string]interface{})
	fmt.Print("Enter plugin name: ")
	config["plugin_name"] = getInput()

	fmt.Print("Enter threshold (0-100): ")
	threshold, err := strconv.Atoi(getInput())
	if err != nil || threshold < 0 || threshold > 100 {
		fmt.Println("Invalid threshold. Using default value of 50.")
		threshold = 50
	}
	config["threshold"] = threshold
	rule["config"] = config

	return rule
}

func removeRule() {
	v := viper.New()
	v.SetConfigFile("config.yaml")
	err := v.ReadInConfig()
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		return
	}

	var ruleType string
	fmt.Print("Enter rule type (input/output): ")
	fmt.Scanln(&ruleType)

	if ruleType != "input" && ruleType != "output" {
		fmt.Println("Invalid rule type. Please enter 'input' or 'output'.")
		return
	}

	rules := v.Get(fmt.Sprintf("filters.%s", ruleType))
	if rules == nil {
		fmt.Printf("No %s rules found.\n", ruleType)
		return
	}

	ruleSlice, ok := rules.([]interface{})
	if !ok {
		fmt.Printf("Error: unexpected format for %s rules.\n", ruleType)
		return
	}

	if len(ruleSlice) == 0 {
		fmt.Printf("No %s rules found.\n", ruleType)
		return
	}

	fmt.Printf("Current %s rules:\n", ruleType)
	for i, rule := range ruleSlice {
		r, ok := rule.(map[string]interface{})
		if !ok {
			fmt.Printf("%d. Unknown rule format\n", i+1)
			continue
		}
		fmt.Printf("%d. %s (%s)\n", i+1, r["name"], r["type"])
	}

	var ruleIndex int
	fmt.Print("Enter the number of the rule to remove: ")
	_, err = fmt.Scanf("%d", &ruleIndex)
	if err != nil || ruleIndex < 1 || ruleIndex > len(ruleSlice) {
		fmt.Println("Invalid rule number.")
		return
	}

	removedRule := ruleSlice[ruleIndex-1]
	ruleSlice = append(ruleSlice[:ruleIndex-1], ruleSlice[ruleIndex:]...)
	v.Set(fmt.Sprintf("filters.%s", ruleType), ruleSlice)

	if err := v.WriteConfig(); err != nil {
		fmt.Printf("Error writing config file: %v\n", err)
		return
	}

	fmt.Printf("Rule '%v' removed successfully.\n", removedRule.(map[string]interface{})["name"])
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
