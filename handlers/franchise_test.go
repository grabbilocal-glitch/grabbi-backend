package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"grabbi-backend/models"

	"github.com/google/uuid"
)

func TestListFranchises(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner1@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	seedFranchise(db, "Franchise A", owner.ID)
	seedFranchise(db, "Franchise B", owner.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/admin/franchises", nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2 franchises, got %d", len(result))
	}
}

func TestGetFranchiseByID(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner2@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "My Franchise", owner.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/franchises/%s", franchise.ID), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "My Franchise" {
		t.Errorf("expected name 'My Franchise', got %v", resp["name"])
	}
}

func TestGetFranchiseNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/franchises/%s", fakeID), nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateFranchise(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner3@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Old Name", owner.ID)

	body := map[string]interface{}{
		"name":    "New Name",
		"address": "Updated Address",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/admin/franchises/%s", franchise.ID), body, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "New Name" {
		t.Errorf("expected name 'New Name', got %v", resp["name"])
	}
	if resp["address"] != "Updated Address" {
		t.Errorf("expected address 'Updated Address', got %v", resp["address"])
	}
}

func TestGetNearestFranchise(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner4@test.com", "franchise_owner", nil)

	// Create franchise at known location (London center)
	franchise := models.Franchise{
		ID:             uuid.New(),
		Name:           "London Store",
		Slug:           "london-store",
		OwnerID:        owner.ID,
		Latitude:       51.5074,
		Longitude:      -0.1278,
		DeliveryRadius: 10.0,
		IsActive:       true,
	}
	db.Create(&franchise)

	// Query with nearby coordinates
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchises/nearest?lat=51.51&lng=-0.13", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["franchise"] == nil {
		t.Error("expected franchise in response")
	}
}

func TestGetNearestFranchiseNoParams(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchises/nearest", nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetFranchiseProducts(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner5@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Products Franchise", owner.ID)
	cat := seedCategory(db, "FranchiseCat")
	prod := seedProduct(db, "Franchise Product", cat.ID, 5.99)

	// Create franchise-product association
	fp := models.FranchiseProduct{
		ID:            uuid.New(),
		FranchiseID:   franchise.ID,
		ProductID:     prod.ID,
		StockQuantity: 50,
		IsAvailable:   true,
	}
	db.Create(&fp)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/franchises/%s/products", franchise.ID), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 franchise product, got %d", len(result))
	}
}

func TestGetFranchisePromotions(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner6@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promos Franchise", owner.ID)

	promo := models.FranchisePromotion{
		ID:          uuid.New(),
		FranchiseID: franchise.ID,
		Title:       "Test Promo",
		IsActive:    true,
	}
	db.Create(&promo)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/franchises/%s/promotions", franchise.ID), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 franchise promotion, got %d", len(result))
	}
}

func TestDeleteFranchiseNoDependencies(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner7@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Delete Me Franchise", owner.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/franchises/%s", franchise.ID), nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["message"] != "Franchise deleted successfully" {
		t.Errorf("expected deletion message, got %v", resp["message"])
	}
}

// ==================== New Tests ====================

func TestCreateFranchiseSuccess(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	body := map[string]interface{}{
		"name":           "New Franchise",
		"slug":           "new-franchise",
		"owner_email":    "newowner@test.com",
		"owner_name":     "New Owner",
		"owner_password": "password123",
		"latitude":       51.5074,
		"longitude":      -0.1278,
		"address":        "123 Test St",
		"city":           "London",
		"post_code":      "SW1A 1AA",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/admin/franchises", body, adminToken))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "New Franchise" {
		t.Errorf("expected 'New Franchise', got %v", resp["name"])
	}

	// Verify store hours were created (should be in the response if Preloaded)
	if storeHours, ok := resp["store_hours"].([]interface{}); ok {
		if len(storeHours) != 7 {
			t.Errorf("expected 7 store hours, got %d", len(storeHours))
		}
	}
}

func TestCreateFranchiseMissingRequiredFields(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	// Missing required fields
	body := map[string]interface{}{
		"name": "Incomplete",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/admin/franchises", body, adminToken))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetFranchiseOrders(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Order Franchise", owner.ID)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OrderProd", cat.ID, 10.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", fmt.Sprintf("/api/admin/franchises/%s/orders", franchise.ID), nil, adminToken))

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2 orders, got %d", len(result))
	}
}

