package llamaguard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openshieldai/openshield/lib"
)

type AnalyzeRequest struct {
	Text               string   `json:"text"`
	Categories         []string `json:"categories,omitempty"`
	ExcludedCategories []string `json:"excluded_categories,omitempty"`
}

type LlamaGuardResponse struct {
	Response string `json:"response"`
}

type AnalyzeResponse struct {
	IsSafe     bool     `json:"is_safe"`
	Categories []string `json:"violated_categories,omitempty"`
	Analysis   string   `json:"analysis"`
}

func SetupRoutes(r chi.Router) {
	r.Post("/llamaguard/analyze", lib.AuthOpenShieldMiddleware(AnalyzeHandler))
}

func AnalyzeHandler(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		lib.ErrorResponse(w, fmt.Errorf("error reading request body: %v", err))
		return
	}

	if req.Text == "" {
		lib.ErrorResponse(w, fmt.Errorf("text field is required"))
		return
	}

	performAuditLogging(r, "llamaguard_analyze", "input", []byte(req.Text))

	resp, err := callLlamaGuardService(r.Context(), req)
	if err != nil {
		lib.ErrorResponse(w, fmt.Errorf("error calling LlamaGuard service: %v", err))
		return
	}

	respBytes, _ := json.Marshal(resp)
	performAuditLogging(r, "llamaguard_analyze", "output", respBytes)

	json.NewEncoder(w).Encode(resp)
}

func callLlamaGuardService(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error) {
	config := lib.GetConfig()
	llamaGuardURL := config.Services.LlamaGuard.BaseUrl + "/analyze"

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", llamaGuardURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned status %d", resp.StatusCode)
	}

	var llamaGuardResp LlamaGuardResponse
	if err := json.NewDecoder(resp.Body).Decode(&llamaGuardResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return parseLlamaGuardResponse(llamaGuardResp.Response), nil
}

func parseLlamaGuardResponse(response string) *AnalyzeResponse {

	result := &AnalyzeResponse{
		Analysis: response,
		IsSafe:   response == "safe",
	}

	if !result.IsSafe {

		for _, category := range []string{"S1", "S2", "S3", "S4", "S5", "S6", "S7",
			"S8", "S9", "S10", "S11", "S12", "S13"} {
			if bytes.Contains([]byte(response), []byte(category)) {
				result.Categories = append(result.Categories, category)
			}
		}
	}

	return result
}

func performAuditLogging(r *http.Request, logType string, messageType string, body []byte) {
	apiKeyId := r.Context().Value("apiKeyId").(uuid.UUID)

	productID, err := getProductIDFromAPIKey(apiKeyId)
	if err != nil {
		hashedApiKeyId := sha256.Sum256([]byte(apiKeyId.String()))
		log.Printf("Failed to retrieve ProductID for apiKeyId %x: %v", hashedApiKeyId, err)
		return
	}

	lib.AuditLogs(string(body), logType, apiKeyId, messageType, productID, r)
}

func getProductIDFromAPIKey(apiKeyId uuid.UUID) (uuid.UUID, error) {
	var productIDStr string
	err := lib.DB().Table("api_keys").Where("id = ?", apiKeyId).Pluck("product_id", &productIDStr).Error
	if err != nil {
		return uuid.Nil, err
	}

	productID, err := uuid.Parse(productIDStr)
	if err != nil {
		return uuid.Nil, errors.New("failed to parse product_id as UUID")
	}

	return productID, nil
}
