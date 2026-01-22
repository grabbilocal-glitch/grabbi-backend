package firebase

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
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
