package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

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

func sendRequest(data Rule) (RuleResult, error) {
	jsonify, err := json.Marshal(data)
	if err != nil {
		return RuleResult{}, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", lib.GetConfig().Settings.RuleServer.Url+"/rule/execute", bytes.NewBuffer(jsonify))
	if err != nil {
		return RuleResult{}, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return RuleResult{}, fmt.Errorf("failed to send request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RuleResult{}, fmt.Errorf("failed to read response body: %v", err)
	}

	var rule RuleResult
	err = json.Unmarshal(body, &rule)
	if err != nil {
		return RuleResult{}, fmt.Errorf("failed to decode response: %v", err)
	}

	return rule, nil
}

func extractUserPrompt(userPrompt openai.ChatCompletionRequest) (string, int, error) {
	var userMessages []string
	var firstUserMessageIndex int = -1

	for i, message := range userPrompt.Messages {
		if message.Role == "user" {
			if firstUserMessageIndex == -1 {
				firstUserMessageIndex = i
			}
			userMessages = append(userMessages, message.Content)
		}
	}

	if firstUserMessageIndex == -1 {
		return "", -1, fmt.Errorf(`{"message": "no user message found in the request"}`)
	}

	concatenatedMessages := strings.Join(userMessages, " ")
	return concatenatedMessages, firstUserMessageIndex, nil
}

func handleRule(inputConfig lib.Rule, userPrompt openai.ChatCompletionRequest, ruleType string) (bool, string, error) {
	if !inputConfig.Enabled {
		return false, "", nil
	}

	log.Printf("%s check enabled", ruleType)
	extractedPrompt, userMessageIndex, err := extractUserPrompt(userPrompt)
	if err != nil {
		log.Println(err)
		return true, err.Error(), err
	}
	log.Printf("Extracted prompt for %s: %s", ruleType, extractedPrompt)

	data := Rule{Prompt: userPrompt, Config: inputConfig.Config}
	rule, err := sendRequest(data)
	if err != nil {
		return true, err.Error(), err
	}

	log.Printf("%s detection result: Match=%v, Score=%f", ruleType, rule.Match, rule.Inspection.Score)

	switch ruleType {
	case inputTypes.InvisibleChars:
		return handleInvisibleCharsAction(inputConfig, rule)
	case inputTypes.LanguageDetection:
		return handleLanguageDetectionAction(rule)
	case inputTypes.PIIFilter:
		return handlePIIFilterAction(inputConfig, rule, userPrompt, userMessageIndex)
	case inputTypes.PromptInjection:
		return handlePromptInjectionAction(inputConfig, rule)
	default:
		log.Printf("%s Rule Not Matched", ruleType)
		return false, "", nil
	}
}

func handleInvisibleCharsAction(inputConfig lib.Rule, rule RuleResult) (bool, string, error) {
	if rule.Match {
		if inputConfig.Action.Type == "block" {
			log.Println("Blocking request due to invalid characters detection.")
			return true, `{"message": "request blocked due to rule match", "rule_type": "invisible_chars"}`, nil
		}
		log.Println("Monitoring request due to invalid characters detection.")
	}
	log.Println("Invalid Characters Rule Not Matched")
	return false, "", nil
}

func handleLanguageDetectionAction(rule RuleResult) (bool, string, error) {
	if !rule.Match {
		log.Printf("English probability too low: %.4f", rule.Inspection.Score)
		return true, `{"message": "request blocked due to rule match", "rule_type": "language_detection"}`, nil
	}
	log.Printf("Language Detection: English probability above threshold (%.4f)", rule.Inspection.Score)
	return false, "", nil
}

func handlePIIFilterAction(inputConfig lib.Rule, rule RuleResult, userPrompt openai.ChatCompletionRequest, userMessageIndex int) (bool, string, error) {
	if rule.Inspection.CheckResult {
		log.Println("PII detected, anonymizing content")
		userPrompt.Messages[userMessageIndex].Content = rule.Inspection.AnonymizedContent
		if inputConfig.Action.Type == "block" {
			log.Println("Blocking request due to PII detection.")
			return true, `{"message": "request blocked due to rule match", "rule_type": "pii_data"}`, nil
		}
		log.Println("Monitoring request due to PII detection.")
	} else {
		log.Println("No PII detected")
	}
	return false, "", nil
}

func handlePromptInjectionAction(inputConfig lib.Rule, rule RuleResult) (bool, string, error) {
	if rule.Match {
		if inputConfig.Action.Type == "block" {
			log.Println("Blocking request due to prompt injection detection.")
			return true, `{"message": "request blocked due to rule match", "rule_type": "prompt_injection"}`, nil
		}
		log.Println("Monitoring request due to prompt injection detection.")
	}
	log.Println("Prompt Injection Rule Not Matched")
	return false, "", nil
}

func Input(_ *http.Request, userPrompt openai.ChatCompletionRequest) (bool, string, error) {
	config := lib.GetConfig()

	log.Println("Starting Input function")

	for input := range config.Rules.Input {
		inputConfig := config.Rules.Input[input]
		log.Printf("Processing input rule: %s", inputConfig.Type)

		var blocked bool
		var message string
		var err error

		switch inputConfig.Type {
		case inputTypes.InvisibleChars:
			blocked, message, err = handleRule(inputConfig, userPrompt, inputTypes.InvisibleChars)
		case inputTypes.LanguageDetection:
			blocked, message, err = handleRule(inputConfig, userPrompt, inputTypes.LanguageDetection)
		case inputTypes.PIIFilter:
			blocked, message, err = handleRule(inputConfig, userPrompt, inputTypes.PIIFilter)
		case inputTypes.PromptInjection:
			blocked, message, err = handleRule(inputConfig, userPrompt, inputTypes.PromptInjection)
		default:
			log.Printf("ERROR: Invalid input filter type %s", inputConfig.Type)
		}

		if blocked {
			return blocked, message, err
		}
	}

	log.Println("Final result: No rules matched, request is not blocked")
	return false, "request is not blocked", nil
}
