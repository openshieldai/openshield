package lib

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/models"
)

func AuditLogs(message string, logType string, apiKeyID uuid.UUID, messageType string, c *fiber.Ctx) {
	config := GetConfig()

	if config.Settings.AuditLogging.Enabled {
		auditLog := models.AuditLogs{
			Message:     message,
			Type:        logType,
			MessageType: messageType,
			ApiKeyID:    apiKeyID,
			IPAddress:   c.IP(),
			RequestId:   c.Locals("requestid").(string),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		db := DB()
		db.Create(&auditLog)
	} else {
		log.Debug("Audit log is disabled")
		return
	}
}