func TestDeleteFranchiseWithDependencies(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Dep Franchise", owner.ID)

	// Add a franchise product to create a dependency
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "DepProd", cat.ID, 5.00)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/franchises/%s", franchise.ID), nil, adminToken))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteFranchiseNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/franchises/%s", fakeID), nil, adminToken))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetNearestFranchiseNoneInRange(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)

	// Create franchise in London with small radius
	franchise := models.Franchise{
		ID:             uuid.New(),
		Name:           "London Store",
		Slug:           "london-store-range",
		OwnerID:        owner.ID,
		Latitude:       51.5074,
		Longitude:      -0.1278,
		DeliveryRadius: 1.0, // Very small radius (1km)
		IsActive:       true,
	}
	db.Create(&franchise)

	// Query with coordinates far away (New York)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchises/nearest?lat=40.7128&lng=-74.0060", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetNearestFranchiseInvalidCoordinates(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	// Invalid lat (not a number)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchises/nearest?lat=abc&lng=-0.13", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid lat, got %d: %s", w.Code, w.Body.String())
	}

	// Invalid lng (not a number)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, httptest.NewRequest("GET", "/api/franchises/nearest?lat=51.51&lng=xyz", nil))
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid lng, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestUpdateFranchiseNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/admin/franchises/%s", fakeID), map[string]interface{}{
		"name": "Ghost",
	}, adminToken))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== Additional Admin Coverage Tests ====================

func TestUpdateFranchiseAllFields(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Original Name", owner.ID)

	body := map[string]interface{}{
		"name":              "Updated Name",
		"address":           "456 New St",
		"city":              "Manchester",
		"post_code":         "M1 2AB",
		"phone":             "+44123456789",
		"email":             "updated@franchise.com",
		"latitude":          52.4862,
		"longitude":         -1.8904,
		"delivery_radius":   8.0,
		"delivery_fee":      3.50,
		"free_delivery_min": 30.0,
		"is_active":         false,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/admin/franchises/%s", franchise.ID), body, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %v", resp["name"])
	}
	if resp["address"] != "456 New St" {
		t.Errorf("expected address '456 New St', got %v", resp["address"])
	}
	if resp["city"] != "Manchester" {
		t.Errorf("expected city 'Manchester', got %v", resp["city"])
	}
	if resp["phone"] != "+44123456789" {
		t.Errorf("expected phone '+44123456789', got %v", resp["phone"])
	}
	if resp["email"] != "updated@franchise.com" {
		t.Errorf("expected email 'updated@franchise.com', got %v", resp["email"])
	}
	if resp["post_code"] != "M1 2AB" {
		t.Errorf("expected post_code 'M1 2AB', got %v", resp["post_code"])
	}
}

func TestListFranchisesWithOrderCount(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	franchise := seedFranchise(db, "Franchise With Orders", owner.ID)
	seedFranchise(db, "Franchise No Orders", owner.ID)

	// Seed an order for the first franchise
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "TestProd", cat.ID, 10.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/admin/franchises", nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2 franchises, got %d", len(result))
	}

	// Verify order_count field exists on response items
	for _, item := range result {
		f := item.(map[string]interface{})
		if _, ok := f["order_count"]; !ok {
			t.Error("expected order_count field in franchise response")
		}
	}
}

func TestGetFranchiseProductsNotFoundFranchise(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/franchises/%s/products", fakeID), nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetFranchiseProductsSearchFilter(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Search Franchise", owner.ID)
	cat := seedCategory(db, "Snacks")
	prod1 := seedProduct(db, "Chocolate Bar", cat.ID, 1.99)
	prod2 := seedProduct(db, "Vanilla Ice Cream", cat.ID, 3.49)

	seedFranchiseProduct(db, franchise.ID, prod1.ID)
	seedFranchiseProduct(db, franchise.ID, prod2.ID)

	// Search for "Chocolate" - should find 1
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/franchises/%s/products?search=Chocolate", franchise.ID), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 product matching search, got %d", len(result))
	}
}

func TestGetFranchiseProductsCategoryFilter(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Category Filter Franchise", owner.ID)
	cat1 := seedCategory(db, "Drinks")
	cat2 := seedCategory(db, "Food")
	prod1 := seedProduct(db, "Cola", cat1.ID, 1.50)
	prod2 := seedProduct(db, "Sandwich", cat2.ID, 4.99)

	seedFranchiseProduct(db, franchise.ID, prod1.ID)
	seedFranchiseProduct(db, franchise.ID, prod2.ID)

	// Filter by cat1 - should find 1
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/franchises/%s/products?category_id=%s", franchise.ID, cat1.ID), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 product for category filter, got %d", len(result))
	}
}

func TestGetFranchiseProductsWithPriceOverrides(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Override Franchise", owner.ID)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OverrideProd", cat.ID, 10.00)
	fp := seedFranchiseProduct(db, franchise.ID, prod.ID)

	// Set retail and promo price overrides
	retailOverride := 8.50
	promoOverride := 6.99
	db.Model(&fp).Updates(map[string]interface{}{
		"retail_price_override":    retailOverride,
		"promotion_price_override": promoOverride,
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/api/franchises/%s/products", franchise.ID), nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Fatalf("expected 1 product, got %d", len(result))
	}

	p := result[0].(map[string]interface{})
	if p["franchise_price"].(float64) != 8.50 {
		t.Errorf("expected franchise_price 8.50, got %v", p["franchise_price"])
	}
	if p["franchise_promo_price"].(float64) != 6.99 {
		t.Errorf("expected franchise_promo_price 6.99, got %v", p["franchise_promo_price"])
	}
}

