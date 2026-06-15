// internal/pkg/middleware/logger.go
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"my-server/internal/pkg/response"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

// Define a custom context key type to prevent collisions
type contextKey string

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	clients = make(map[string]*client)
	mu      sync.Mutex // Ensures map access is completely thread-safe
)

// Define a unique context key for the User ID to prevent package collisions
const UserIDKey contextKey = "user_id"

// Secure key used to sign tokens (must match the secret key in user.go exactly)
var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func init() {
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()
}

// RateLimiter prevents clients from flooding the system with excess requests
func RateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the client's IP address
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		mu.Lock()
		if _, exists := clients[ip]; !exists {
			// Limit to 5 requests per second with a max burst allowance of 10 items
			clients[ip] = &client{
				limiter: rate.NewLimiter(rate.Every(200*time.Millisecond), 10),
			}
		}
		clients[ip].lastSeen = time.Now()
		limiter := clients[ip].limiter
		mu.Unlock()

		// Check if the client has an available token to consume
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Rate limit exceeded. Maximum 5 requests per second allowed."}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

const RequestIDKey contextKey = "request_id"

// responseWriterInterceptor helps us capture the HTTP status code written by our handlers
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterInterceptor) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// generateRequestID creates a short, unique hex string for tracing
func generateRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(bytes)
}

// StructuredLogger returns a middleware that logs request lifecycle metadata as structured JSON
func StructuredLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := generateRequestID()

		// Inject the Request ID into the request context so downstream handlers can find it
		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		r = r.WithContext(ctx)

		// Intercept the response writer to catch the outgoing status code
		interceptor := &responseWriterInterceptor{ResponseWriter: w, statusCode: http.StatusOK}

		// Process the actual request down the pipeline
		next.ServeHTTP(interceptor, r)

		// Log the final results using structured fields
		slog.Info("HTTP Request Processed",
			slog.String("request_id", reqID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
			slog.Int("status", interceptor.statusCode),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		apiKey := r.Header.Get("X-API-KEY")
		secret := os.Getenv("MY_SECRET_KEY")

		if apiKey != secret {
			http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
			return // Stop the train here!
		}

		// 2. If everything is fine, call the next handler
		next.ServeHTTP(w, r)
	})
}

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Extract the request ID from context to link the panic to the traffic source
				reqID, _ := r.Context().Value(RequestIDKey).(string)

				// Log the absolute entire stack trace cleanly as a structured block
				slog.Error("CRITICAL: Internal Server Panic Recovered",
					slog.String("request_id", reqID),
					slog.Any("panic_error", err),
					slog.String("stack_trace", string(debug.Stack())),
				)

				// Respond with a uniform JSON message instead of letting the connection hang
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"An unexpected internal server error occurred"}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// RequireAuth blocks unauthenticated traffic and injects the user's ID into the context
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Error(w, http.StatusUnauthorized, "Missing authorization token")
			return
		}

		// 2. The header must follow the standard "Bearer <token>" format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Error(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}
		tokenString := parts[1]

		// 3. Parse and cryptographically verify the token
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			// Enforce the expected HMAC signing method
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			response.Error(w, http.StatusUnauthorized, "Invalid or expired session token")
			return
		}

		// 4. Extract the claims and pull out the User ID ("sub")
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "Invalid token data claims")
			return
		}

		// JWT numeric fields map to float64 in Go; cast it safely to an integer
		userIDFloat, ok := claims["sub"].(float64)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "Malformed user identification within token")
			return
		}
		userID := int(userIDFloat)

		// 5. Inject the User ID into the request context and pass it forward down the pipeline
		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
