package utils

import (
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/go-playground/validator/v10"
)

// AllowedImageContentTypes is the set of allowed content types for image uploads.
var AllowedImageContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// MaxUploadSize is the maximum allowed file size for uploads (5MB).
const MaxUploadSize = 5 << 20 // 5MB

// ValidateFileUpload checks that the uploaded file has a valid image content type
// and does not exceed the maximum file size.
func ValidateFileUpload(fh *multipart.FileHeader) error {
	// Check file size
	if fh.Size > MaxUploadSize {
		return fmt.Errorf("file size %d bytes exceeds maximum allowed size of 5MB", fh.Size)
	}

	// Check content type
	contentType := fh.Header.Get("Content-Type")
	if !AllowedImageContentTypes[contentType] {
		return fmt.Errorf("invalid file type '%s'; allowed types: image/jpeg, image/png, image/webp, image/gif", contentType)
	}

	return nil
}

// SanitizeValidationError takes a validator error and returns a user-friendly message
// without leaking internal Go struct names.
func SanitizeValidationError(err error) string {
	if err == nil {
		return ""
	}

	// Try to cast to validator.ValidationErrors
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		// If it's not a validation error, return a generic message
		// Check for common binding error patterns
		errMsg := err.Error()
		if strings.Contains(errMsg, "cannot unmarshal") || strings.Contains(errMsg, "invalid character") {
			return "Invalid request body"
		}
		return "Invalid request body"
	}

	// Build user-friendly error messages from field-level errors
	var messages []string
	for _, fe := range validationErrors {
		field := strings.ToLower(fe.Field())
		switch fe.Tag() {
		case "required":
			messages = append(messages, fmt.Sprintf("%s is required", field))
		case "email":
			messages = append(messages, fmt.Sprintf("%s must be a valid email address", field))
		case "min":
			messages = append(messages, fmt.Sprintf("%s must be at least %s characters", field, fe.Param()))
		case "max":
			messages = append(messages, fmt.Sprintf("%s must be at most %s characters", field, fe.Param()))
		default:
			messages = append(messages, fmt.Sprintf("%s is invalid", field))
		}
	}

	if len(messages) == 0 {
		return "Invalid request body"
	}

	return strings.Join(messages, "; ")
}
