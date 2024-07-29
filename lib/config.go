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
	Rules     Rules     `mapstructure:"rules"`
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
	RuleServer   *RuleServer     `mapstructure:"rule_server"`
}

type RuleServer struct {
	Url string `mapstructure:"url,default=http://localhost:8000"`
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
	Enabled bool `mapstructure:"enabled,default=false"`
	TTL     int  `mapstructure:"ttl,default=60"`
}

// Rules section contains input and output rule configurations
type Rules struct {
	Input  []Rule `mapstructure:"input,default=[]"`
	Output []Rule `mapstructure:"output,default=[]"`
}

// Rule defines a rule configuration
type Rule struct {
	Enabled bool   `mapstructure:"enabled,default=false"`
	Name    string `mapstructure:"name"`
	Type    string `mapstructure:"type"`
	Config  Config `mapstructure:"config"`
	Action  Action `mapstructure:"action"`
}

// Config holds the configuration specifics of a filter
type Config struct {
	PluginName string      `mapstructure:"plugin_name"`
	Threshold  int         `mapstructure:"threshold,omitempty,default=0.5"`
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
	viperCfg.AddConfigPath("./config.yaml")
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
