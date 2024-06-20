package lib

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/keyauth"
	"github.com/openshieldai/openshield/models"
)

func AuthOpenShieldMiddleware() fiber.Handler {

	return keyauth.New(keyauth.Config{
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			var apiKey = models.ApiKeys{ApiKey: key, Status: "active"}
			result := DB().First(&apiKey)
			if result.Error != nil {
				log.Println("Error: ", result.Error)
				return false, keyauth.ErrMissingOrMalformedAPIKey
			}

			hashedAPIKey := sha256.Sum256([]byte(key))
			hashedKey := sha256.Sum256([]byte(apiKey.ApiKey))

			if subtle.ConstantTimeCompare(hashedAPIKey[:], hashedKey[:]) == 1 {
				return true, nil
			}
			return false, keyauth.ErrMissingOrMalformedAPIKey
		},
	})
}

func AuthHeaderParser(c *fiber.Ctx) (string, error) {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("missing Authorization header")
	}

	splitHeader := strings.Split(authHeader, "Bearer ")
	if len(splitHeader) != 2 {
		return "", errors.New("malformed Authorization header")
	}

	token := splitHeader[1]
	if token == "" {
		return "", errors.New("empty token in Authorization header")
	}

	return token, nil
}
