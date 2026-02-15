package utils

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func init() {
	os.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests")
}

func TestGenerateToken(t *testing.T) {
	userID := uuid.New()
	email := "tokengen@test.com"
	role := "customer"

	token, err := GenerateToken(userID, email, role, nil)
	if err != nil {
		t.Fatalf("expected no error generating token, got: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token string")
	}

	// Verify the token has three parts (header.payload.signature)
	parts := 0
	for _, c := range token {
		if c == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("expected JWT with 2 dots, got %d dots", parts)
	}
}

func TestValidateToken(t *testing.T) {
	userID := uuid.New()
	email := "validate@test.com"
	role := "admin"
	franchiseID := uuid.New()

	token, err := GenerateToken(userID, email, role, &franchiseID)
	if err != nil {
		t.Fatalf("expected no error generating token, got: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("expected no error validating token, got: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected user_id %s, got %s", userID, claims.UserID)
	}
	if claims.Email != email {
		t.Errorf("expected email %s, got %s", email, claims.Email)
	}
	if claims.Role != role {
		t.Errorf("expected role %s, got %s", role, claims.Role)
	}
	if claims.FranchiseID == nil || *claims.FranchiseID != franchiseID {
		t.Errorf("expected franchise_id %s, got %v", franchiseID, claims.FranchiseID)
	}
	if claims.Issuer != "grabbi-backend" {
		t.Errorf("expected issuer 'grabbi-backend', got %s", claims.Issuer)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	secret := os.Getenv("JWT_SECRET")
	userID := uuid.New()

	claims := Claims{
		UserID: userID,
		Email:  "expired@test.com",
		Role:   "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "grabbi-backend",
		},
	}

	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, err := tokenObj.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	_, err = ValidateToken(expiredToken)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestTokenWithoutFranchiseID(t *testing.T) {
	userID := uuid.New()
	email := "nofranchise@test.com"
	role := "customer"

	token, err := GenerateToken(userID, email, role, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if claims.FranchiseID != nil {
		t.Errorf("expected nil franchise_id, got %v", claims.FranchiseID)
	}
}
