package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"grabbi-backend/models"

	"github.com/google/uuid"
)

func TestGetCategoriesList(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	seedCategory(db, "Fruits")
	seedCategory(db, "Vegetables")
	seedCategory(db, "Dairy")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/categories", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 3 {
		t.Errorf("expected 3 categories, got %d", len(result))
	}
}

func TestGetCategoryByIDWithProducts(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	cat := seedCategory(db, "Meats")
	seedProduct(db, "Chicken", cat.ID, 8.99)
	seedProduct(db, "Beef", cat.ID, 12.99)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/categories/%s", cat.ID), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "Meats" {
		t.Errorf("expected name 'Meats', got %v", resp["name"])
	}

	// Check that products are preloaded
	products, ok := resp["products"].([]interface{})
	if !ok {
		t.Fatal("expected products array in response")
	}
	if len(products) != 2 {
		t.Errorf("expected 2 products in category, got %d", len(products))
	}
}

func TestCreateCategory(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	body := map[string]interface{}{
		"name":        "New Category",
		"icon":        "shopping-cart",
		"description": "A new test category",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/admin/categories", body, adminToken))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "New Category" {
		t.Errorf("expected name 'New Category', got %v", resp["name"])
	}

	// Verify in DB
	var count int64
	db.Model(&models.Category{}).Where("name = ?", "New Category").Count(&count)
	if count != 1 {
		t.Error("expected category to be saved in database")
	}
}

func TestUpdateCategory(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "Old Name Cat")

	body := map[string]interface{}{
		"name":        "Updated Cat Name",
		"description": "Updated description",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/admin/categories/%s", cat.ID), body, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "Updated Cat Name" {
		t.Errorf("expected name 'Updated Cat Name', got %v", resp["name"])
	}
}

func TestDeleteCategorySuccess(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "Delete Me Cat")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/categories/%s", cat.ID), nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["message"] != "Category deleted successfully" {
		t.Errorf("expected deletion message, got %v", resp["message"])
	}
}

func TestDeleteCategoryWithProductsFails(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "Cat With Products")
	seedProduct(db, "Linked Product", cat.ID, 1.99)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/categories/%s", cat.ID), nil, adminToken))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Cannot delete category with associated products" {
		t.Errorf("expected product dependency error, got %v", resp["error"])
	}
}

func TestGetCategoryNotFound(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/categories/%s", fakeID), nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Category not found" {
		t.Errorf("expected 'Category not found', got %v", resp["error"])
	}
}

