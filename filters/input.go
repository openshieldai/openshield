package filters

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
)

type InputTypes struct {
	LanguageDetection string
	PromptInjection   string
	PIIFilter         string
}

var inputTypes = InputTypes{
	LanguageDetection: "language_detection",
	PromptInjection:   "prompt_injection",
	PIIFilter:         "pii_filter",
}

func Input(c *fiber.Ctx, userPrompt string) error {
	config := lib.GetConfig()

	for input := range config.Filters.Input {
		inputConfig := config.Filters.Input[input]
		switch inputConfig.Type {
		case inputTypes.LanguageDetection:
			if inputConfig.Enabled {
				log.Println("Language Detection")
			}
		case inputTypes.PromptInjection:
			if inputConfig.Enabled {
				agent := fiber.Post(inputConfig.Config.ModelURL+inputConfig.Config.ModelName).Set("Authorization", "Bearer "+config.Secrets.HuggingFaceAPIKey)

				log.Printf("User Prompt: %s\n", userPrompt)
				agent.Body([]byte(userPrompt)) // set body received by request
				statusCode, body, errs := agent.Bytes()
				if len(errs) > 0 {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"errs": errs,
					})
				}

				log.Println("Prompt Injection")
				log.Printf("Status Code: %d\n", statusCode)

				// pass status code and body received by the proxy
				return c.Status(statusCode).Send(body)
			}
		case inputTypes.PIIFilter:
			if inputConfig.Enabled {
				log.Println("PII Filter")
			}
		default:
			log.Printf("ERROR: Invalid input filter type %s\n", inputConfig.Type)
		}
	}
	return nil
}
