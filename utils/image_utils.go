package utils

import (
	"fmt"
	"strings"
)

// ExtractObjectPath extracts storage object path from full Firebase URL
func ExtractObjectPath(url string) (string, error) {
	const prefix = "https://storage.googleapis.com/"
	if !strings.HasPrefix(url, prefix) {
		return "", fmt.Errorf("invalid URL")
	}

	// Remove prefix and bucket name
	path := strings.TrimPrefix(url, prefix)
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid URL format")
	}

	return parts[1], nil
}