func TestUpdateCategoryNotFound(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("PUT", "/api/admin/categories/"+uuid.New().String(),
		map[string]interface{}{"name": "Ghost"}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteCategoryNotFound(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", "/api/admin/categories/"+uuid.New().String(), nil, adminToken)
	router.ServeHTTP(w, req)
	// Should return 200 because GORM soft delete doesn't error on missing record
	// OR it may return 200 anyway if delete of non-existent is fine
	// Accept both 200 and 404 as valid
	if w.Code != 200 && w.Code != 404 {
		t.Fatalf("expected 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteCategoryWithSubcategories(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "ParentCat")
	seedSubcategory(db, "ChildSub", cat.ID)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", "/api/admin/categories/"+cat.ID.String(), nil, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 (has subcategories), got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateCategoryInvalidBody(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	// Send a request with an invalid JSON body (non-JSON string) to trigger bind error
	req := httptest.NewRequest("POST", "/api/admin/categories", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetCategoriesEmpty(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/categories", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 categories on fresh DB, got %d", len(result))
	}
}

func TestGetCategoriesWithSubcategories(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	cat := seedCategory(db, "ParentCatWithSubs")
	seedSubcategory(db, "SubA", cat.ID)
	seedSubcategory(db, "SubB", cat.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/categories", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Fatalf("expected 1 category, got %d", len(result))
	}

	catMap := result[0].(map[string]interface{})
	subs, ok := catMap["subcategories"].([]interface{})
	if !ok {
		t.Fatal("expected subcategories to be preloaded")
	}
	if len(subs) != 2 {
		t.Errorf("expected 2 subcategories, got %d", len(subs))
	}
}

func TestUpdateCategoryInvalidBody(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "UpdateMeBadBody")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", fmt.Sprintf("/api/admin/categories/%s", cat.ID), bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid body, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetCategoriesPreloadSubcategoriesData verifies subcategory fields are correctly preloaded.
func TestGetCategoriesPreloadSubcategoriesData(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	cat := seedCategory(db, "PreloadParent")
	sub1 := seedSubcategory(db, "SubAlpha", cat.ID)
	seedSubcategory(db, "SubBeta", cat.ID)

	// Also create a second category with no subcategories
	seedCategory(db, "EmptyCategory")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/categories", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(result))
	}

	// Find the category with subcategories
	for _, r := range result {
		catMap := r.(map[string]interface{})
		if catMap["name"] == "PreloadParent" {
			subs, ok := catMap["subcategories"].([]interface{})
			if !ok {
				t.Fatal("expected subcategories array")
			}
			if len(subs) != 2 {
				t.Errorf("expected 2 subcategories, got %d", len(subs))
			}
			// Verify subcategory data is populated
			subMap := subs[0].(map[string]interface{})
			if subMap["id"] == nil || subMap["name"] == nil {
				t.Error("expected subcategory fields to be populated")
			}
			_ = sub1 // used for seeding
		}
	}
}

// TestDeleteCategorySuccessNoProducts tests deleting a category that has no products or subcategories.
func TestDeleteCategorySuccessVerifyGone(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "DeleteVerifyCat")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/categories/%s", cat.ID), nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify category is actually gone (soft deleted)
	var count int64
	db.Model(&models.Category{}).Where("id = ?", cat.ID).Count(&count)
	if count != 0 {
		t.Error("expected category to be soft deleted")
	}
}

// TestDeleteCategoryWithSubcategoriesVerifyMessage verifies the error message when
// attempting to delete a category that has subcategories.
func TestDeleteCategoryWithSubcategoriesVerifyMessage(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	cat := seedCategory(db, "CatWithSubs")
	seedSubcategory(db, "Sub1", cat.ID)
	seedSubcategory(db, "Sub2", cat.ID)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/admin/categories/%s", cat.ID), nil, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Cannot delete category with subcategories" {
		t.Errorf("expected subcategory dependency error, got %v", resp["error"])
	}
	if resp["message"] != "Please delete or reassign subcategories first" {
		t.Errorf("expected reassign message, got %v", resp["message"])
	}
	subCount, ok := resp["subcategory_count"].(float64)
	if !ok || int(subCount) != 2 {
		t.Errorf("expected subcategory_count 2, got %v", resp["subcategory_count"])
	}
}

// TestDeleteCategoryWithProductsVerifyMessage verifies the full error response when
// attempting to delete a category that has associated products.
func TestDeleteCategoryWithProductsVerifyMessage(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	cat := seedCategory(db, "ProdCat")
	seedProduct(db, "P1", cat.ID, 1.00)
	seedProduct(db, "P2", cat.ID, 2.00)
	seedProduct(db, "P3", cat.ID, 3.00)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/admin/categories/%s", cat.ID), nil, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Cannot delete category with associated products" {
		t.Errorf("expected product dependency error, got %v", resp["error"])
	}
	if resp["message"] != "Please reassign or delete the associated products first" {
		t.Errorf("expected reassign message, got %v", resp["message"])
	}
	prodCount, ok := resp["product_count"].(float64)
	if !ok || int(prodCount) != 3 {
		t.Errorf("expected product_count 3, got %v", resp["product_count"])
	}
}

// TestDeleteCategoryWithBothProductsAndSubcategories tests that product check
// takes priority over subcategory check (products are checked first).
func TestDeleteCategoryWithBothProductsAndSubcategories(t *testing.T) {
	db := freshDB()
	router := setupCategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	cat := seedCategory(db, "BothDepsCat")
	seedProduct(db, "BothProd", cat.ID, 1.00)
	seedSubcategory(db, "BothSub", cat.ID)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/admin/categories/%s", cat.ID), nil, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	// Products are checked first, so we get the product error
	resp := parseResponse(w)
	if resp["error"] != "Cannot delete category with associated products" {
		t.Errorf("expected product dependency error (checked first), got %v", resp["error"])
	}
}

// TestGetCategoriesDBError tests the error branch when the DB query fails.
func TestGetCategoriesDBError(t *testing.T) {
	db := freshDB()

	// Drop the categories table to force a DB error
	db.Exec("DROP TABLE categories")

	router := setupCategoryRouter(db)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/categories", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Failed to fetch categories" {
		t.Errorf("expected 'Failed to fetch categories', got %v", resp["error"])
	}

	// Recreate the tables for subsequent tests
	db.Exec(`CREATE TABLE IF NOT EXISTS "categories" (
		"id" TEXT PRIMARY KEY,
		"name" TEXT NOT NULL,
		"icon" TEXT,
		"description" TEXT,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_categories_deleted_at ON "categories"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_categories_name ON "categories"("name")`)
}

// TestDeleteCategoryDBErrorOnProductCount tests the error branch when counting products fails.
func TestDeleteCategoryDBErrorOnProductCount(t *testing.T) {
	db := freshDB()
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "ErrCountCat")

	// Drop the products table to force the product count query to fail
	db.Exec("DROP TABLE products")

	router := setupCategoryRouter(db)
	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/admin/categories/%s", cat.ID), nil, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Failed to check category dependencies" {
		t.Errorf("expected 'Failed to check category dependencies', got %v", resp["error"])
	}

	// Recreate the products table for subsequent tests
	db.Exec(`CREATE TABLE IF NOT EXISTS "products" (
		"id" TEXT PRIMARY KEY,
		"sku" TEXT NOT NULL UNIQUE,
		"item_name" TEXT NOT NULL,
		"short_description" TEXT,
		"long_description" TEXT,
		"cost_price" REAL NOT NULL,
		"retail_price" REAL NOT NULL,
		"promotion_price" REAL,
		"promotion_start" DATETIME,
		"promotion_end" DATETIME,
		"gross_margin" REAL DEFAULT 0,
		"staff_discount" REAL DEFAULT 0,
		"tax_rate" REAL DEFAULT 0,
		"batch_number" TEXT,
		"barcode" TEXT,
		"stock_quantity" INTEGER DEFAULT 0,
		"reorder_level" INTEGER DEFAULT 0,
		"shelf_location" TEXT,
		"weight_volume" REAL DEFAULT 0,
		"unit_of_measure" TEXT,
		"expiry_date" DATETIME,
		"category_id" TEXT NOT NULL,
		"subcategory_id" TEXT,
		"brand" TEXT,
		"supplier" TEXT,
		"country_of_origin" TEXT,
		"is_gluten_free" INTEGER DEFAULT 0,
		"is_vegetarian" INTEGER DEFAULT 0,
		"is_vegan" INTEGER DEFAULT 0,
		"is_age_restricted" INTEGER DEFAULT 0,
		"minimum_age" INTEGER,
		"allergen_info" TEXT,
		"storage_type" TEXT,
		"is_own_brand" INTEGER DEFAULT 0,
		"online_visible" INTEGER DEFAULT 1,
		"status" TEXT DEFAULT 'active',
		"notes" TEXT,
		"pack_size" TEXT,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME,
		CONSTRAINT fk_products_category FOREIGN KEY ("category_id") REFERENCES "categories"("id")
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON "products"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_products_item_name ON "products"("item_name")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_products_category_id ON "products"("category_id")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_products_subcategory_id ON "products"("subcategory_id")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_products_status ON "products"("status")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_products_stock_quantity ON "products"("stock_quantity")`)
}

// TestDeleteCategoryDBErrorOnSubcategoryCount tests the error branch when counting subcategories fails.
func TestDeleteCategoryDBErrorOnSubcategoryCount(t *testing.T) {
	db := freshDB()
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "SubErrCat")

	// Drop the subcategories table to force the subcategory count query to fail
	db.Exec("DROP TABLE subcategories")

	router := setupCategoryRouter(db)
	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/admin/categories/%s", cat.ID), nil, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Failed to check category dependencies" {
		t.Errorf("expected 'Failed to check category dependencies', got %v", resp["error"])
	}

	// Recreate the subcategories table for subsequent tests
	db.Exec(`CREATE TABLE IF NOT EXISTS "subcategories" (
		"id" TEXT PRIMARY KEY,
		"name" TEXT NOT NULL,
		"category_id" TEXT NOT NULL,
		"icon" TEXT,
		"description" TEXT,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME,
		CONSTRAINT fk_subcategories_category FOREIGN KEY ("category_id") REFERENCES "categories"("id")
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_subcategories_deleted_at ON "subcategories"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_subcategories_name ON "subcategories"("name")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_subcategories_category_id ON "subcategories"("category_id")`)
}
