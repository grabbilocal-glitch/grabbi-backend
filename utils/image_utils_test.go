package utils

import "testing"

func TestExtractObjectPathValid(t *testing.T) {
	path, err := ExtractObjectPath("https://storage.googleapis.com/my-bucket/products/image.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if path != "products/image.jpg" {
		t.Errorf("expected 'products/image.jpg', got '%s'", path)
	}
}

func TestExtractObjectPathInvalidPrefix(t *testing.T) {
	_, err := ExtractObjectPath("https://example.com/my-bucket/products/image.jpg")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}
}

func TestExtractObjectPathNoBucketSeparator(t *testing.T) {
	_, err := ExtractObjectPath("https://storage.googleapis.com/nobucket")
	if err == nil {
		t.Fatal("expected error for no bucket separator")
	}
}
