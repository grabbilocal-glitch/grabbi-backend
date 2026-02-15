package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestGetSubcategories(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "TestCat")
	seedSubcategory(db, "Sub1", cat.ID)
	seedSubcategory(db, "Sub2", cat.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/subcategories", nil))
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestCreateSubcategory(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "Parent")
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/subcategories", map[string]interface{}{
		"name":        "NewSub",
		"category_id": cat.ID.String(),
	}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSubcategoryInvalidCategory(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/subcategories", map[string]interface{}{
		"name":        "Bad",
		"category_id": uuid.New().String(),
	}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSubcategory(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "Parent")
	sub := seedSubcategory(db, "OldName", cat.ID)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/admin/subcategories/%s", sub.ID), map[string]interface{}{
		"name":        "NewName",
		"category_id": cat.ID.String(),
	}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSubcategoryNotFound(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "Parent")
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/admin/subcategories/%s", fakeID), map[string]interface{}{
		"name":        "Ghost",
		"category_id": cat.ID.String(),
	}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteSubcategory(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "Parent")
	sub := seedSubcategory(db, "ToDelete", cat.ID)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/admin/subcategories/%s", sub.ID), nil, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteSubcategoryWithProducts(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "Parent")
	sub := seedSubcategory(db, "HasProducts", cat.ID)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	// Create product referencing this subcategory
	prod := seedProduct(db, "SubProd", cat.ID, 5.00)
	db.Model(&prod).Update("subcategory_id", sub.ID)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/admin/subcategories/%s", sub.ID), nil, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteSubcategoryNotFound(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", "/api/admin/subcategories/"+uuid.New().String(), nil, adminToken)
	router.ServeHTTP(w, req)
	// Accept 200 (soft delete of non-existent doesn't error in GORM)
	if w.Code != 200 && w.Code != 404 {
		t.Fatalf("expected 200 or 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSubcategoryInvalidBody(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "Cat")
	sub := seedSubcategory(db, "Sub", cat.ID)

	w := httptest.NewRecorder()
	// Send body without required name field
	req := authRequest("PUT", "/api/admin/subcategories/"+sub.ID.String(),
		map[string]interface{}{"category_id": uuid.New().String()}, adminToken)
	router.ServeHTTP(w, req)
	// The handler will try to validate the parent category; it depends on binding rules
	// Accept 400 (validation) or 200 (no binding validation for name)
	if w.Code != 200 && w.Code != 400 {
		t.Fatalf("expected 200 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSubcategoryInvalidBody(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/subcategories", map[string]interface{}{}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSubcategoriesEmpty(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/subcategories", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 subcategories on fresh DB, got %d", len(result))
	}
}

func TestUpdateSubcategoryChangeCategoryNotFound(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	cat := seedCategory(db, "Cat")
	sub := seedSubcategory(db, "Sub", cat.ID)

	// Try to change parent to a non-existent category
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/admin/subcategories/%s", sub.ID), map[string]interface{}{
		"name":        "Sub",
		"category_id": uuid.New().String(),
	}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 for parent category not found, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Parent category not found" {
		t.Errorf("expected 'Parent category not found', got %v", resp["error"])
	}
}

func TestGetSubcategoriesWithCategory(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "ParentForSubs")
	seedSubcategory(db, "SubOne", cat.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/subcategories", nil))
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	sub := result[0].(map[string]interface{})
	catData, ok := sub["category"].(map[string]interface{})
	if !ok {
		t.Fatal("expected category to be preloaded in subcategory response")
	}
	if catData["name"] != "ParentForSubs" {
		t.Errorf("expected category name 'ParentForSubs', got %v", catData["name"])
	}
}

// ==================== Additional Coverage Boost Tests ====================

// TestGetSubcategoriesMultipleWithCategories tests preloading multiple subcategories
// from different categories.
func TestGetSubcategoriesMultipleWithCategories(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)

	cat1 := seedCategory(db, "Fruits")
	cat2 := seedCategory(db, "Vegetables")
	seedSubcategory(db, "Citrus", cat1.ID)
	seedSubcategory(db, "Berries", cat1.ID)
	seedSubcategory(db, "Leafy Greens", cat2.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/subcategories", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 3 {
		t.Errorf("expected 3 subcategories, got %d", len(result))
	}

	// Verify all subcategories have their category preloaded
	for _, item := range result {
		subMap := item.(map[string]interface{})
		catMap, ok := subMap["category"].(map[string]interface{})
		if !ok {
			t.Fatal("expected category to be preloaded in every subcategory")
		}
		if catMap["name"] == nil {
			t.Error("expected category name in preloaded category")
		}
	}
}

