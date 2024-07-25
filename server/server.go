package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/openshieldai/openshield/lib/openai"
	"golang.org/x/sync/errgroup"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/openshieldai/openshield/lib"
)

var (
	app    *fiber.App
	config lib.Configuration
)

func StartServer() error {
	config = lib.GetConfig()

	app = fiber.New(fiber.Config{
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
	//setupOpenShieldRoutes(app)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Start the server
	g.Go(func() error {
		addr := fmt.Sprintf(":%d", config.Settings.Network.Port)
		fmt.Printf("Server is starting on %s...\n", addr)
		return app.Listen(addr)
	})

	// Handle graceful shutdown
	g.Go(func() error {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

		select {
		case <-quit:
			fmt.Println("Shutting down server...")
			return app.Shutdown()
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	if err := g.Wait(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		return err
	}

	return nil
}

func StopServer() error {
	if app != nil {
		fmt.Println("Stopping the server...")
		return app.Shutdown()
	}
	return fmt.Errorf("server is not running")
}
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
	config := lib.GetRouteSettings()
	routes := map[string]lib.RouteSettings{
		"/openai/v1/models":           config,
		"/openai/v1/models/:model":    config,
		"/openai/v1/chat/completions": config,
	}

	for path, routeSettings := range routes {
		setupRoute(app, path, routeSettings)
	}

	app.Get("/openai/v1/models", lib.AuthOpenShieldMiddleware(), openai.ListModelsHandler)
	app.Get("/openai/v1/models/:model", lib.AuthOpenShieldMiddleware(), openai.GetModelHandler)
	app.Post("/openai/v1/chat/completions", lib.AuthOpenShieldMiddleware(), openai.ChatCompletionHandler)
}

//func setupOpenShieldRoutes(app *fiber.App) {
//  config := lib.GetConfig()
//  routes := map[string]lib.Route{
//     "/tokenizer/:model": settings.Routes.Tokenizer,
//  }
//
//  for path := range routes {
//     setupRoute(app, path, lib.GetRouteSettings())
//  }
//
//  app.Post("/tokenizer/:model", lib.AuthOpenShieldMiddleware(), lib.TokenizerHandler)
//}
