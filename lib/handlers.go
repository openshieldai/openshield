package lib

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/tiktoken-go/tokenizer"
)

func TokenizerHandler(c *fiber.Ctx) error {

	modelID := c.Params("model")
	body := string(c.Body())
	var model tokenizer.Model
	var price float64
	switch {
	case strings.Contains(modelID, "davinci"):
		model = "davinci"
		price = 0.0000020
	case strings.Contains(modelID, "babbage"):
		model = "babbage"
		price = 0.0000004
	case strings.Contains(modelID, "ada"):
		model = "ada"
		price = 0.00000010
	case strings.Contains(modelID, "tts"):
		switch {
		case strings.Contains(modelID, "tts-hd"):
			model = "tts-hd"
			price = 0.00003000
		case strings.Contains(modelID, "tts"):
			model = "tts"
			price = 0.00001500
		}
	case strings.Contains(modelID, "gpt-3.5"):
		switch {
		case strings.Contains(modelID, "gpt-3.5-turbo-1106"):
			model = "gpt-3.5-turbo"
			price = 0.0000010
		case strings.Contains(modelID, "gpt-3.5-turbo-0613"):
			model = "gpt-3.5-turbo"
			price = 0.0000015
		case strings.Contains(modelID, "gpt-3.5-turbo-16k-0613"):
			model = "gpt-3.5-turbo"
			price = 0.0000030
		case strings.Contains(modelID, "gpt-3.5-turbo-0301"):
			model = "gpt-3.5-turbo"
			price = 0.0000015
		case strings.Contains(modelID, "gpt-3.5-turbo-instruct"):
			model = "gpt-3.5-turbo"
			price = 0.0000015
		case strings.Contains(modelID, "gpt-3.5-turbo-0125"):
			model = "gpt-3.5-turbo"
			price = 0.0000005
		default:
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid model",
			})
		}
	case strings.Contains(modelID, "gpt-4"):
		switch {
		case strings.Contains(modelID, "gpt-4o"):
			model = "gpt-4o"
			price = 0.0000020
		case strings.Contains(modelID, "gpt-4o-2024-05-13"):
			model = "gpt-4o"
			price = 0.0000020
		case strings.Contains(modelID, "gpt-4-turbo"):
			model = "gpt-4-turbo"
			price = 0.0000020
		case strings.Contains(modelID, "gpt-4-turbo-2024-04-09"):
			model = "gpt-4-turbo"
			price = 0.0000020
		case strings.Contains(modelID, "gpt-4"):
			model = "gpt-4"
			price = 0.0000020
		case strings.Contains(modelID, "gpt-4-32k"):
			model = "gpt-4"
			price = 0.0000020
		case strings.Contains(modelID, "gpt-4-0125-preview"):
			model = "gpt-4"
			price = 0.0000020
		case strings.Contains(modelID, "gpt-4-1106-preview"):
			model = "gpt-4"
			price = 0.0000020
		case strings.Contains(modelID, "gpt-4-vision-preview"):
			model = "gpt-4"
			price = 0.0000020
		default:
			return c.Status(400).JSON(fiber.Map{
				"error": "Invalid model",
			})
		}
	default:
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid model",
		})
	}

	enc, err := tokenizer.ForModel(model)
	if err != nil {
		fmt.Printf("Error: %v\n", err.Error())
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	ids, _, _ := enc.Encode(body)
	return c.JSON(fiber.Map{
		"model":    modelID,
		"prompts":  body,
		"tokens":   len(ids),
		"currency": "USD",
		"price":    price * float64(len(ids)),
	})
}
