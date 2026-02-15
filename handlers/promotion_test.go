package handlers

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"grabbi-backend/middleware"
	"grabbi-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestGetPromotionsList(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	seedPromotion(db, "Summer Sale", true)
	seedPromotion(db, "Winter Sale", true)
	seedPromotion(db, "Hidden Promo", false) // inactive

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	// Public endpoint only returns active promotions
	if len(result) != 2 {
		t.Errorf("expected 2 active promotions, got %d", len(result))
	}
}

func TestGetPromotionByID(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	promo := seedPromotion(db, "Flash Sale", true)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/promotions/%s", promo.ID), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["title"] != "Flash Sale" {
		t.Errorf("expected title 'Flash Sale', got %v", resp["title"])
	}
}

func TestGetPromotionNotFound(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/promotions/%s", fakeID), nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Promotion not found" {
		t.Errorf("expected 'Promotion not found', got %v", resp["error"])
	}
}

func TestDeletePromotionSuccess(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	promo := seedPromotion(db, "Delete Me Promo", true)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/promotions/%s", promo.ID), nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["message"] != "Promotion deleted successfully" {
		t.Errorf("expected deletion message, got %v", resp["message"])
	}

	// Verify soft delete
	var count int64
	db.Model(&models.Promotion{}).Where("id = ?", promo.ID).Count(&count)
	if count != 0 {
		t.Error("expected promotion to be soft deleted")
	}
}

