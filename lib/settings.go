package lib

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
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
	Log        log
	OpenAI     openai
	OpenShield openShield
	Routes     Routes
}

type log struct {
	DisableColor bool
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

type openShield struct {
	APIKey      string
	Port        int
	Environment string
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
	settingsLogDisableColor := getEnvAsBool("SETTINGS_LOG_DISABLE_COLORS", true)
	testingOpenAIApiKey := getEnvAsString("TESTING_OPENAI_API_KEY", "")
	settingsOpenAIApiKeyHash := getEnvAsString("SETTINGS_OPENAI_API_KEY_HASH", "")
	//settingsRoutesStorageMemcacheServers := getEnvAsString("SETTINGS_ROUTES_STORAGE_MEMCACHE_SERVERS", "localhost:11211")
	settingsRoutesOpenAIModelsRateLimitMax := getEnvAsInt("SETTINGS_ROUTES_OPENAI_MODELS_MAX", 50)
	settingsRoutesOpenAIModelRateLimitMax := getEnvAsInt("SETTINGS_ROUTES_OPENAI_MODEL_MAX", 50)
	settingsRoutesOpenAIModelRateLimitExpiration := getEnvAsInt("SETTINGS_ROUTES_OPENAI_MODEL_EXPIRATION", 1)
	settingsRoutesOpenAIModelsRateLimitExpiration := getEnvAsInt("SETTINGS_ROUTES_OPENAI_MODELS_EXPIRATION", 1)
	settingsRoutesOpenAIModelRateLimitTime := getEnvAsDuration("SETTINGS_ROUTES_OPENAI_MODEL_TIME", time.Minute)
	settingsRoutesOpenAIModelsRateLimitTime := getEnvAsDuration("SETTINGS_ROUTES_OPENAI_MODELS_TIME", time.Minute)
	settingsRoutesOpenAIChatCompletionsRateLimitMax := getEnvAsInt("SETTINGS_ROUTES_OPENAI_CHAT_COMPLETIONS_MAX", 50)
	settingsRoutesOpenAIChatCompletionsRateLimitExpiration := getEnvAsInt("SETTINGS_ROUTES_OPENAI_CHAT_COMPLETIONS_EXPIRATION", 1)
	settingsRoutesOpenAIChatCompletionsRateLimitTime := getEnvAsDuration("SETTINGS_ROUTES_OPENAI_CHAT_COMPLETIONS_TIME", time.Minute)
	settingsRoutesTokenizerRateLimitMax := getEnvAsInt("SETTINGS_ROUTES_OPENAI_TOKENIZER_MAX", 50)
	settingsRoutesTokenizerRateLimitExpiration := getEnvAsInt("SETTINGS_ROUTES_OPENAI_TOKENIZER_EXPIRATION", 1)
	settingsRoutesTokenizerRateLimitTime := getEnvAsDuration("SETTINGS_ROUTES_OPENAI_TOKENIZER_TIME", time.Minute)
	settingsOpenShieldApiKey := getEnvAsString("SETTINGS_OPENSHIELD_API_KEY", "")
	if settingsOpenShieldApiKey == "" {
		panic("SETTINGS_OPENSHIELD_API_KEY is required")
	}
	settingsRoutesStorageRedisURL := getEnvAsString("SETTINGS_ROUTES_STORAGE_REDIS_URL", "redis://localhost:6379")
	settingsRoutesStorageRedisTLS := getEnvAsBool("SETTINGS_ROUTES_STORAGE_REDIS_TLS", false)
	var redisTlsCfg *tls.Config
	if settingsRoutesStorageRedisTLS {
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

	if env == "testing" {
		if testingOpenAIApiKey == "" {
			fmt.Println("TESTING_OPENAI_API_KEY is required")
			os.Exit(1)
		}
	}

	if settingsOpenAIApiKeyHash == "" {
		fmt.Println("SETTINGS_OPENAI_API_KEY_HASH is required")
		os.Exit(1)
	}

	keyGenerator := func(c *fiber.Ctx) string {
		key, err := AuthHeaderParser(c)
		if err != nil {
			return ""
		}
		hashedKey := sha256.Sum256([]byte(key))
		hashedKeyString := hex.EncodeToString(hashedKey[:])
		return hashedKeyString
	}

	return Settings{
		Log: log{
			DisableColor: settingsLogDisableColor,
		},
		OpenAI: openai{
			APIKey:     testingOpenAIApiKey,
			APIKeyHash: settingsOpenAIApiKeyHash,
		},
		OpenShield: openShield{
			APIKey:      settingsOpenShieldApiKey,
			Port:        settingsOpenShieldPort,
			Environment: env,
		},
		Routes: Routes{
			Storage: redis.New(
				redis.Config{
					URL:       settingsRoutesStorageRedisURL,
					PoolSize:  10 * runtime.GOMAXPROCS(0),
					Reset:     false,
					TLSConfig: redisTlsCfg,
				}),
			KeyGenerator: keyGenerator,
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
