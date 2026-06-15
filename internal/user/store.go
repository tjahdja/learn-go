package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Database interface {
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row // Fixed: returns *sql.Row, no error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type Store struct {
	DB Database
}

func NewStore(db Database) *Store {
	return &Store{
		DB: db,
	}
}

type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // The "-" tag ensures the hash is NEVER leaked in JSON responses
	CreatedAt    time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func (s *Store) Register(ctx context.Context, req RegisterRequest) error {
	// 1. Enforce simple presence validation rules
	if req.Email == "" || req.Password == "" {
		return fmt.Errorf("email and password fields are required")
	}

	// 2. Transform the plain text password into a secure hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		return fmt.Errorf("failed to process password encryption: %w", err)
	}

	// 3. Write the execution query into Postgres
	query := `INSERT INTO users (email, password_hash) VALUES ($1, $2)`
	_, err = s.DB.ExecContext(ctx, query, req.Email, string(hashedPassword))
	if err != nil {
		// Check for duplicate key violations (Postgres Error Code 23505)
		return fmt.Errorf("registration failure (account may already exist): %w", err)
	}

	return nil
}

func (s *Store) Login(ctx context.Context, req LoginRequest) (string, error) {
	if req.Email == "" || req.Password == "" {
		return "", errors.New("email and password fields are required")
	}

	// 1. Look up the user record by email
	query := `SELECT id, email, password_hash FROM users WHERE email = $1`
	var u User
	err := s.DB.QueryRowContext(ctx, query, req.Email).Scan(&u.ID, &u.Email, &u.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("invalid email or password")
		}
		return "", fmt.Errorf("database query failure: %w", err)
	}

	// 2. Compare the incoming cleartext password with the stored bcrypt hash
	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password))
	if err != nil {
		// Return a generic error so attackers don't know if the email or password was wrong
		return "", errors.New("invalid email or password")
	}

	// 3. Create the JWT claims payload
	claims := jwt.MapClaims{
		"sub":   u.ID,                                  // "sub" (Subject) -> The unique User ID
		"email": u.Email,                               // Include the email for quick access
		"exp":   time.Now().Add(time.Hour * 72).Unix(), // Token expires in 72 hours
		"iat":   time.Now().Unix(),                     // Issued At time
	}

	// 4. Cryptographically sign the token using our secret key
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}
