package config

import (
	"os"
	"testing"
)

func TestLoadEnv(t *testing.T) {
	// LoadEnv returns nil when no .env file exists
	err := LoadEnv()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateEnvAllSet(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("DATABASE_URL", "test-db-url")
	defer os.Unsetenv("JWT_SECRET")
	defer os.Unsetenv("DATABASE_URL")

	err := ValidateEnv()
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateEnvMissingJWTSecret(t *testing.T) {
	os.Unsetenv("JWT_SECRET")
	os.Setenv("DATABASE_URL", "test-db-url")
	defer os.Unsetenv("DATABASE_URL")

	err := ValidateEnv()
	if err == nil {
		t.Error("expected error for missing JWT_SECRET")
	}
}

func TestValidateEnvMissingDatabaseURL(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret")
	os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("JWT_SECRET")

	err := ValidateEnv()
	if err == nil {
		t.Error("expected error for missing DATABASE_URL")
	}
}

func TestValidateEnvMissingBoth(t *testing.T) {
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("DATABASE_URL")

	err := ValidateEnv()
	if err == nil {
		t.Error("expected error for missing both")
	}
}

func TestGetEnvExisting(t *testing.T) {
	os.Setenv("TEST_GET_ENV_KEY", "test-value")
	defer os.Unsetenv("TEST_GET_ENV_KEY")

	result := GetEnv("TEST_GET_ENV_KEY", "default")
	if result != "test-value" {
		t.Errorf("expected 'test-value', got '%s'", result)
	}
}

func TestGetEnvMissing(t *testing.T) {
	os.Unsetenv("TEST_GET_ENV_MISSING")
	result := GetEnv("TEST_GET_ENV_MISSING", "fallback")
	if result != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", result)
	}
}
