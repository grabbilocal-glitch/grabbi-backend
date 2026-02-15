package utils

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID      uuid.UUID  `json:"user_id"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	FranchiseID *uuid.UUID `json:"franchise_id,omitempty"`
	jwt.RegisteredClaims
}

func getJWTSecret() string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		panic("FATAL: JWT_SECRET environment variable is not set. Refusing to start with an insecure configuration.")
	}
	return secret
}

func GenerateToken(userID uuid.UUID, email, role string, franchiseID *uuid.UUID) (string, error) {
	secret := getJWTSecret()

	claims := Claims{
		UserID:      userID,
		Email:       email,
		Role:        role,
		FranchiseID: franchiseID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "grabbi-backend",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func GenerateRefreshToken(userID uuid.UUID, email, role string, franchiseID *uuid.UUID) (string, error) {
	secret := getJWTSecret()

	claims := Claims{
		UserID:      userID,
		Email:       email,
		Role:        role,
		FranchiseID: franchiseID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "grabbi-refresh",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateToken(tokenString string) (*Claims, error) {
	secret := getJWTSecret()

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}
