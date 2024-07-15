package lib

import (
	"errors"
	"log"

	"github.com/openshieldai/openshield/models"
)

func GetApiKey(key string) (models.ApiKeys, error) {
	var apiKey = models.ApiKeys{ApiKey: key, Status: models.Active}
	result := DB().Find(&apiKey)
	if result.Error != nil {
		log.Println("Error: ", result.Error)
		return models.ApiKeys{}, result.Error
	}
	if apiKey.ApiKey == "" {
		log.Println("Error: apiKey is nil")
		return models.ApiKeys{}, errors.New("apiKey is nil")
	}
	return apiKey, nil
}

func GetModel(model string) (models.AiModels, error) {
	var aiModel = models.AiModels{Model: model}
	result := DB().Find(&aiModel)
	if result.Error != nil {
		log.Println("Error: ", result.Error)
		return models.AiModels{}, result.Error
	}
	return aiModel, nil
}
