package firebase

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	firebase "firebase.google.com/go"
	"github.com/google/uuid"
	"google.golang.org/api/option"
)

var App *firebase.App

// sanitizeFilename removes special characters from filenames and limits length.
func sanitizeFilename(filename string) string {
	// Replace path separators and other dangerous characters
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := re.ReplaceAllString(filename, "_")

	// Limit length to 100 characters
	if len(sanitized) > 100 {
		sanitized = sanitized[:100]
	}

	// Ensure it's not empty
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		sanitized = "file"
	}

	return sanitized
}

// isPrivateIP checks whether an IP address is a private/reserved address.
func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		network *net.IPNet
	}{
		{parseCIDR("10.0.0.0/8")},
		{parseCIDR("172.16.0.0/12")},
		{parseCIDR("192.168.0.0/16")},
		{parseCIDR("127.0.0.0/8")},
		{parseCIDR("169.254.0.0/16")},
		{parseCIDR("0.0.0.0/8")},
		{parseCIDR("::1/128")},
		{parseCIDR("fc00::/7")},
		{parseCIDR("fe80::/10")},
	}

	for _, r := range privateRanges {
		if r.network.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(fmt.Sprintf("invalid CIDR: %s", cidr))
	}
	return network
}

// validateExternalURL validates that a URL is safe to fetch (prevents SSRF).
func validateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	// Only allow http and https schemes
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL scheme '%s' is not allowed; only http and https are permitted", parsed.Scheme)
	}

	// Resolve hostname to IP addresses
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no hostname")
	}

	// Check for localhost
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("requests to localhost are not allowed")
	}

	// Resolve DNS
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname '%s': %v", host, err)
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("URL resolves to private IP address %s, which is not allowed", ip.String())
		}
	}

	return nil
}

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
		sanitizeFilename(filename),
	)

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return "", err
	}

	obj := bucket.Object(objectPath)
	wc := obj.NewWriter(ctx)
	wc.ContentType = contentType

	if _, err := io.Copy(wc, file); err != nil {
		wc.Close()
		return "", err
	}

	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize upload: %v", err)
	}

	// Make object publicly readable so the URL works without authentication
	if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		log.Printf("Warning: failed to set public ACL on %s: %v", objectPath, err)
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
		sanitizeFilename(filename),
	)

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return "", err
	}

	obj := bucket.Object(objectPath)
	wc := obj.NewWriter(ctx)
	wc.ContentType = contentType

	if _, err := io.Copy(wc, file); err != nil {
		wc.Close()
		return "", err
	}

	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize upload: %v", err)
	}

	// Make object publicly readable so the URL works without authentication
	if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		log.Printf("Warning: failed to set public ACL on %s: %v", objectPath, err)
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

	// SSRF prevention: validate the URL before fetching
	if err := validateExternalURL(imageURL); err != nil {
		return "", fmt.Errorf("URL validation failed for %s: %v", imageURL, err)
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
		sanitizeFilename(productID),
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
	obj := bucket.Object(objectPath)
	wc := obj.NewWriter(ctx)
	wc.ContentType = contentType

	if _, err := io.Copy(wc, resp.Body); err != nil {
		wc.Close()
		return "", fmt.Errorf("failed to upload image to Firebase: %v", err)
	}

	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize upload: %v", err)
	}

	// Make object publicly readable so the URL works without authentication
	if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		log.Printf("Warning: failed to set public ACL on %s: %v", objectPath, err)
	}

	return fmt.Sprintf(
		"https://storage.googleapis.com/%s/%s",
		bucketName,
		objectPath,
	), nil
}
