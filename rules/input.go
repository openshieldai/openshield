package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
	"github.com/sashabaranov/go-openai"
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

type Rule struct {
	Prompt openai.ChatCompletionRequest `json:"prompt"`
	Config lib.Config                   `json:"config"`
}

type RuleInspection struct {
	CheckResult       bool    `json:"check_result"`
	Score             float64 `json:"score"`
	AnonymizedContent string  `json:"anonymized_content"`
}

type RuleResult struct {
	Match      bool           `json:"match"`
	Inspection RuleInspection `json:"inspection"`
}

type LanguageScore struct {
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

var inputTypes = InputTypes{
	LanguageDetection: "language_detection",
	PromptInjection:   "prompt_injection",
	PIIFilter:         "pii_filter",
}

func Input(_ *fiber.Ctx, userPrompt openai.ChatCompletionRequest) (bool, string, error) {
	config := lib.GetConfig()

	log.Println("Starting Input function")

	for input := range config.Rules.Input {
		inputConfig := config.Rules.Input[input]
		log.Printf("Processing input rule: %s", inputConfig.Type)

		switch inputConfig.Type {
		case inputTypes.LanguageDetection:
			if inputConfig.Enabled {
				log.Println("Language Detection enabled")
				extractedPrompt := ""
				for _, message := range userPrompt.Messages {
					if message.Role == "user" {
						extractedPrompt = message.Content
						break
					}
				}
				if extractedPrompt == "" {
					log.Println("No user message found in the request")
					return true, "No user message found in the request", fmt.Errorf("no user message found in the request")
				}
				log.Printf("Extracted prompt for language detection: %s", extractedPrompt)
				englishScore, err := detectEnglish(extractedPrompt)
				if err != nil {
					log.Printf("Language detection failed: %v", err)
					return true, fmt.Sprintf("Language detection failed: %v", err), err
				}
				log.Printf("English language probability: %.4f", englishScore)
				if englishScore <= 0.85 {
					log.Printf("English probability too low: %.4f", englishScore)
					return true, fmt.Sprintf("English probability too low: %.4f", englishScore), fmt.Errorf("English probability too low: %.4f", englishScore)
				}
				log.Printf("Language Detection: English probability above threshold (%.4f)", englishScore)
			}

		case inputTypes.PIIFilter:
			if inputConfig.Enabled {
				log.Println("PII Filter enabled")
				extractedPrompt := ""
				var userMessageIndex int
				for i, message := range userPrompt.Messages {
					if message.Role == "user" {
						extractedPrompt = message.Content
						userMessageIndex = i
						break
					}
				}
				if extractedPrompt == "" {
					log.Println("No user message found in the request")
					return true, "No user message found in the request", fmt.Errorf("no user message found in the request")
				}

				data := Rule{
					Prompt: userPrompt,
					Config: inputConfig.Config,
				}

				jsonData, err := json.Marshal(data)
				log.Printf("Request being sent to Python endpoint for PII:\n%s", string(jsonData))
				if err != nil {
					log.Printf("Failed to marshal PII request: %v", err)
					return true, fmt.Sprintf("Failed to marshal PII request: %v", err), err
				}

				agent := fiber.Post(config.Settings.RuleServer.Url + "/rule/execute")
				agent.Body(jsonData)
				agent.Set("Content-Type", "application/json")
				_, body, _ := agent.Bytes()
				log.Printf("Response received from Python endpoint for PII:\n%s", string(body))

				var piiResult RuleResult
				if err := json.Unmarshal(body, &piiResult); err != nil {
					log.Printf("Failed to decode PII filter response: %v", err)
					return true, fmt.Sprintf("Failed to decode PII filter response: %v", err), err
				}

				log.Printf("PII detection result: %+v", piiResult)

				if piiResult.Inspection.CheckResult {
					log.Println("PII detected, anonymizing content")
					userPrompt.Messages[userMessageIndex].Content = piiResult.Inspection.AnonymizedContent

					if inputConfig.Action.Type == "block" {
						log.Println("Blocking request due to PII detection.")
						return true, "request blocked due to PII detection", nil
					} else if inputConfig.Action.Type == "monitoring" {
						log.Println("Monitoring request due to PII detection.")
						// Continue processing
					}
				} else {
					log.Println("No PII detected")
				}
			}

		case inputTypes.PromptInjection:
			if inputConfig.Enabled {
				log.Println("Prompt Injection check enabled")
				agent := fiber.Post(config.Settings.RuleServer.Url + "/rule/execute")
				data := Rule{
					Prompt: userPrompt,
					Config: inputConfig.Config,
				}
				jsonify, err := json.Marshal(data)
				if err != nil {
					log.Printf("Failed to marshal prompt injection request: %v", err)
					return true, fmt.Sprintf("Failed to marshal prompt injection request: %v", err), err
				}

				log.Printf("Request being sent to Python endpoint for Prompt Injection:\n%s", string(jsonify))

				agent.Body(jsonify)
				agent.Set("Content-Type", "application/json")
				_, body, _ := agent.Bytes()

				log.Printf("Response received from Python endpoint for Prompt Injection:\n%s", string(body))

				var rule RuleResult
				err = json.Unmarshal(body, &rule)
				if err != nil {
					log.Printf("Failed to decode prompt injection response: %v", err)
					return true, fmt.Sprintf("Failed to decode prompt injection response: %v", err), err
				}

				log.Printf("Prompt Injection detection result: Match=%v, Score=%f", rule.Match, rule.Inspection.Score)

				if rule.Match {
					if inputConfig.Action.Type == "block" {
						log.Println("Blocking request due to prompt injection detection.")
						return true, "request blocked due to rule match", nil
					} else if inputConfig.Action.Type == "monitoring" {
						log.Println("Monitoring request due to prompt injection detection.")
						// Continue processing
					}
				} else {
					log.Println("Prompt Injection Rule Not Matched")
				}
			}

		default:
			log.Printf("ERROR: Invalid input filter type %s", inputConfig.Type)
		}
	}

	log.Println("Final result: No rules matched, request is not blocked")
	return false, "request is not blocked", nil
}

func detectEnglish(text string) (float64, error) {
	log.Println("Starting detectEnglish function")
	config := lib.GetConfig()
	apiKey := os.Getenv("HUGGINGFACE_API_KEY")
	if apiKey == "" {
		log.Println("HUGGINGFACE_API_KEY environment variable not set")
		return 0, fmt.Errorf("HUGGINGFACE_API_KEY environment variable not set")
	}

	url := config.Settings.EnglishDetectionURL
	if url == "" {
		log.Println("English detection URL not set in configuration")
		return 0, fmt.Errorf("English detection URL not set in configuration")
	}

	payload := map[string]string{"inputs": text}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal payload: %v", err)
		return 0, fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return 0, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send request: %v", err)
		return 0, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return 0, fmt.Errorf("failed to read response body: %v", err)
	}

	log.Printf("Response from Hugging Face API: %s", string(body))

	var results [][]LanguageScore
	if err := json.Unmarshal(body, &results); err != nil {
		log.Printf("Failed to unmarshal response: %v", err)
		return 0, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if len(results) == 0 || len(results[0]) == 0 {
		log.Println("Unexpected response format")
		return 0, fmt.Errorf("unexpected response format")
	}

	for _, score := range results[0] {
		if score.Label == "en" {
			log.Printf("English score: %f", score.Score)
			return score.Score, nil
		}
	}

	log.Println("English not found in the response")
	return 0, nil // English not found in the response
}
