package firebase

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
	"github.com/google/uuid"
)

var App *firebase.App

func Init() {
	credJSON := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	var opts []option.ClientOption

	if credJSON != "" {
		if strings.HasPrefix(credJSON, "{") {
			log.Println("Using Firebase credentials from environment variable")
			opts = append(opts, option.WithCredentialsJSON([]byte(credJSON)))
		} else {
			// It's a file path
			log.Println("Using Firebase credentials from file:", credJSON)
			opts = append(opts, option.WithCredentialsFile(credJSON))
		}
	} else {
		log.Println("Warning: GOOGLE_APPLICATION_CREDENTIALS not set, using default credentials")
	}

	app, err := firebase.NewApp(context.Background(), nil, opts...)
	if err != nil {
		log.Fatalf("Firebase init failed: %v", err)
	}

	App = app
	log.Println("Firebase initialized successfully")
}

func UploadProductImage(
	file multipart.File,
	filename string,
	contentType string,
) (string, error) {

	ctx := context.Background()
	bucketName := os.Getenv("FIREBASE_STORAGE_BUCKET")
	if bucketName == "" {
		return "", fmt.Errorf("FIREBASE_STORAGE_BUCKET not set")
	}

	client, err := App.Storage(ctx)
	if err != nil {
		return "", err
	}

	objectPath := fmt.Sprintf(
		"products/%d_%s",
		time.Now().Unix(),
		filename,
	)

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return "", err
	}

	wc := bucket.Object(objectPath).NewWriter(ctx)
	wc.ContentType = contentType
	defer wc.Close()

	if _, err := io.Copy(wc, file); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"https://storage.googleapis.com/%s/%s",
		bucketName,
		objectPath,
	), nil
}

// DeleteFile deletes a file from Firebase Storage given its object path
func DeleteFile(objectPath string) error {
	if App == nil {
		return fmt.Errorf("firebase app not initialized")
	}

	ctx := context.Background()
	bucketName := os.Getenv("FIREBASE_STORAGE_BUCKET")
	if bucketName == "" {
		return fmt.Errorf("FIREBASE_STORAGE_BUCKET not set")
	}

	client, err := App.Storage(ctx)
	if err != nil {
		return err
	}

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return err
	}

	obj := bucket.Object(objectPath)
	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object %s: %v", objectPath, err)
	}

	log.Printf("Deleted file %s from bucket %s", objectPath, bucketName)
	return nil
}

func UploadPromotionImage(
	file multipart.File,
	filename string,
	contentType string,
) (string, error) {

	if App == nil {
		return "", fmt.Errorf("firebase app not initialized")
	}

	ctx := context.Background()
	bucketName := os.Getenv("FIREBASE_STORAGE_BUCKET")
	if bucketName == "" {
		return "", fmt.Errorf("FIREBASE_STORAGE_BUCKET not set")
	}

	client, err := App.Storage(ctx)
	if err != nil {
		return "", err
	}

	objectPath := fmt.Sprintf(
		"promotions/%d_%s",
		time.Now().Unix(),
		filename,
	)

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return "", err
	}

	wc := bucket.Object(objectPath).NewWriter(ctx)
	wc.ContentType = contentType
	defer wc.Close()

	if _, err := io.Copy(wc, file); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"https://storage.googleapis.com/%s/%s",
		bucketName,
		objectPath,
	), nil
}

// DownloadAndUploadImage downloads an image from URL and uploads to Firebase Storage
// Returns the Firebase storage URL
func DownloadAndUploadImage(imageURL string, productID string) (string, error) {
	if App == nil {
		return "", fmt.Errorf("firebase app not initialized")
	}

	ctx := context.Background()
	bucketName := os.Getenv("FIREBASE_STORAGE_BUCKET")
	if bucketName == "" {
		return "", fmt.Errorf("FIREBASE_STORAGE_BUCKET not set")
	}

	// Download image from URL
	client, err := App.Storage(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get storage client: %v", err)
	}

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return "", fmt.Errorf("failed to get bucket: %v", err)
	}

	// Generate object path for Firebase - use productID for uniqueness instead of timestamp
	// This prevents concurrent uploads from overwriting each other
	objectPath := fmt.Sprintf(
		"products/%s_%s.jpg",
		productID,
		uuid.New().String()[:8], // Use first 8 chars of UUID for uniqueness
	)

	// Create HTTP client to download image
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := httpClient.Get(imageURL)
	if err != nil {
		return "", fmt.Errorf("failed to download image from %s: %v", imageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download image: HTTP %d", resp.StatusCode)
	}

	// Check content type - must be an image
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return "", fmt.Errorf("no content-type header returned from %s", imageURL)
	}

	// Validate that we actually got an image, not HTML or other content
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("URL %s returned non-image content-type: %s (expected image/*)", imageURL, contentType)
	}

	// Upload to Firebase
	wc := bucket.Object(objectPath).NewWriter(ctx)
	wc.ContentType = contentType
	defer wc.Close()

	if _, err := io.Copy(wc, resp.Body); err != nil {
		return "", fmt.Errorf("failed to upload image to Firebase: %v", err)
	}

	return fmt.Sprintf(
		"https://storage.googleapis.com/%s/%s",
		bucketName,
		objectPath,
	), nil
}
