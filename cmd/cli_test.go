package cmd

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCreateMockData(t *testing.T) {

	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func(sqlDB *sql.DB) {
		err := sqlDB.Close()
		if err != nil {

		}
	}(sqlDB)

	dialector := postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	})
	db, err := gorm.Open(dialector, &gorm.Config{})
	assert.NoError(t, err)

	createExpectations := func(tableName string, count int, argCount int) {
		for i := 0; i < count; i++ {
			mock.ExpectBegin()
			args := make([]driver.Value, argCount)
			for j := range args {
				args[j] = sqlmock.AnyArg()
			}
			mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "` + tableName + `"`)).
				WithArgs(args...).
				WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
					AddRow(uuid.New(), time.Now(), time.Now()))
			mock.ExpectCommit()
		}
	}

	createExpectations("tags", 10, 6)
	createExpectations("ai_models", 2, 10)
	createExpectations("api_keys", 2, 7)
	createExpectations("audit_logs", 2, 11)
	createExpectations("products", 2, 7)
	createExpectations("usages", 2, 10)
	createExpectations("workspaces", 2, 6)

	createMockData(db)

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestCreateTables(t *testing.T) {

	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func(sqlDB *sql.DB) {
		err := sqlDB.Close()
		if err != nil {
			_ = fmt.Errorf("failed to create mock db %v", err)
		}
	}(sqlDB)

	dialector := postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	})
	db, err := gorm.Open(dialector, &gorm.Config{})
	assert.NoError(t, err)

	tables := []string{"tags", "ai_models", "api_keys", "audit_logs", "products", "usage", "workspaces"}
	for _, table := range tables {
		mock.ExpectQuery(`SELECT EXISTS \(SELECT FROM information_schema.tables WHERE table_name = \$1\)`).
			WithArgs(table).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	}

	for _, table := range tables {
		var exists bool
		err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = ?)", table).Scan(&exists).Error
		assert.NoError(t, err)
		assert.True(t, exists, "Table %s should exist", table)
	}

	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}
func TestAddAndRemoveRuleConfig(t *testing.T) {

	tmpfile, err := ioutil.TempFile("", "config.*.yaml")
	if err != nil {
		t.Fatal(err)
	}

	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			return
		}
	}(tmpfile.Name())
	{
		err := os.Setenv("OPENSHIELD_CONFIG_FILE", tmpfile.Name())
		if err != nil {
			return
		}
		defer func() {
			err := os.Unsetenv("OPENSHIELD_CONFIG_FILE")
			if err != nil {
				return
			}
		}()
	}
	initialConfig := `
filters:
  input:
    - name: existing_rule
      type: pii_filter
      enabled: true
      action:
        type: redact
      config:
        plugin_name: pii_plugin
        threshold: 80
`
	if _, err := tmpfile.Write([]byte(initialConfig)); err != nil {
		t.Fatal(err)
	}
	{
		err := tmpfile.Close()
		if err != nil {
			return
		}
	}
	viper.Reset()
	viper.SetConfigFile(tmpfile.Name())
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Error reading config file: %v", err)
	}

	t.Run("AddRule", func(t *testing.T) {
		// Create a pipe
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("Failed to create pipe: %v", err)
		}

		// Save original stdin
		oldStdin := os.Stdin
		defer func() {
			os.Stdin = oldStdin
			r.Close()
			w.Close()
		}()

		// Set stdin to our reader
		os.Stdin = r

		// Prepare input
		inputs := []string{
			"input\n",
			"new_rule\n",
			"sentiment_filter\n",
			"block\n",
			"sentiment_plugin\n",
			"90\n",
		}

		// Start a goroutine to feed input
		go func() {
			defer w.Close()
			for _, input := range inputs {
				t.Logf("Providing input: %q", strings.TrimSpace(input))
				_, err := w.Write([]byte(input))
				if err != nil {
					t.Logf("Error writing input: %v", err)
					return
				}
				time.Sleep(100 * time.Millisecond) // Small delay to ensure input is processed
			}
		}()

		// Execute the command
		output, err := executeCommand(rootCmd, "config", "add-rule")
		if err != nil {
			t.Fatalf("Error executing add-rule command: %v", err)
		}

		t.Logf("Add Rule Command Output:\n%s", output)

		// Verify the output
		assert.Contains(t, output, "Rule added successfully")

		// Verify the config was modified
		v := viper.New()
		v.SetConfigFile(tmpfile.Name())
		err = v.ReadInConfig()
		if err != nil {
			t.Fatalf("Error reading updated config: %v", err)
		}

		rules := v.Get("filters.input")
		rulesSlice, ok := rules.([]interface{})
		if !ok {
			t.Fatalf("Expected rules to be a slice, got %T", rules)
		}

		assert.Len(t, rulesSlice, 2, "Expected 2 rules after addition")
		if len(rulesSlice) > 1 {
			newRule := rulesSlice[1].(map[string]interface{})
			assert.Equal(t, "new_rule", newRule["name"])
			assert.Equal(t, "sentiment_filter", newRule["type"])
			assert.Equal(t, true, newRule["enabled"])
			assert.Equal(t, "block", newRule["action"].(map[string]interface{})["type"])
			assert.Equal(t, "sentiment_plugin", newRule["config"].(map[string]interface{})["plugin_name"])
			assert.Equal(t, float64(90), newRule["config"].(map[string]interface{})["threshold"])
		}
	})

	// Test removeRule
	t.Run("RemoveRule", func(t *testing.T) {
		input := "input\n2\n"
		t.Logf("RemoveRule Input:\n%s", input)
		inputBuffer := bytes.NewBufferString(input)
		output, err := executeCommandWithInput(rootCmd, inputBuffer, "config", "remove-rule")
		if err != nil {
			t.Fatalf("Error executing remove-rule command: %v", err)
		}

		t.Logf("Remove Rule Command Output:\n%s", output)

		// Verify the output
		assert.Contains(t, output, "Rule 'new_rule' removed successfully")

		// Verify the config was modified
		v := viper.New()
		v.SetConfigFile(tmpfile.Name())
		err = v.ReadInConfig()
		if err != nil {
			t.Fatalf("Error reading updated config: %v", err)
		}

		rules := v.Get("filters.input")
		rulesSlice, ok := rules.([]interface{})
		if !ok {
			t.Fatalf("Expected rules to be a slice, got %T", rules)
		}

		assert.Len(t, rulesSlice, 1, "Expected 1 rule after removal")
		if len(rulesSlice) > 0 {
			remainingRule := rulesSlice[0].(map[string]interface{})
			assert.Equal(t, "existing_rule", remainingRule["name"], "Expected 'existing_rule' to remain")
		}
	})
}

type stepReader struct {
	inputs []string
	index  int
	t      *testing.T
}

func (r *stepReader) Read(p []byte) (n int, err error) {
	if r.index >= len(r.inputs) {
		return 0, io.EOF
	}
	input := r.inputs[r.index]
	r.t.Logf("Providing input: %q", strings.TrimSpace(input))
	n = copy(p, input)
	r.index++
	return n, nil
}

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}
func executeCommandWithInput(cmd *cobra.Command, input *bytes.Buffer, args ...string) (string, error) {
	cmd.SetArgs(args)

	// Save the original stdin, stdout, and stderr
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	// Create pipes for stdin, stdout, and stderr
	inr, inw, _ := os.Pipe()
	outr, outw, _ := os.Pipe()
	errr, errw, _ := os.Pipe()

	os.Stdin = inr
	os.Stdout = outw
	os.Stderr = errw

	// Write the input to the pipe in a separate goroutine
	go func() {
		defer func(inw *os.File) {
			err := inw.Close()
			if err != nil {
				return
			}
		}(inw)
		_, err := input.WriteTo(inw)
		if err != nil {
			return
		}
	}()

	// Capture the output and error in separate goroutines
	output := &bytes.Buffer{}
	outputDone := make(chan bool)
	go func() {
		_, err := io.Copy(output, outr)
		if err != nil {
			return
		}
		outputDone <- true
	}()

	errorOutput := &bytes.Buffer{}
	errorDone := make(chan bool)
	go func() {
		_, err := io.Copy(errorOutput, errr)
		if err != nil {
			return
		}
		errorDone <- true
	}()

	// Execute the command
	err := cmd.Execute()

	// Close the write end of the pipes
	{
		err := outw.Close()
		if err != nil {
			return "", err
		}
	}
	{
		err := errw.Close()
		if err != nil {
			return "", err
		}
	}

	// Wait for the output and error to be fully read
	<-outputDone
	<-errorDone

	// Combine stdout and stderr
	combinedOutput := output.String() + errorOutput.String()

	return combinedOutput, err
}
