package rules

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
	"github.com/pemistahl/lingua-go"
	"log"
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

type ChatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func Input(c *fiber.Ctx, userPrompt string) (string, error) {
	config := lib.GetConfig()
	var result string

	for input := range config.Rules.Input {
		inputConfig := config.Rules.Input[input]
		switch inputConfig.Type {
		case inputTypes.LanguageDetection:
			if inputConfig.Enabled {
				languageMap := make(map[string]lingua.Language)

				for i := lingua.Afrikaans; i < lingua.Unknown; i++ {
					languageMap[i.String()] = i
				}

				var languages []lingua.Language
				for _, lang := range inputConfig.Config.Languages {
					if l, ok := languageMap[lang]; ok {
						languages = append(languages, l)
					} else {
						log.Printf("WARNING: Unsupported language in config: %s\n", lang)
					}
				}
				detector := lingua.NewLanguageDetectorBuilder().
					FromLanguages(languages...).
					Build()

				englishConfidence := detector.ComputeLanguageConfidence(userPrompt, lingua.English)

				result := "above 85%"
				if englishConfidence <= 0.85 {
					result = "below 85%"
					return result, fmt.Errorf("English probability too low: %.2f", englishConfidence)
				}
				log.Printf("Language Detection: English probability %s (%.2f)\n", result, englishConfidence)
				return userPrompt, nil
			}

		case inputTypes.PromptInjection:
			if inputConfig.Enabled {
				//agent := fiber.Post(inputConfig.Config.ModelURL+inputConfig.Config.ModelName).Set("Authorization", "Bearer "+config.Secrets.HuggingFaceAPIKey)
				//
				//log.Printf("User Prompt: %s\n", userPrompt)
				//agent.Body([]byte(userPrompt)) // set body received by request
				//statusCode, body, errs := agent.Bytes()
				//if len(errs) > 0 {
				//	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				//		"errs": errs,
				//	})
				//}
				//
				//log.Println("Prompt Injection")
				//log.Printf("Status Code: %d\n", statusCode)
				log.Printf("User Prompt: %s\n", userPrompt)

				// pass status code and body received by the proxy
				result = userPrompt
			}
		case inputTypes.PIIFilter:
			if inputConfig.Enabled {
				log.Println("PII Filter")
			}
		default:
			log.Printf("ERROR: Invalid input filter type %s\n", inputConfig.Type)
		}
	}
	return result, nil
}