func TestGetFranchiseOrdersEmpty(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	fakeID := uuid.New()

	// Should return empty array for non-existent franchise, not an error
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", fmt.Sprintf("/api/admin/franchises/%s/orders", fakeID), nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 orders for non-existent franchise, got %d", len(result))
	}
}

func TestCreateFranchiseWithExistingOwner(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	// Pre-create the owner user
	seedTestUser(db, "existingowner@test.com", "customer", nil)

	body := map[string]interface{}{
		"name":           "Owner Reuse Franchise",
		"slug":           "owner-reuse-franchise",
		"owner_email":    "existingowner@test.com",
		"owner_name":     "Existing Owner",
		"owner_password": "password123",
		"latitude":       51.5074,
		"longitude":      -0.1278,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/admin/franchises", body, adminToken))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "Owner Reuse Franchise" {
		t.Errorf("expected name 'Owner Reuse Franchise', got %v", resp["name"])
	}
}

func TestDeleteFranchiseWithStaffDependency(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Staff Dep Franchise", owner.ID)

	// Add staff to create a dependency
	staffUser, _ := seedTestUser(db, "staff@test.com", "franchise_staff", &franchise.ID)
	seedFranchiseStaff(db, franchise.ID, staffUser.ID, "staff")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/franchises/%s", franchise.ID), nil, adminToken))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteFranchiseWithOrderDependency(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Order Dep Franchise", owner.ID)

	// Add an order to create a dependency
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "TestProd", cat.ID, 5.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/admin/franchises/%s", franchise.ID), nil, adminToken))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["order_count"] == nil {
		t.Error("expected order_count in conflict response")
	}
}

// ==================== Franchise Portal Additional Coverage Tests ====================

func TestGetMyFranchiseNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	// Create a franchise owner with a non-existent franchise_id
	fakeFranchiseID := uuid.New()
	_, token := seedTestUser(db, "orphan-owner@test.com", "franchise_owner", &fakeFranchiseID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/me", nil, token))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateMyFranchiseAllFieldsViaPortal(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "My Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	body := map[string]interface{}{
		"address":           "789 Updated Road",
		"phone":             "+44987654321",
		"city":              "Bristol",
		"post_code":         "BS1 1AA",
		"email":             "mystore@test.com",
		"delivery_radius":   10.0,
		"delivery_fee":      2.99,
		"free_delivery_min": 25.0,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/me", body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["address"] != "789 Updated Road" {
		t.Errorf("expected address '789 Updated Road', got %v", resp["address"])
	}
	if resp["phone"] != "+44987654321" {
		t.Errorf("expected phone '+44987654321', got %v", resp["phone"])
	}
	if resp["city"] != "Bristol" {
		t.Errorf("expected city 'Bristol', got %v", resp["city"])
	}
	if resp["post_code"] != "BS1 1AA" {
		t.Errorf("expected post_code 'BS1 1AA', got %v", resp["post_code"])
	}
	if resp["email"] != "mystore@test.com" {
		t.Errorf("expected email 'mystore@test.com', got %v", resp["email"])
	}
}

func TestUpdateMyFranchiseNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	fakeFranchiseID := uuid.New()
	_, token := seedTestUser(db, "orphan@test.com", "franchise_owner", &fakeFranchiseID)

	body := map[string]interface{}{
		"address": "Nowhere",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/me", body, token))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetMyProductsWithSearch(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Search Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "Beverages")
	prod1 := seedProduct(db, "Orange Juice", cat.ID, 2.50)
	prod2 := seedProduct(db, "Apple Cider", cat.ID, 3.99)
	seedFranchiseProduct(db, franchise.ID, prod1.ID)
	seedFranchiseProduct(db, franchise.ID, prod2.ID)

	// Search for "Orange" - should find 1
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/products?search=Orange", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 product matching search, got %d", len(result))
	}
}

func TestPortalUpdateOrderStatusMissingBody(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Order Status Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "Food")
	prod := seedProduct(db, "Pizza", cat.ID, 9.99)
	customer, _ := seedTestUser(db, "customer@test.com", "customer", nil)
	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)

	// Empty body - missing required "status"
	body := map[string]interface{}{}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/franchise/orders/%s/status", order.ID), body, token))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPortalUpdateOrderStatusFullChain(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Chain Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "Food")
	prod := seedProduct(db, "Burger", cat.ID, 8.99)
	customer, _ := seedTestUser(db, "customer@test.com", "customer", nil)
	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)

	// Walk through the full order lifecycle
	transitions := []string{"confirmed", "preparing", "ready", "out_for_delivery", "delivered"}
	for _, status := range transitions {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/franchise/orders/%s/status", order.ID), map[string]interface{}{
			"status": status,
		}, token))

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for transition to '%s', got %d: %s", status, w.Code, w.Body.String())
		}

		resp := parseResponse(w)
		if resp["status"] != status {
			t.Errorf("expected status '%s', got %v", status, resp["status"])
		}
	}
}

