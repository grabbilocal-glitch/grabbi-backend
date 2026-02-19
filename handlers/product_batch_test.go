package handlers

import (
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"grabbi-backend/dtos"
	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/google/uuid"
)

// strPtr is a helper function to get a pointer to a string literal
func strPtr(s string) *string {
	return &s
}

// waitForJob polls the job store until the batch job completes or times out.
func waitForJob(jobID string, timeout time.Duration) {
	id, err := uuid.Parse(jobID)
	if err != nil {
		return
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, ok := utils.Store.GetJob(id)
		if ok && (job.Status == dtos.JobStatusCompleted || job.Status == dtos.JobStatusFailed) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// ==================== parseImageURLs Tests ====================

func TestParseImageURLsStringArray(t *testing.T) {
	result := parseImageURLs([]string{"url1", "url2"})
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestParseImageURLsCommaSeparated(t *testing.T) {
	result := parseImageURLs("url1,url2,url3")
	if len(result) != 3 {
		t.Errorf("expected 3, got %d", len(result))
	}
}

func TestParseImageURLsNewlineSeparated(t *testing.T) {
	result := parseImageURLs("url1\nurl2\nurl3")
	if len(result) != 3 {
		t.Errorf("expected 3, got %d", len(result))
	}
}

func TestParseImageURLsInterfaceArray(t *testing.T) {
	result := parseImageURLs([]interface{}{"url1", "url2"})
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestParseImageURLsEmpty(t *testing.T) {
	result := parseImageURLs("")
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestParseImageURLsNil(t *testing.T) {
	result := parseImageURLs(nil)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestParseImageURLsMixedSeparators(t *testing.T) {
	result := parseImageURLs("url1\nurl2,url3\r\nurl4")
	if len(result) != 4 {
		t.Errorf("expected 4, got %d", len(result))
	}
}

// ==================== cleanURL Tests ====================

func TestCleanURLTrailingComma(t *testing.T) {
	if cleanURL("http://example.com,") != "http://example.com" {
		t.Error("failed to clean trailing comma")
	}
}

func TestCleanURLWhitespace(t *testing.T) {
	if cleanURL("  http://example.com  ") != "http://example.com" {
		t.Error("failed to clean whitespace")
	}
}

func TestCleanURLNewlines(t *testing.T) {
	if cleanURL("http://example.com\n") != "http://example.com" {
		t.Error("failed to clean newlines")
	}
}

func TestCleanURLCarriageReturn(t *testing.T) {
	if cleanURL("http://example.com\r\n") != "http://example.com" {
		t.Error("failed to clean carriage return")
	}
}

func TestCleanURLAlreadyClean(t *testing.T) {
	if cleanURL("http://example.com") != "http://example.com" {
		t.Error("clean URL should remain unchanged")
	}
}

// ==================== Batch Import Tests ====================

func TestBatchImportProductsSuccess(t *testing.T) {
	db := freshDB()
	router := setupProductRouter(db)
	cat := seedCategory(db, "TestCat")
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	body := map[string]interface{}{
		"products": []map[string]interface{}{
			{
				"item_name":      "Batch Product",
				"cost_price":     5.0,
				"retail_price":   10.0,
				"stock_quantity": 50,
				"reorder_level":  10,
				"category_id":   cat.ID.String(),
				"status":        "active",
			},
		},
	}

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/products/batch", body, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 202 {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["job_id"] == nil {
		t.Error("expected job_id in response")
	}
	if resp["status"] != "processing" {
		t.Errorf("expected status 'processing', got %v", resp["status"])
	}
	// Wait for the background goroutine to finish to avoid test isolation issues
	if jobID, ok := resp["job_id"].(string); ok {
		waitForJob(jobID, 5*time.Second)
	}
}

func TestBatchImportProductsValidationError(t *testing.T) {
	db := freshDB()
	router := setupProductRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	// Empty products array should fail validation (min=1)
	body := map[string]interface{}{
		"products": []map[string]interface{}{},
	}

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/products/batch", body, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBatchImportProductsMissingProducts(t *testing.T) {
	db := freshDB()
	router := setupProductRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	// No products field at all
	body := map[string]interface{}{}

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/products/batch", body, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== GetBatchJobStatus Tests ====================

func TestGetBatchJobStatusNotFound(t *testing.T) {
	db := freshDB()
	router := setupProductRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	fakeJobID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/admin/products/batch/"+fakeJobID.String(), nil, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetBatchJobStatusInvalidID(t *testing.T) {
	db := freshDB()
	router := setupProductRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/admin/products/batch/not-a-uuid", nil, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBatchImportMultipleProducts(t *testing.T) {
	db := freshDB()
	router := setupProductRouter(db)
	cat := seedCategory(db, "TestCat")
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	body := map[string]interface{}{
		"products": []map[string]interface{}{
			{
				"item_name":      "Batch Product 1",
				"cost_price":     5.0,
				"retail_price":   10.0,
				"stock_quantity": 50,
				"reorder_level":  10,
				"category_id":   cat.ID.String(),
				"status":        "active",
			},
			{
				"item_name":      "Batch Product 2",
				"cost_price":     3.0,
				"retail_price":   8.0,
				"stock_quantity": 25,
				"reorder_level":  5,
				"category_id":   cat.ID.String(),
				"status":        "active",
			},
		},
	}

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/products/batch", body, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 202 {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	total, ok := resp["total"].(float64)
	if !ok || int(total) != 2 {
		t.Errorf("expected total 2, got %v", resp["total"])
	}
	// Wait for the background goroutine to finish to avoid test isolation issues
	if jobID, ok := resp["job_id"].(string); ok {
		waitForJob(jobID, 5*time.Second)
	}
}

// ==================== Direct Function Tests ====================

func TestUploadImagesConcurrently(t *testing.T) {
	mock := newMockStorage()
	productID := uuid.New().String()
	urls := []string{"https://example.com/img1.jpg", "https://example.com/img2.jpg", "https://example.com/img3.jpg"}

	images, errors := uploadImagesConcurrently(mock, productID, urls)

	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(images))
	}
	for _, err := range errors {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
	// First image should be primary
	if !images[0].IsPrimary {
		t.Error("expected first image to be primary")
	}
	if images[1].IsPrimary {
		t.Error("expected second image to not be primary")
	}
	if mock.UploadCallCount != 3 {
		t.Errorf("expected 3 upload calls, got %d", mock.UploadCallCount)
	}
}

func TestUploadImagesConcurrentlyWithErrors(t *testing.T) {
	mock := newMockStorage()
	callCount := 0
	mock.DownloadAndUploadImageFn = func(imageURL, productID string) (string, error) {
		callCount++
		if callCount == 2 {
			return "", fmt.Errorf("download failed")
		}
		return "https://storage.googleapis.com/bucket/img.jpg", nil
	}

	images, errors := uploadImagesConcurrently(mock, uuid.New().String(), []string{"url1", "url2", "url3"})

	hasError := false
	for _, err := range errors {
		if err != nil {
			hasError = true
		}
	}
	if !hasError {
		t.Error("expected at least one error")
	}

	// Check that successful uploads still have URLs
	successCount := 0
	for _, img := range images {
		if img.ImageURL != "" {
			successCount++
		}
	}
	if successCount != 2 {
		t.Errorf("expected 2 successful uploads, got %d", successCount)
	}
}

func TestPrepareProductForBatchNewProduct(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mutex := &sync.Mutex{}

	data := dtos.ProductImportItem{
		SKU:           "BATCH-SKU-001",
		ItemName:      "Batch Product",
		CostPrice:     5.00,
		RetailPrice:   10.00,
		StockQuantity: 50,
		ReorderLevel:  10,
		CategoryID:    cat.ID.String(),
		Status:        "active",
		Brand:         "TestBrand",
	}

	product, images, fieldsChanged, imagesChanged, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.ItemName != "Batch Product" {
		t.Errorf("expected 'Batch Product', got %s", product.ItemName)
	}
	if product.ID == uuid.Nil {
		t.Error("expected product ID to be set")
	}
	// New product, so fieldsChanged should be false (only relevant for updates)
	_ = fieldsChanged
	_ = imagesChanged
	_ = images
}

func TestPrepareProductForBatchExistingBySKU(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	existingProd := seedProduct(db, "OldName", cat.ID, 5.00)
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mutex := &sync.Mutex{}

	data := dtos.ProductImportItem{
		SKU:           existingProd.SKU,
		ItemName:      "NewName",
		CostPrice:     6.00,
		RetailPrice:   12.00,
		StockQuantity: 75,
		ReorderLevel:  15,
		CategoryID:    cat.ID.String(),
		Status:        "active",
	}

	product, _, fieldsChanged, _, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.ID != existingProd.ID {
		t.Errorf("expected existing product ID %s, got %s", existingProd.ID, product.ID)
	}
	if product.ItemName != "NewName" {
		t.Errorf("expected 'NewName', got %s", product.ItemName)
	}
	if !fieldsChanged {
		t.Error("expected fieldsChanged to be true")
	}
}

func TestPrepareProductForBatchExistingByID(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	existingProd := seedProduct(db, "OldName", cat.ID, 5.00)
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mutex := &sync.Mutex{}

	idStr := existingProd.ID.String()
	data := dtos.ProductImportItem{
		ID:            &idStr,
		SKU:           existingProd.SKU,
		ItemName:      "Updated By ID",
		CostPrice:     6.00,
		RetailPrice:   12.00,
		StockQuantity: 75,
		ReorderLevel:  15,
		CategoryID:    cat.ID.String(),
		Status:        "active",
	}

	product, _, fieldsChanged, _, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.ID != existingProd.ID {
		t.Error("expected same product ID for update by ID")
	}
	if !fieldsChanged {
		t.Error("expected fieldsChanged to be true")
	}
}

func TestPrepareProductForBatchInvalidCategoryID(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{}
	mutex := &sync.Mutex{}

	data := dtos.ProductImportItem{
		SKU:         "BAD-CAT",
		ItemName:    "Bad Cat",
		CostPrice:   5.00,
		RetailPrice: 10.00,
		CategoryID:  "not-a-uuid",
		Status:      "active",
	}

	_, _, _, _, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err == nil {
		t.Fatal("expected error for invalid category ID")
	}
}

func TestPrepareProductForBatchCategoryNotFound(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{}
	mutex := &sync.Mutex{}

	data := dtos.ProductImportItem{
		SKU:         "MISS-CAT",
		ItemName:    "Missing Cat",
		CostPrice:   5.00,
		RetailPrice: 10.00,
		CategoryID:  uuid.New().String(),
		Status:      "active",
	}

	_, _, _, _, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err == nil {
		t.Fatal("expected error for category not found")
	}
}

func TestPrepareProductForBatchWithImages(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mutex := &sync.Mutex{}

	data := dtos.ProductImportItem{
		SKU:            "IMG-SKU",
		ItemName:       "Image Product",
		CostPrice:      5.00,
		RetailPrice:    10.00,
		StockQuantity:  50,
		ReorderLevel:   10,
		CategoryID:     cat.ID.String(),
		Status:         "active",
		ImageURLs:      []string{"https://example.com/img1.jpg", "https://example.com/img2.jpg"},
		ImagesProvided: true,
	}

	product, images, _, _, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.ItemName != "Image Product" {
		t.Errorf("expected 'Image Product', got %s", product.ItemName)
	}
	if len(images) != 2 {
		t.Errorf("expected 2 images, got %d", len(images))
	}
}

func TestPrepareProductForBatchWithDates(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mutex := &sync.Mutex{}

	data := dtos.ProductImportItem{
		SKU:            "DATE-SKU",
		ItemName:       "Date Product",
		CostPrice:      5.00,
		RetailPrice:    10.00,
		StockQuantity:  50,
		ReorderLevel:   10,
		CategoryID:     cat.ID.String(),
		Status:         "active",
		PromotionStart: "2026-01-01",
		PromotionEnd:   "2026-12-31",
		ExpiryDate:     "2027-06-30",
	}

	product, _, _, _, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.PromotionStart == nil {
		t.Error("expected PromotionStart to be set")
	}
	if product.PromotionEnd == nil {
		t.Error("expected PromotionEnd to be set")
	}
	if product.ExpiryDate == nil {
		t.Error("expected ExpiryDate to be set")
	}
}

func TestPrepareProductForBatchWithSubcategory(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	sub := seedSubcategory(db, "SubCat", cat.ID)
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mutex := &sync.Mutex{}

	subID := sub.ID.String()
	data := dtos.ProductImportItem{
		SKU:           "SUB-SKU",
		ItemName:      "Sub Product",
		CostPrice:     5.00,
		RetailPrice:   10.00,
		StockQuantity: 50,
		ReorderLevel:  10,
		CategoryID:    cat.ID.String(),
		SubcategoryID: &subID,
		Status:        "active",
	}

	product, _, _, _, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.SubcategoryID == nil || *product.SubcategoryID != sub.ID {
		t.Error("expected subcategory ID to be set")
	}
}

func TestPrepareProductForBatchInvalidProductID(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	categoryCache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mutex := &sync.Mutex{}

	badID := "not-a-uuid"
	data := dtos.ProductImportItem{
		ID:          &badID,
		SKU:         "BAD-PID",
		ItemName:    "Bad PID",
		CostPrice:   5.00,
		RetailPrice: 10.00,
		CategoryID:  cat.ID.String(),
		Status:      "active",
	}

	_, _, _, _, err := handler.prepareProductForBatch(data, categoryCache, mutex)
	if err == nil {
		t.Fatal("expected error for invalid product ID")
	}
}

func TestProcessBatchImportDirect(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	products := []dtos.ProductImportItem{
		{
			SKU:           "DIRECT-001",
			ItemName:      "Direct Product 1",
			CostPrice:     5.00,
			RetailPrice:   10.00,
			StockQuantity: 50,
			ReorderLevel:  10,
			CategoryID:    cat.ID.String(),
			Status:        "active",
			Barcode:       strPtr("BAR-DIRECT-001"),
		},
		{
			SKU:           "DIRECT-002",
			ItemName:      "Direct Product 2",
			CostPrice:     3.00,
			RetailPrice:   8.00,
			StockQuantity: 25,
			ReorderLevel:  5,
			CategoryID:    cat.ID.String(),
			Status:        "active",
			Barcode:       strPtr("BAR-DIRECT-002"),
		},
	}

	job := utils.Store.CreateJob(len(products))
	handler.processBatchImport(job, products, false)

	// Verify job completed
	result, exists := utils.Store.GetJob(job.ID)
	if !exists {
		t.Fatal("job not found")
	}
	if result.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}
	if result.Created != 2 {
		t.Errorf("expected 2 created, got %d", result.Created)
	}
}

func TestProcessBatchImportWithDeleteMissing(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	// Create an existing product that will be "missing" from import
	existingProd := seedProduct(db, "ToDelete", cat.ID, 5.00)

	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	// Import only one new product - the existing one should be deleted
	products := []dtos.ProductImportItem{
		{
			SKU:           "KEEP-001",
			ItemName:      "Keep Product",
			CostPrice:     5.00,
			RetailPrice:   10.00,
			StockQuantity: 50,
			ReorderLevel:  10,
			CategoryID:    cat.ID.String(),
			Status:        "active",
			Barcode:       strPtr("BAR-KEEP-001"),
		},
	}

	job := utils.Store.CreateJob(len(products))
	handler.processBatchImport(job, products, true)

	result, _ := utils.Store.GetJob(job.ID)
	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}
	// The existing product should have been deleted
	if result.Deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", result.Deleted)
	}

	// Verify it's soft-deleted
	var count int64
	db.Model(&models.Product{}).Where("id = ?", existingProd.ID).Count(&count)
	if count != 0 {
		t.Error("expected existing product to be soft-deleted")
	}
}

func TestProcessBatchImportDeleteFlag(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	products := []dtos.ProductImportItem{
		{
			SKU:           "DEL-FLAG-001",
			ItemName:      "Flagged Delete",
			CostPrice:     5.00,
			RetailPrice:   10.00,
			StockQuantity: 50,
			ReorderLevel:  10,
			CategoryID:    cat.ID.String(),
			Status:        "active",
			Delete:        true, // This product should be skipped
		},
	}

	job := utils.Store.CreateJob(len(products))
	handler.processBatchImport(job, products, false)

	result, _ := utils.Store.GetJob(job.ID)
	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}
	// Product with Delete flag should be skipped, not created
	if result.Created != 0 {
		t.Errorf("expected 0 created (delete-flagged product skipped), got %d", result.Created)
	}
}

func TestProcessBatchImportUpdateExisting(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	existingProd := seedProduct(db, "OldName", cat.ID, 5.00)
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	products := []dtos.ProductImportItem{
		{
			SKU:           existingProd.SKU,
			ItemName:      "NewName",
			CostPrice:     6.00,
			RetailPrice:   12.00,
			StockQuantity: 75,
			ReorderLevel:  15,
			CategoryID:    cat.ID.String(),
			Status:        "active",
			Barcode:       existingProd.Barcode,
		},
	}

	job := utils.Store.CreateJob(len(products))
	handler.processBatchImport(job, products, false)

	result, _ := utils.Store.GetJob(job.ID)
	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}
	if result.Updated != 1 {
		t.Errorf("expected 1 updated, got %d", result.Updated)
	}
}

