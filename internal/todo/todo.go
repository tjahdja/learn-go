package todo

import (
	"encoding/json"
	"log/slog"
	"my-server/internal/pkg/middleware"
	"my-server/internal/pkg/response"
	"net/http"
	"strconv"
	"time"
)

type TodoItem struct {
	ID        int       `json:"id"`
	Task      string    `json:"task"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTodoRequest struct {
	Task string `json:"task"`
}

type UpdateTodoStatusRequest struct {
	Completed bool `json:"completed"`
}

func (c *Store) TodoHandler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case http.MethodGet:
		c.handleGetTodos(w, r)
	case http.MethodPost: // --- ADD THIS ROUTE ---
		c.handlePostTodo(w, r)
	case http.MethodPatch: // --- ADD THIS ROUTE ---
		c.handlePatchTodo(w, r)
	case http.MethodDelete: // --- ADD THIS ROUTE ---
		c.handleDeleteTodo(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

}

func (c *Store) handleGetTodos(w http.ResponseWriter, r *http.Request) {
	// Pass the HTTP request context into our database layer
	reqID, _ := r.Context().Value(middleware.UserIDKey).(int)

	items, err := c.FetchAll(r.Context(), reqID)
	if err != nil {
		slog.Error("Failed to fetch todos from database",
			slog.String("request_id", strconv.Itoa(reqID)),
			slog.Any("error", err),
		)
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve todo collection")
		return
	}

	// Single line instead of setting headers and running encoders manually!
	response.JSON(w, http.StatusOK, items)
}

func (s *Store) handlePostTodo(w http.ResponseWriter, r *http.Request) {
	var req CreateTodoRequest

	// 1. Decode the incoming JSON payload from the request body
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// 2. Perform fundamental validation (Separation of Concerns: business rules check)
	if req.Task == "" {
		response.Error(w, http.StatusUnprocessableEntity, "Task content cannot be empty")
		return
	}

	// 3. Pass execution down to the pure data logic layer
	newItem, err := s.Insert(r.Context(), req.Task)
	if err != nil {
		// Log the actual low-level system error internally for debugging
		response.Error(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	response.JSON(w, http.StatusCreated, newItem)
}

func (s *Store) handlePatchTodo(w http.ResponseWriter, r *http.Request) {
	// 1. Extract and validate the ID from the URL Query string (?id=X)
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		response.Error(w, http.StatusBadRequest, "Missing required 'id' query parameter")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid 'id' parameter; must be an integer")
		return
	}

	// 2. Decode the status configuration from the JSON body
	var req UpdateTodoStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	// 3. Pass execution to our Database Interface layer
	updatedItem, err := s.UpdateStatus(r.Context(), id, req.Completed)
	if err != nil {
		// If Postgres returns an error because the row wasn't found
		response.Error(w, http.StatusNotFound, "Todo item not found or update failed")
		return
	}
	response.JSON(w, http.StatusOK, updatedItem)
}

func (s *Store) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	// 1. Extract and validate the ID from the URL Query string (?id=X)
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		response.Error(w, http.StatusBadRequest, "Missing required 'id' query parameter")
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Query parameter 'id' must be an integer value")
		return
	}

	// 2. Pass execution to the database layer
	err = s.Delete(r.Context(), id)
	if err != nil {
		// Differentiate between an intentional "not found" vs a broken database connection
		if err.Error() == "todo item not found" {
			response.Error(w, http.StatusNotFound, "Target todo item record was not found")
		} else {
			response.Error(w, http.StatusInternalServerError, "Database delete operation failed")
		}
		return
	}

	// 3. Return a successful confirmation payload
	response.JSON(w, http.StatusOK, map[string]string{"message": "Todo item deleted successfully"})
}
