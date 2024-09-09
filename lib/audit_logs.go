package lib

import (
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/openshieldai/openshield/models"
)

func AuditLogs(message string, logType string, apiKeyID uuid.UUID, messageType string, productID uuid.UUID, r *http.Request) {
	config := GetConfig()

	if config.Settings.AuditLogging.Enabled {
		auditLog := models.AuditLogs{
			Message:     message,
			Type:        logType,
			MessageType: messageType,
			ApiKeyID:    apiKeyID,
			IPAddress:   getIPAddress(r),
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

func getRequestID(r *http.Request) string {
	requestID := r.Context().Value("requestid")
	if requestID != nil {
		return requestID.(string)
	}
	return ""
}
