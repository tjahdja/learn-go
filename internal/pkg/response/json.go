// internal/pkg/response/json.go
package response

import (
	"encoding/json"
	"net/http"
)

// ErrorPayload defines a standard, predictable JSON structure for API errors
type ErrorPayload struct {
	Error string `json:"error"`
}

// JSON writes a standardized successful JSON object to the connection stream
func JSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// Fallback if formatting memory structures fails entirely
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// Error writes a structured, predictable JSON error message back over the wire
func Error(w http.ResponseWriter, statusCode int, message string) {
	payload := ErrorPayload{Error: message}
	JSON(w, statusCode, payload)
}
