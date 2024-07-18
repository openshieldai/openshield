package rules

import (
	"encoding/json"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/openshieldai/openshield/lib"
	"github.com/sashabaranov/go-openai"
)

type InputTypes struct {
	LanguageDetection string
	PromptInjection   string
	PIIFilter         string
}

type Rule struct {
	PluginName     string                       `json:"plugin_name"`
	Label          string                       `json:"label"`
	InjectionScore float64                      `json:"injection_score"`
	Prompt         openai.ChatCompletionRequest `json:"prompt"`
}

type RuleInspection struct {
	CheckResult    bool    `json:"check_result"`
	InjectionScore float64 `json:"injection_score"`
}

type RuleResult struct {
	Match      bool           `json:"match"`
	Inspection RuleInspection `json:"inspection"`
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
			}
		case inputTypes.PromptInjection:
			if inputConfig.Enabled {
				agent := fiber.Post(config.Settings.RuleServer.Url + "/rule/execute")
				data := Rule{
					PluginName:     inputConfig.Config.PluginName,
					InjectionScore: float64(inputConfig.Config.Threshold),
					Prompt:         userPrompt,
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

				if rule.Match {
					if inputConfig.Action.Type == "block" {
						// Log the error or prepare an error message
						log.Println("Blocking request due to rule match.")
						result = true
						errorMessage = "request blocked due to rule match"
					}
					if inputConfig.Action.Type == "monitoring" {
						// Log the error or prepare an error message
						log.Println("Monitoring request due to rule match.")
						result = false
						errorMessage = "request is being monitored due to rule match"
					}
				} else {
					log.Println("Rule Not Matched")
					result = false
					errorMessage = "request is not blocked"
				}
			}
		case inputTypes.PIIFilter:
			if inputConfig.Enabled {
				log.Println("PII Filter")
			}
		default:
			log.Printf("ERROR: Invalid input filter type %s\n", inputConfig.Type)
		}
	}
	return result, errorMessage, nil // Convert the JSON bytes to a string and return
}
