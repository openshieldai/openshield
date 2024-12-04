package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/openshieldai/openshield/lib/huggingface"

	"github.com/openshieldai/openshield/lib/anthropic"
	"github.com/openshieldai/openshield/lib/nvidia"
	"github.com/openshieldai/openshield/lib/openai"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	_ "github.com/openshieldai/openshield/docs"
	"github.com/openshieldai/openshield/lib"
	"github.com/redis/go-redis/v9"
	"github.com/swaggo/http-swagger"
	"golang.org/x/sync/errgroup"
)

var (
	router chi.Router
	config lib.Configuration
)

type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   string `json:"param"`
		Code    string `json:"code"`
	} `json:"error"`
}

func StartServer() error {
	config = lib.GetConfig()

	router = chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second))

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	setupOpenAIRoutes(router)
	router.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		addr := fmt.Sprintf(":%d", config.Settings.Network.Port)
		fmt.Printf("Server is starting on %s...\n", addr)
		return http.ListenAndServe(addr, router)
	})

	if err := g.Wait(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		return err
	}

	return nil
}

func setupOpenAIRoutes(r chi.Router) {
	routeSettings, _ := lib.GetRouteSettings()
	routes := map[string]lib.RouteSettings{
		"/openai/v1/models":                routeSettings,
		"/openai/v1/models/{model}":        routeSettings,
		"/openai/v1/chat/completions":      routeSettings,
		"/openai/v1/threads":               routeSettings,
		"/openai/v1/threads/{thread_id}":   routeSettings,
		"/anthropic/v1/messages":           routeSettings,
		"/huggingface/v1/chat/completions": routeSettings,
	}

	for _, routeSettings := range routes {
		setupRoute(r, routeSettings)
	}
	r.Route("/openai/v1", func(r chi.Router) {
		r.Get("/models", lib.AuthOpenShieldMiddleware(openai.ListModelsHandler))
		r.Get("/models/{model}", lib.AuthOpenShieldMiddleware(openai.GetModelHandler))
		r.Post("/chat/completions", lib.AuthOpenShieldMiddleware(openai.ChatCompletionHandler))
		r.Post("/threads", lib.AuthOpenShieldMiddleware(openai.CreateThreadHandler))
		r.Get("/threads/{thread_id}", lib.AuthOpenShieldMiddleware(openai.GetThreadHandler))
		r.Post("/threads/{thread_id}", lib.AuthOpenShieldMiddleware(openai.ModifyThreadHandler))
		r.Delete("/threads/{thread_id}", lib.AuthOpenShieldMiddleware(openai.DeleteThreadHandler))
		r.Post("/threads/{thread_id}/messages", lib.AuthOpenShieldMiddleware(openai.CreateMessageHandler))
		r.Get("/threads/{thread_id}/messages", lib.AuthOpenShieldMiddleware(openai.ListMessagesHandler))
		r.Get("/threads/{thread_id}/messages/{message_id}", lib.AuthOpenShieldMiddleware(openai.RetrieveMessageHandler))
		r.Post("/threads/{thread_id}/messages/{message_id}", lib.AuthOpenShieldMiddleware(openai.ModifyMessageHandler))
		r.Delete("/threads/{thread_id}/messages/{message_id}", lib.AuthOpenShieldMiddleware(openai.DeleteMessageHandler))
		r.Post("/assistants", lib.AuthOpenShieldMiddleware(openai.CreateAssistantHandler))
		r.Get("/assistants", lib.AuthOpenShieldMiddleware(openai.ListAssistantsHandler))
		r.Get("/assistants/{assistant_id}", lib.AuthOpenShieldMiddleware(openai.RetrieveAssistantHandler))
		r.Post("/assistants/{assistant_id}", lib.AuthOpenShieldMiddleware(openai.ModifyAssistantHandler))
		r.Delete("/assistants/{assistant_id}", lib.AuthOpenShieldMiddleware(openai.DeleteAssistantHandler))

		// Run routes
		r.Post("/threads/{thread_id}/runs", lib.AuthOpenShieldMiddleware(openai.CreateRunHandler))
		r.Get("/threads/{thread_id}/runs", lib.AuthOpenShieldMiddleware(openai.ListRunsHandler))
		r.Get("/threads/{thread_id}/runs/{run_id}", lib.AuthOpenShieldMiddleware(openai.RetrieveRunHandler))
		r.Post("/threads/{thread_id}/runs/{run_id}", lib.AuthOpenShieldMiddleware(openai.ModifyRunHandler))
		r.Post("/threads/{thread_id}/runs/{run_id}/cancel", lib.AuthOpenShieldMiddleware(openai.CancelRunHandler))
		r.Post("/threads/{thread_id}/runs/{run_id}/submit_tool_outputs", lib.AuthOpenShieldMiddleware(openai.SubmitToolOutputsHandler))
		r.Post("/threads/runs", lib.AuthOpenShieldMiddleware(openai.CreateThreadAndRunHandler))

		// Run Step routes
		r.Get("/threads/{thread_id}/runs/{run_id}/steps", lib.AuthOpenShieldMiddleware(openai.ListRunStepsHandler))
		r.Get("/threads/{thread_id}/runs/{run_id}/steps/{step_id}", lib.AuthOpenShieldMiddleware(openai.RetrieveRunStepHandler))
	})
	r.Route("/anthropic/v1", func(r chi.Router) {
		r.Post("/messages", lib.AuthOpenShieldMiddleware(anthropic.CreateMessageHandler))
	})
	r.Route("/nvidia/v1", func(r chi.Router) {
		r.Post("/chat/completions", lib.AuthOpenShieldMiddleware(nvidia.ChatCompletionHandler))
	})
	r.Route("/huggingface/v1", func(r chi.Router) {
		r.Post("/chat/completions", lib.AuthOpenShieldMiddleware(huggingface.ChatCompletionHandler))
	})
}

var redisClient *redis.Client

func setupRoute(r chi.Router, routeSettings lib.RouteSettings) {

	config := lib.GetConfig()

	if redisClient == nil {
		lib.InitRedisClient(&config)
	}

	r.Use(httprate.Limit(
		routeSettings.RateLimit.Max,
		time.Duration(routeSettings.RateLimit.Window),
		lib.WithKeyByRealIP(),
		httprateredis.WithRedisLimitCounter(&httprateredis.Config{
			Client: redisClient,
		}),
	))
}
