package lib

import (
	"github.com/gofiber/fiber/v2/log"
	"github.com/openshieldai/openshield/models"
)

func Usage(modelName string, predictedTokensCount int, promptTokensCount int, completionTokens int, totalTokens int, finishReason string, requestType string) {
	settings := NewSettings()

	if settings.Log.Usage {
		aiModel, err := GetModel(modelName)
		if err != nil {
			log.Error("Error: ", err)
			return
		}

		costs := models.Usage{
			ModelID:              aiModel.Id,
			PredictedTokensCount: predictedTokensCount,
			PromptTokensCount:    promptTokensCount,
			CompletionTokens:     completionTokens,
			TotalTokens:          totalTokens,
			FinishReason:         models.FinishReason(finishReason),
			RequestType:          requestType,
		}
		db := DB()
		db.Create(&costs)
	} else {
		log.Debug("Cost logs is disabled")
		return
	}

}
