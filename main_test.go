package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
	"github.com/stretchr/testify/assert"
)

func setupApp() *fiber.App {

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		c.Set("Accept", "application/json")
		c.Set("Server", "openshield")
		return c.Next()
	})

	setupOpenAIRoutes(app)
	setupOpenShieldRoutes(app)

	return app
}

//func TestClean(t *testing.T) {
//	settings := lib.NewSettings()
//	hashedAPIKey, _ := hex.DecodeString(settings.OpenAI.APIKeyHash)
//	store := redis.New(redis.Config{
//		Host:  "127.0.0.1",
//		Port:  6379,
//		Reset: true,
//	})
//	err := store.Delete(fmt.Sprintf("%x", sha256.Sum256(hashedAPIKey)))
//	if err != nil {
//		return
//	}
//}

func TestAuth(t *testing.T) {
	app := setupApp() // Assuming you have a setupApp function that sets up your fiber app

	req, _ := http.NewRequest("GET", "/openai/v1/models/gpt-4", nil)
	resp, _ := app.Test(req)

	assert.Equal(t, 401, resp.StatusCode, "Expected status code 401")
	//TestClean(t)
}

func TestListModels(t *testing.T) {
	app := setupApp()
	settings := lib.NewSettings()
	//TestClean(t)

	req, _ := http.NewRequest("GET", "/openai/v1/models", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set(
		"Authorization",
		fmt.Sprintf("Bearer %s", settings.OpenAI.APIKey),
	)
	resp, _ := app.Test(req)

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
}

func TestGetModel(t *testing.T) {
	app := setupApp()
	settings := lib.NewSettings()
	//TestClean(t)

	req, _ := http.NewRequest("GET", "/openai/v1/models/gpt-4", nil)
	req.Header.Set(
		"Authorization",
		fmt.Sprintf("Bearer %s", settings.OpenAI.APIKey),
	)
	resp, _ := app.Test(req)

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
}

func TestTokenizer(t *testing.T) {
	app := setupApp()
	settings := lib.NewSettings()
	//TestClean(t)

	reqBody := bytes.NewBuffer([]byte("thisateststringfortokenizer"))
	req, _ := http.NewRequest("POST", "/tokenizer/gpt-3.5", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(
		"Authorization",
		fmt.Sprintf("Bearer %s", settings.OpenShield.APIKey),
	)
	resp, _ := app.Test(req)
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyString := string(bodyBytes)

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
	assert.Equal(t, `{"model":"gpt-3.5","prompts":"thisateststringfortokenizer","tokens":6}`, bodyString, `Expected {"model":"gpt-3.5","prompts":"thisateststringfortokenizer","tokens":6}}`)
}

func TestChatCompletion(t *testing.T) {
	app := setupApp()
	if app == nil {
		t.Fatal("setupApp returned nil")
	}

	settings := lib.NewSettings()

	//TestClean(t)

	reqBody := bytes.NewBuffer([]byte(`{"model":"gpt-4","messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"What is the meaning of life?"}]}`))
	req, err := http.NewRequest("POST", "/openai/v1/chat/completions", reqBody)
	if err != nil {
		t.Fatalf("http.NewRequest returned an error: %v", err)
	}

	req.Header.Set(
		"Authorization",
		fmt.Sprintf("Bearer %s", settings.OpenAI.APIKey),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, 10000)
	if err != nil {
		t.Fatalf("app.Test returned an error: %v", err)
	}

	assert.Equal(t, 200, resp.StatusCode, "Expected status code 200")
}
