package lib

import (
	"log"

	"github.com/openshieldai/openshield/models"
)

func GetModel(model string) (models.AiModels, error) {
	var aiModel = models.AiModels{Model: model}
	result := DB().Find(&aiModel)
	if result.Error != nil {
		log.Println("Error: ", result.Error)
		return models.AiModels{}, result.Error
	}
	return aiModel, nil
}