func TestDeleteProductsBatchDirect(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	prod1 := seedProduct(db, "Del1", cat.ID, 1.00)
	prod2 := seedProduct(db, "Del2", cat.ID, 2.00)

	// Add images to products
	img1 := models.ProductImage{ID: uuid.New(), ProductID: prod1.ID, ImageURL: "https://storage.googleapis.com/test-bucket/products/del1.jpg", IsPrimary: true}
	img2 := models.ProductImage{ID: uuid.New(), ProductID: prod2.ID, ImageURL: "https://storage.googleapis.com/test-bucket/products/del2.jpg", IsPrimary: true}
	db.Create(&img1)
	db.Create(&img2)

	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	handler.deleteProductsBatch([]models.Product{prod1, prod2})

	// Verify products are soft-deleted
	var count int64
	db.Model(&models.Product{}).Where("id IN ?", []uuid.UUID{prod1.ID, prod2.ID}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 products after batch delete, got %d", count)
	}

	// Verify images were deleted from Firebase
	if len(mock.DeleteFileCalls) != 2 {
		t.Errorf("expected 2 DeleteFile calls, got %d", len(mock.DeleteFileCalls))
	}
}

func TestDeleteProductsBatchEmpty(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	// Should not panic on empty slice
	handler.deleteProductsBatch([]models.Product{})
	if len(mock.DeleteFileCalls) != 0 {
		t.Error("expected no DeleteFile calls for empty batch")
	}
}

