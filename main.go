package main

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/openshieldai/openshield/lib"
	"github.com/openshieldai/openshield/lib/openai"
)

func setupRoute(app *fiber.App, path string, routesSettings lib.RouteSettings, keyGenerator ...func(c *fiber.Ctx) string) {
	config := limiter.Config{
		Max:        routesSettings.RateLimit.Max,
		Expiration: time.Duration(routesSettings.RateLimit.Expiration) * time.Second * time.Duration(routesSettings.RateLimit.Window),
		Storage:    routesSettings.Storage,
	}

	if len(keyGenerator) > 0 {
		config.KeyGenerator = keyGenerator[0]
	}

	app.Use(path, limiter.New(config))
}

func setupOpenAIRoutes(app *fiber.App) {
	settings := lib.NewSettings()
	routes := map[string]lib.Route{
		"/openai/v1/models":           settings.Routes.OpenAI.Models,
		"/openai/v1/models/:model":    settings.Routes.OpenAI.Model,
		"/openai/v1/chat/completions": settings.Routes.OpenAI.ChatCompletions,
	}

	for path := range routes {
		setupRoute(app, path, lib.GetRouteSettings())
	}

	app.Get("/openai/v1/models", lib.AuthOpenShieldMiddleware(), openai.ListModelsHandler)
	app.Get("/openai/v1/models/:model", lib.AuthOpenShieldMiddleware(), openai.GetModelHandler)
	app.Post("/openai/v1/chat/completions", lib.AuthOpenShieldMiddleware(), openai.ChatCompletionHandler)
}

func setupOpenShieldRoutes(app *fiber.App) {
	settings := lib.NewSettings()
	routes := map[string]lib.Route{
		"/tokenizer/:model": settings.Routes.Tokenizer,
	}

	for path := range routes {
		setupRoute(app, path, lib.GetRouteSettings())
	}

	app.Post("/tokenizer/:model", lib.AuthOpenShieldMiddleware(), lib.TokenizerHandler)
}

func main() {
	config := lib.GetConfig()

	app := fiber.New(fiber.Config{
		Prefork:           false,
		CaseSensitive:     false,
		StrictRouting:     true,
		StreamRequestBody: true,
		ServerHeader:      "openshield",
		AppName:           "OpenShield",
	})
	app.Use(requestid.New())
	app.Use(logger.New())

	app.Use(logger.New(logger.Config{
		Format: "${pid} ${locals:requestid} ${status} - ${method} ${path}\n",
	}))

	app.Use(func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		c.Set("Accept", "application/json")
		return c.Next()
	})

	setupOpenAIRoutes(app)
	setupOpenShieldRoutes(app)

	err := app.Listen(":" + strconv.Itoa(config.Settings.Network.Port))
	if err != nil {
		panic(err.Error())
	}
}
