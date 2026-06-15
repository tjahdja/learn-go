// internal/user/user_test.go
package user

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTGenerationAndValidation(t *testing.T) {
	// 1. Arrange: Define a test user ID and claims boundary
	testUserID := 42
	testEmail := "tester@example.com"

	// Mock up standard successful login claims matching your code architecture
	claims := jwt.MapClaims{
		"sub":   testUserID,
		"email": testEmail,
		"exp":   time.Now().Add(time.Hour * 1).Unix(),
		"iat":   time.Now().Unix(),
	}

	// 2. Act: Generate and sign the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("Failed to sign test token: %v", err)
	}

	// 3. Assert: Verify the token string is not empty and has standard JWT segments
	if tokenString == "" {
		t.Error("Expected a valid token string, got an empty result")
	}

	segments := strings.Split(tokenString, ".")
	if len(segments) != 3 {
		t.Errorf("Expected standard 3-part JWT structure, got %d parts", len(segments))
	}

	// 4. Act (Reverse): Parse the generated token back out to verify integrity
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		t.Fatalf("Failed to parse valid token string: %v", err)
	}

	parsedClaims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok || !parsedToken.Valid {
		t.Fatal("Token failed internal validity verification checks")
	}

	// 5. Assert: Verify extracted fields match our source input parameters exactly
	extractedID := int(parsedClaims["sub"].(float64))
	if extractedID != testUserID {
		t.Errorf("Data corruption: expected User ID %d, extracted %d", testUserID, extractedID)
	}

	extractedEmail := parsedClaims["email"].(string)
	if extractedEmail != testEmail {
		t.Errorf("Data corruption: expected Email %s, extracted %s", testEmail, extractedEmail)
	}
}
