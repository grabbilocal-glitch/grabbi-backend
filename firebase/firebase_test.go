package firebase

import (
	"net"
	"strings"
	"testing"
)

func TestSanitizeFilenameNormal(t *testing.T) {
	result := sanitizeFilename("image_test-file.jpg")
	if result != "image_test-file.jpg" {
		t.Errorf("expected 'image_test-file.jpg', got '%s'", result)
	}
}

func TestSanitizeFilenameSpecialChars(t *testing.T) {
	result := sanitizeFilename("my file (1)@#$.jpg")
	if strings.ContainsAny(result, " ()@#$") {
		t.Errorf("special chars not replaced: '%s'", result)
	}
}

func TestSanitizeFilenameTooLong(t *testing.T) {
	long := strings.Repeat("a", 200)
	result := sanitizeFilename(long)
	if len(result) != 100 {
		t.Errorf("expected length 100, got %d", len(result))
	}
}

func TestSanitizeFilenameEmpty(t *testing.T) {
	result := sanitizeFilename("")
	if result != "file" {
		t.Errorf("expected 'file', got '%s'", result)
	}
}

func TestSanitizeFilenameDots(t *testing.T) {
	if sanitizeFilename(".") != "file" {
		t.Error("single dot should become 'file'")
	}
	if sanitizeFilename("..") != "file" {
		t.Error("double dots should become 'file'")
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}
	for _, tc := range tests {
		ip := net.ParseIP(tc.ip)
		result := isPrivateIP(ip)
		if result != tc.expected {
			t.Errorf("isPrivateIP(%s) = %v, want %v", tc.ip, result, tc.expected)
		}
	}
}

func TestIsPrivateIPv6(t *testing.T) {
	ip := net.ParseIP("::1")
	if !isPrivateIP(ip) {
		t.Error("::1 should be private")
	}
}

func TestParseCIDRValid(t *testing.T) {
	result := parseCIDR("10.0.0.0/8")
	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestParseCIDRInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid CIDR")
		}
	}()
	parseCIDR("not-a-cidr")
}

func TestValidateExternalURLInvalidScheme(t *testing.T) {
	err := validateExternalURL("ftp://example.com/file.txt")
	if err == nil {
		t.Error("expected error for ftp scheme")
	}
}

func TestValidateExternalURLLocalhost(t *testing.T) {
	err := validateExternalURL("http://localhost/image.jpg")
	if err == nil {
		t.Error("expected error for localhost")
	}
}

func TestValidateExternalURLEmptyHost(t *testing.T) {
	err := validateExternalURL("http:///path")
	if err == nil {
		t.Error("expected error for empty host")
	}
}

func TestValidateExternalURLValidHTTPS(t *testing.T) {
	// This test does DNS resolution, so only run if network is available
	err := validateExternalURL("https://example.com/image.jpg")
	if err != nil {
		t.Skipf("skipping due to DNS resolution requirement: %v", err)
	}
}
