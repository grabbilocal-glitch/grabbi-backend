package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"grabbi-backend/models"

	"github.com/google/uuid"
)

func TestCreateOrderSuccess(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	user, token := seedTestUser(db, "orderer@test.com", "customer", nil)
	cat := seedCategory(db, "OrderCat")
	prod := seedProduct(db, "Order Product", cat.ID, 10.00)

	// Add item to cart
	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  2,
	}
	db.Create(&cartItem)

	body := map[string]interface{}{
		"delivery_address": "123 Test St, London",
		"payment_method":   "card",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/orders", body, token))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["status"] != "pending" {
		t.Errorf("expected status 'pending', got %v", resp["status"])
	}
	if resp["delivery_address"] != "123 Test St, London" {
		t.Errorf("expected delivery_address, got %v", resp["delivery_address"])
	}

	// Verify cart is cleared after order
	var cartCount int64
	db.Model(&models.CartItem{}).Where("user_id = ?", user.ID).Count(&cartCount)
	if cartCount != 0 {
		t.Errorf("expected cart to be cleared after order, got %d items", cartCount)
	}
}

func TestCreateOrderEmptyCartError(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	_, token := seedTestUser(db, "emptycart@test.com", "customer", nil)

	body := map[string]interface{}{
		"delivery_address": "123 Test St",
		"payment_method":   "card",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/orders", body, token))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Cart is empty" {
		t.Errorf("expected 'Cart is empty', got %v", resp["error"])
	}
}

func TestCreateOrderValidation(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	_, token := seedTestUser(db, "validate@test.com", "customer", nil)

	// Missing required delivery_address
	body := map[string]interface{}{
		"payment_method": "card",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/orders", body, token))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetOrdersForUser(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	user, token := seedTestUser(db, "myorders@test.com", "customer", nil)

	// Create an order directly
	order := models.Order{
		ID:              uuid.New(),
		UserID:          user.ID,
		OrderNumber:     "ORD-TEST-001",
		Status:          models.OrderStatusPending,
		Subtotal:        20.00,
		Total:           23.75,
		DeliveryFee:     3.75,
		DeliveryAddress: "123 Main St",
	}
	db.Create(&order)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/orders", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 order, got %d", len(result))
	}
}

func TestGetOrderByID(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	user, token := seedTestUser(db, "orderbyid@test.com", "customer", nil)

	order := models.Order{
		ID:              uuid.New(),
		UserID:          user.ID,
		OrderNumber:     "ORD-TEST-002",
		Status:          models.OrderStatusPending,
		Subtotal:        15.00,
		Total:           18.75,
		DeliveryFee:     3.75,
		DeliveryAddress: "456 Oak Ave",
	}
	db.Create(&order)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", fmt.Sprintf("/api/orders/%s", order.ID), nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["order_number"] != "ORD-TEST-002" {
		t.Errorf("expected order_number 'ORD-TEST-002', got %v", resp["order_number"])
	}
}

func TestGetOrderNotFound(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	_, token := seedTestUser(db, "noorder@test.com", "customer", nil)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", fmt.Sprintf("/api/orders/%s", fakeID), nil, token))

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateOrderStatusSuccess(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)

	order := models.Order{
		ID:              uuid.New(),
		UserID:          customer.ID,
		OrderNumber:     "ORD-TEST-003",
		Status:          models.OrderStatusPending,
		Subtotal:        10.00,
		Total:           13.75,
		DeliveryFee:     3.75,
		DeliveryAddress: "789 Pine Rd",
	}
	db.Create(&order)

	body := map[string]string{
		"status": "confirmed",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/admin/orders/%s/status", order.ID), body, adminToken))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["status"] != "confirmed" {
		t.Errorf("expected status 'confirmed', got %v", resp["status"])
	}
}

func TestUpdateOrderStatusInvalidTransition(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)

	order := models.Order{
		ID:              uuid.New(),
		UserID:          customer.ID,
		OrderNumber:     "ORD-TEST-004",
		Status:          models.OrderStatusPending,
		Subtotal:        10.00,
		Total:           13.75,
		DeliveryFee:     3.75,
		DeliveryAddress: "101 Elm St",
	}
	db.Create(&order)

	// Cannot go from pending directly to delivered
	body := map[string]string{
		"status": "delivered",
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/admin/orders/%s/status", order.ID), body, adminToken))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	errMsg, _ := resp["error"].(string)
	if errMsg == "" {
		t.Error("expected error message about invalid transition")
	}
}

