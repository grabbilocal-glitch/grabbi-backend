package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"grabbi-backend/models"

	"github.com/google/uuid"
)

// ==================== GetMyFranchise ====================

func TestGetMyFranchise(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "My Franchise", owner.ID)
	db.Model(&owner).Update("franchise_id", franchise.ID)

	// Regenerate token with franchise_id
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/me", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["name"] != "My Franchise" {
		t.Errorf("expected 'My Franchise', got %v", resp["name"])
	}
}

// ==================== UpdateMyFranchise ====================

func TestUpdateMyFranchise(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Old Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("PUT", "/api/franchise/me", map[string]interface{}{
		"address": "123 New Street",
		"phone":   "07700900000",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["address"] != "123 New Street" {
		t.Errorf("expected '123 New Street', got %v", resp["address"])
	}
}

// ==================== GetMyProducts ====================

func TestGetMyProducts(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Product Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "FP1", cat.ID, 5.00)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/products", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 product, got %d", len(result))
	}
}

func TestGetMyProductsEmpty(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Empty Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/products", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 products, got %d", len(result))
	}
}

// ==================== UpdateProductStock ====================

func TestUpdateProductStockSuccess(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Stock Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "StockProd", cat.ID, 5.00)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/products/%s/stock", prod.ID), map[string]interface{}{
		"stock_quantity": 200,
		"reorder_level":  10,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProductStockNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Stock Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/products/%s/stock", fakeID), map[string]interface{}{
		"stock_quantity": 100,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== UpdateProductPricing ====================

func TestUpdateProductPricingSuccess(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Pricing Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "PricingProd", cat.ID, 5.00)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	overridePrice := 12.99
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/products/%s/pricing", prod.ID), map[string]interface{}{
		"retail_price_override": overridePrice,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProductPricingNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Pricing Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/products/%s/pricing", fakeID), map[string]interface{}{
		"retail_price_override": 9.99,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== GetMyOrders ====================

func TestGetMyOrders(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Order Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OrderProd", cat.ID, 10.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/orders", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 order, got %d", len(result))
	}
}

func TestGetMyOrdersFilterByStatus(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Order Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OrderProd", cat.ID, 10.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)

	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)
	// Update one order to confirmed
	db.Model(&order).Update("status", "confirmed")
	// Create another order that stays pending
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	// Filter by pending - should get 1
	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/orders?status=pending", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 pending order, got %d", len(result))
	}
}

func TestGetMyOrdersEmpty(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Empty Order Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/orders", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 orders, got %d", len(result))
	}
}

// ==================== UpdateOrderStatus ====================

func TestPortalUpdateOrderStatusValidTransition(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Status Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "StatusProd", cat.ID, 10.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)

	// pending -> confirmed is valid
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/orders/%s/status", order.ID), map[string]interface{}{
		"status": "confirmed",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPortalUpdateOrderStatusInvalidTransition(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Status Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "StatusProd", cat.ID, 10.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)

	// pending -> delivered is invalid
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/orders/%s/status", order.ID), map[string]interface{}{
		"status": "delivered",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPortalUpdateOrderStatusNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Status Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/orders/%s/status", fakeID), map[string]interface{}{
		"status": "confirmed",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== GetMyStaff ====================

func TestGetMyStaff(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Staff Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	staffUser, _ := seedTestUser(db, "staff@test.com", "franchise_staff", &franchise.ID)
	seedFranchiseStaff(db, franchise.ID, staffUser.ID, "staff")

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/staff", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 staff member, got %d", len(result))
	}
}

// ==================== InviteStaff ====================

