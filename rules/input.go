package rules

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
	"github.com/sashabaranov/go-openai"
)

type InputTypes struct {
	LanguageDetection string
	PromptInjection   string
	PIIFilter         string
	InvisibleChars    string
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
	InvisibleChars:    "invisible_chars",
}

func Input(_ *fiber.Ctx, userPrompt openai.ChatCompletionRequest) (bool, string, error) {
	config := lib.GetConfig()

	log.Println("Starting Input function")

	for input := range config.Rules.Input {
		inputConfig := config.Rules.Input[input]
		log.Printf("Processing input rule: %s", inputConfig.Type)

		switch inputConfig.Type {

		case inputTypes.InvisibleChars:
			if inputConfig.Enabled {
				log.Println("Invisible Characters check enabled")
				agent := fiber.Post(config.Settings.RuleServer.Url + "/rule/execute")
				data := Rule{
					Prompt: userPrompt,
					Config: inputConfig.Config,
				}
				jsonify, err := json.Marshal(data)
				if err != nil {
					log.Printf("Failed to marshal invalid characters: %v", err)
					return true, fmt.Sprintf("Failed to marshal invalid characters request: %v", err), err
				}

				log.Printf("Request being sent to Python endpoint for Invalid Characters:\n%s", string(jsonify))

				agent.Body(jsonify)
				agent.Set("Content-Type", "application/json")
				_, body, _ := agent.Bytes()

				log.Printf("Response received from Python endpoint for Invalid Characters:\n%s", string(body))

				var rule RuleResult
				err = json.Unmarshal(body, &rule)
				if err != nil {
					log.Printf("Failed to decode Invalid Characters response: %v", err)
					return true, fmt.Sprintf("Failed to decode Invalid Characters response: %v", err), err
				}

				log.Printf("Invalid Characters detection result: Match=%v, Score=%f", rule.Match, rule.Inspection.Score)

				if rule.Match {
					if inputConfig.Action.Type == "block" {
						log.Println("Blocking request due to invalid characters detection.")
						return true, "request blocked due to rule match", nil
					} else if inputConfig.Action.Type == "monitoring" {
						log.Println("Monitoring request due to invalid characters detection.")
						// Continue processing
					}
				} else {
					log.Println("Invalid Characters Rule Not Matched")
				}
			}
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

				agent := fiber.Post(config.Settings.RuleServer.Url + "/rule/execute")
				data := Rule{
					Prompt: userPrompt,
					Config: inputConfig.Config,
				}
				jsonData, err := json.Marshal(data)
				if err != nil {
					log.Printf("Failed to marshal language detection request: %v", err)
					return true, fmt.Sprintf("Failed to marshal language detection request: %v", err), err
				}

				log.Printf("Request being sent to Python endpoint for Language Detection:\n%s", string(jsonData))

				agent.Body(jsonData)
				agent.Set("Content-Type", "application/json")
				_, body, _ := agent.Bytes()

				log.Printf("Response received from Python endpoint for Language Detection:\n%s", string(body))

				var rule RuleResult
				err = json.Unmarshal(body, &rule)
				if err != nil {
					log.Printf("Failed to decode language detection response: %v", err)
					return true, fmt.Sprintf("Failed to decode language detection response: %v", err), err
				}

				log.Printf("Language Detection result: Match=%v, Score=%f", rule.Match, rule.Inspection.Score)

				if !rule.Match {
					log.Printf("English probability too low: %.4f", rule.Inspection.Score)
					return true, fmt.Sprintf("English probability too low: %.4f", rule.Inspection.Score), fmt.Errorf("English probability too low: %.4f", rule.Inspection.Score)
				}
				log.Printf("Language Detection: English probability above threshold (%.4f)", rule.Inspection.Score)
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
