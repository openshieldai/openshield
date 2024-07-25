package lib

import (
	"crypto/tls"
	"log"
	"runtime"
	"strconv"
	"time"

	hash "github.com/cespare/xxhash/v2"
	"github.com/gofiber/storage/redis/v3"
)

func getRedisConfig(config *Configuration) redis.Config {
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

	return redis.Config{
		URL:       config.Settings.Redis.URI,
		PoolSize:  10 * runtime.GOMAXPROCS(0),
		Reset:     false,
		TLSConfig: redisTlsCfg,
	}
}

func GetCache(key string) ([]byte, bool, error) {
	config := GetConfig()
	storage := redis.New(getRedisConfig(&config))

	if config.Settings.Cache.Enabled {
		hashed := hash.Sum64([]byte(key))

		value, err := storage.Get(strconv.FormatUint(hashed, 10))
		if err != nil {
			return nil, false, err
		}
		if value == nil {
			log.Println("Cache miss")
			return nil, false, nil
		}
		log.Printf("Cache hit: %s", string(value))
		return value, true, nil
	} else {
		log.Println("Cache is disabled")
		return nil, false, nil
	}
}

func SetCache(key string, value []byte) ([]byte, error) {
	config := GetConfig()
	storage := redis.New(getRedisConfig(&config))

	if config.Settings.Cache.Enabled {
		hashedKey := strconv.FormatUint(hash.Sum64([]byte(key)), 10)
		err := storage.Set(hashedKey, value, time.Duration(config.Settings.Cache.TTL)*time.Second)
		if err != nil {
			return nil, err // Return the error instead of panicking
		}
		return value, nil // Return the original value to indicate success
	} else {
		log.Println("Cache is disabled")
		return value, nil // Caching is disabled, return the original value
	}
}