func TestCreateOrderWithFranchiseID(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "TestProd", cat.ID, 10.00)

	user, token := seedTestUser(db, "customer@test.com", "customer", nil)
	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "TestFranch", owner.ID)

	// Add to cart
	cart := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  1,
	}
	db.Create(&cart)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/orders", map[string]interface{}{
		"delivery_address": "123 Test St",
		"payment_method":   "card",
		"franchise_id":     franchise.ID.String(),
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateOrderInvalidFranchiseID(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "TestProd", cat.ID, 10.00)
	user, token := seedTestUser(db, "customer@test.com", "customer", nil)

	cart := models.CartItem{
		ID: uuid.New(), UserID: user.ID, ProductID: prod.ID, Quantity: 1,
	}
	db.Create(&cart)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/orders", map[string]interface{}{
		"delivery_address": "123 Test St",
		"franchise_id":     "not-a-uuid",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateOrderFranchiseNotFound(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "TestProd", cat.ID, 10.00)
	user, token := seedTestUser(db, "customer@test.com", "customer", nil)

	cart := models.CartItem{
		ID: uuid.New(), UserID: user.ID, ProductID: prod.ID, Quantity: 1,
	}
	db.Create(&cart)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/orders", map[string]interface{}{
		"delivery_address": "123 Test St",
		"franchise_id":     uuid.New().String(),
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateOrderWithCoordinates(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "TestProd", cat.ID, 10.00)
	user, token := seedTestUser(db, "customer@test.com", "customer", nil)
	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	seedFranchise(db, "NearbyFranch", owner.ID)

	cart := models.CartItem{
		ID: uuid.New(), UserID: user.ID, ProductID: prod.ID, Quantity: 1,
	}
	db.Create(&cart)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/orders", map[string]interface{}{
		"delivery_address": "123 Test St",
		"customer_lat":     51.5074,
		"customer_lng":     -0.1278,
	}, token)
	router.ServeHTTP(w, req)
	// Should find the nearby franchise (same coordinates)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateOrderNoFranchiseInRange(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "TestProd", cat.ID, 10.00)
	user, token := seedTestUser(db, "customer@test.com", "customer", nil)
	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	seedFranchise(db, "FarFranch", owner.ID) // London coordinates

	cart := models.CartItem{
		ID: uuid.New(), UserID: user.ID, ProductID: prod.ID, Quantity: 1,
	}
	db.Create(&cart)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/orders", map[string]interface{}{
		"delivery_address": "Far Away Place",
		"customer_lat":     35.6762, // Tokyo
		"customer_lng":     139.6503,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateOrderInsufficientStock(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "LowStock", cat.ID, 10.00)
	db.Model(&prod).Update("stock_quantity", 1)
	user, token := seedTestUser(db, "customer@test.com", "customer", nil)

	cart := models.CartItem{
		ID: uuid.New(), UserID: user.ID, ProductID: prod.ID, Quantity: 5,
	}
	db.Create(&cart)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/orders", map[string]interface{}{
		"delivery_address": "123 Test St",
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetOrdersAsAdmin(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "TestProd", cat.ID, 10.00)
	user, _ := seedTestUser(db, "customer@test.com", "customer", nil)
	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "TestFranch", owner.ID)
	seedOrder(db, user.ID, franchise.ID, prod.ID)

	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/orders", nil, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) == 0 {
		t.Error("admin should see all orders")
	}
}

func TestUpdateOrderStatusNotFound(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := authRequest("PUT", "/api/admin/orders/"+uuid.New().String()+"/status", map[string]interface{}{
		"status": "confirmed",
	}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateOrderStatusInvalidBody(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)

	order := models.Order{
		ID:              uuid.New(),
		UserID:          customer.ID,
		OrderNumber:     "ORD-TEST-INV-BODY",
		Status:          models.OrderStatusPending,
		Subtotal:        10.00,
		Total:           13.75,
		DeliveryFee:     3.75,
		DeliveryAddress: "123 Test St",
	}
	db.Create(&order)

	// Send empty body
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/admin/orders/%s/status", order.ID), map[string]interface{}{}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 for missing status, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateOrderStatusDeliveredToAnything(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)

	order := models.Order{
		ID:              uuid.New(),
		UserID:          customer.ID,
		OrderNumber:     "ORD-TEST-DELIVERED",
		Status:          models.OrderStatusDelivered,
		Subtotal:        10.00,
		Total:           13.75,
		DeliveryFee:     3.75,
		DeliveryAddress: "123 Test St",
	}
	db.Create(&order)

	// Try to go from delivered to pending (invalid)
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/admin/orders/%s/status", order.ID), map[string]interface{}{
		"status": "pending",
	}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid transition from delivered, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	errMsg, _ := resp["error"].(string)
	if errMsg == "" {
		t.Error("expected error message about invalid transition from delivered")
	}
}