func TestDeletePromotionNotFound(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/promotions/%s", fakeID), nil, adminToken))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== New Tests ====================

func TestCreatePromotionSuccess(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	req := multipartRequest("POST", "/api/admin/promotions",
		map[string]string{
			"title":       "Summer Sale",
			"description": "50% off",
			"is_active":   "true",
		},
		map[string]string{"image": "promo.jpg"},
		adminToken,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePromotionMissingImage(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	// POST without image -> 400
	req := multipartRequest("POST", "/api/admin/promotions",
		map[string]string{"title": "No Image", "is_active": "true"},
		nil, // no files
		adminToken,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePromotionUploadFailure(t *testing.T) {
	db := freshDB()
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	mock := newMockStorage()
	mock.UploadPromotionImageFn = func(file multipart.File, filename, contentType string) (string, error) {
		return "", fmt.Errorf("upload failed")
	}

	r := gin.New()
	handler := &PromotionHandler{DB: db, Storage: mock}
	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.POST("/promotions", handler.CreatePromotion)

	req := multipartRequest("POST", "/api/admin/promotions",
		map[string]string{"title": "Fail", "is_active": "true"},
		map[string]string{"image": "promo.jpg"},
		adminToken,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePromotionSuccess(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	promo := seedPromotion(db, "Old Title", true)

	req := multipartRequest("PUT", fmt.Sprintf("/api/admin/promotions/%s", promo.ID),
		map[string]string{"title": "New Title", "is_active": "true"},
		nil,
		adminToken,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["title"] != "New Title" {
		t.Errorf("expected 'New Title', got %v", resp["title"])
	}
}

func TestUpdatePromotionWithNewImage(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	promo := seedPromotion(db, "With Image", true)
	// Set an existing image URL
	db.Model(&promo).Update("image", "https://storage.googleapis.com/test-bucket/promotions/old.jpg")

	req := multipartRequest("PUT", fmt.Sprintf("/api/admin/promotions/%s", promo.ID),
		map[string]string{"title": "Updated", "is_active": "true"},
		map[string]string{"image": "new_promo.jpg"},
		adminToken,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePromotionNotFound(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	req := multipartRequest("PUT", fmt.Sprintf("/api/admin/promotions/%s", uuid.New()),
		map[string]string{"title": "Ghost"},
		nil,
		adminToken,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetPromotionsEmpty(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 promotions on fresh DB, got %d", len(result))
	}
}

func TestGetPromotionsOnlyActive(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	// Create 3 active and 2 inactive promotions
	seedPromotion(db, "Active 1", true)
	seedPromotion(db, "Active 2", true)
	seedPromotion(db, "Active 3", true)
	seedPromotion(db, "Inactive 1", false)
	seedPromotion(db, "Inactive 2", false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 3 {
		t.Errorf("expected 3 active promotions, got %d", len(result))
	}
}

func TestDeletePromotionWithImage(t *testing.T) {
	db := freshDB()
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	promo := seedPromotion(db, "ImgPromo", true)
	db.Model(&promo).Update("image", "https://storage.googleapis.com/test-bucket/promotions/test.jpg")

	mock := newMockStorage()
	r := gin.New()
	handler := &PromotionHandler{DB: db, Storage: mock}
	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.DELETE("/promotions/:id", handler.DeletePromotion)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/admin/promotions/%s", promo.ID), nil, adminToken)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	// Verify DeleteFile was called
	if len(mock.DeleteFileCalls) == 0 {
		t.Error("expected DeleteFile to be called for promotion image")
	}
}

// ==================== Additional Coverage Boost Tests ====================

// TestGetPromotionsActiveOnly tests that only active promotions are returned.
func TestGetPromotionsActiveOnly(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	seedPromotion(db, "Active Promo 1", true)
	seedPromotion(db, "Active Promo 2", true)
	seedPromotion(db, "Inactive Promo", false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2 active promotions, got %d", len(result))
	}

	// Verify all returned promotions are active
	for _, item := range result {
		promoMap := item.(map[string]interface{})
		if promoMap["is_active"] != true {
			t.Errorf("expected is_active true, got %v", promoMap["is_active"])
		}
	}
}

// TestUpdatePromotionFieldsWithoutImage tests updating promotion fields without
// uploading a new image (no image field in multipart form).
func TestUpdatePromotionFieldsWithoutImage(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)
	_, adminToken := seedTestUser(db, "admin-upd@test.com", "admin", nil)

	promo := seedPromotion(db, "Original", true)

	req := multipartRequest("PUT", fmt.Sprintf("/api/admin/promotions/%s", promo.ID),
		map[string]string{
			"title":       "Updated Title",
			"description": "Updated Description",
			"product_url": "https://example.com/updated",
			"is_active":   "false",
		},
		nil, // no image upload
		adminToken,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["title"] != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %v", resp["title"])
	}
	if resp["description"] != "Updated Description" {
		t.Errorf("expected description 'Updated Description', got %v", resp["description"])
	}
	if resp["product_url"] != "https://example.com/updated" {
		t.Errorf("expected product_url updated, got %v", resp["product_url"])
	}
}

// TestUpdatePromotionWithImageUploadFailure tests the error path when image upload fails.
func TestUpdatePromotionWithImageUploadFailure(t *testing.T) {
	db := freshDB()
	_, adminToken := seedTestUser(db, "admin-upd-fail@test.com", "admin", nil)
	promo := seedPromotion(db, "Upload Fail", true)

	mock := newMockStorage()
	mock.UploadPromotionImageFn = func(file multipart.File, filename, contentType string) (string, error) {
		return "", fmt.Errorf("upload failed")
	}

	r := gin.New()
	handler := &PromotionHandler{DB: db, Storage: mock}
	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.PUT("/promotions/:id", handler.UpdatePromotion)

	req := multipartRequest("PUT", fmt.Sprintf("/api/admin/promotions/%s", promo.ID),
		map[string]string{"title": "Should Fail", "is_active": "true"},
		map[string]string{"image": "new_image.jpg"},
		adminToken,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Image upload failed" {
		t.Errorf("expected 'Image upload failed', got %v", resp["error"])
	}
}

// TestUpdatePromotionNewImageDeletesOld tests that updating with a new image
// deletes the old image from storage.
func TestUpdatePromotionNewImageDeletesOld(t *testing.T) {
	db := freshDB()
	_, adminToken := seedTestUser(db, "admin-del-old@test.com", "admin", nil)
	promo := seedPromotion(db, "Old Image", true)
	db.Model(&promo).Update("image", "https://storage.googleapis.com/test-bucket/promotions/old_promo.jpg")

	mock := newMockStorage()
	r := gin.New()
	handler := &PromotionHandler{DB: db, Storage: mock}
	admin := r.Group("/api/admin")
	admin.Use(middleware.AuthMiddleware())
	admin.Use(middleware.AdminMiddleware())
	admin.PUT("/promotions/:id", handler.UpdatePromotion)

	req := multipartRequest("PUT", fmt.Sprintf("/api/admin/promotions/%s", promo.ID),
		map[string]string{"title": "New Image", "is_active": "true"},
		map[string]string{"image": "new_promo.jpg"},
		adminToken,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify old image was deleted
	if len(mock.DeleteFileCalls) == 0 {
		t.Error("expected DeleteFile to be called for old image")
	}
}

// TestUpdatePromotionIsActiveToggle tests toggling is_active through update.
func TestUpdatePromotionIsActiveToggle(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)
	_, adminToken := seedTestUser(db, "admin-toggle@test.com", "admin", nil)

	promo := seedPromotion(db, "Toggle Promo", true)

	// Update to inactive
	req := multipartRequest("PUT", fmt.Sprintf("/api/admin/promotions/%s", promo.ID),
		map[string]string{
			"title":     "Toggle Promo",
			"is_active": "false",
		},
		nil,
		adminToken,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["is_active"] != false {
		t.Errorf("expected is_active false, got %v", resp["is_active"])
	}

	// Update back to active
	req2 := multipartRequest("PUT", fmt.Sprintf("/api/admin/promotions/%s", promo.ID),
		map[string]string{
			"title":     "Toggle Promo",
			"is_active": "true",
		},
		nil,
		adminToken,
	)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	resp2 := parseResponse(w2)
	if resp2["is_active"] != true {
		t.Errorf("expected is_active true, got %v", resp2["is_active"])
	}
}

// TestGetPromotionsSingle tests getting promotions when exactly one active exists.
func TestGetPromotionsSingle(t *testing.T) {
	db := freshDB()
	router := setupPromotionRouter(db)

	seedPromotion(db, "Only Active", true)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 promotion, got %d", len(result))
	}

	promoMap := result[0].(map[string]interface{})
	if promoMap["title"] != "Only Active" {
		t.Errorf("expected title 'Only Active', got %v", promoMap["title"])
	}
}