func TestInviteStaffNewUser(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Invite Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/franchise/staff", map[string]interface{}{
		"email":    "newstaff@test.com",
		"name":     "New Staff",
		"password": "password123",
		"role":     "staff",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInviteStaffExistingUser(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Invite Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Create an existing user first
	seedTestUser(db, "existing@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/franchise/staff", map[string]interface{}{
		"email":    "existing@test.com",
		"name":     "Existing User",
		"password": "password123",
		"role":     "manager",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInviteStaffInvalidRole(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Invite Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/franchise/staff", map[string]interface{}{
		"email":    "badstaff@test.com",
		"name":     "Bad Role",
		"password": "password123",
		"role":     "admin",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== RemoveStaff ====================

func TestRemoveStaffSuccess(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Remove Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	staffUser, _ := seedTestUser(db, "staff@test.com", "franchise_staff", &franchise.ID)
	staff := seedFranchiseStaff(db, franchise.ID, staffUser.ID, "staff")

	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/franchise/staff/%s", staff.ID), nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRemoveStaffNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Remove Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/franchise/staff/%s", fakeID), nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== GetStoreHours ====================

func TestGetStoreHours(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Hours Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	seedStoreHours(db, franchise.ID)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/hours", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 7 {
		t.Errorf("expected 7 store hours entries, got %d", len(result))
	}
}

// ==================== UpdateStoreHours ====================

func TestUpdateStoreHours(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Hours Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	seedStoreHours(db, franchise.ID)

	// Update Monday hours
	w := httptest.NewRecorder()
	req := authRequest("PUT", "/api/franchise/hours", []map[string]interface{}{
		{
			"day_of_week": 1,
			"open_time":   "08:00",
			"close_time":  "22:00",
			"is_closed":   false,
		},
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 7 {
		t.Errorf("expected 7 store hours returned, got %d", len(result))
	}
}

// ==================== Franchise Promotions ====================

func TestCreateFranchisePromotion(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/franchise/promotions", map[string]interface{}{
		"title":       "Local Sale",
		"description": "10% off today",
		"is_active":   true,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetMyPromotions(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	seedFranchisePromotion(db, franchise.ID, "Promo 1")
	seedFranchisePromotion(db, franchise.ID, "Promo 2")

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/promotions", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2 promotions, got %d", len(result))
	}
}

func TestUpdateFranchisePromotion(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	promo := seedFranchisePromotion(db, franchise.ID, "Old Promo")

	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/promotions/%s", promo.ID), map[string]interface{}{
		"title": "Updated Promo",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["title"] != "Updated Promo" {
		t.Errorf("expected 'Updated Promo', got %v", resp["title"])
	}
}

func TestDeleteFranchisePromotion(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	promo := seedFranchisePromotion(db, franchise.ID, "Delete Promo")

	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/franchise/promotions/%s", promo.ID), nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteFranchisePromotionNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("DELETE", fmt.Sprintf("/api/franchise/promotions/%s", fakeID), nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateFranchisePromotionNotFound(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/franchise/promotions/%s", fakeID), map[string]interface{}{
		"title": "Ghost",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== GetDashboard ====================

func TestGetDashboard(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Dashboard Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Seed some data
	cat := seedCategory(db, "DashCat")
	prod := seedProduct(db, "DashProd", cat.ID, 10.00)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	staffUser, _ := seedTestUser(db, "staff@test.com", "franchise_staff", &franchise.ID)
	seedFranchiseStaff(db, franchise.ID, staffUser.ID, "staff")

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/dashboard", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["total_orders"] == nil {
		t.Error("expected total_orders in dashboard response")
	}
	if resp["staff_count"] == nil {
		t.Error("expected staff_count in dashboard response")
	}
	if resp["product_count"] == nil {
		t.Error("expected product_count in dashboard response")
	}
}

func TestGetDashboardEmpty(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Empty Dashboard", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/dashboard", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	// Should return zero values for empty franchise
	totalOrders, ok := resp["total_orders"].(float64)
	if !ok || totalOrders != 0 {
		t.Errorf("expected 0 total_orders, got %v", resp["total_orders"])
	}
}

// ==================== Access Control ====================

func TestFranchisePortalRequiresAuth(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	// Request without auth token
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/api/franchise/me", nil))
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchisePortalRequiresFranchiseRole(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	// Regular customer should not have access
	_, customerToken := seedTestUser(db, "customer@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/me", nil, customerToken)
	router.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// ==================== Franchise Portal with FranchiseProduct and StoreHours ====================

func TestGetMyProductsWithStockOverride(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Override Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OverrideProd", cat.ID, 5.00)
	fp := seedFranchiseProduct(db, franchise.ID, prod.ID)

	// Override stock
	db.Model(&fp).Update("stock_quantity", 999)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/products", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 product, got %d", len(result))
	}

	// Verify the franchise product stock quantity was overridden
	fpMap, ok := result[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected result to be a map")
	}
	stockQty, ok := fpMap["stock_quantity"].(float64)
	if !ok || int(stockQty) != 999 {
		t.Errorf("expected stock_quantity 999, got %v", fpMap["stock_quantity"])
	}
}

func TestGetStoreHoursEmpty(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "No Hours Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Don't seed store hours

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/hours", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 store hours, got %d", len(result))
	}
}

func TestUpdateMyFranchiseAllFields(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Full Update Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	body := map[string]interface{}{
		"address":           "789 Updated Road",
		"city":              "Bristol",
		"post_code":         "BS1 1AA",
		"phone":             "+44111222333",
		"email":             "updated@store.com",
		"delivery_radius":   15.0,
		"delivery_fee":      2.50,
		"free_delivery_min": 40.0,
	}

	w := httptest.NewRecorder()
	req := authRequest("PUT", "/api/franchise/me", body, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["city"] != "Bristol" {
		t.Errorf("expected city 'Bristol', got %v", resp["city"])
	}
	if resp["email"] != "updated@store.com" {
		t.Errorf("expected email 'updated@store.com', got %v", resp["email"])
	}
}

func TestGetDashboardWithData(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Dashboard Data Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "DashCat")
	prod := seedProduct(db, "DashProd", cat.ID, 10.00)
	fp := seedFranchiseProduct(db, franchise.ID, prod.ID)
	// Set low stock to trigger alert
	db.Model(&fp).Updates(map[string]interface{}{
		"stock_quantity": 2,
		"reorder_level":  5,
	})

	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	staffUser, _ := seedTestUser(db, "staff@test.com", "franchise_staff", &franchise.ID)
	seedFranchiseStaff(db, franchise.ID, staffUser.ID, "staff")

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/franchise/dashboard", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	totalOrders := resp["total_orders"].(float64)
	if int(totalOrders) != 2 {
		t.Errorf("expected total_orders 2, got %v", resp["total_orders"])
	}
	staffCount := resp["staff_count"].(float64)
	if int(staffCount) != 1 {
		t.Errorf("expected staff_count 1, got %v", resp["staff_count"])
	}
	lowStockAlerts := resp["low_stock_alerts"].(float64)
	if int(lowStockAlerts) != 1 {
		t.Errorf("expected low_stock_alerts 1, got %v", resp["low_stock_alerts"])
	}
}

// ==================== Additional Coverage Tests ====================

// TestInviteStaffDuplicateUser tests inviting a user who is already staff at a franchise.
// This triggers the UNIQUE constraint on franchise_staffs.user_id.
func TestInviteStaffDuplicateUser(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "dup-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Dup Staff Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Create a user who is already a staff member
	staffUser, _ := seedTestUser(db, "already-staff@test.com", "franchise_staff", &franchise.ID)
	seedFranchiseStaff(db, franchise.ID, staffUser.ID, "staff")

	// Try to invite the same user again
	body := map[string]interface{}{
		"email":    "already-staff@test.com",
		"name":     "Already Staff",
		"password": "password123",
		"role":     "staff",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/franchise/staff", body, token))

	// Should fail because user_id is unique in franchise_staffs
	if w.Code != http.StatusConflict && w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 409 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// TestInviteStaffExistingUserVerifyFieldUpdates tests that when inviting an existing user,
// their franchise_id and role are properly updated.
func TestInviteStaffExistingUserVerifyFieldUpdates(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "verify-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Verify Staff Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Create an existing customer user (no franchise association)
	existingUser, _ := seedTestUser(db, "become-staff@test.com", "customer", nil)

	body := map[string]interface{}{
		"email":    "become-staff@test.com",
		"name":     "Become Staff",
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

	// Verify the user record was updated
	var updatedUser models.User
	db.First(&updatedUser, existingUser.ID)
	if updatedUser.Role != "franchise_staff" {
		t.Errorf("expected user role 'franchise_staff', got %s", updatedUser.Role)
	}
	if updatedUser.FranchiseID == nil || *updatedUser.FranchiseID != franchise.ID {
		t.Errorf("expected franchise_id %s, got %v", franchise.ID, updatedUser.FranchiseID)
	}

	// Verify user is preloaded in response
	userResp, ok := resp["user"].(map[string]interface{})
	if !ok {
		t.Fatal("expected user to be preloaded in staff response")
	}
	if userResp["email"] != "become-staff@test.com" {
		t.Errorf("expected preloaded user email, got %v", userResp["email"])
	}
}

// TestGetMyStaffWithPreloadedUser tests that staff members include preloaded user data.
func TestGetMyStaffWithPreloadedUser(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "preload-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Preload Staff Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	staffUser1, _ := seedTestUser(db, "preload-staff1@test.com", "franchise_staff", &franchise.ID)
	staffUser2, _ := seedTestUser(db, "preload-staff2@test.com", "franchise_staff", &franchise.ID)
	staffUser3, _ := seedTestUser(db, "preload-staff3@test.com", "franchise_staff", &franchise.ID)
	seedFranchiseStaff(db, franchise.ID, staffUser1.ID, "staff")
	seedFranchiseStaff(db, franchise.ID, staffUser2.ID, "manager")
	seedFranchiseStaff(db, franchise.ID, staffUser3.ID, "staff")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/staff", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 3 {
		t.Errorf("expected 3 staff members, got %d", len(result))
	}

	// Verify user is preloaded
	for _, item := range result {
		staffMap := item.(map[string]interface{})
		userMap, ok := staffMap["user"].(map[string]interface{})
		if !ok {
			t.Fatal("expected user to be preloaded in each staff member")
		}
		if userMap["email"] == nil {
			t.Error("expected user email in preloaded user")
		}
	}
}

// TestGetMyStaffEmpty tests getting staff when none exist.
func TestGetMyStaffEmpty(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "empty-staff-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Empty Staff Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/staff", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 staff members, got %d", len(result))
	}
}

// TestGetStoreHoursWithData tests getting store hours that have been seeded with full day data.
func TestGetStoreHoursWithData(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "hours-data-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Hours Data Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	seedStoreHours(db, franchise.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/hours", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 7 {
		t.Fatalf("expected 7 store hours entries, got %d", len(result))
	}

	// Verify they are ordered by day_of_week
	for i, item := range result {
		hourMap := item.(map[string]interface{})
		dayOfWeek := int(hourMap["day_of_week"].(float64))
		if dayOfWeek != i {
			t.Errorf("expected day_of_week %d at index %d, got %d", i, i, dayOfWeek)
		}
		if hourMap["open_time"] != "09:00" {
			t.Errorf("expected open_time '09:00', got %v", hourMap["open_time"])
		}
		if hourMap["close_time"] != "21:00" {
			t.Errorf("expected close_time '21:00', got %v", hourMap["close_time"])
		}
	}
}

// TestGetMyPromotionsWithData tests getting promotions with data and verifies ordering.
func TestGetMyPromotionsWithData(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "promo-data-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Data Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	seedFranchisePromotion(db, franchise.ID, "First Promo")
	seedFranchisePromotion(db, franchise.ID, "Second Promo")
	seedFranchisePromotion(db, franchise.ID, "Third Promo")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/promotions", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 3 {
		t.Errorf("expected 3 promotions, got %d", len(result))
	}

	// Verify response items have expected fields
	for _, item := range result {
		promoMap := item.(map[string]interface{})
		if promoMap["title"] == nil {
			t.Error("expected title field in promotion")
		}
		if promoMap["franchise_id"] == nil {
			t.Error("expected franchise_id field in promotion")
		}
	}
}

// TestGetMyPromotionsEmpty tests getting promotions when none exist.
func TestGetMyPromotionsEmpty(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "empty-promo-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Empty Promo Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/promotions", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 promotions, got %d", len(result))
	}
}

// TestGetMyProductsSearchNoMatch tests search that returns no results.
func TestGetMyProductsSearchNoMatch(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "search-nomatch-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Search NoMatch Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "SearchNoMatchCat")
	prod := seedProduct(db, "Existing Product", cat.ID, 5.00)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/products?search=NonexistentProduct", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 products for non-matching search, got %d", len(result))
	}
}