func TestDeleteProductsBatchWithOrderReferences(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OrderRef", cat.ID, 10.00)

	imageURL := "https://storage.googleapis.com/test-bucket/products/orderref.jpg"
	img := models.ProductImage{ID: uuid.New(), ProductID: prod.ID, ImageURL: imageURL, IsPrimary: true}
	db.Create(&img)

	// Create order referencing this image
	user, _ := seedTestUser(db, "customer@test.com", "customer", nil)
	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "TestFranch", owner.ID)
	order := seedOrder(db, user.ID, franchise.ID, prod.ID)
	db.Model(&models.OrderItem{}).Where("order_id = ?", order.ID).Update("image_url", imageURL)

	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	handler.deleteProductsBatch([]models.Product{prod})

	// DeleteFile should NOT be called for order-referenced image
	if len(mock.DeleteFileCalls) != 0 {
		t.Error("expected no DeleteFile calls for order-referenced image")
	}
}

func TestDeleteProductsNotInImportDirect(t *testing.T) {
	db := freshDB()
	cat := seedCategory(db, "TestCat")
	keepProd := seedProduct(db, "Keep", cat.ID, 5.00)
	deleteProd := seedProduct(db, "Delete", cat.ID, 5.00)

	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	importedIDs := map[uuid.UUID]bool{keepProd.ID: true}
	job := utils.Store.CreateJob(1)
	handler.deleteProductsNotInImport(job, importedIDs)

	// Verify keepProd still exists
	var keepCount int64
	db.Model(&models.Product{}).Where("id = ?", keepProd.ID).Count(&keepCount)
	if keepCount != 1 {
		t.Error("expected keepProd to still exist")
	}

	// Verify deleteProd is soft-deleted
	var delCount int64
	db.Model(&models.Product{}).Where("id = ?", deleteProd.ID).Count(&delCount)
	if delCount != 0 {
		t.Error("expected deleteProd to be soft-deleted")
	}

	result, _ := utils.Store.GetJob(job.ID)
	if result.Deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", result.Deleted)
	}
}

