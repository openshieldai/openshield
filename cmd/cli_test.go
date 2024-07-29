package cmd

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"regexp"
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
	createExpectations("ai_models", 1, 10)
	createExpectations("api_keys", 1, 7)
	createExpectations("audit_logs", 1, 11)
	createExpectations("products", 1, 7)
	createExpectations("usages", 1, 10)
	createExpectations("workspaces", 1, 6)
	lib.SetDB(db)
	createMockData()
	lib.DB()
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
	lib.SetDB(db)

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
	defer os.Remove(tmpfile.Name())

	err = os.Setenv("OPENSHIELD_CONFIG_FILE", tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer os.Unsetenv("OPENSHIELD_CONFIG_FILE")

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
	err = tmpfile.Close()
	if err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	viper.SetConfigFile(tmpfile.Name())
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Error reading config file: %v", err)
	}

	t.Run("AddRule", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = r
		defer func() {
			os.Stdin = oldStdin
			r.Close()
			w.Close()
		}()

		inputs := []string{
			"input\n",
			"new_rule\n",
			"sentiment_filter\n",
			"block\n",
			"sentiment_plugin\n",
			"90\n",
		}

		go func() {
			defer w.Close()
			for _, input := range inputs {
				_, err := w.Write([]byte(input))
				if err != nil {
					t.Logf("Error writing input: %v", err)
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()

		output, err := executeCommand(rootCmd, "config", "add-rule")
		if err != nil {
			t.Fatalf("Error executing add-rule command: %v", err)
		}

		t.Logf("Add Rule Command Output:\n%s", output)

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
			assert.Equal(t, int(90), newRule["config"].(map[string]interface{})["threshold"])
		}
	})

	t.Run("RemoveRule", func(t *testing.T) {
		oldStdin := os.Stdin
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stdin = r
		defer func() {
			os.Stdin = oldStdin
			r.Close()
			w.Close()
		}()

		input := "input\n2\n"
		go func() {
			defer w.Close()
			_, err := w.Write([]byte(input))
			if err != nil {
				t.Logf("Error writing input: %v", err)
				return
			}
		}()

		output, err := executeCommand(rootCmd, "config", "remove-rule")
		if err != nil {
			t.Fatalf("Error executing remove-rule command: %v", err)
		}

		t.Logf("Remove Rule Command Output:\n%s", output)

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

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	err := root.Execute()

	output := stdout.String() + stderr.String()
	return output, err
}
