package lib

import (
	"github.com/google/uuid"
	"github.com/openshieldai/openshield/models"
	"log"
)

func LogUsage(modelName string, predictedTokensCount int, promptTokensCount int, completionTokens int, totalTokens int, finishReason string, requestType string, productID uuid.UUID, auditLogID uuid.UUID) {
	config := GetConfig()

	if config.Settings.UsageLogging.Enabled {
		aiModel, err := GetModel(modelName)
		if err != nil {
			log.Printf("Error: %v", err)
			return
		}

		usage := models.Usage{
			ModelID:              aiModel.Id,
			PredictedTokensCount: predictedTokensCount,
			PromptTokensCount:    promptTokensCount,
			CompletionTokens:     completionTokens,
			TotalTokens:          totalTokens,
			FinishReason:         models.FinishReason(finishReason),
			RequestType:          requestType,
			ProductID:            productID,
			AuditLogID:           auditLogID,
		}
		db := DB()
		db.Create(&usage)
	} else {
		log.Printf("Usage logs is disabled")
		return
	}
}
