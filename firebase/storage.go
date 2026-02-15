package firebase

import "mime/multipart"

// StorageClient abstracts Firebase Storage operations for dependency injection and testing.
type StorageClient interface {
	UploadProductImage(file multipart.File, filename, contentType string) (string, error)
	UploadPromotionImage(file multipart.File, filename, contentType string) (string, error)
	DeleteFile(objectPath string) error
	DownloadAndUploadImage(imageURL, productID string) (string, error)
}

// FirebaseStorageClient is the real implementation that delegates to package-level functions.
type FirebaseStorageClient struct{}

func NewStorageClient() StorageClient {
	return &FirebaseStorageClient{}
}

func (f *FirebaseStorageClient) UploadProductImage(file multipart.File, filename, contentType string) (string, error) {
	return UploadProductImage(file, filename, contentType)
}

func (f *FirebaseStorageClient) UploadPromotionImage(file multipart.File, filename, contentType string) (string, error) {
	return UploadPromotionImage(file, filename, contentType)
}

func (f *FirebaseStorageClient) DeleteFile(objectPath string) error {
	return DeleteFile(objectPath)
}

func (f *FirebaseStorageClient) DownloadAndUploadImage(imageURL, productID string) (string, error) {
	return DownloadAndUploadImage(imageURL, productID)
}
