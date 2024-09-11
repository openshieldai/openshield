package lib

import (
	"bytes"
	"encoding/json"
	"github.com/go-chi/chi/v5/middleware"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/openshieldai/openshield/models"
)

func AuditLogs(message string, logType string, apiKeyID uuid.UUID, messageType string, productID uuid.UUID, r *http.Request) *models.AuditLogs {
	config := GetConfig()

	if config.Settings.AuditLogging.Enabled {
		minifiedMessage, err := minifyJSON(message)
		if err != nil {
			log.Printf("Error minifying JSON: %v", err)
			return nil
		}

		ipAddress, err := KeyByRealIP(r)
		if err != nil {
			log.Printf("Error getting IP: %v", err)
			return nil
		}

		auditLog := models.AuditLogs{
			Message:     minifiedMessage,
			Type:        logType,
			MessageType: messageType,
			ApiKeyID:    apiKeyID,
			IPAddress:   ipAddress,
			RequestId:   getRequestID(r),
			ProductID:   productID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		db := DB()
		result := db.Create(&auditLog)
		if result.Error != nil {
			log.Printf("Error creating audit log: %v", result.Error)
			return nil
		}

		return &auditLog
	} else {
		log.Println("Audit log is disabled")
		return nil
	}
}

func minifyJSON(jsonStr string) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(jsonStr)); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func getRequestID(r *http.Request) string {
	requestID := middleware.GetReqID(r.Context())
	if requestID != "" {
		return requestID
	}
	return ""
}
