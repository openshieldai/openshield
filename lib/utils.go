package lib

import (
	"log"

	"github.com/openshieldai/openshield/models"
)

func GetApiKey(key string) (models.ApiKeys, error) {
	var apiKey = models.ApiKeys{ApiKey: key, Status: "active"}
	result := DB().First(&apiKey)
	if result.Error != nil {
		log.Println("Error: ", result.Error)
		return models.ApiKeys{}, result.Error
	}
	return apiKey, nil
}
