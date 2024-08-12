package lib

import (
	"crypto/tls"
	"github.com/redis/go-redis/v9"
)

type Settings struct {
	Log        Log
	OpenAI     openai
	OpenShield openShield
	Routes     Routes
	Database   Database
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
type Redis struct {
	Options *redis.Options
}

//func getEnvAsStatus(envVar string, defaultValue string) string {
//	value := os.Getenv(envVar)
//	switch value {
//	case "active":
//	case "inactive":
//		return value
//	}
//	return defaultValue
//}

type RouteSettings struct {
	RateLimit *RateLimiting
	Redis     Redis
}

func GetRouteSettings() (RouteSettings, error) {
	config := GetConfig()

	// Parse the Redis URL
	redisOptions, err := redis.ParseURL(config.Settings.Redis.URI)
	if err != nil {
		return RouteSettings{}, err
	}

	if config.Settings.Redis.SSL {
		redisOptions.TLSConfig = &tls.Config{
			MinVersion:       tls.VersionTLS12,
			CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}
	}

	return RouteSettings{
		RateLimit: &RateLimiting{
			Max:        config.Settings.RateLimit.Max,
			Window:     config.Settings.RateLimit.Window,
			Expiration: config.Settings.RateLimit.Expiration,
		},
		Redis: Redis{
			Options: redisOptions,
		},
	}, nil
}

//func NewSettings() Settings {
//	if os.Getenv("ENV") == "development" {
//		cwd, _ := os.Getwd()
//		path := fmt.Sprintf("%s/.env", cwd)
//		err := godotenv.Load(path)
//		if err != nil {
//			fmt.Println("Error loading .env file")
//		}
//	}
//	settingsRoutesOpenAIModelsRateLimitMax := getEnvAsInt("ROUTES_OPENAI_MODELS_MAX", 50)
//	settingsRoutesOpenAIModelRateLimitMax := getEnvAsInt("ROUTES_OPENAI_MODEL_MAX", 50)
//	settingsRoutesOpenAIModelRateLimitExpiration := getEnvAsInt("ROUTES_OPENAI_MODEL_EXPIRATION", 1)
//	settingsRoutesOpenAIModelsRateLimitExpiration := getEnvAsInt("ROUTES_OPENAI_MODELS_EXPIRATION", 1)
//	settingsRoutesOpenAIModelRateLimitTime := getEnvAsDuration("ROUTES_OPENAI_MODEL_TIME", time.Minute)
//	settingsRoutesOpenAIModelsRateLimitTime := getEnvAsDuration("ROUTES_OPENAI_MODELS_TIME", time.Minute)
//	settingsRoutesOpenAIChatCompletionsRateLimitMax := getEnvAsInt("ROUTES_OPENAI_CHAT_COMPLETIONS_MAX", 50)
//	settingsRoutesOpenAIChatCompletionsRateLimitExpiration := getEnvAsInt("ROUTES_OPENAI_CHAT_COMPLETIONS_EXPIRATION", 1)
//	settingsRoutesOpenAIChatCompletionsRateLimitTime := getEnvAsDuration("ROUTES_OPENAI_CHAT_COMPLETIONS_TIME", time.Minute)
//	settingsRoutesTokenizerRateLimitMax := getEnvAsInt("ROUTES_OPENAI_TOKENIZER_MAX", 50)
//	settingsRoutesTokenizerRateLimitExpiration := getEnvAsInt("ROUTES_OPENAI_TOKENIZER_EXPIRATION", 1)
//	settingsRoutesTokenizerRateLimitTime := getEnvAsDuration("ROUTES_OPENAI_TOKENIZER_TIME", time.Minute)
//
//	return Settings{
//		Routes: Routes{
//			OpenAI: OpenAIRoutes{
//				Model: Route{
//					RateLimitMax:        settingsRoutesOpenAIModelRateLimitMax,
//					RateLimitExpiration: settingsRoutesOpenAIModelRateLimitExpiration,
//					RateLimitTime:       settingsRoutesOpenAIModelRateLimitTime,
//				},
//				Models: Route{
//					RateLimitMax:        settingsRoutesOpenAIModelsRateLimitMax,
//					RateLimitExpiration: settingsRoutesOpenAIModelsRateLimitExpiration,
//					RateLimitTime:       settingsRoutesOpenAIModelsRateLimitTime,
//				},
//				ChatCompletions: Route{
//					RateLimitMax:        settingsRoutesOpenAIChatCompletionsRateLimitMax,
//					RateLimitExpiration: settingsRoutesOpenAIChatCompletionsRateLimitExpiration,
//					RateLimitTime:       settingsRoutesOpenAIChatCompletionsRateLimitTime,
//				},
//			},
//			Tokenizer: Route{
//				RateLimitMax:        settingsRoutesTokenizerRateLimitMax,
//				RateLimitExpiration: settingsRoutesTokenizerRateLimitExpiration,
//				RateLimitTime:       settingsRoutesTokenizerRateLimitTime,
//			},
//		},
//	}
//}
