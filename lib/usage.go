package lib

import (
	"github.com/gofiber/fiber/v2/log"
	"github.com/openshieldai/openshield/models"
)

func Usage(modelName string, predictedTokensCount int, promptTokensCount int, completionTokens int, totalTokens int, finishReason string, requestType string) {
	config := GetConfig()

	if config.Settings.UsageLogging.Enabled {
		aiModel, err := GetModel(modelName)
		if err != nil {
			log.Error("Error: ", err)
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
		}
		db := DB()
		db.Create(&usage)
	} else {
		log.Debug("Usage logs is disabled")
		return
	}

}
