package lib

import (
	"crypto/tls"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/storage/redis"
	"github.com/joho/godotenv"
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
	RateLimitMax        int
	RateLimitExpiration int
	RateLimitTime       time.Duration
}

type OpenAIRoutes struct {
	Models          Route
	Model           Route
	ChatCompletions Route
}

type Routes struct {
	OpenAI       OpenAIRoutes
	Tokenizer    Route
	Storage      *redis.Storage
	KeyGenerator func(c *fiber.Ctx) string
}

type pIIService struct {
	URL    string
	Status bool
}

type openShield struct {
	APIKey      string
	Port        int
	Environment string
	PIIService  pIIService
}

type Database struct {
	URL           string
	AutoMigration bool
}

func getEnvAsInt(envVar string, defaultValue int) int {
	value, err := strconv.Atoi(os.Getenv(envVar))
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(envVar string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(envVar)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
func getEnvAsString(envVar string, defaultValue string) string {
	value := os.Getenv(envVar)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsBool(envVar string, defaultValue bool) bool {
	value, err := strconv.ParseBool(os.Getenv(envVar))
	if err != nil {
		return defaultValue
	}
	return value
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

func NewSettings() Settings {
	if os.Getenv("ENV") == "development" {
		cwd, _ := os.Getwd()
		path := fmt.Sprintf("%s/.env", cwd)
		err := godotenv.Load(path)
		if err != nil {
			fmt.Println("Error loading .env file")
		}
	}
	env := getEnvAsString("ENV", "production")
	settingsOpenShieldPort := getEnvAsInt("PORT", 3005)
	settingsLogDisableColor := getEnvAsBool("LOG_DISABLE_COLORS", true)
	openAIApiKey := getEnvAsString("OPENAI_API_KEY", "")
	settingsRoutesOpenAIModelsRateLimitMax := getEnvAsInt("ROUTES_OPENAI_MODELS_MAX", 50)
	settingsRoutesOpenAIModelRateLimitMax := getEnvAsInt("ROUTES_OPENAI_MODEL_MAX", 50)
	settingsRoutesOpenAIModelRateLimitExpiration := getEnvAsInt("ROUTES_OPENAI_MODEL_EXPIRATION", 1)
	settingsRoutesOpenAIModelsRateLimitExpiration := getEnvAsInt("ROUTES_OPENAI_MODELS_EXPIRATION", 1)
	settingsRoutesOpenAIModelRateLimitTime := getEnvAsDuration("ROUTES_OPENAI_MODEL_TIME", time.Minute)
	settingsRoutesOpenAIModelsRateLimitTime := getEnvAsDuration("ROUTES_OPENAI_MODELS_TIME", time.Minute)
	settingsRoutesOpenAIChatCompletionsRateLimitMax := getEnvAsInt("ROUTES_OPENAI_CHAT_COMPLETIONS_MAX", 50)
	settingsRoutesOpenAIChatCompletionsRateLimitExpiration := getEnvAsInt("ROUTES_OPENAI_CHAT_COMPLETIONS_EXPIRATION", 1)
	settingsRoutesOpenAIChatCompletionsRateLimitTime := getEnvAsDuration("ROUTES_OPENAI_CHAT_COMPLETIONS_TIME", time.Minute)
	settingsRoutesTokenizerRateLimitMax := getEnvAsInt("ROUTES_OPENAI_TOKENIZER_MAX", 50)
	settingsRoutesTokenizerRateLimitExpiration := getEnvAsInt("ROUTES_OPENAI_TOKENIZER_EXPIRATION", 1)
	settingsRoutesTokenizerRateLimitTime := getEnvAsDuration("ROUTES_OPENAI_TOKENIZER_TIME", time.Minute)
	settingsRateLimitRedisURL := getEnvAsString("RATE_LIMIT_REDIS_URL", "redis://localhost:6379")
	settingsRateLimitRedisTLS := getEnvAsBool("RATE_LIMIT_REDIS_TLS", false)
	var redisTlsCfg *tls.Config
	if settingsRateLimitRedisTLS {
		redisTlsCfg = &tls.Config{
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
	settingsDatabaseURL := getEnvAsString("DATABASE_URL", "")
	if settingsDatabaseURL == "" {
		fmt.Println("DATABASE_URL is required")
		os.Exit(1)
	}
	settingsAutoMigration := getEnvAsBool("AUTO_MIGRATION", false)

	if openAIApiKey == "" {
		fmt.Println("OPENAI_API_KEY is required")
		os.Exit(1)
	}

	settingsAuditLog := getEnvAsBool("AUDIT_LOG_ENABLED", false)
	settingsOpenShieldPIIServiceURL := getEnvAsString("OPENSHIELD_PII_SERVICE_URL", "")
	settingsOpenShieldPIIServiceStatus := getEnvAsBool("OPENSHIELD_PII_SERVICE_ENABLED", false)
	if settingsOpenShieldPIIServiceURL == "" && settingsOpenShieldPIIServiceStatus {
		fmt.Println("OPENSHIELD_PII_SERVICE_URL is required")
		os.Exit(1)
	}

	settingsUsage := getEnvAsBool("USAGE_ENABLED", false)

	return Settings{
		Database: Database{
			URL:           settingsDatabaseURL,
			AutoMigration: settingsAutoMigration,
		},
		Log: Log{
			DisableColor: settingsLogDisableColor,
			AuditLog:     settingsAuditLog,
			Usage:        settingsUsage,
		},
		OpenAI: openai{
			APIKey: openAIApiKey,
		},
		OpenShield: openShield{
			Port:        settingsOpenShieldPort,
			Environment: env,
			PIIService: pIIService{
				URL:    settingsOpenShieldPIIServiceURL,
				Status: settingsOpenShieldPIIServiceStatus,
			},
		},
		Routes: Routes{
			Storage: redis.New(
				redis.Config{
					URL:       settingsRateLimitRedisURL,
					PoolSize:  10 * runtime.GOMAXPROCS(0),
					Reset:     false,
					TLSConfig: redisTlsCfg,
				}),
			OpenAI: OpenAIRoutes{

				Model: Route{
					RateLimitMax:        settingsRoutesOpenAIModelRateLimitMax,
					RateLimitExpiration: settingsRoutesOpenAIModelRateLimitExpiration,
					RateLimitTime:       settingsRoutesOpenAIModelRateLimitTime,
				},
				Models: Route{
					RateLimitMax:        settingsRoutesOpenAIModelsRateLimitMax,
					RateLimitExpiration: settingsRoutesOpenAIModelsRateLimitExpiration,
					RateLimitTime:       settingsRoutesOpenAIModelsRateLimitTime,
				},
				ChatCompletions: Route{
					RateLimitMax:        settingsRoutesOpenAIChatCompletionsRateLimitMax,
					RateLimitExpiration: settingsRoutesOpenAIChatCompletionsRateLimitExpiration,
					RateLimitTime:       settingsRoutesOpenAIChatCompletionsRateLimitTime,
				},
			},
			Tokenizer: Route{
				RateLimitMax:        settingsRoutesTokenizerRateLimitMax,
				RateLimitExpiration: settingsRoutesTokenizerRateLimitExpiration,
				RateLimitTime:       settingsRoutesTokenizerRateLimitTime,
			},
		},
	}
}