func TestCreateFranchisePromotionInvalidBody(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Invalid Promo Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Empty body - missing required "title"
	body := map[string]interface{}{}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/franchise/promotions", body, token))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInviteStaffMissingEmail(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Missing Email Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Missing required email
	body := map[string]interface{}{
		"password": "password123",
		"role":     "staff",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/franchise/staff", body, token))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInviteStaffDefaultRole(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Default Role Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Omit role - should default to "staff"
	body := map[string]interface{}{
		"email":    "default-role@test.com",
		"name":     "Default Role User",
		"password": "password123",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/franchise/staff", body, token))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["role"] != "staff" {
		t.Errorf("expected default role 'staff', got %v", resp["role"])
	}
}

func TestInviteStaffManagerRole(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Manager Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	body := map[string]interface{}{
		"email":    "manager@test.com",
		"name":     "New Manager",
		"password": "password123",
		"role":     "manager",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/franchise/staff", body, token))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["role"] != "manager" {
		t.Errorf("expected role 'manager', got %v", resp["role"])
	}
}

func TestRemoveStaffVerifyUserReset(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Reset Staff Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	staffUser, _ := seedTestUser(db, "staff@test.com", "franchise_staff", &franchise.ID)
	staff := seedFranchiseStaff(db, franchise.ID, staffUser.ID, "staff")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/franchise/staff/%s", staff.ID), nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the user's role was reset to customer
	var updatedUser models.User
	db.Where("id = ?", staffUser.ID).First(&updatedUser)
	if updatedUser.Role != "customer" {
		t.Errorf("expected user role reset to 'customer', got %v", updatedUser.Role)
	}
}

func TestUpdateStoreHoursClosedDay(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Closed Day Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	seedStoreHours(db, franchise.ID)

	// Mark Monday (day 1) as closed
	body := []map[string]interface{}{
		{"day_of_week": 1, "open_time": "09:00", "close_time": "21:00", "is_closed": true},
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/hours", body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProductStockAllFields(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Stock All Fields Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "Dairy")
	prod := seedProduct(db, "Milk", cat.ID, 1.99)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	body := map[string]interface{}{
		"stock_quantity": 200,
		"reorder_level":  10,
		"shelf_location": "Aisle 3",
		"is_available":   true,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/franchise/products/%s/stock", prod.ID), body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if int(resp["stock_quantity"].(float64)) != 200 {
		t.Errorf("expected stock_quantity 200, got %v", resp["stock_quantity"])
	}
	if resp["shelf_location"] != "Aisle 3" {
		t.Errorf("expected shelf_location 'Aisle 3', got %v", resp["shelf_location"])
	}
}

func TestUpdateProductPricingWithPromo(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "base-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Pricing Promo Store", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "Bakery")
	prod := seedProduct(db, "Bread", cat.ID, 2.50)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	body := map[string]interface{}{
		"retail_price_override":    3.99,
		"promotion_price_override": 2.99,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/franchise/products/%s/pricing", prod.ID), body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["retail_price_override"].(float64) != 3.99 {
		t.Errorf("expected retail_price_override 3.99, got %v", resp["retail_price_override"])
	}
	if resp["promotion_price_override"].(float64) != 2.99 {
		t.Errorf("expected promotion_price_override 2.99, got %v", resp["promotion_price_override"])
	}
}

// ---------- Additional coverage tests for UpdatePromotion ----------

func TestPortalUpdatePromotionAllFields(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "promo-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Update Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	promo := seedFranchisePromotion(db, franchise.ID, "Original Title")

	isActive := false
	body := map[string]interface{}{
		"title":       "Updated Title",
		"description": "Updated description text",
		"image":       "https://example.com/new-image.png",
		"product_url": "https://example.com/product/1",
		"is_active":   isActive,
		"start_date":  "2025-06-01T00:00:00Z",
		"end_date":    "2025-12-31T23:59:59Z",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/promotions/"+promo.ID.String(), body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["title"] != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %v", resp["title"])
	}
	if resp["description"] != "Updated description text" {
		t.Errorf("expected description 'Updated description text', got %v", resp["description"])
	}
	if resp["image"] != "https://example.com/new-image.png" {
		t.Errorf("expected image updated, got %v", resp["image"])
	}
	if resp["product_url"] != "https://example.com/product/1" {
		t.Errorf("expected product_url updated, got %v", resp["product_url"])
	}
	if resp["is_active"] != false {
		t.Errorf("expected is_active false, got %v", resp["is_active"])
	}
}

// ---------- Additional coverage for GetFranchiseOrders (admin) ----------

func TestGetFranchiseOrdersWithItems(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "order-owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Order Franchise", owner.ID)
	cat := seedCategory(db, "OrderCat")
	prod := seedProduct(db, "OrderProd", cat.ID, 9.99)
	customer, _ := seedTestUser(db, "customer@test.com", "customer", nil)

	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/admin/franchises/"+franchise.ID.String()+"/orders", nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Fatalf("expected 1 order, got %d", len(result))
	}

	orderResp := result[0].(map[string]interface{})
	if orderResp["id"].(string) != order.ID.String() {
		t.Errorf("expected order ID %s, got %v", order.ID.String(), orderResp["id"])
	}
}

