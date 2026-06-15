// internal/user/integration_test.go
package user

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq" // Ensure your Postgres driver is imported
)

var testDB *sql.DB

// TestMain handles the global database setup and teardown for our integration tests
func TestMain(m *testing.M) {
	// 1. Connect to our isolated Docker test database (Port 5433)
	connStr := "postgres://postgres:test_password_123@127.0.0.1:5434/todo_test_db?sslmode=disable"
	var err error
	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Could not connect to test database: %v", err)
	}

	// 2. Clear any old schema data to guarantee a completely clean test environment
	_, _ = testDB.Exec("DROP TABLE IF EXISTS users CASCADE;")

	// 3. Run the raw schema definition required for this user test suite
	schema := `
	CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = testDB.Exec(schema)
	if err != nil {
		log.Fatalf("Failed to build test database tables: %v", err)
	}

	// 4. Execute the test suites
	code := m.Run()

	// 5. Clean up and close connections
	testDB.Close()
	os.Exit(code)
}

func TestRegisterEndpointIntegration(t *testing.T) {
	// 1. Arrange: Instantiate our real user Store with the test DB connection
	store := NewStore(testDB)

	// Build a valid JSON registration payload
	regPayload := RegisterRequest{
		Email:    "integration_tester@example.com",
		Password: "superPassword999",
	}
	body, _ := json.Marshal(regPayload)

	// 2. Act: Simulate an incoming network packet request using standard HTTP libraries
	req, err := http.NewRequest("POST", "/user/register", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create test request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// httptest.NewRecorder() creates a mock response window that catches our API output
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(store.RegisterHandler)

	// Force the handler to run synchronously against our mock objects
	handler.ServeHTTP(rr, req)

	// 3. Assert: Verify the HTTP Response code matches '211 Created'
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Verify the JSON response text contains our confirmation message
	var responseMap map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &responseMap); err != nil {
		t.Fatalf("Failed to parse handler JSON response: %v", err)
	}

	if msg := responseMap["message"]; msg != "User account successfully registered" {
		t.Errorf("Unexpected response content body: got %q", msg)
	}

	// 4. Verify Database Integrity: Check if the record was actually committed to PostgreSQL
	var dbEmail string
	var dbHash string
	err = testDB.QueryRow("SELECT email, password_hash FROM users WHERE email = $1", regPayload.Email).Scan(&dbEmail, &dbHash)
	if err != nil {
		t.Fatalf("Database verification query failed: record was not created! Error: %v", err)
	}

	if dbEmail != regPayload.Email {
		t.Errorf("Database data mismatch: expected %s, got %s", regPayload.Email, dbEmail)
	}

	if !strings.HasPrefix(dbHash, "$2a$") {
		t.Errorf("Security flaw: password_hash was not safely processed by bcrypt! Got: %s", dbHash)
	}
}
