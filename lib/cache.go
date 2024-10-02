package lib

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/redis/go-redis/v9"
)

var (
    redisClient *redis.Client
    once        sync.Once
)

func InitRedisClient(config *Configuration) {
    once.Do(func() {
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
    })
}

func GetCache(key string) ([]byte, bool, error) {
    config := GetConfig()
    InitRedisClient(&config)

	if redisClient == nil {
		InitRedisClient(&config)
	}

	if config.Settings.Cache.Enabled {
		cachePrefix := config.Settings.Cache.Prefix
		hashedKey := cachePrefix + ":" + HashKey(key)

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
    InitRedisClient(&config)

	if redisClient == nil {
		InitRedisClient(&config)
	}

	if config.Settings.Cache.Enabled {
		cachePrefix := config.Settings.Cache.Prefix
		hashedKey := cachePrefix + ":" + HashKey(key)

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

func HashKey(key string) string {
	return strconv.FormatUint(xxhash.Sum64([]byte(key)), 10)
}

func GetContextCache(prompt string, productID string) (string, error) {
	config := GetConfig()
	cacheURL := config.Settings.ContextCache.URL
	requestBody, err := json.Marshal(map[string]string{
		"product_id": productID,
		"prompt":     prompt,
	})
	if err != nil {
		log.Printf("Error marshaling request body: %v", err)
		return "", nil
	}

	resp, err := http.Post(cacheURL+"/get", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error sending request to cache: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return "", err
	}
	log.Printf("Cache response: %s", string(body))

	if resp.StatusCode == http.StatusNotFound {
		log.Printf("Cache miss for prompt: %s, product_id: %s", prompt, productID)
		return string(body), nil
	} else if resp.StatusCode != http.StatusOK {
		log.Printf("Something went wrong with cache: %s", resp.Status)
		return string(body), nil
	}

	log.Printf("Cache hit for prompt: %s, product_id: %s", prompt, productID)
	return string(body), errors.New("cache hit")
}

func SetContextCache(prompt, answer, productID string) error {
	config := GetConfig()
	cacheURL := config.Settings.ContextCache.URL

	requestBody, err := json.Marshal(map[string]string{
		"product_id": productID,
		"prompt":     prompt,
		"answer":     answer,
	})
	if err != nil {
		log.Printf("Error marshaling request body: %v", err)
		return err
	}

	resp, err := http.Post(cacheURL+"/put", "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error sending request to cache: %v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Error putting to cache. Status: %d, Body: %s", resp.StatusCode, string(body))
		return fmt.Errorf("failed to put to cache: %s", resp.Status)
	}

	log.Printf("Successfully put to cache. Prompt: %s, ProductID: %s", prompt, productID)
	return nil
}