// TestGetMyProductsSearchCaseInsensitive tests case-insensitive search.
func TestGetMyProductsSearchCaseInsensitive(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "search-case-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Search Case Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	cat := seedCategory(db, "CaseCat")
	prod := seedProduct(db, "Premium Chocolate", cat.ID, 5.00)
	seedFranchiseProduct(db, franchise.ID, prod.ID)

	// Search with lowercase
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/products?search=premium", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 product for case-insensitive search, got %d", len(result))
	}
}

// TestGetMyStaffDBError tests the error path when the DB query fails.
func TestGetMyStaffDBError(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "staff-err-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Staff Err Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Drop the table to cause a DB error
	db.Exec("DROP TABLE franchise_staffs")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/staff", nil, token))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Failed to fetch staff" {
		t.Errorf("expected 'Failed to fetch staff', got %v", resp["error"])
	}

	// Recreate the table for subsequent tests
	db.Exec(`CREATE TABLE IF NOT EXISTS "franchise_staffs" (
		"id" TEXT PRIMARY KEY,
		"franchise_id" TEXT NOT NULL,
		"user_id" TEXT NOT NULL UNIQUE,
		"role" TEXT NOT NULL DEFAULT 'staff',
		"created_at" DATETIME,
		"updated_at" DATETIME,
		CONSTRAINT fk_franchise_staffs_franchise FOREIGN KEY ("franchise_id") REFERENCES "franchises"("id"),
		CONSTRAINT fk_franchise_staffs_user FOREIGN KEY ("user_id") REFERENCES "users"("id")
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_franchise_staffs_franchise_id ON "franchise_staffs"("franchise_id")`)
}

