package utils

import (
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestSanitizeValidationErrorEmail(t *testing.T) {
	// Simulate a validator.ValidationErrors for an email field
	validate := validator.New()

	type TestReq struct {
		Email string `validate:"required,email"`
	}

	err := validate.Struct(TestReq{Email: "not-an-email"})
	if err == nil {
		t.Fatal("expected validation error for invalid email")
	}

	msg := SanitizeValidationError(err)
	if !strings.Contains(msg, "email") {
		t.Errorf("expected error message to mention email, got: %s", msg)
	}
	if !strings.Contains(msg, "valid email address") {
		t.Errorf("expected user-friendly email error, got: %s", msg)
	}
}

func TestSanitizeValidationErrorRequired(t *testing.T) {
	validate := validator.New()

	type TestReq struct {
		Name     string `validate:"required"`
		Password string `validate:"required,min=8"`
	}

	err := validate.Struct(TestReq{})
	if err == nil {
		t.Fatal("expected validation error for missing required fields")
	}

	msg := SanitizeValidationError(err)
	if !strings.Contains(msg, "required") {
		t.Errorf("expected error message to mention 'required', got: %s", msg)
	}
}

func TestSanitizeValidationErrorNilReturnsEmpty(t *testing.T) {
	msg := SanitizeValidationError(nil)
	if msg != "" {
		t.Errorf("expected empty string for nil error, got: %s", msg)
	}
}

func TestSanitizeValidationErrorMinLength(t *testing.T) {
	validate := validator.New()

	type TestReq struct {
		Password string `validate:"required,min=8"`
	}

	err := validate.Struct(TestReq{Password: "short"})
	if err == nil {
		t.Fatal("expected validation error")
	}

	msg := SanitizeValidationError(err)
	if !strings.Contains(msg, "at least") {
		t.Errorf("expected min length message, got: %s", msg)
	}
}

func TestValidateFileUploadValidJPEG(t *testing.T) {
	header := &multipart.FileHeader{
		Filename: "test.jpg",
		Size:     1024,
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "image/jpeg")

	err := ValidateFileUpload(header)
	if err != nil {
		t.Errorf("expected no error for valid JPEG, got: %v", err)
	}
}

func TestValidateFileUploadTooLarge(t *testing.T) {
	header := &multipart.FileHeader{
		Filename: "huge.jpg",
		Size:     10 << 20, // 10MB
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "image/jpeg")

	err := ValidateFileUpload(header)
	if err == nil {
		t.Error("expected error for file exceeding max size")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected size error, got: %v", err)
	}
}

func TestValidateFileUploadInvalidType(t *testing.T) {
	header := &multipart.FileHeader{
		Filename: "document.pdf",
		Size:     1024,
		Header:   make(textproto.MIMEHeader),
	}
	header.Header.Set("Content-Type", "application/pdf")

	err := ValidateFileUpload(header)
	if err == nil {
		t.Error("expected error for invalid file type")
	}
	if !strings.Contains(err.Error(), "invalid file type") {
		t.Errorf("expected content type error, got: %v", err)
	}
}

func TestValidateFileUploadAllowedTypes(t *testing.T) {
	allowedTypes := []string{"image/jpeg", "image/png", "image/webp", "image/gif"}

	for _, ct := range allowedTypes {
		header := &multipart.FileHeader{
			Filename: "test.img",
			Size:     1024,
			Header:   make(textproto.MIMEHeader),
		}
		header.Header.Set("Content-Type", ct)

		err := ValidateFileUpload(header)
		if err != nil {
			t.Errorf("expected no error for content type %s, got: %v", ct, err)
		}
	}
}
