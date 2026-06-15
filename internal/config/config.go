package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBConn string
	Port   string
}

func LoadConfig() *Config {
	godotenv.Load() // It's okay if this fails in production

	// Fallback to a default if the environment variable is empty
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		DBConn: os.Getenv("DB_CONN"), // Standard name used by many clouds
		Port:   port,
	}
}
