package lib

import (
	"log"

	"github.com/openshieldai/openshield/models"
)

func GetApiKey(key string) (models.ApiKeys, error) {
	var apiKey = models.ApiKeys{ApiKey: key, Status: models.Active}
	result := DB().First(&apiKey)
	if result.Error != nil {
		log.Println("Error: ", result.Error)
		return models.ApiKeys{}, result.Error
	}
	return apiKey, nil
}

func GetModel(model string) (models.AiModels, error) {
	var aiModel = models.AiModels{Model: model}
	result := DB().First(&aiModel)
	if result.Error != nil {
		log.Println("Error: ", result.Error)
		return models.AiModels{}, result.Error
	}
	return aiModel, nil
}
