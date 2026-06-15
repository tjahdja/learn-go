// internal/todo/handler_test.go
package todo

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// MockDB satisfies your Database interface for testing purposes
type MockDB struct {
	MockQueryContext func(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Implement the required interface methods.
// We only fill out the ones we plan to trigger in our test scenario.
func (m *MockDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return m.MockQueryContext(ctx, query, args...)
}

func (m *MockDB) Query(query string, args ...any) (*sql.Rows, error)                      { return nil, nil }
func (m *MockDB) Exec(query string, args ...any) (sql.Result, error)                      { return nil, nil }
func (m *MockDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row { return nil }

// internal/todo/handler_test.go (continued)

func TestHandleGetTodos_Success(t *testing.T) {
	// 1. Create a fresh sqlmock database instance and mock controller
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// 2. Define the columns and rows we want our fake database to return
	// We will simulate returning two items to make sure our loop works perfectly
	columns := []string{"id", "task", "completed", "created_at"}
	mockRows := sqlmock.NewRows(columns).
		AddRow(1, "Finish the Go backend", false, time.Now()).
		AddRow(2, "Learn unit testing", true, time.Now())

	// 3. Set expectations: Tell the mock exactly what SQL query it should expect
	// and what rows it should return when that query fires.
	mock.ExpectQuery(`SELECT id, task, completed, created_at FROM todos`).
		WillReturnRows(mockRows)

	// 4. Instantiate our Store injecting our fake mock connection
	store := NewStore(db)

	// 5. Construct our HTTP test environment
	req, err := http.NewRequest(http.MethodGet, "/todo", nil)
	if err != nil {
		t.Fatalf("Failed to construct test request: %v", err)
	}
	rr := httptest.NewRecorder()

	// 6. Invoke the handler
	store.handleGetTodos(rr, req)

	// 7. Assertions
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned incorrect status code: got %v want %v", status, http.StatusOK)
	}

	expectedContentType := "application/json"
	if contentType := rr.Header().Get("Content-Type"); contentType != expectedContentType {
		t.Errorf("Handler set wrong Content-Type: got %v want %v", contentType, expectedContentType)
	}

	// 8. Ensure all database expectations we defined above were actually met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

// Test Case 1: Verifies that an empty payload triggers our validation logic
func TestHandlePostTodo_ValidationError_EmptyTask(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	store := NewStore(db)

	// Send an empty JSON object to trigger the validation rule
	jsonPayload := `{"task": ""}`
	req, err := http.NewRequest(http.MethodPost, "/todo", bytes.NewBufferString(jsonPayload))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()

	store.handlePostTodo(rr, req)

	// We expect a 422 Unprocessable Entity status code
	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status %v, got %v", http.StatusUnprocessableEntity, rr.Code)
	}

	// Verify our custom error response utility formatting matches
	expectedErrorMsg := `{"error":"Task content cannot be empty"}`
	if strings.TrimSpace(rr.Body.String()) != expectedErrorMsg {
		t.Errorf("unexpected error payload: got %q want %q", rr.Body.String(), expectedErrorMsg)
	}
}

// Test Case 2: Verifies that a valid payload runs an INSERT statement and returns a 201
func TestHandlePostTodo_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	// Define the single row our RETURNING clause will pass back to Go
	columns := []string{"id", "task", "completed", "created_at"}
	mockRows := sqlmock.NewRows(columns).AddRow(1, "Write a unit test", false, time.Now())

	// Expect an INSERT query and return our mocked row
	mock.ExpectQuery(`INSERT INTO todos`).
		WithArgs("Write a unit test").
		WillReturnRows(mockRows)

	store := NewStore(db)

	jsonPayload := `{"task": "Write a unit test"}`
	req, err := http.NewRequest(http.MethodPost, "/todo", bytes.NewBufferString(jsonPayload))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()

	store.handlePostTodo(rr, req)

	// Assertions
	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %v, got %v", http.StatusCreated, rr.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled db expectations: %s", err)
	}
}

func TestHandleDeleteTodo_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	// Expect an Exec call targeting the specific ID, and mock 1 row affected
	mock.ExpectExec(`DELETE FROM todos WHERE id = \$1`).
		WithArgs(5).
		WillReturnResult(sqlmock.NewResult(0, 1)) // 0 last inserted ID, 1 row affected

	store := NewStore(db)

	req, err := http.NewRequest(http.MethodDelete, "/todo?id=5", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	rr := httptest.NewRecorder()

	store.handleDeleteTodo(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %v, got %v", http.StatusOK, rr.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled db expectations: %s", err)
	}
}