// ---------- Additional coverage for CreateFranchise with product backfill ----------

func TestCreateFranchiseWithProductBackfill(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	// Create some active products first that should be backfilled
	cat := seedCategory(db, "BackfillCat")
	seedProduct(db, "BackfillProd1", cat.ID, 5.99)
	seedProduct(db, "BackfillProd2", cat.ID, 7.99)

	body := map[string]interface{}{
		"name":           "Backfill Franchise",
		"slug":           "backfill-franchise",
		"owner_email":    "backfill-owner@test.com",
		"owner_name":     "Backfill Owner",
		"owner_password": "password123",
		"latitude":       51.5,
		"longitude":      -0.12,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/admin/franchises", body, adminToken))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	franchiseIDStr := resp["id"].(string)

	// Verify franchise products were backfilled
	var fps []models.FranchiseProduct
	fID, _ := uuid.Parse(franchiseIDStr)
	db.Where("franchise_id = ?", fID).Find(&fps)
	if len(fps) != 2 {
		t.Errorf("expected 2 backfilled franchise products, got %d", len(fps))
	}

	// Verify default delivery values were applied
	if resp["delivery_radius"].(float64) != 5 {
		t.Errorf("expected default delivery_radius 5, got %v", resp["delivery_radius"])
	}
	if resp["delivery_fee"].(float64) != 4.99 {
		t.Errorf("expected default delivery_fee 4.99, got %v", resp["delivery_fee"])
	}
	if resp["free_delivery_min"].(float64) != 50 {
		t.Errorf("expected default free_delivery_min 50, got %v", resp["free_delivery_min"])
	}
}

// ---------- Additional coverage for CreateFranchise with custom delivery values ----------

func TestCreateFranchiseWithCustomDeliveryValues(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	body := map[string]interface{}{
		"name":              "Custom Delivery Franchise",
		"slug":              "custom-delivery-franchise",
		"owner_email":       "custom-delivery@test.com",
		"owner_name":        "Custom Owner",
		"owner_password":    "password123",
		"latitude":          52.0,
		"longitude":         -1.0,
		"delivery_radius":   15.0,
		"delivery_fee":      3.50,
		"free_delivery_min": 30.0,
		"address":           "100 Custom Road",
		"city":              "Manchester",
		"post_code":         "M1 1AA",
		"phone":             "+441234567890",
		"email":             "franchise@custom.com",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/admin/franchises", body, adminToken))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["delivery_radius"].(float64) != 15.0 {
		t.Errorf("expected delivery_radius 15.0, got %v", resp["delivery_radius"])
	}
	if resp["delivery_fee"].(float64) != 3.50 {
		t.Errorf("expected delivery_fee 3.50, got %v", resp["delivery_fee"])
	}
	if resp["free_delivery_min"].(float64) != 30.0 {
		t.Errorf("expected free_delivery_min 30.0, got %v", resp["free_delivery_min"])
	}

	// Verify store hours were created (7 days)
	storeHours, ok := resp["store_hours"].([]interface{})
	if !ok || len(storeHours) != 7 {
		t.Errorf("expected 7 store hours, got %v", resp["store_hours"])
	}
}

// ---------- Additional coverage for GetMyOrders with status filter ----------

func TestGetMyOrdersWithStatusFilter(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "order-filter-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Order Filter Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "OFCat")
	prod := seedProduct(db, "OFProd", cat.ID, 5.99)
	customer, _ := seedTestUser(db, "cust-filter@test.com", "customer", nil)

	order1 := seedOrder(db, customer.ID, franchise.ID, prod.ID)
	order2 := seedOrder(db, customer.ID, franchise.ID, prod.ID)
	// Update order2 status to confirmed
	db.Model(&order2).Update("status", "confirmed")

	// Without filter - should return all orders
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/orders", nil, token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	allOrders := parseResponseArray(w)
	if len(allOrders) != 2 {
		t.Errorf("expected 2 orders without filter, got %d", len(allOrders))
	}

	// With status filter - should return only matching orders
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("GET", "/api/franchise/orders?status=pending", nil, token))
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	filteredOrders := parseResponseArray(w2)
	if len(filteredOrders) != 1 {
		t.Errorf("expected 1 pending order, got %d", len(filteredOrders))
	}
	_ = order1 // used for seeding
}

// ---------- Additional coverage for InviteStaff with existing user ----------