func TestPrepareProductForBatchUpdateWithImageChanges(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	cat := seedCategory(db, "ImgDiffCat")
	prod := seedProduct(db, "ImgDiffProd", cat.ID, 5.00)

	// Add existing images to the product
	existingImg := models.ProductImage{
		ID:        uuid.New(),
		ProductID: prod.ID,
		ImageURL:  "https://storage.googleapis.com/test-bucket/products/old.jpg",
		IsPrimary: true,
	}
	db.Create(&existingImg)

	cache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mu := &sync.Mutex{}

	sku := prod.SKU
	productData := dtos.ProductImportItem{
		SKU:            sku,
		ItemName:       "ImgDiffProd Updated",
		RetailPrice:    6.99,
		CategoryID:     cat.ID.String(),
		Status:         "active",
		ImagesProvided: true,
		ImageURLs: []string{
			"https://example.com/new-image.jpg",
		},
	}

	product, images, fieldsChanged, imagesChanged, err := handler.prepareProductForBatch(productData, cache, mu)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if product.ID != prod.ID {
		t.Errorf("expected existing product ID %s, got %s", prod.ID, product.ID)
	}
	if !fieldsChanged {
		t.Error("expected fieldsChanged=true for name/price update")
	}
	if !imagesChanged {
		t.Error("expected imagesChanged=true for image swap")
	}
	// Should have new image from upload
	if len(images) == 0 {
		t.Error("expected new images from upload")
	}
	// Old image should be deleted from DB
	var oldCount int64
	db.Model(&models.ProductImage{}).Where("id = ?", existingImg.ID).Count(&oldCount)
	if oldCount != 0 {
		t.Error("expected old image to be deleted from DB")
	}
}

