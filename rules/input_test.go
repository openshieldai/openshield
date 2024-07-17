package rules

import (
	"encoding/json"
	"github.com/valyala/fasthttp"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestInput(t *testing.T) {
	app := fiber.New()

	t.Run("English Input", func(t *testing.T) {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.Set("Content-Type", "application/json")
		ctx.Request().Header.Set("Authorization", "Bearer ....")

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

		ctx.Request().SetBody(jsonBody)

		result, err := Input(ctx, "What is the meaning of life?")

		if err != nil {
			t.Errorf("Expected no error, but got: %v", err)
		}
		if result != "What is the meaning of life?" {
			t.Errorf("Expected result 'What is the meaning of life?', but got %s", result)
		}
	})

	t.Run("Hungarian Input", func(t *testing.T) {
		ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
		defer app.ReleaseCtx(ctx)

		ctx.Request().Header.Set("Content-Type", "application/json")
		ctx.Request().Header.Set("Authorization", "Bearer ....")

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

		ctx.Request().SetBody(jsonBody)

		result, err := Input(ctx, "Ennek nem kéne angolnak lennie")

		if err == nil {
			t.Errorf("Expected an error, but got none")
		}
		if result != "below 85%" {
			t.Errorf("Expected result 'below 85%%', but got %s", result)
		}
	})
}
