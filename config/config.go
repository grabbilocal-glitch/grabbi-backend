package config

import (
	"fmt"
	"log"
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

// ValidateEnv checks that critical environment variables are set.
// Returns an error if any critical variable is missing.
func ValidateEnv() error {
	var missing []string

	// Critical variables - application cannot function without these
	if os.Getenv("JWT_SECRET") == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if os.Getenv("DATABASE_URL") == "" {
		missing = append(missing, "DATABASE_URL")
	}

	if len(missing) > 0 {
		return fmt.Errorf("critical environment variables not set: %v", missing)
	}

	// Non-critical variables - log warnings but don't fail
	if os.Getenv("FIREBASE_STORAGE_BUCKET") == "" {
		log.Println("WARNING: FIREBASE_STORAGE_BUCKET not set - file uploads will fail")
	}
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		log.Println("WARNING: GOOGLE_APPLICATION_CREDENTIALS not set - Firebase features may not work")
	}
	if os.Getenv("FRONTEND_URL") == "" {
		log.Println("WARNING: FRONTEND_URL not set - CORS may not work correctly")
	}
	if os.Getenv("ADMIN_URL") == "" {
		log.Println("WARNING: ADMIN_URL not set")
	}
	if os.Getenv("SMTP_HOST") == "" {
		log.Println("WARNING: SMTP_HOST not set - email notifications will not work")
	}
	if os.Getenv("SMTP_PORT") == "" {
		log.Println("WARNING: SMTP_PORT not set - email notifications will not work")
	}
	if os.Getenv("SMTP_FROM") == "" {
		log.Println("WARNING: SMTP_FROM not set - email notifications will not work")
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
