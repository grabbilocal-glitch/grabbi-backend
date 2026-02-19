package handlers

import "mime/multipart"

type mockStorage struct {
	UploadProductImageFn        func(file multipart.File, filename, contentType string) (string, error)
	UploadPromotionImageFn      func(file multipart.File, filename, contentType string) (string, error)
	DeleteFileFn                func(objectPath string) error
	DownloadAndUploadImageFn    func(imageURL, productID string) (string, error)
	CopyImageToOrderStorageFn   func(sourceImageURL, orderID, productID string) (string, error)
	DeleteFileCalls             []string
	UploadCallCount             int
	CopyImageToOrderStorageCalls []struct {
		SourceImageURL string
		OrderID        string
		ProductID      string
	}
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		DeleteFileCalls: []string{},
	}
}

func (m *mockStorage) UploadProductImage(file multipart.File, filename, contentType string) (string, error) {
	m.UploadCallCount++
	if m.UploadProductImageFn != nil {
		return m.UploadProductImageFn(file, filename, contentType)
	}
	return "https://storage.googleapis.com/test-bucket/products/test_image.jpg", nil
}

func (m *mockStorage) UploadPromotionImage(file multipart.File, filename, contentType string) (string, error) {
	m.UploadCallCount++
	if m.UploadPromotionImageFn != nil {
		return m.UploadPromotionImageFn(file, filename, contentType)
	}
	return "https://storage.googleapis.com/test-bucket/promotions/test_image.jpg", nil
}

func (m *mockStorage) DeleteFile(objectPath string) error {
	m.DeleteFileCalls = append(m.DeleteFileCalls, objectPath)
	if m.DeleteFileFn != nil {
		return m.DeleteFileFn(objectPath)
	}
	return nil
}

func (m *mockStorage) DownloadAndUploadImage(imageURL, productID string) (string, error) {
	m.UploadCallCount++
	if m.DownloadAndUploadImageFn != nil {
		return m.DownloadAndUploadImageFn(imageURL, productID)
	}
	return "https://storage.googleapis.com/test-bucket/products/" + productID + "_image.jpg", nil
}

func (m *mockStorage) CopyImageToOrderStorage(sourceImageURL, orderID, productID string) (string, error) {
	m.CopyImageToOrderStorageCalls = append(m.CopyImageToOrderStorageCalls, struct {
		SourceImageURL string
		OrderID        string
		ProductID      string
	}{sourceImageURL, orderID, productID})
	if m.CopyImageToOrderStorageFn != nil {
		return m.CopyImageToOrderStorageFn(sourceImageURL, orderID, productID)
	}
	return "https://storage.googleapis.com/test-bucket/orders/" + orderID + "/" + productID + "_image.jpg", nil
}
