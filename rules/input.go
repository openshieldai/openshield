package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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
type LanguageScore struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

func Input(c *fiber.Ctx, userPrompt string) (string, error) {
	config := lib.GetConfig()
	var result string
	var chatRequest ChatRequest

	if err := json.Unmarshal([]byte(userPrompt), &chatRequest); err != nil {
		return "", fmt.Errorf("failed to unmarshal request: %v", err)
	}
	for input := range config.Rules.Input {
		inputConfig := config.Rules.Input[input]
		switch inputConfig.Type {
		case inputTypes.LanguageDetection:
			if inputConfig.Enabled {
				extractedPrompt := ""
				for _, message := range chatRequest.Messages {
					if message.Role == "user" {
						extractedPrompt = message.Content
						break
					}
				}

				if extractedPrompt == "" {
					return "", fmt.Errorf("no user message found in the request")
				}
				log.Printf("Extracted prompt: %s\n", extractedPrompt)
				englishScore, err := detectEnglish(extractedPrompt)
				if err != nil {
					return "", fmt.Errorf("language detection failed: %v", err)
				}

				log.Printf("English language probability: %.4f\n", englishScore)

				if englishScore <= 0.85 {
					return "", fmt.Errorf("English probability too low: %.4f", englishScore)
				}

				log.Printf("Language Detection: English probability above threshold (%.4f)\n", englishScore)
				return extractedPrompt, nil
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
func detectEnglish(text string) (float64, error) {
	apiKey := os.Getenv("HUGGINGFACE_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("HUGGINGFACE_API_KEY environment variable not set")
	}

	url := "https://api-inference.huggingface.co/models/papluca/xlm-roberta-base-language-detection"
	payload := map[string]string{"inputs": text}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %v", err)
	}

	var results [][]LanguageScore
	if err := json.Unmarshal(body, &results); err != nil {
		return 0, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(results) == 0 || len(results[0]) == 0 {
		return 0, fmt.Errorf("unexpected response format")
	}

	for _, score := range results[0] {
		if score.Label == "en" {
			return score.Score, nil
		}
	}

	return 0, nil // English not found in the response
}
