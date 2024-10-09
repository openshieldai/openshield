package lib

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// FeatureToggle is used to enable or disable features
type FeatureToggle struct {
	Enabled bool `mapstructure:"enabled,default=false"`
}

// Configuration Represents the entire YAML configuration
type Configuration struct {
	Settings  Setting   `mapstructure:"settings"`
	Rules     Rules     `mapstructure:"rules"`
	Secrets   Secrets   `mapstructure:"secrets"`
	Providers Providers `mapstructure:"providers"`
}

type Providers struct {
	OpenAI    *ProviderOpenAI    `mapstructure:"openai"`
	Anthropic *ProviderAnthropic `mapstructure:"anthropic"`
	Nvidia    *ProviderNvidia    `mapstructure:"nvidia"`
}

type ProviderOpenAI struct {
	Enabled bool   `mapstructure:"enabled"`
	BaseUrl string `mapstructure:"url"`
}

type ProviderAnthropic struct {
	Enabled bool   `mapstructure:"enabled"`
	BaseUrl string `mapstructure:"url"`
}
type ProviderNvidia struct {
	Enabled bool   `mapstructure:"enabled"`
	BaseUrl string `mapstructure:"url"`
}

// Secrets section contains all the secrets
type Secrets struct {
	OpenAIApiKey      string `mapstructure:"openai_api_key"`
	HuggingFaceAPIKey string `mapstructure:"huggingface_api_key"`
	AnthropicApiKey   string `mapstructure:"anthropic_api_key"`
	NvidiaApiKey      string `mapstructure:"nvidia_api_key"`
}

// Setting can include various configurations like database, cache, and different logging types
type Setting struct {
	Redis               *RedisConfig    `mapstructure:"redis"`
	Database            *DatabaseConfig `mapstructure:"database"`
	Cache               *CacheConfig    `mapstructure:"cache"`
	ContextCache        *ContextCache   `mapstructure:"context_cache"`
	AuditLogging        *FeatureToggle  `mapstructure:"audit_logging,default=false"`
	UsageLogging        *FeatureToggle  `mapstructure:"usage_logging,default=false"`
	Network             *Network        `mapstructure:"network"`
	RateLimit           *RateLimiting   `mapstructure:"rate_limiting"`
	RuleServer          *RuleServer     `mapstructure:"rule_server"`
	EnglishDetectionURL string          `mapstructure:"english_detection_url"`
}

// ContextCache holds configuration for the context cache
type ContextCache struct {
	Enabled bool   `mapstructure:"enabled,default=false"`
	URL     string `mapstructure:"url,default=http://localhost:8001"`
}

// RuleServer holds configuration for the rule server
type RuleServer struct {
	Url string `mapstructure:"url,default=http://localhost:8000"`
}

// RateLimiting holds configuration for rate limiting settings
type RateLimiting struct {
	*FeatureToggle
	Window int `mapstructure:"window"`
	Max    int `mapstructure:"max"`
}

// RedisConfig holds configuration for the redis cache
type RedisConfig struct {
	URI string `mapstructure:"uri"`
	SSL bool   `mapstructure:"ssl,default=false"`
}

// Network holds configuration for network settings
type Network struct {
	Port int `mapstructure:"port,default=8080"`
}

// DatabaseConfig holds configuration for the database
type DatabaseConfig struct {
	URI           string `mapstructure:"uri"`
	AutoMigration bool   `mapstructure:"auto_migration,default=false"`
}

// CacheConfig holds configuration for cache settings
type CacheConfig struct {
	Enabled bool   `mapstructure:"enabled,default=false"`
	TTL     int    `mapstructure:"ttl,default=60"`
	Prefix  string `mapstructure:"prefix,default=openshield"`
}

// Rules section contains input and output rule configurations
type Rules struct {
	Input  []Rule `mapstructure:"input,default=[]"`
	Output []Rule `mapstructure:"output,default=[]"`
}

// Rule defines a rule configuration
type Rule struct {
	Enabled     bool   `mapstructure:"enabled,default=false"`
	Name        string `mapstructure:"name"`
	Type        string `mapstructure:"type"`
	Config      Config `mapstructure:"config"`
	Action      Action `mapstructure:"action"`
	OrderNumber int    `mapstructure:"order_number"`
}

// Config holds the configuration specifics of a filter
type Config struct {
	PluginName string      `mapstructure:"plugin_name"`
	Threshold  float64     `mapstructure:"threshold,omitempty,default=0.5"`
	Url        string      `mapstructure:"url,omitempty"`
	ApiKey     string      `mapstructure:"api_key,omitempty"`
	PIIService interface{} `mapstructure:"piiservice,omitempty"`
}

type ActionType string

// Action defines what actions are associated with filters
type Action struct {
	Type ActionType `mapstructure:"type"`
}

var AppConfig Configuration

func init() {
	viperCfg := viper.New()

	viperCfg.SetConfigName("config")
	viperCfg.SetConfigType("yaml")

	if strings.ToLower(os.Getenv("NONVIPER_CONFIG")) != "true" {
		configDir, err := findConfigPath()
		if err != nil {
			print(os.Getenv("NONVIPER_CONFIG"))
			panic(err)
		}

		viperCfg.AddConfigPath(configDir)
	}
	viperCfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viperCfg.SetDefault("providers.openai.base_url", "https://api.openai.com/v1")
	if strings.ToLower(os.Getenv("NONVIPER_CONFIG")) != "true" {
		err := viperCfg.ReadInConfig()
		if err != nil {
			panic(err)
		}
	}

	if viperCfg.Get("providers.openai.enabled") == true && os.Getenv("ENV") != "test" {
		if os.Getenv("OPENAI_API_KEY") == "" {
			log.Fatal("OPENAI_API_KEY Environment variable is not set")
		}
		viperCfg.Set("secrets.openai_api_key", os.Getenv("OPENAI_API_KEY"))
	}

	if viperCfg.Get("providers.huggingface.enabled") == true && os.Getenv("ENV") != "test" {
		if os.Getenv("HUGGINGFACE_API_KEY") == "" {
			log.Fatal("HUGGINGFACE_API_KEY Environment variable is not set")
		}
		viperCfg.Set("secrets.huggingface_api_key", os.Getenv("HUGGINGFACE_API_KEY"))
	}

	if viperCfg.Get("settings.cache.enabled") == true || viperCfg.Get("settings.rate_limiting.enabled") == true {
		if viperCfg.Get("settings.redis.uri") == "" || viperCfg.Get("settings.redis.uri") == nil {
			log.Fatal("settings.redis.uri is not set")
		}
	}

	err := viperCfg.Unmarshal(&AppConfig)
	if err != nil {
		panic(err)
	}

	viperCfg.WatchConfig()
	viperCfg.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		if err = viperCfg.Unmarshal(&AppConfig); err != nil {
			fmt.Println(err)
		}
	})

}

func GetConfig() Configuration {
	return AppConfig
}
func SetConfig(config Configuration) {
	AppConfig = config
}
func findConfigPath() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(currentDir, "config.yaml")); err == nil {
			return currentDir, nil
		}

		// Move up to the parent directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// We've reached the root of the file system without finding go.mod
			return "", fmt.Errorf("unable to find path root (no config.yaml found)")
		}
		currentDir = parentDir
	}
}