func TestInviteStaffExistingUserUpdate(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "staff-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Staff Update Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Create an existing customer user
	existingUser, _ := seedTestUser(db, "existing-customer@test.com", "customer", nil)

	body := map[string]interface{}{
		"email":    "existing-customer@test.com",
		"name":     "Existing Customer",
		"password": "password123",
		"role":     "staff",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/franchise/staff", body, token))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the user's role was updated to franchise_staff
	var updatedUser models.User
	db.First(&updatedUser, existingUser.ID)
	if updatedUser.Role != "franchise_staff" {
		t.Errorf("expected role 'franchise_staff', got %s", updatedUser.Role)
	}
	if updatedUser.FranchiseID == nil || *updatedUser.FranchiseID != franchise.ID {
		t.Errorf("expected franchise_id %s, got %v", franchise.ID, updatedUser.FranchiseID)
	}
}

// ---------- Additional coverage for UpdateOrderStatus invalid transition ----------

func TestPortalUpdateOrderStatusFromDelivered(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "invalid-trans-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Invalid Trans Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "ITCat")
	prod := seedProduct(db, "ITProd", cat.ID, 5.99)
	customer, _ := seedTestUser(db, "cust-invalid@test.com", "customer", nil)

	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)
	// Update to delivered status
	db.Model(&order).Update("status", "delivered")

	// Try to transition from delivered to pending (invalid)
	body := map[string]interface{}{
		"status": "pending",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/orders/"+order.ID.String()+"/status", body, token))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------- Additional coverage for UpdateOrderStatus order not found ----------

func TestPortalUpdateOrderStatusOrderMissing(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "notfound-order-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "NotFound Order Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	body := map[string]interface{}{
		"status": "confirmed",
	}

	fakeID := uuid.New().String()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/orders/"+fakeID+"/status", body, token))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------- Additional coverage for GetFranchisePromotions with date-filtered promotions ----------

func TestGetFranchisePromotionsDateFiltering(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "promo-date-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Date Franchise", owner.ID)

	// Create an active promo (should appear)
	seedFranchisePromotion(db, franchise.ID, "Active Promo")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchises/"+franchise.ID.String()+"/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) < 1 {
		t.Errorf("expected at least 1 promotion, got %d", len(result))
	}
}

// ---------- Additional coverage for UpdateFranchise with partial fields ----------

func TestUpdateFranchisePartialFields(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "partial-owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	franchise := seedFranchise(db, "Partial Update Franchise", owner.ID)

	// Update only name and address
	body := map[string]interface{}{
		"name":    "Renamed Franchise",
		"address": "New Address 123",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/admin/franchises/"+franchise.ID.String(), body, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["name"] != "Renamed Franchise" {
		t.Errorf("expected name 'Renamed Franchise', got %v", resp["name"])
	}
	if resp["address"] != "New Address 123" {
		t.Errorf("expected address 'New Address 123', got %v", resp["address"])
	}
}

// ---------- Additional coverage for DeletePromotion success path ----------

func TestPortalDeletePromotionSuccess(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "del-promo-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Del Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	promo := seedFranchisePromotion(db, franchise.ID, "To Delete Promo")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", "/api/franchise/promotions/"+promo.ID.String(), nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify promotion is deleted
	var count int64
	db.Model(&models.FranchisePromotion{}).Where("id = ?", promo.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected promotion to be deleted, but still exists")
	}
}

// ---------- Additional coverage for UpdateProductStock not found ----------

func TestPortalUpdateProductStockNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "stock-nf-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Stock NF Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	fakeID := uuid.New().String()
	body := map[string]interface{}{
		"stock_quantity": 10,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/products/"+fakeID+"/stock", body, token))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------- Additional coverage for UpdateProductPricing not found ----------

func TestPortalUpdateProductPricingNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "pricing-nf-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Pricing NF Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	fakeID := uuid.New().String()
	body := map[string]interface{}{
		"retail_price_override": 5.99,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/products/"+fakeID+"/pricing", body, token))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------- Additional coverage for UpdateMyFranchise invalid JSON ----------

func TestUpdateMyFranchiseInvalidJSON(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "bad-json-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Bad JSON Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Send malformed JSON
	req := httptest.NewRequest("PUT", "/api/franchise/me", strings.NewReader("not valid json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------- Additional coverage for CreatePromotion with all fields ----------

func TestCreatePromotionAllFields(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "full-promo-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Full Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	body := map[string]interface{}{
		"title":       "Summer Sale",
		"description": "Big summer discounts",
		"image":       "https://example.com/summer.png",
		"product_url": "https://example.com/summer-deals",
		"is_active":   true,
		"start_date":  "2025-06-01T00:00:00Z",
		"end_date":    "2025-08-31T23:59:59Z",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/franchise/promotions", body, token))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["title"] != "Summer Sale" {
		t.Errorf("expected title 'Summer Sale', got %v", resp["title"])
	}
	if resp["description"] != "Big summer discounts" {
		t.Errorf("expected description set, got %v", resp["description"])
	}
	if resp["is_active"] != true {
		t.Errorf("expected is_active true, got %v", resp["is_active"])
	}
}

// ---------- Additional coverage for GetMyProducts without search ----------

func TestGetMyProductsNoSearch(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "prods-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Products Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "ProdsCat")
	prod := seedProduct(db, "ProdsItem", cat.ID, 4.99)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/products", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 product, got %d", len(result))
	}
}

// ---------- Additional coverage for UpdateStoreHours with multiple days ----------

func TestUpdateStoreHoursMultipleDays(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "hours-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Hours Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Seed store hours for the franchise
	seedStoreHours(db, franchise.ID)

	body := []map[string]interface{}{
		{"day_of_week": 1, "open_time": "07:00", "close_time": "23:00", "is_closed": false},
		{"day_of_week": 2, "open_time": "08:00", "close_time": "22:00", "is_closed": false},
		{"day_of_week": 6, "open_time": "10:00", "close_time": "18:00", "is_closed": true},
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", "/api/franchise/hours", body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 7 {
		t.Errorf("expected 7 store hours entries, got %d", len(result))
	}
}

// ---------- Additional coverage for GetMyStaff with multiple staff ----------

func TestGetMyStaffMultiple(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "multi-staff-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Multi Staff Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	staff1, _ := seedTestUser(db, "staff1@test.com", "franchise_staff", &franchise.ID)
	staff2, _ := seedTestUser(db, "staff2@test.com", "franchise_staff", &franchise.ID)
	seedFranchiseStaff(db, franchise.ID, staff1.ID, "staff")
	seedFranchiseStaff(db, franchise.ID, staff2.ID, "manager")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/staff", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2 staff members, got %d", len(result))
	}
}

// ---------- Additional coverage for GetMyPromotions with multiple promotions ----------

func TestGetMyPromotionsMultiple(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "multi-promo-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Multi Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	seedFranchisePromotion(db, franchise.ID, "Promo A")
	seedFranchisePromotion(db, franchise.ID, "Promo B")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/promotions", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2 promotions, got %d", len(result))
	}
}

