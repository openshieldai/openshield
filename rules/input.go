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
	PluginName     string                       `json:"plugin_name"`
	InjectionScore float64                      `json:"injection_score"`
	Prompt         openai.ChatCompletionRequest `json:"prompt"`
	Config         lib.Config                   `json:"config"`
}

type RuleInspection struct {
	CheckResult       bool    `json:"check_result"`
	InjectionScore    float64 `json:"injection_score"`
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
	var result bool
	var errorMessage string

	for input := range config.Rules.Input {
		inputConfig := config.Rules.Input[input]
		switch inputConfig.Type {
		case inputTypes.LanguageDetection:
			if inputConfig.Enabled {
				log.Println("Language Detection")
				extractedPrompt := ""
				for _, message := range userPrompt.Messages {
					if message.Role == "user" {
						extractedPrompt = message.Content
						break
					}
				}
				if extractedPrompt == "" {
					return true, "No user message found in the request", fmt.Errorf("no user message found in the request")
				}
				log.Printf("Extracted prompt: %s\n", extractedPrompt)
				englishScore, err := detectEnglish(extractedPrompt)
				if err != nil {
					return true, fmt.Sprintf("Language detection failed: %v", err), err
				}
				log.Printf("English language probability: %.4f\n", englishScore)
				if englishScore <= 0.85 {
					return true, fmt.Sprintf("English probability too low: %.4f", englishScore), fmt.Errorf("English probability too low: %.4f", englishScore)
				}
				log.Printf("Language Detection: English probability above threshold (%.4f)\n", englishScore)
			}
		case inputTypes.PIIFilter:
			if inputConfig.Enabled {
				log.Println("PII Filter")
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
					return true, "No user message found in the request", fmt.Errorf("no user message found in the request")
				}

				data := Rule{
					PluginName:     inputConfig.Config.PluginName,
					InjectionScore: float64(inputConfig.Config.Threshold),
					Prompt:         userPrompt,
					Config:         inputConfig.Config,
				}

				jsonData, err := json.Marshal(data)
				log.Printf("Request being sent to Python endpoint:\n%s", string(jsonData))
				if err != nil {
					return true, fmt.Sprintf("Failed to marshal PII request: %v", err), err
				}

				agent := fiber.Post(config.Settings.RuleServer.Url + "/rule/execute")
				agent.Body(jsonData)
				agent.Set("Content-Type", "application/json")
				_, body, _ := agent.Bytes()

				var piiResult RuleResult
				if err := json.Unmarshal(body, &piiResult); err != nil {
					return true, fmt.Sprintf("Failed to decode PII filter response: %v", err), err
				}

				if piiResult.Inspection.CheckResult {
					// Update the user message with the anonymized content
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
				agent := fiber.Post(config.Settings.RuleServer.Url + "/rule/execute")
				data := Rule{
					PluginName:     inputConfig.Config.PluginName,
					InjectionScore: float64(inputConfig.Config.Threshold),
					Prompt:         userPrompt,
					Config:         inputConfig.Config,
				}
				jsonify, err := json.Marshal(data)
				if err != nil {
					log.Println(err)
				}

				agent.Body(jsonify)
				agent.Set("Content-Type", "application/json")
				_, body, _ := agent.Bytes()

				var rule RuleResult
				err = json.Unmarshal(body, &rule)
				if err != nil {
					log.Println(err)
				}

				log.Printf("Rule match: %v, Injection score: %f", rule.Match, rule.Inspection.InjectionScore)

				if rule.Inspection.InjectionScore > float64(inputConfig.Config.Threshold) {
					if inputConfig.Action.Type == "block" {
						log.Println("Blocking request due to high injection score.")
						result = true
						errorMessage = "request blocked due to rule match"
					} else if inputConfig.Action.Type == "monitoring" {
						log.Println("Monitoring request due to high injection score.")
						result = false
						errorMessage = "request is being monitored due to rule match"
					}
				} else {
					log.Println("Rule Not Matched")
					result = false
					errorMessage = "request is not blocked"
				}
			}
		default:
			log.Printf("ERROR: Invalid input filter type %s\n", inputConfig.Type)
		}
	}
	log.Printf("Final result: blocked=%v, errorMessage=%s", result, errorMessage)
	return result, errorMessage, nil // Convert the JSON bytes to a string and return
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
