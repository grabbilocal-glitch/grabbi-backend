package routes

import (
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockStorage struct{}

func (m *mockStorage) UploadProductImage(file multipart.File, filename, contentType string) (string, error) {
	return "", nil
}
func (m *mockStorage) UploadPromotionImage(file multipart.File, filename, contentType string) (string, error) {
	return "", nil
}
func (m *mockStorage) DeleteFile(objectPath string) error { return nil }
func (m *mockStorage) DownloadAndUploadImage(imageURL, productID string) (string, error) {
	return "", nil
}
func (m *mockStorage) CopyImageToOrderStorage(sourceImageURL, orderID, productID string) (string, error) {
	return "", nil
}

func init() {
	gin.SetMode(gin.TestMode)
	os.Setenv("JWT_SECRET", "test-secret-for-routes")
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	tables := []string{
		`CREATE TABLE IF NOT EXISTS "users" (
			"id" TEXT PRIMARY KEY, "email" TEXT NOT NULL UNIQUE, "password" TEXT NOT NULL,
			"name" TEXT, "role" TEXT DEFAULT 'customer', "franchise_id" TEXT,
			"loyalty_points" INTEGER DEFAULT 0, "phone" TEXT, "is_blocked" INTEGER DEFAULT 0,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "categories" (
			"id" TEXT PRIMARY KEY, "name" TEXT NOT NULL, "icon" TEXT, "description" TEXT,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "subcategories" (
			"id" TEXT PRIMARY KEY, "name" TEXT NOT NULL, "category_id" TEXT NOT NULL,
			"icon" TEXT, "description" TEXT, "created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "products" (
			"id" TEXT PRIMARY KEY, "sku" TEXT NOT NULL UNIQUE, "item_name" TEXT NOT NULL,
			"short_description" TEXT, "long_description" TEXT, "cost_price" REAL NOT NULL,
			"retail_price" REAL NOT NULL, "promotion_price" REAL, "promotion_start" DATETIME,
			"promotion_end" DATETIME, "gross_margin" REAL DEFAULT 0, "staff_discount" REAL DEFAULT 0,
			"tax_rate" REAL DEFAULT 0, "batch_number" TEXT, "barcode" TEXT, "stock_quantity" INTEGER DEFAULT 0,
			"reorder_level" INTEGER DEFAULT 0, "shelf_location" TEXT, "weight_volume" REAL DEFAULT 0,
			"unit_of_measure" TEXT, "expiry_date" DATETIME, "category_id" TEXT NOT NULL,
			"subcategory_id" TEXT, "brand" TEXT, "supplier" TEXT, "country_of_origin" TEXT,
			"is_gluten_free" INTEGER DEFAULT 0, "is_vegetarian" INTEGER DEFAULT 0, "is_vegan" INTEGER DEFAULT 0,
			"is_age_restricted" INTEGER DEFAULT 0, "minimum_age" INTEGER, "allergen_info" TEXT,
			"storage_type" TEXT, "is_own_brand" INTEGER DEFAULT 0, "online_visible" INTEGER DEFAULT 1,
			"status" TEXT DEFAULT 'active', "notes" TEXT, "pack_size" TEXT,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "product_images" (
			"id" TEXT PRIMARY KEY, "product_id" TEXT NOT NULL, "image_url" TEXT NOT NULL,
			"is_primary" INTEGER DEFAULT 0, "created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "promotions" (
			"id" TEXT PRIMARY KEY, "title" TEXT NOT NULL, "description" TEXT, "image" TEXT,
			"product_url" TEXT, "is_active" INTEGER DEFAULT 1,
			"start_date" DATETIME, "end_date" DATETIME,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "franchises" (
			"id" TEXT PRIMARY KEY, "name" TEXT NOT NULL, "slug" TEXT NOT NULL UNIQUE,
			"owner_id" TEXT NOT NULL, "address" TEXT, "city" TEXT, "post_code" TEXT,
			"latitude" REAL NOT NULL, "longitude" REAL NOT NULL, "delivery_radius" REAL DEFAULT 5,
			"delivery_fee" REAL DEFAULT 4.99, "free_delivery_min" REAL DEFAULT 50,
			"phone" TEXT, "email" TEXT, "is_active" INTEGER DEFAULT 1,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "store_hours" (
			"id" TEXT PRIMARY KEY, "franchise_id" TEXT NOT NULL, "day_of_week" INTEGER NOT NULL,
			"open_time" TEXT NOT NULL DEFAULT '09:00', "close_time" TEXT NOT NULL DEFAULT '21:00',
			"is_closed" INTEGER DEFAULT 0, "created_at" DATETIME, "updated_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "franchise_products" (
			"id" TEXT PRIMARY KEY, "franchise_id" TEXT NOT NULL, "product_id" TEXT NOT NULL,
			"retail_price_override" REAL, "promotion_price_override" REAL,
			"promotion_start_override" DATETIME, "promotion_end_override" DATETIME,
			"stock_quantity" INTEGER DEFAULT 0, "reorder_level" INTEGER DEFAULT 5,
			"shelf_location" TEXT, "is_available" INTEGER DEFAULT 1,
			"created_at" DATETIME, "updated_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "franchise_staffs" (
			"id" TEXT PRIMARY KEY, "franchise_id" TEXT NOT NULL, "user_id" TEXT NOT NULL UNIQUE,
			"role" TEXT NOT NULL DEFAULT 'staff', "created_at" DATETIME, "updated_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "franchise_promotions" (
			"id" TEXT PRIMARY KEY, "franchise_id" TEXT NOT NULL, "title" TEXT NOT NULL,
			"description" TEXT, "image" TEXT, "product_url" TEXT, "is_active" INTEGER DEFAULT 1,
			"start_date" DATETIME, "end_date" DATETIME,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "cart_items" (
			"id" TEXT PRIMARY KEY, "user_id" TEXT NOT NULL, "product_id" TEXT NOT NULL,
			"franchise_id" TEXT, "quantity" INTEGER DEFAULT 1,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "orders" (
			"id" TEXT PRIMARY KEY, "user_id" TEXT NOT NULL, "franchise_id" TEXT,
			"order_number" TEXT NOT NULL UNIQUE, "status" TEXT DEFAULT 'pending',
			"subtotal" REAL NOT NULL, "delivery_fee" REAL DEFAULT 0, "total" REAL NOT NULL,
			"delivery_address" TEXT, "payment_method" TEXT, "points_earned" INTEGER DEFAULT 0,
			"customer_lat" REAL, "customer_lng" REAL,
			"created_at" DATETIME, "updated_at" DATETIME, "deleted_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "order_items" (
			"id" TEXT PRIMARY KEY, "order_id" TEXT NOT NULL, "product_id" TEXT NOT NULL,
			"image_url" TEXT, "product_name" TEXT, "product_sku" TEXT,
			"quantity" INTEGER NOT NULL, "price" REAL NOT NULL,
			"created_at" DATETIME, "updated_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "password_reset_tokens" (
			"id" TEXT PRIMARY KEY, "user_id" TEXT NOT NULL, "token" TEXT NOT NULL UNIQUE,
			"expires_at" DATETIME NOT NULL, "used_at" DATETIME, "created_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "loyalty_histories" (
			"id" TEXT PRIMARY KEY, "user_id" TEXT NOT NULL, "points" INTEGER NOT NULL,
			"type" TEXT NOT NULL, "description" TEXT, "order_id" TEXT, "created_at" DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS "refresh_tokens" (
			"id" TEXT PRIMARY KEY, "user_id" TEXT NOT NULL, "token" TEXT NOT NULL UNIQUE,
			"expires_at" DATETIME NOT NULL, "revoked_at" DATETIME, "created_at" DATETIME
		)`,
	}

	for _, sql := range tables {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func setupRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	db := setupTestDB(t)
	r := gin.New()
	SetupRoutes(r, db, &mockStorage{})
	return r, db
}

func TestHealthCheck(t *testing.T) {
	r, _ := setupRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPublicProductsRoute(t *testing.T) {
	r, _ := setupRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/products", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProtectedRouteRequiresAuth(t *testing.T) {
	r, _ := setupRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/cart", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminRouteBlocksNonAdmin(t *testing.T) {
	r, _ := setupRouter(t)
	token, _ := utils.GenerateToken(uuid.New(), "user@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/products", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseRouteBlocksNonFranchise(t *testing.T) {
	r, _ := setupRouter(t)
	token, _ := utils.GenerateToken(uuid.New(), "user@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
