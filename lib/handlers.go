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
	if strings.Contains(modelID, "davinci") {
		model = tokenizer.Davinci
	}
	if strings.Contains(modelID, "curie") {
		model = tokenizer.Curie
	}
	if strings.Contains(modelID, "babbage") {
		model = tokenizer.Babbage
	}
	if strings.Contains(modelID, "ada") {
		model = tokenizer.Ada
	}
	if strings.Contains(modelID, "gpt-3.5") {
		model = tokenizer.GPT35Turbo
	}
	if strings.Contains(modelID, "gpt-4") {
		model = tokenizer.GPT4
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
		"model":   modelID,
		"prompts": body,
		"tokens":  len(ids),
	})
}
