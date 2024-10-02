package lib

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"github.com/openshieldai/openshield/models"
)

func AuthOpenShieldMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		splitToken := strings.Split(authHeader, "Bearer ")
		if len(splitToken) != 2 {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		key := splitToken[1]

		apiKey := models.ApiKeys{ApiKey: key, Status: models.Active}
		result := DB().Where(&apiKey).First(&apiKey)
		if result.Error != nil {
			log.Println("Error: ", result.Error)
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		hashedAPIKey := sha256.Sum256([]byte(key))
		hashedKey := sha256.Sum256([]byte(apiKey.ApiKey))

		if subtle.ConstantTimeCompare(hashedAPIKey[:], hashedKey[:]) == 1 {
			// Store the API key ID in the request context
			ctx := r.Context()
			ctx = context.WithValue(ctx, "apiKeyId", apiKey.Id)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
		}
	}
}

//func AuthHeaderParser(c *fiber.Ctx) (string, error) {
//	authHeader := c.Get("Authorization")
//	if authHeader == "" {
//		return "", errors.New("missing Authorization header")
//	}
//
//	splitHeader := strings.Split(authHeader, "Bearer ")
//	if len(splitHeader) != 2 {
//		return "", errors.New("malformed Authorization header")
//	}
//
//	token := splitHeader[1]
//	if token == "" {
//		return "", errors.New("empty token in Authorization header")
//	}
//
//	return token, nil
//}
