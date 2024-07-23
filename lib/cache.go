package lib

import (
	"log"
	"strconv"
	"time"

	hash "github.com/cespare/xxhash/v2"
	"github.com/gofiber/storage/redis"
)

func GetCache(key string) ([]byte, bool, error) {
	storage := redis.New()
	config := GetConfig()

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
	storage := redis.New()
	config := GetConfig()

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
