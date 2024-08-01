package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/lib"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

var (
	mock sqlmock.Sqlmock
	db   *gorm.DB
)

func setupTestConfig() {
	log.Printf("Test configuration set up: %+v", lib.AppConfig)
}

func setupMockDB() (*gorm.DB, sqlmock.Sqlmock, error) {
	var err error
	var sqlDB *sql.DB

	sqlDB, mock, err = sqlmock.New()
	if err != nil {
		return nil, nil, err
	}

	dialector := postgres.New(postgres.Config{
		Conn: sqlDB,
	})

	db, err = gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}

	return db, mock, nil
}

func TestMain(m *testing.M) {
	var err error

	// Set up test configuration
	setupTestConfig()

	// Set up mock database
	db, mock, err = setupMockDB()
	if err != nil {
		panic(err)
	}

	lib.SetDB(db)

	// Run tests
	code := m.Run()

	// Exit
	os.Exit(code)
}

func TestChatCompletion(t *testing.T) {
	setupTestConfig()
	APIKey := lib.AppConfig.Secrets.OpenAIApiKey
	if APIKey == "" {
		t.Skip("Skipping testing against production OpenAI API. Set OPENAI_TOKEN environment variable to enable it.")
	}
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer log.SetOutput(nil)

	db, mock, err := setupMockDB()
	if err != nil {
		t.Fatalf("Failed to set up mock database: %v", err)
	}
	lib.SetDB(db)

	app := setupTestServer()
	if app == nil {
		t.Fatal("setupTestServer returned nil")
	}

	// Generate a test API key ID
	apiKeyID := uuid.New()

	// Mock the database query for API key validation
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "api_keys" WHERE ("api_keys"."api_key" = $1 AND "api_keys"."status" = $2) AND "api_keys"."deleted_at" IS NULL ORDER BY "api_keys"."id" LIMIT $3`)).
		WithArgs(APIKey, "active", 1).
		WillReturnRows(mock.NewRows([]string{"id", "api_key", "status", "active"}).
			AddRow(apiKeyID, APIKey, "active", true))

	// Mock the database operations for input audit log
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "audit_logs" ("request_id","deleted_at","api_key_id","ip_address","message","message_type","type","metadata","created_at","updated_at") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING "id","created_at","updated_at"`)).
		WithArgs(sqlmock.AnyArg(), nil, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "input", "openai_chat_completion", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(uuid.New(), time.Now(), time.Now()))
	mock.ExpectCommit()

	// Mock the database operations for output audit log
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "audit_logs" ("request_id","deleted_at","api_key_id","ip_address","message","message_type","type","metadata","created_at","updated_at") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING "id","created_at","updated_at"`)).
		WithArgs(sqlmock.AnyArg(), nil, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "output", "openai_chat_completion", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(uuid.New(), time.Now(), time.Now()))
	mock.ExpectCommit()

	// Create the request body
	reqBody := bytes.NewBufferString(`{"model":"gpt-4","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"What is the meaning of life?"}]}`)

	// Create the request
	req, _ := http.NewRequest("POST", "/openai/v1/chat/completions", reqBody)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", APIKey))
	req.Header.Set("Content-Type", "application/json")
	requestID := uuid.New().String()
	req.Header.Set("X-Request-ID", requestID)

	// Perform the request
	resp, err := app.Test(req, 30000)
	if err != nil {
		t.Fatalf("app.Test returned an error: %v", err)
	}

	// Print captured logs
	t.Logf("Captured logs:\n%s", logBuffer.String())

	// Check for errors in the logs
	if strings.Contains(logBuffer.String(), "Error in AuditLogs") {
		t.Fatalf("Errors found in AuditLogs: %s", logBuffer.String())
	}

	// Print out the full response details
	t.Logf("Response status: %d", resp.StatusCode)
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	t.Logf("Response body: %s", string(bodyBytes))

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d. Response body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse and assert the response body
	var result map[string]interface{}
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		t.Fatalf("Failed to decode response body: %v. Body: %s", err, bodyBytes)
	}

	// Assert the response structure
	assert.Contains(t, result, "id", "Response should contain an 'id' field")
	assert.Contains(t, result, "object", "Response should contain an 'object' field")
	assert.Contains(t, result, "created", "Response should contain a 'created' field")
	assert.Contains(t, result, "model", "Response should contain a 'model' field")
	assert.Contains(t, result, "choices", "Response should contain a 'choices' field")

	// Ensure all database expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}
func setupTestServer() *fiber.App {
	app := fiber.New(fiber.Config{
		Prefork:           false,
		CaseSensitive:     false,
		StrictRouting:     true,
		StreamRequestBody: true,
		ServerHeader:      "openshield",
		AppName:           "OpenShield",
	})

	app.Use(func(c *fiber.Ctx) error {
		// Ensure apiKeyId is set in the context
		if c.Locals("apiKeyId") == nil {
			c.Locals("apiKeyId", uuid.New())
		}
		// Ensure requestid is set in the context
		if c.Locals("requestid") == nil {
			c.Locals("requestid", c.Get("X-Request-ID", uuid.New().String()))
		}
		log.Printf("Context set up: apiKeyId=%v, requestid=%v", c.Locals("apiKeyId"), c.Locals("requestid"))
		return c.Next()
	})

	setupOpenAIRoutes(app)

	return app
}
