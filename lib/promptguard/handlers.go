package promptguard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/openshieldai/openshield/lib"
)

type AnalyzeRequest struct {
	Text      string  `json:"text"`
	Threshold float64 `json:"threshold"`
}

type AnalyzeResponse struct {
	Score   float64 `json:"score"`
	Details struct {
		BenignProbability    float64 `json:"benign_probability"`
		InjectionProbability float64 `json:"injection_probability"`
		JailbreakProbability float64 `json:"jailbreak_probability"`
	} `json:"details"`
}

func SetupRoutes(r chi.Router) {
	r.Post("/promptguard/analyze", lib.AuthOpenShieldMiddleware(AnalyzeHandler))
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

	resp, err := callPromptGuardService(r.Context(), req)
	if err != nil {
		lib.ErrorResponse(w, fmt.Errorf("error calling PromptGuard service: %v", err))
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func callPromptGuardService(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error) {
	config := lib.GetConfig()
	promptGuardURL := config.Services.PromptGuard.BaseUrl + "/analyze"

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", promptGuardURL, bytes.NewBuffer(reqBody))
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

	var result AnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return &result, nil
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