// ==================== Additional Coverage Boost Tests ====================

// TestGetFranchiseOrdersWithMultipleOrders tests the admin GetFranchiseOrders with
// multiple orders that include items and users preloaded.
func TestGetFranchiseOrdersWithMultipleOrders(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "multi-order-owner@test.com", "franchise_owner", nil)
	_, adminToken := seedTestUser(db, "admin-order@test.com", "admin", nil)
	franchise := seedFranchise(db, "Multi Order Franchise", owner.ID)
	cat := seedCategory(db, "MultiOrderCat")
	prod1 := seedProduct(db, "OrderProd1", cat.ID, 10.00)
	prod2 := seedProduct(db, "OrderProd2", cat.ID, 20.00)
	customer1, _ := seedTestUser(db, "cust1-order@test.com", "customer", nil)
	customer2, _ := seedTestUser(db, "cust2-order@test.com", "customer", nil)

	seedOrder(db, customer1.ID, franchise.ID, prod1.ID)
	seedOrder(db, customer1.ID, franchise.ID, prod2.ID)
	seedOrder(db, customer2.ID, franchise.ID, prod1.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/admin/franchises/"+franchise.ID.String()+"/orders", nil, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 3 {
		t.Errorf("expected 3 orders, got %d", len(result))
	}

	// Verify preloaded items and user
	for _, item := range result {
		orderMap := item.(map[string]interface{})
		items, ok := orderMap["items"].([]interface{})
		if !ok || len(items) == 0 {
			t.Error("expected order items to be preloaded")
		}
		if orderMap["user"] == nil {
			t.Error("expected user to be preloaded in order")
		}
	}
}

// TestGetFranchisePromotionsExpired tests that expired promotions are filtered out.
func TestGetFranchisePromotionsExpired(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "expired-promo-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Expired Promo Franchise", owner.ID)

	// Create an active promo with no date restrictions
	activePromo := seedFranchisePromotion(db, franchise.ID, "Active No Dates")

	// Create an expired promo (end_date in the past)
	pastDate := time.Now().Add(-48 * time.Hour)
	expiredPromo := seedFranchisePromotion(db, franchise.ID, "Expired Promo")
	db.Model(&expiredPromo).Update("end_date", pastDate)

	// Create a future promo (start_date in the future)
	futureDate := time.Now().Add(48 * time.Hour)
	futurePromo := seedFranchisePromotion(db, franchise.ID, "Future Promo")
	db.Model(&futurePromo).Update("start_date", futureDate)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchises/"+franchise.ID.String()+"/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	// Only the active promo with no date restrictions should appear
	if len(result) != 1 {
		t.Errorf("expected 1 active non-expired promotion, got %d", len(result))
	}

	if len(result) > 0 {
		promoMap := result[0].(map[string]interface{})
		if promoMap["title"] != "Active No Dates" {
			t.Errorf("expected 'Active No Dates', got %v", promoMap["title"])
		}
	}
	_ = activePromo
}

// TestGetFranchisePromotionsInactive tests that inactive promotions are filtered out.
func TestGetFranchisePromotionsInactive(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "inactive-promo-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Inactive Promo Franchise", owner.ID)

	// Create an active promo
	seedFranchisePromotion(db, franchise.ID, "Active Promo")

	// Create an inactive promo
	inactivePromo := seedFranchisePromotion(db, franchise.ID, "Inactive Promo")
	db.Model(&inactivePromo).Update("is_active", false)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchises/"+franchise.ID.String()+"/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	// Only the active promo should appear
	if len(result) != 1 {
		t.Errorf("expected 1 active promotion, got %d", len(result))
	}
}