// TestGetStoreHoursDBError tests the error path when the DB query fails.
func TestGetStoreHoursDBError(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "hours-err-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Hours Err Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Drop the table to cause a DB error
	db.Exec("DROP TABLE store_hours")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/hours", nil, token))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Failed to fetch store hours" {
		t.Errorf("expected 'Failed to fetch store hours', got %v", resp["error"])
	}

	// Recreate the table
	db.Exec(`CREATE TABLE IF NOT EXISTS "store_hours" (
		"id" TEXT PRIMARY KEY,
		"franchise_id" TEXT NOT NULL,
		"day_of_week" INTEGER NOT NULL,
		"open_time" TEXT NOT NULL DEFAULT '09:00',
		"close_time" TEXT NOT NULL DEFAULT '21:00',
		"is_closed" INTEGER DEFAULT 0,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		CONSTRAINT fk_store_hours_franchise FOREIGN KEY ("franchise_id") REFERENCES "franchises"("id")
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_store_hours_franchise_id ON "store_hours"("franchise_id")`)
}

// TestGetMyPromotionsDBError tests the error path when the DB query fails.
func TestGetMyPromotionsDBError(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "promo-err-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Promo Err Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Drop the table to cause a DB error
	db.Exec("DROP TABLE franchise_promotions")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/promotions", nil, token))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Failed to fetch promotions" {
		t.Errorf("expected 'Failed to fetch promotions', got %v", resp["error"])
	}

	// Recreate the table
	db.Exec(`CREATE TABLE IF NOT EXISTS "franchise_promotions" (
		"id" TEXT PRIMARY KEY,
		"franchise_id" TEXT NOT NULL,
		"title" TEXT NOT NULL,
		"description" TEXT,
		"image" TEXT,
		"product_url" TEXT,
		"is_active" INTEGER DEFAULT 1,
		"start_date" DATETIME,
		"end_date" DATETIME,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME,
		CONSTRAINT fk_franchise_promotions_franchise FOREIGN KEY ("franchise_id") REFERENCES "franchises"("id")
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_franchise_promotions_deleted_at ON "franchise_promotions"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_franchise_promotions_franchise_id ON "franchise_promotions"("franchise_id")`)
}

