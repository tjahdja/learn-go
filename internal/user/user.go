package user

import (
	"encoding/json"
	"net/http"

	"my-server/internal/pkg/response"
)

func (s *Store) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Enforce correct REST behavior
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		response.Error(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	// 2. Parse the request payload
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body structure")
		return
	}
	defer r.Body.Close()

	// 3. Pass processing execution to our cryptographic Store method
	err := s.Register(r.Context(), req)
	if err != nil {
		// Differentiate simple validation/duplicate issues from true database drops
		response.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// 4. Return success confirmation
	response.JSON(w, http.StatusCreated, map[string]string{
		"message": "User account successfully registered",
	})
}

func (s *Store) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		response.Error(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body structure")
		return
	}
	defer r.Body.Close()

	// Process authentication
	token, err := s.Login(r.Context(), req)
	if err != nil {
		// Return a 401 Unauthorized status code if credentials fail
		response.Error(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Return the token payload back to the client
	response.JSON(w, http.StatusOK, AuthResponse{Token: token})
}