func TestUpdateOrderStatusCancelledToAnything(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)

	order := models.Order{
		ID:              uuid.New(),
		UserID:          customer.ID,
		OrderNumber:     "ORD-TEST-CANCELLED",
		Status:          models.OrderStatusCancelled,
		Subtotal:        10.00,
		Total:           13.75,
		DeliveryFee:     3.75,
		DeliveryAddress: "123 Test St",
	}
	db.Create(&order)

	// Try to go from cancelled to confirmed (invalid)
	w := httptest.NewRecorder()
	req := authRequest("PUT", fmt.Sprintf("/api/admin/orders/%s/status", order.ID), map[string]interface{}{
		"status": "confirmed",
	}, adminToken)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid transition from cancelled, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetOrdersAsFranchiseOwner(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "OrderFranchise", owner.ID)
	fID := franchise.ID
	// Update owner's franchise ID
	db.Model(&owner).Update("franchise_id", fID)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OrderProd", cat.ID, 10.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	seedOrder(db, customer.ID, franchise.ID, prod.ID)

	// Create token with franchise_id
	_, ownerToken := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/orders", nil, ownerToken)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 order for franchise owner, got %d", len(result))
	}
}

func TestGetOrderAsFranchiseOwner(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "OrderFranchise", owner.ID)

	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OrderProd", cat.ID, 10.00)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)
	order := seedOrder(db, customer.ID, franchise.ID, prod.ID)

	_, ownerToken := seedFranchiseOwnerWithToken(db, franchise)

	w := httptest.NewRecorder()
	req := authRequest("GET", fmt.Sprintf("/api/orders/%s", order.ID), nil, ownerToken)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["status"] != "pending" {
		t.Errorf("expected status 'pending', got %v", resp["status"])
	}
}

func TestGetOrdersEmpty(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	_, token := seedTestUser(db, "emptyorders@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/orders", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected 0 orders, got %d", len(result))
	}
}

func TestUpdateOrderStatusFullChain(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)
	_, adminToken := seedTestUser(db, "admin@test.com", "admin", nil)
	customer, _ := seedTestUser(db, "cust@test.com", "customer", nil)

	order := models.Order{
		ID:              uuid.New(),
		UserID:          customer.ID,
		OrderNumber:     "ORD-TEST-CHAIN",
		Status:          models.OrderStatusPending,
		Subtotal:        10.00,
		Total:           13.75,
		DeliveryFee:     3.75,
		DeliveryAddress: "123 Test St",
	}
	db.Create(&order)

	// Walk through the full order lifecycle: pending -> confirmed -> preparing -> ready -> out_for_delivery -> delivered
	transitions := []string{"confirmed", "preparing", "ready", "out_for_delivery", "delivered"}
	for _, status := range transitions {
		w := httptest.NewRecorder()
		req := authRequest("PUT", fmt.Sprintf("/api/admin/orders/%s/status", order.ID),
			map[string]interface{}{"status": status}, adminToken)
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("expected 200 for transition to %s, got %d: %s", status, w.Code, w.Body.String())
		}
		resp := parseResponse(w)
		if resp["status"] != status {
			t.Errorf("expected status '%s', got %v", status, resp["status"])
		}
	}
}

func TestCreateOrderWithFranchisePriceOverride(t *testing.T) {
	db := freshDB()
	router := setupOrderRouter(db)

	user, token := seedTestUser(db, "customer@test.com", "customer", nil)
	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "PriceFranch", owner.ID)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "OverrideProd", cat.ID, 10.00)

	// Set franchise product with price override
	fp := seedFranchiseProduct(db, franchise.ID, prod.ID)
	overridePrice := 7.50
	db.Model(&fp).Update("retail_price_override", overridePrice)

	cart := models.CartItem{
		ID: uuid.New(), UserID: user.ID, ProductID: prod.ID, Quantity: 1,
	}
	db.Create(&cart)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/orders", map[string]interface{}{
		"delivery_address": "123 Test St",
		"franchise_id":     franchise.ID.String(),
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	// Subtotal should reflect the overridden price (7.50 * 1)
	subtotal, ok := resp["subtotal"].(float64)
	if !ok {
		t.Fatal("expected subtotal in response")
	}
	if subtotal != 7.50 {
		t.Errorf("expected subtotal 7.50 (overridden price), got %v", subtotal)
	}
}