// TestGetMyProductsDBError tests the error path when the DB query fails.
func TestGetMyProductsDBError(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "prod-err-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Prod Err Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Drop the table to cause a DB error
	db.Exec("DROP TABLE franchise_products")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/products", nil, token))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Failed to fetch products" {
		t.Errorf("expected 'Failed to fetch products', got %v", resp["error"])
	}

	// Recreate the table
	db.Exec(`CREATE TABLE IF NOT EXISTS "franchise_products" (
		"id" TEXT PRIMARY KEY,
		"franchise_id" TEXT NOT NULL,
		"product_id" TEXT NOT NULL,
		"retail_price_override" REAL,
		"promotion_price_override" REAL,
		"promotion_start_override" DATETIME,
		"promotion_end_override" DATETIME,
		"stock_quantity" INTEGER DEFAULT 0,
		"reorder_level" INTEGER DEFAULT 5,
		"shelf_location" TEXT,
		"is_available" INTEGER DEFAULT 1,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		CONSTRAINT fk_franchise_products_franchise FOREIGN KEY ("franchise_id") REFERENCES "franchises"("id"),
		CONSTRAINT fk_franchise_products_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
	)`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_franchise_product ON "franchise_products"("franchise_id","product_id")`)
}

// TestGetMyOrdersDBError tests the error path when the DB query fails.
func TestGetMyOrdersDBError(t *testing.T) {
	db := freshDB()
	router := setupFranchisePortalRouter(db)

	owner, _ := seedTestUser(db, "order-err-owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Order Err Franchise", owner.ID)
	_, token := seedFranchiseOwnerWithToken(db, franchise)

	// Drop the table to cause a DB error
	db.Exec("DROP TABLE order_items")
	db.Exec("DROP TABLE orders")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/franchise/orders", nil, token))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Failed to fetch orders" {
		t.Errorf("expected 'Failed to fetch orders', got %v", resp["error"])
	}

	// Recreate the tables
	db.Exec(`CREATE TABLE IF NOT EXISTS "orders" (
		"id" TEXT PRIMARY KEY,
		"user_id" TEXT NOT NULL,
		"franchise_id" TEXT,
		"order_number" TEXT NOT NULL UNIQUE,
		"status" TEXT DEFAULT 'pending',
		"subtotal" REAL NOT NULL,
		"delivery_fee" REAL DEFAULT 0,
		"total" REAL NOT NULL,
		"delivery_address" TEXT,
		"payment_method" TEXT,
		"points_earned" INTEGER DEFAULT 0,
		"customer_lat" REAL,
		"customer_lng" REAL,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME,
		CONSTRAINT fk_orders_user FOREIGN KEY ("user_id") REFERENCES "users"("id")
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_deleted_at ON "orders"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_user_id ON "orders"("user_id")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_franchise_id ON "orders"("franchise_id")`)
	db.Exec(`CREATE TABLE IF NOT EXISTS "order_items" (
		"id" TEXT PRIMARY KEY,
		"order_id" TEXT NOT NULL,
		"product_id" TEXT NOT NULL,
		"image_url" TEXT,
		"quantity" INTEGER NOT NULL,
		"price" REAL NOT NULL,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		CONSTRAINT fk_order_items_order FOREIGN KEY ("order_id") REFERENCES "orders"("id"),
		CONSTRAINT fk_order_items_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON "order_items"("order_id")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON "order_items"("product_id")`)
}

// Ensure all imported identifiers are used
var _ = fmt.Sprintf
var _ = http.StatusOK
var _ = httptest.NewRecorder
var _ models.Franchise
var _ = uuid.New