func TestPrepareProductForBatchUpdateRemoveAllImages(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	cat := seedCategory(db, "RemoveImgCat")
	prod := seedProduct(db, "RemoveImgProd", cat.ID, 5.00)

	// Add existing images
	existingImg := models.ProductImage{
		ID:        uuid.New(),
		ProductID: prod.ID,
		ImageURL:  "https://storage.googleapis.com/test-bucket/products/to-remove.jpg",
		IsPrimary: true,
	}
	db.Create(&existingImg)

	cache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mu := &sync.Mutex{}

	sku := prod.SKU
	productData := dtos.ProductImportItem{
		SKU:            sku,
		ItemName:       "RemoveImgProd",
		RetailPrice:    5.00,
		CategoryID:     cat.ID.String(),
		Status:         "active",
		ImagesProvided: true,
		ImageURLs:      nil, // empty images = remove all
	}

	_, _, _, imagesChanged, err := handler.prepareProductForBatch(productData, cache, mu)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !imagesChanged {
		t.Error("expected imagesChanged=true when removing all images")
	}
	// Verify old image deleted from DB
	var count int64
	db.Model(&models.ProductImage{}).Where("product_id = ?", prod.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 images after removal, got %d", count)
	}
	// Verify DeleteFile was called
	if len(mock.DeleteFileCalls) == 0 {
		t.Error("expected DeleteFile to be called for removed image")
	}
}

