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

func AuditLogs(message string, logType string, apiKeyID uuid.UUID, messageType string, r *http.Request) {
	config := GetConfig()

	if config.Settings.AuditLogging.Enabled {
		minifiedMessage, err := minifyJSON(message)
		if err != nil {
			log.Printf("Error minifying JSON: %v", err)
			return
		}

		auditLog := models.AuditLogs{
			Message:     minifiedMessage,
			Type:        logType,
			MessageType: messageType,
			ApiKeyID:    apiKeyID,
			IPAddress:   getIPAddress(r),
			RequestId:   getRequestID(r),
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

func getIPAddress(r *http.Request) string {
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
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
