package config

import (
	"os"

	"github.com/joho/godotenv"
)

func LoadEnv() error {
	// Try to load .env file if it exists (for local development)
	// On production (Render), environment variables are set directly
	err := godotenv.Load()
	if err != nil {
		// .env file not found is not an error - it might be on production
		// Environment variables are already available in os.Getenv()
		return nil
	}
	return nil
}

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