func TestPrepareProductForBatchUpdateDetectsFieldChanges(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	cat := seedCategory(db, "NoChangeCat")
	prod := seedProduct(db, "NoChangeProd", cat.ID, 5.00)

	cache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mu := &sync.Mutex{}

	sku := prod.SKU
	// Change the price -> fieldsChanged should be true
	productData := dtos.ProductImportItem{
		SKU:         sku,
		ItemName:    prod.ItemName,
		RetailPrice: prod.RetailPrice + 1.00,
		CategoryID:  cat.ID.String(),
		Status:      prod.Status,
	}

	_, _, fieldsChanged, imagesChanged, err := handler.prepareProductForBatch(productData, cache, mu)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fieldsChanged {
		t.Error("expected fieldsChanged=true when price differs")
	}
	if imagesChanged {
		t.Error("expected imagesChanged=false when no image data provided")
	}
}

func TestPrepareProductForBatchUpdateKeepExistingImages(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	cat := seedCategory(db, "KeepImgCat")
	prod := seedProduct(db, "KeepImgProd", cat.ID, 5.00)

	// Add existing image
	existingImg := models.ProductImage{
		ID:        uuid.New(),
		ProductID: prod.ID,
		ImageURL:  "https://storage.googleapis.com/test-bucket/products/keep.jpg",
		IsPrimary: true,
	}
	db.Create(&existingImg)

	cache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mu := &sync.Mutex{}

	sku := prod.SKU
	// Update with ImagesProvided=false -> should preserve existing images
	productData := dtos.ProductImportItem{
		SKU:            sku,
		ItemName:       "KeepImgProd Updated",
		RetailPrice:    7.99,
		CategoryID:     cat.ID.String(),
		Status:         "active",
		ImagesProvided: false,
		ImageURLs:      []string{"https://storage.googleapis.com/test-bucket/products/keep.jpg"},
	}

	_, _, fieldsChanged, imagesChanged, err := handler.prepareProductForBatch(productData, cache, mu)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fieldsChanged {
		t.Error("expected fieldsChanged=true for name/price update")
	}
	if imagesChanged {
		t.Error("expected imagesChanged=false when ImagesProvided=false")
	}
	// Verify existing image still in DB
	var count int64
	db.Model(&models.ProductImage{}).Where("product_id = ?", prod.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 image to be preserved, got %d", count)
	}
}

