package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/openshieldai/go-openai"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/openshieldai/openshield/lib"
)

type InputTypes struct {
	LanguageDetection string
	PromptInjection   string
	PIIFilter         string
	InvisibleChars    string
}

type Rule struct {
	Prompt interface{} `json:"prompt"`
	Config lib.Config  `json:"config"`
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

func handlePIIFilterAction(inputConfig lib.Rule, rule RuleResult, messages interface{}, userMessageIndex int) (bool, string, error) {
	if rule.Inspection.CheckResult {
		log.Println("PII detected, anonymizing content")

		switch msg := messages.(type) {
		case []openai.ChatCompletionMessage:
			msg[userMessageIndex].Content = rule.Inspection.AnonymizedContent
		case []openai.ThreadMessage:
			msg[userMessageIndex].Content = rule.Inspection.AnonymizedContent
		default:
			return true, "Invalid message type", fmt.Errorf("unsupported message type")
		}

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

func Input(r *http.Request, request interface{}) (bool, string, error) {
	config := lib.GetConfig()

	log.Println("Starting Input function")

	sort.Slice(config.Rules.Input, func(i, j int) bool {
		return config.Rules.Input[i].OrderNumber < config.Rules.Input[j].OrderNumber
	})

	var messages []openai.ChatCompletionMessage
	var model string
	var maxTokens int

	switch req := request.(type) {
	case struct {
		Model     string                         `json:"model"`
		Messages  []openai.ChatCompletionMessage `json:"messages"`
		MaxTokens int                            `json:"max_tokens"`
		Stream    bool                           `json:"stream"`
	}:
		messages = req.Messages
		model = req.Model
		maxTokens = req.MaxTokens
	case openai.ChatCompletionRequest:
		messages = req.Messages
		model = req.Model
		maxTokens = req.MaxTokens
	default:
		return true, "Invalid request type", fmt.Errorf("unsupported request type")
	}

	for _, inputConfig := range config.Rules.Input {
		log.Printf("Processing input rule: %s (Order: %d)", inputConfig.Type, inputConfig.OrderNumber)

		if !inputConfig.Enabled {
			log.Printf("Rule %s is disabled, skipping", inputConfig.Type)
			continue
		}

		blocked, message, err := handleRule(inputConfig, messages, model, maxTokens, inputConfig.Type)
		if blocked {
			return blocked, message, err
		}
	}

	log.Println("Final result: No rules matched, request is not blocked")
	return false, "request is not blocked", nil
}

func handleRule(inputConfig lib.Rule, messages []openai.ChatCompletionMessage, model string, maxTokens int, ruleType string) (bool, string, error) {
	log.Printf("%s check enabled (Order: %d)", ruleType, inputConfig.OrderNumber)

	extractedPrompt, userMessageIndex, err := extractUserPromptFromChat(messages)
	if err != nil {
		log.Println(err)
		return true, err.Error(), err
	}
	log.Printf("Extracted prompt for %s: %s", ruleType, extractedPrompt)

	data := Rule{
		Prompt: struct {
			Messages  []openai.ChatCompletionMessage `json:"messages"`
			Model     string                         `json:"model"`
			MaxTokens int                            `json:"max_tokens"`
		}{
			Messages:  messages,
			Model:     model,
			MaxTokens: maxTokens,
		},
		Config: inputConfig.Config,
	}
	rule, err := sendRequest(data)
	if err != nil {
		return true, err.Error(), err
	}

	log.Printf("Rule result for %s: %+v", ruleType, rule)

	return handleRuleAction(inputConfig, rule, ruleType, messages, userMessageIndex)
}

func extractUserPromptFromCreateThreadAndRun(request openai.CreateThreadAndRunRequest) (string, int, error) {
	if len(request.Thread.Messages) > 0 {
		return extractUserPromptFromThread(request.Thread.Messages)
	}
	return "", -1, nil
}
func handleRuleAction(inputConfig lib.Rule, rule RuleResult, ruleType string, messages interface{}, userMessageIndex int) (bool, string, error) {
	log.Printf("%s detection result: Match=%v, Score=%f", ruleType, rule.Match, rule.Inspection.Score)

	switch ruleType {
	case inputTypes.InvisibleChars:
		return handleInvisibleCharsAction(inputConfig, rule)
	case inputTypes.LanguageDetection:
		return handleLanguageDetectionAction(rule)
	case inputTypes.PIIFilter:
		return handlePIIFilterAction(inputConfig, rule, messages, userMessageIndex)
	case inputTypes.PromptInjection:
		return handlePromptInjectionAction(inputConfig, rule)
	default:
		log.Printf("%s Rule Not Matched", ruleType)
		return false, "", nil
	}
}

func extractUserPromptFromChat(messages []openai.ChatCompletionMessage) (string, int, error) {
	var userMessages []string
	var firstUserMessageIndex int = -1

	for i, message := range messages {
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

func extractUserPromptFromThread(messages []openai.ThreadMessage) (string, int, error) {
	var userMessages []string
	var firstUserMessageIndex int = -1

	for i, message := range messages {
		if message.Role == "user" {
			if firstUserMessageIndex == -1 {
				firstUserMessageIndex = i
			}
			userMessages = append(userMessages, message.Content)
		}
	}

	if firstUserMessageIndex == -1 {
		log.Println("No user message found in the ThreadRequest, continuing processing other rules.")
		return "", -1, nil
	}

	concatenatedMessages := strings.Join(userMessages, " ")
	return concatenatedMessages, firstUserMessageIndex, nil
}
func extractUserPromptFromMessage(message openai.MessageRequest) (string, int, error) {
	if message.Role == "user" {
		return message.Content, 0, nil
	}
	return "", -1, fmt.Errorf(`{"message": "no user message found in the request"}`)
}
