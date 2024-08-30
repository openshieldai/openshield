package lib

import (
	"github.com/redis/go-redis/v9"
)

type Settings struct {
	Log        Log
	OpenAI     openai
	OpenShield openShield
	Routes     Routes
	Database   Database
	Redis      RedisSettings
}

type Log struct {
	DisableColor bool
	AuditLog     bool
	Usage        bool
}

type openai struct {
	APIKey     string
	APIKeyHash string
}

type Route struct {
	RateLimit RateLimiting
}

type OpenAIRoutes struct {
	Models          Route
	Model           Route
	ChatCompletions Route
}

type Routes struct {
	OpenAI    OpenAIRoutes
	Tokenizer Route
	Storage   *redis.Client
}

type openShield struct {
	APIKey      string
	Port        int
	Environment string
}

type Database struct {
	URL           string
	AutoMigration bool
}

type RedisSettings struct {
	URI string
	SSL bool
}

type Redis struct {
	Options *redis.Options
}

type RouteSettings struct {
	RateLimit *RateLimiting
	Redis     Redis
}

func GetRouteSettings() (RouteSettings, error) {
	config := GetConfig()

	return RouteSettings{
		RateLimit: &RateLimiting{
			Max:    config.Settings.RateLimit.Max,
			Window: config.Settings.RateLimit.Window,
		},
	}, nil
}
