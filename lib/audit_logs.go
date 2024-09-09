package lib

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/openshieldai/openshield/models"
)

func AuditLogs(message string, logType string, apiKeyID uuid.UUID, messageType string, productID uuid.UUID, r *http.Request) {
	config := GetConfig()

	if config.Settings.AuditLogging.Enabled {
		minifiedMessage, err := minifyJSON(message)
		if err != nil {
			log.Printf("Error minifying JSON: %v", err)
			return
		}

		ipAddress, err := KeyByRealIP(r)
		if err != nil {
			log.Printf("Error getting IP: %v", err)
			return
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
		db.Create(&auditLog)
	} else {
		log.Println("Audit log is disabled")
		return
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
	requestID := r.Context().Value("requestid")
	if requestID != nil {
		return requestID.(string)
	}
	return ""
}
