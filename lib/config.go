package lib

import (
	"fmt"
	"log"
	"os"
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
	Filters   Filters   `mapstructure:"filters"`
	Secrets   Secrets   `mapstructure:"secrets"`
	Providers Providers `mapstructure:"providers"`
}

// Providers section contains all the providers
type Providers struct {
	OpenAI      *FeatureToggle `mapstructure:"openai"`
	HuggingFace *FeatureToggle `mapstructure:"huggingface"`
}

// Secrets section contains all the secrets
type Secrets struct {
	OpenAIApiKey      string `mapstructure:"openai_api_key"`
	HuggingFaceAPIKey string `mapstructure:"huggingface_api_key"`
}

// Setting can include various configurations like database, cache, and different logging types
type Setting struct {
	Redis        *RedisConfig    `mapstructure:"redis"`
	Database     *DatabaseConfig `mapstructure:"database"`
	Cache        *CacheConfig    `mapstructure:"cache"`
	AuditLogging *FeatureToggle  `mapstructure:"audit_logging,default=false"`
	UsageLogging *FeatureToggle  `mapstructure:"usage_logging,default=false"`
	Network      *Network        `mapstructure:"network"`
	RateLimit    *RateLimiting   `mapstructure:"rate_limiting"`
}

type RateLimiting struct {
	*FeatureToggle
	Window     int `mapstructure:"window"`
	Max        int `mapstructure:"max"`
	Expiration int `mapstructure:"expiration"`
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
	AutoMigration bool   `mapstructure:"auto_migration,omitempty,default=false"`
}

// CacheConfig holds configuration for cache settings
type CacheConfig struct {
	RedisURI string `mapstructure:"redis_uri"`
	SSL      bool   `mapstructure:"ssl,default=false"`
	Enabled  bool   `mapstructure:"enabled,default=false"`
	TTL      int    `mapstructure:"ttl,default=60"`
}

// Filters section contains input and output filter configurations
type Filters struct {
	Input  []Filter `mapstructure:"input,default=[]"`
	Output []Filter `mapstructure:"output,default=[]"`
}

// Filter defines a filter configuration
type Filter struct {
	Enabled bool   `mapstructure:"enabled,default=false"`
	Name    string `mapstructure:"name"`
	Type    string `mapstructure:"type"`
	Config  Config `mapstructure:"config"`
	Action  Action `mapstructure:"action"`
}

// Config holds the configuration specifics of a filter
type Config struct {
	ModelName string `mapstructure:"model_name"`
	ModelURL  string `mapstructure:"model_url"`
	ModelType string `mapstructure:"model_type"`
	Threshold int    `mapstructure:"threshold,omitempty,default=0.5"`
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
	viperCfg.AddConfigPath(".")
	viperCfg.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	err := viperCfg.ReadInConfig()
	if err != nil {
		panic(err)
	}

	if viperCfg.Get("providers.openai.enabled") == true {
		if os.Getenv("OPENAI_API_KEY") == "" {
			log.Fatal("OPENAI_API_KEY Environment variable is not set")
		}
		viperCfg.Set("secrets.openai_api_key", os.Getenv("OPENAI_API_KEY"))
	}

	if viperCfg.Get("providers.huggingface.enabled") == true {
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

	err = viperCfg.Unmarshal(&AppConfig)
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
