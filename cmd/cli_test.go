package cmd

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/google/uuid"
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

	// Set up expectations for all tables
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
	// Create a new mock database connection
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
