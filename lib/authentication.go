package lib

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/keyauth"
)

func AuthOpenAIMiddleware() fiber.Handler {
	settings := NewSettings()

	return keyauth.New(keyauth.Config{
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			hashedAPIKey, _ := hex.DecodeString(settings.OpenAI.APIKeyHash)
			hashedKey := sha256.Sum256([]byte(key))

			if subtle.ConstantTimeCompare(hashedAPIKey[:], hashedKey[:]) == 1 {
				return true, nil
			}
			return false, keyauth.ErrMissingOrMalformedAPIKey
		},
	})
}

func AuthOpenShieldMiddleware() fiber.Handler {
	settings := NewSettings()

	return keyauth.New(keyauth.Config{
		Validator: func(c *fiber.Ctx, key string) (bool, error) {
			hashedAPIKey := sha256.Sum256([]byte(settings.OpenShield.APIKey))
			hashedKey := sha256.Sum256([]byte(key))

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