func TestPrepareProductForBatchUpdateImageOrderReferenced(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	cat := seedCategory(db, "OrderRefCat")
	prod := seedProduct(db, "OrderRefProd", cat.ID, 5.00)

	// Add existing image
	existingImg := models.ProductImage{
		ID:        uuid.New(),
		ProductID: prod.ID,
		ImageURL:  "https://storage.googleapis.com/test-bucket/products/order-ref.jpg",
		IsPrimary: true,
	}
	db.Create(&existingImg)

	// Create an order item referencing this image
	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "RefFranchise", owner.ID)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)
	// Update the order item to reference the image URL
	db.Model(&models.OrderItem{}).Where("order_id = ?", order.ID).Update("image_url", existingImg.ImageURL)

	cache := map[uuid.UUID]*models.Category{cat.ID: &cat}
	mu := &sync.Mutex{}

	sku := prod.SKU
	productData := dtos.ProductImportItem{
		SKU:            sku,
		ItemName:       "OrderRefProd",
		RetailPrice:    5.00,
		CategoryID:     cat.ID.String(),
		Status:         "active",
		ImagesProvided: true,
		ImageURLs:      nil, // remove all images
	}

	_, _, _, _, err := handler.prepareProductForBatch(productData, cache, mu)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// DeleteFile should NOT be called because image is referenced in order
	if len(mock.DeleteFileCalls) != 0 {
		t.Error("expected no DeleteFile calls for order-referenced image")
	}
}

func TestDeleteProductsNotInImportWithImages(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	cat := seedCategory(db, "DelImgCat")
	prod := seedProduct(db, "DelImgProd", cat.ID, 5.00)

	// Add an image to the product
	img := models.ProductImage{
		ID:        uuid.New(),
		ProductID: prod.ID,
		ImageURL:  "https://storage.googleapis.com/test-bucket/products/del.jpg",
		IsPrimary: true,
	}
	db.Create(&img)

	importedIDs := map[uuid.UUID]bool{} // empty = delete everything
	job := utils.Store.CreateJob(1)
	handler.deleteProductsNotInImport(job, importedIDs)

	// Product should be soft-deleted
	var prodCount int64
	db.Model(&models.Product{}).Where("id = ?", prod.ID).Count(&prodCount)
	if prodCount != 0 {
		t.Error("expected product to be soft-deleted")
	}

	// DeleteFile should be called for image cleanup
	if len(mock.DeleteFileCalls) == 0 {
		t.Error("expected DeleteFile to be called for product image")
	}
}

func TestDeleteProductsNotInImportOrderReferenced(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	cat := seedCategory(db, "OrderRefDelCat")
	prod := seedProduct(db, "OrderRefDelProd", cat.ID, 5.00)

	// Create an order referencing this product
	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "RefDelFranchise", owner.ID)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	importedIDs := map[uuid.UUID]bool{} // empty = would delete everything
	job := utils.Store.CreateJob(1)
	handler.deleteProductsNotInImport(job, importedIDs)

	// Product should NOT be deleted because it's referenced in an order
	var prodCount int64
	db.Model(&models.Product{}).Where("id = ?", prod.ID).Count(&prodCount)
	if prodCount != 1 {
		t.Error("expected product to be preserved due to order reference")
	}
}

func TestDeleteProductsNotInImportNoneToDelete(t *testing.T) {
	db := freshDB()
	mock := newMockStorage()
	handler := &ProductHandler{DB: db, Storage: mock}

	cat := seedCategory(db, "NoneDelCat")
	prod := seedProduct(db, "KeepProd", cat.ID, 5.00)

	// All products are in the import
	importedIDs := map[uuid.UUID]bool{prod.ID: true}
	job := utils.Store.CreateJob(1)
	handler.deleteProductsNotInImport(job, importedIDs)

	// Product should still exist
	var prodCount int64
	db.Model(&models.Product{}).Where("id = ?", prod.ID).Count(&prodCount)
	if prodCount != 1 {
		t.Error("expected product to still exist")
	}
}
