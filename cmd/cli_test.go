package cmd

import (
	"database/sql"
	"database/sql/driver"
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
	// Create a new mock database connection
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func(sqlDB *sql.DB) {
		err := sqlDB.Close()
		if err != nil {

		}
	}(sqlDB)

	// Create a new GORM DB instance with the mock database
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
