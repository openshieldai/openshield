package lib

import (
	"encoding/json"
	"net/http"
	"strings"
)

func ErrorResponse(w http.ResponseWriter, err error) {
	var statusCode int
	var errorResponse map[string]interface{}

	if strings.Contains(err.Error(), "404") {
		statusCode = http.StatusNotFound
		errorResponse = map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Model not found",
				"type":    "invalid_request_error",
				"param":   "model",
				"code":    "model_not_found",
			},
		}
	} else if strings.Contains(err.Error(), "403") {
		statusCode = http.StatusForbidden
		errorResponse = map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Model not found",
				"type":    "invalid_request_error",
				"param":   "model",
				"code":    "model_not_found",
			},
		}
	} else if strings.Contains(err.Error(), "401") {
		statusCode = http.StatusUnauthorized
		errorResponse = map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Unauthorized",
				"type":    "invalid_request_error",
				"param":   "Authorization",
				"code":    "invalid_header",
			},
		}
	} else if strings.Contains(err.Error(), "500") {
		statusCode = http.StatusInternalServerError
		errorResponse = map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Internal Server Error",
				"type":    "invalid_request_error",
				"param":   "server",
				"code":    "internal_server_error",
			},
		}
	} else {
		statusCode = http.StatusInternalServerError
		errorResponse = map[string]interface{}{
			"error": map[string]interface{}{
				"message": "Internal Server Error",
				"type":    "invalid_request_error",
				"param":   "server",
				"code":    "internal_server_error",
			},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse)
}
