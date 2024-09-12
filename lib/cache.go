package lib

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func InitRedisClient(config *Configuration) {
	var redisTlsCfg *tls.Config
	if config.Settings.Redis.SSL {
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

	opt, err := redis.ParseURL(config.Settings.Redis.URI)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}

	opt.TLSConfig = redisTlsCfg

	redisClient = redis.NewClient(opt)
}

func GetCache(key string) ([]byte, bool, error) {
	config := GetConfig()

	if redisClient == nil {
		InitRedisClient(&config)
	}

	if config.Settings.Cache.Enabled {
		cachePrefix := config.Settings.Cache.Prefix
		hashedKey := cachePrefix + ":" + hashKey(key)

		ctx := context.Background()
		value, err := redisClient.Get(ctx, hashedKey).Bytes()
		if errors.Is(err, redis.Nil) {
			log.Println("Cache miss")
			return nil, false, nil
		} else if err != nil {
			return nil, false, err
		}

		log.Printf("Cache hit: %s", hashedKey)
		return value, true, nil
	} else {
		return nil, false, nil
	}
}

func SetCache(key string, value interface{}) error {
	config := GetConfig()

	if redisClient == nil {
		InitRedisClient(&config)
	}

	if config.Settings.Cache.Enabled {
		cachePrefix := config.Settings.Cache.Prefix
		hashedKey := cachePrefix + ":" + hashKey(key)

		// Convert value to JSON if it's not already a []byte
		var jsonValue []byte
		var err error
		switch v := value.(type) {
		case []byte:
			jsonValue = v
		default:
			jsonValue, err = json.Marshal(v)
			if err != nil {
				return err
			}
		}

		ctx := context.Background()
		err = redisClient.Set(ctx, hashedKey, jsonValue, time.Duration(config.Settings.Cache.TTL)*time.Second).Err()
		if err != nil {
			return err
		}

		return nil
	} else {
		return nil
	}
}

func hashKey(key string) string {
	return strconv.FormatUint(xxhash.Sum64([]byte(key)), 10)
}