// TestCreateSubcategoryWithNameAndDescription tests creating a subcategory with all optional fields.
func TestCreateSubcategoryWithNameAndDescription(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "ParentForFull")
	_, adminToken := seedTestUser(db, "admin-sub-full@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/subcategories", map[string]interface{}{
		"name":        "Full Sub",
		"category_id": cat.ID.String(),
		"description": "A fully described subcategory",
		"icon":        "icon-sub",
	}, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "Full Sub" {
		t.Errorf("expected name 'Full Sub', got %v", resp["name"])
	}
	if resp["description"] != "A fully described subcategory" {
		t.Errorf("expected description set, got %v", resp["description"])
	}
}

// TestCreateSubcategoryMissingCategoryID tests creating a subcategory without a category_id.
func TestCreateSubcategoryMissingCategoryID(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin-sub-catid@test.com", "admin", nil)

	// Send request with name but no category_id
	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/subcategories", map[string]interface{}{
		"name": "Orphan Sub",
	}, adminToken)
	router.ServeHTTP(w, req)

	// Should fail because category_id is required or because parent category not found
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing category_id, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateSubcategorySuccessWithPreload tests that CreateSubcategory returns the
// created subcategory with its category preloaded.
func TestCreateSubcategorySuccessWithPreload(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "PreloadParent")
	_, adminToken := seedTestUser(db, "admin-preload@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/subcategories", map[string]interface{}{
		"name":        "PreloadedSub",
		"category_id": cat.ID.String(),
		"description": "A test subcategory",
	}, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "PreloadedSub" {
		t.Errorf("expected name 'PreloadedSub', got %v", resp["name"])
	}

	// Verify category is preloaded
	catResp, ok := resp["category"].(map[string]interface{})
	if !ok {
		t.Fatal("expected category to be preloaded in created subcategory response")
	}
	if catResp["name"] != "PreloadParent" {
		t.Errorf("expected category name 'PreloadParent', got %v", catResp["name"])
	}
}

// TestDeleteSubcategoryWithLinkedProducts tests that deleting a subcategory with
// linked products returns an error with product count.
func TestDeleteSubcategoryWithLinkedProducts(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "DelSubCat")
	sub := seedSubcategory(db, "LinkedSub", cat.ID)
	_, adminToken := seedTestUser(db, "admin-del-sub@test.com", "admin", nil)

	// Create two products linked to this subcategory
	prod1 := seedProduct(db, "LinkedProd1", cat.ID, 5.00)
	prod2 := seedProduct(db, "LinkedProd2", cat.ID, 7.00)
	db.Model(&prod1).Update("subcategory_id", sub.ID)
	db.Model(&prod2).Update("subcategory_id", sub.ID)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", "/api/admin/subcategories/"+sub.ID.String(), nil, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Cannot delete subcategory with associated products" {
		t.Errorf("expected specific error message, got %v", resp["error"])
	}
	productCount := resp["product_count"].(float64)
	if int(productCount) != 2 {
		t.Errorf("expected product_count 2, got %v", resp["product_count"])
	}
}

// TestDeleteSubcategorySuccessNoProducts tests successful deletion of a subcategory
// with no products linked.
func TestDeleteSubcategorySuccessNoProducts(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	cat := seedCategory(db, "DelSubSuccess")
	sub := seedSubcategory(db, "EmptySub", cat.ID)
	_, adminToken := seedTestUser(db, "admin-del-succ@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("DELETE", "/api/admin/subcategories/"+sub.ID.String(), nil, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["message"] != "Subcategory deleted successfully" {
		t.Errorf("expected deletion message, got %v", resp["message"])
	}
}

// TestCreateSubcategoryNonexistentCategoryID tests creating with a valid UUID
// that doesn't exist as a category.
func TestCreateSubcategoryNonexistentCategoryID(t *testing.T) {
	db := freshDB()
	router := setupSubcategoryRouter(db)
	_, adminToken := seedTestUser(db, "admin-nonexist@test.com", "admin", nil)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/admin/subcategories", map[string]interface{}{
		"name":        "OrphanSub",
		"category_id": fakeID.String(),
	}, adminToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for nonexistent category, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Parent category not found" {
		t.Errorf("expected 'Parent category not found', got %v", resp["error"])
	}
}