// TestCreateFranchiseDuplicateSlug tests creating a franchise with a duplicate slug.
func TestCreateFranchiseDuplicateSlug(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)
	_, adminToken := seedTestUser(db, "admin-dup@test.com", "admin", nil)

	// Create first franchise
	body1 := map[string]interface{}{
		"name":           "First Franchise",
		"slug":           "unique-slug",
		"owner_email":    "owner-slug1@test.com",
		"owner_name":     "Owner 1",
		"owner_password": "password123",
		"latitude":       51.5074,
		"longitude":      -0.1278,
	}

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, authRequest("POST", "/api/admin/franchises", body1, adminToken))
	if w1.Code != http.StatusCreated {
		t.Fatalf("expected 201 for first franchise, got %d: %s", w1.Code, w1.Body.String())
	}

	// Try to create second franchise with the same slug
	body2 := map[string]interface{}{
		"name":           "Second Franchise",
		"slug":           "unique-slug",
		"owner_email":    "owner-slug2@test.com",
		"owner_name":     "Owner 2",
		"owner_password": "password123",
		"latitude":       52.0,
		"longitude":      -1.0,
	}

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("POST", "/api/admin/franchises", body2, adminToken))
	if w2.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for duplicate slug, got %d: %s", w2.Code, w2.Body.String())
	}
}

// TestCreateFranchiseStoreHoursAndProductBackfillVerified tests that CreateFranchise
// properly creates 7 store hours and backfills active products.
func TestCreateFranchiseStoreHoursAndProductBackfillVerified(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)
	_, adminToken := seedTestUser(db, "admin-backfill@test.com", "admin", nil)

	// Create products before franchise
	cat := seedCategory(db, "BackfillVerifyCat")
	prod1 := seedProduct(db, "BackfillVerify1", cat.ID, 5.99)
	prod2 := seedProduct(db, "BackfillVerify2", cat.ID, 7.99)
	prod3 := seedProduct(db, "BackfillVerify3", cat.ID, 9.99)
	// Deactivate one product
	db.Model(&prod3).Update("status", "inactive")

	body := map[string]interface{}{
		"name":           "Backfill Verify Franchise",
		"slug":           "backfill-verify-franchise",
		"owner_email":    "bfv-owner@test.com",
		"owner_name":     "BFV Owner",
		"owner_password": "password123",
		"latitude":       51.5074,
		"longitude":      -0.1278,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/admin/franchises", body, adminToken))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	franchiseIDStr := resp["id"].(string)
	fID, _ := uuid.Parse(franchiseIDStr)

	// Verify 7 store hours were created
	var storeHours []models.StoreHours
	db.Where("franchise_id = ?", fID).Find(&storeHours)
	if len(storeHours) != 7 {
		t.Errorf("expected 7 store hours, got %d", len(storeHours))
	}

	// Verify only active products were backfilled (2 active, not the inactive one)
	var fps []models.FranchiseProduct
	db.Where("franchise_id = ?", fID).Find(&fps)
	if len(fps) != 2 {
		t.Errorf("expected 2 backfilled franchise products (active only), got %d", len(fps))
	}

	_ = prod1
	_ = prod2
}

// TestGetFranchisePromotionsWithDateRange tests promotions with valid start/end dates.
func TestGetFranchisePromotionsWithDateRange(t *testing.T) {
	db := freshDB()
	router := setupFranchiseRouter(db)

	owner, _ := seedTestUser(db, "date-range-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Date Range Franchise", owner.ID)

	// Create a promo that started yesterday and ends tomorrow (should be active)
	promo := seedFranchisePromotion(db, franchise.ID, "Date Range Promo")
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now().Add(24 * time.Hour)
	db.Model(&promo).Updates(map[string]interface{}{
		"start_date": startDate,
		"end_date":   endDate,
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchises/"+franchise.ID.String()+"/promotions", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 active promotion within date range, got %d", len(result))
	}
}

// TestGetMyProductsMultipleProducts tests getting multiple products with various data.
func TestGetMyProductsMultipleProducts(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "multi-prod-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Multi Prod Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat1 := seedCategory(db, "Cat1")
	cat2 := seedCategory(db, "Cat2")
	prod1 := seedProduct(db, "Product A", cat1.ID, 5.00)
	prod2 := seedProduct(db, "Product B", cat1.ID, 10.00)
	prod3 := seedProduct(db, "Product C", cat2.ID, 15.00)

	seedFranchiseProduct(db, franchise.ID, prod1.ID)
	seedFranchiseProduct(db, franchise.ID, prod2.ID)
	seedFranchiseProduct(db, franchise.ID, prod3.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/products", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 3 {
		t.Errorf("expected 3 products, got %d", len(result))
	}
}
