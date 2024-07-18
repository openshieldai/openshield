package rules

import (
	"encoding/json"
	"github.com/valyala/fasthttp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestInput(t *testing.T) {
	app := fiber.New()

	t.Run("English Input", func(t *testing.T) {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		requestBody := ChatRequest{
			Model: "gpt-4",
			Messages: []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "What is the meaning of life?"},
			},
		}

		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}

		result, err := Input(ctx, string(jsonBody))

		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
		if result != "What is the meaning of life?" {
			t.Errorf("Expected result 'What is the meaning of life?', but got %s", result)
		}
	})

	t.Run("Non-English Input", func(t *testing.T) {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		requestBody := ChatRequest{
			Model: "gpt-4",
			Messages: []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				{Role: "system", Content: "You are a helpful assistant."},
				{Role: "user", Content: "Ennek nem kéne angolnak lennie"},
			},
		}

		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}

		result, err := Input(ctx, string(jsonBody))

		if err == nil {
			t.Errorf("Expected an error, but got none")
		} else if !strings.Contains(err.Error(), "English probability too low") {
			t.Errorf("Expected error message to contain 'English probability too low', but got '%s'", err.Error())
		}
		if result != "" {
			t.Errorf("Expected empty result, but got %s", result)
		}
	})
}
