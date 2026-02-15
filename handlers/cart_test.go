package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"grabbi-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestAddToCartSuccess(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	_, token := seedTestUser(db, "cart@test.com", "customer", nil)
	cat := seedCategory(db, "CartCat")
	prod := seedProduct(db, "Cart Product", cat.ID, 5.99)

	body := map[string]interface{}{
		"product_id": prod.ID.String(),
		"quantity":   2,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/cart", body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	qty, ok := resp["quantity"].(float64)
	if !ok || int(qty) != 2 {
		t.Errorf("expected quantity 2, got %v", resp["quantity"])
	}
}

func TestGetCartSuccess(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "getcart@test.com", "customer", nil)
	cat := seedCategory(db, "GetCartCat")
	prod := seedProduct(db, "Get Cart Product", cat.ID, 4.99)

	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  3,
	}
	db.Create(&cartItem)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/cart", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Errorf("expected 1 cart item, got %d", len(result))
	}
}

func TestUpdateCartItemQuantity(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "updatecart@test.com", "customer", nil)
	cat := seedCategory(db, "UpdateCartCat")
	prod := seedProduct(db, "Update Cart Product", cat.ID, 6.99)

	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  1,
	}
	db.Create(&cartItem)

	body := map[string]interface{}{
		"quantity": 5,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/cart/%s", cartItem.ID), body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	qty, ok := resp["quantity"].(float64)
	if !ok || int(qty) != 5 {
		t.Errorf("expected quantity 5, got %v", resp["quantity"])
	}
}

func TestRemoveCartItem(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "removecart@test.com", "customer", nil)
	cat := seedCategory(db, "RemoveCartCat")
	prod := seedProduct(db, "Remove Cart Product", cat.ID, 8.99)

	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  1,
	}
	db.Create(&cartItem)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/cart/%s", cartItem.ID), nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["message"] != "Item removed from cart" {
		t.Errorf("expected removal message, got %v", resp["message"])
	}

	// Verify item is deleted
	var count int64
	db.Model(&models.CartItem{}).Where("id = ?", cartItem.ID).Count(&count)
	if count != 0 {
		t.Error("expected cart item to be deleted")
	}
}

func TestClearCart(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "clearcart@test.com", "customer", nil)
	cat := seedCategory(db, "ClearCartCat")
	prod1 := seedProduct(db, "Clear Cart Product 1", cat.ID, 3.99)
	prod2 := seedProduct(db, "Clear Cart Product 2", cat.ID, 4.99)

	db.Create(&models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod1.ID, Quantity: 1})
	db.Create(&models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod2.ID, Quantity: 2})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", "/api/cart", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["message"] != "Cart cleared" {
		t.Errorf("expected 'Cart cleared', got %v", resp["message"])
	}

	var count int64
	db.Model(&models.CartItem{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 cart items, got %d", count)
	}
}

func TestAddDuplicateToCartMerges(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "dupcart@test.com", "customer", nil)
	cat := seedCategory(db, "DupCartCat")
	prod := seedProduct(db, "Dup Cart Product", cat.ID, 7.99)

	// Add product to cart first
	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  2,
	}
	db.Create(&cartItem)

	// Add same product again
	body := map[string]interface{}{
		"product_id": prod.ID.String(),
		"quantity":   3,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/cart", body, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	qty, ok := resp["quantity"].(float64)
	if !ok || int(qty) != 5 {
		t.Errorf("expected merged quantity 5 (2+3), got %v", resp["quantity"])
	}

	// Verify only one cart item exists for this product
	var count int64
	db.Model(&models.CartItem{}).Where("user_id = ? AND product_id = ?", user.ID, prod.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 cart item (merged), got %d", count)
	}
}

func TestAddToCartProductNotFound(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)
	_, token := seedTestUser(db, "user@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/cart", map[string]interface{}{
		"product_id": uuid.New(),
		"quantity":   1,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddToCartInsufficientStock(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)
	cat := seedCategory(db, "TestCat")
	prod := seedProduct(db, "LowStock", cat.ID, 5.00)
	db.Model(&prod).Update("stock_quantity", 2)
	_, token := seedTestUser(db, "user@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/cart", map[string]interface{}{
		"product_id": prod.ID,
		"quantity":   5,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddToCartInvalidBody(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)
	_, token := seedTestUser(db, "user@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("POST", "/api/cart", map[string]interface{}{}, token)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateCartItemNotFound(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)
	_, token := seedTestUser(db, "user@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("PUT", "/api/cart/"+uuid.New().String(), map[string]interface{}{
		"quantity": 2,
	}, token)
	router.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateCartItemInvalidBody(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)
	_, token := seedTestUser(db, "user@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("PUT", "/api/cart/"+uuid.New().String(), map[string]interface{}{}, token)
	router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetCartEmpty(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)
	_, token := seedTestUser(db, "user@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/cart", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	result := parseResponseArray(w)
	if len(result) != 0 {
		t.Errorf("expected empty cart, got %d items", len(result))
	}
}

func TestGetCartWithPreloadedProduct(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "preload@test.com", "customer", nil)
	cat := seedCategory(db, "PreloadCat")
	prod := seedProduct(db, "Preloaded Product", cat.ID, 9.99)

	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  2,
	}
	db.Create(&cartItem)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/cart", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Fatalf("expected 1 cart item, got %d", len(result))
	}

	item := result[0].(map[string]interface{})
	product, ok := item["product"].(map[string]interface{})
	if !ok {
		t.Fatal("expected product to be preloaded in cart item")
	}
	if product["item_name"] != "Preloaded Product" {
		t.Errorf("expected product name 'Preloaded Product', got %v", product["item_name"])
	}
	// Verify category is also preloaded
	category, ok := product["category"].(map[string]interface{})
	if !ok {
		t.Fatal("expected category to be preloaded in product")
	}
	if category["name"] != "PreloadCat" {
		t.Errorf("expected category name 'PreloadCat', got %v", category["name"])
	}
}

func TestUpdateCartItemInsufficientStock(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "stockcart@test.com", "customer", nil)
	cat := seedCategory(db, "StockCat")
	prod := seedProduct(db, "Low Stock Cart Product", cat.ID, 3.99)
	db.Model(&prod).Update("stock_quantity", 2)

	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  1,
	}
	db.Create(&cartItem)

	// Try to update quantity beyond available stock
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("PUT", fmt.Sprintf("/api/cart/%s", cartItem.ID), map[string]interface{}{
		"quantity": 10,
	}, token))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Insufficient stock" {
		t.Errorf("expected 'Insufficient stock', got %v", resp["error"])
	}
}

func TestAddToCartMergeExceedsStock(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "mergecap@test.com", "customer", nil)
	cat := seedCategory(db, "MergeCat")
	prod := seedProduct(db, "Merge Cap Product", cat.ID, 7.99)
	db.Model(&prod).Update("stock_quantity", 5)

	// Create existing cart item with quantity 3
	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  3,
	}
	db.Create(&cartItem)

	// Add 4 more - total would be 7 but stock is 5, should return 400
	body := map[string]interface{}{
		"product_id": prod.ID.String(),
		"quantity":   4,
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/cart", body, token))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if _, ok := resp["error"]; !ok {
		t.Errorf("expected error message in response, got %v", resp)
	}
}

func TestGetCartMultipleItems(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "multicart@test.com", "customer", nil)
	cat := seedCategory(db, "MultiCartCat")
	prod1 := seedProduct(db, "Cart Product A", cat.ID, 4.99)
	prod2 := seedProduct(db, "Cart Product B", cat.ID, 6.99)

	db.Create(&models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod1.ID, Quantity: 1})
	db.Create(&models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod2.ID, Quantity: 3})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/cart", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 2 {
		t.Errorf("expected 2 cart items, got %d", len(result))
	}
}

func TestRemoveFromCartVerifyOtherItemsRemain(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "removeone@test.com", "customer", nil)
	cat := seedCategory(db, "RemoveOneCat")
	prod1 := seedProduct(db, "Remove Cart Prod 1", cat.ID, 4.99)
	prod2 := seedProduct(db, "Remove Cart Prod 2", cat.ID, 6.99)

	item1 := models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod1.ID, Quantity: 1}
	item2 := models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod2.ID, Quantity: 2}
	db.Create(&item1)
	db.Create(&item2)

	// Remove item1 only
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/cart/%s", item1.ID), nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify item1 is deleted but item2 remains
	var count int64
	db.Model(&models.CartItem{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 remaining cart item, got %d", count)
	}
}

func TestClearCartVerifyEmpty(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "clearverify@test.com", "customer", nil)
	cat := seedCategory(db, "ClearVerifyCat")
	prod1 := seedProduct(db, "Clear Verify Prod 1", cat.ID, 3.99)
	prod2 := seedProduct(db, "Clear Verify Prod 2", cat.ID, 5.99)
	prod3 := seedProduct(db, "Clear Verify Prod 3", cat.ID, 7.99)

	db.Create(&models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod1.ID, Quantity: 1})
	db.Create(&models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod2.ID, Quantity: 2})
	db.Create(&models.CartItem{ID: uuid.New(), UserID: user.ID, ProductID: prod3.ID, Quantity: 3})

	// Clear all
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", "/api/cart", nil, token))
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify cart is empty via GET
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("GET", "/api/cart", nil, token))
	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w2.Code, w2.Body.String())
	}
	result := parseResponseArray(w2)
	if len(result) != 0 {
		t.Errorf("expected empty cart after clear, got %d items", len(result))
	}
}

// TestGetCartNoUserIDInContext tests the unauthorized branch when user_id is not set in context.
func TestGetCartNoUserIDInContext(t *testing.T) {
	db := freshDB()
	// Set up a router WITHOUT auth middleware so user_id is never set in context.
	r := gin.New()
	cartHandler := &CartHandler{DB: db}
	r.GET("/api/cart", cartHandler.GetCart)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/cart", nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Unauthorized" {
		t.Errorf("expected 'Unauthorized', got %v", resp["error"])
	}
}

// TestRemoveFromCartNoUserIDInContext tests the unauthorized branch for RemoveFromCart.
func TestRemoveFromCartNoUserIDInContext(t *testing.T) {
	db := freshDB()
	r := gin.New()
	cartHandler := &CartHandler{DB: db}
	r.DELETE("/api/cart/:id", cartHandler.RemoveFromCart)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/cart/"+uuid.New().String(), nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Unauthorized" {
		t.Errorf("expected 'Unauthorized', got %v", resp["error"])
	}
}

// TestClearCartNoUserIDInContext tests the unauthorized branch for ClearCart.
func TestClearCartNoUserIDInContext(t *testing.T) {
	db := freshDB()
	r := gin.New()
	cartHandler := &CartHandler{DB: db}
	r.DELETE("/api/cart", cartHandler.ClearCart)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/cart", nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Unauthorized" {
		t.Errorf("expected 'Unauthorized', got %v", resp["error"])
	}
}

// TestGetCartWithProductImages verifies that product images are preloaded in cart items.
func TestGetCartWithProductImages(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user, token := seedTestUser(db, "cartimg@test.com", "customer", nil)
	cat := seedCategory(db, "ImgCartCat")
	prod := seedProduct(db, "Cart Img Product", cat.ID, 12.99)

	// Add a product image
	img := models.ProductImage{
		ID:        uuid.New(),
		ProductID: prod.ID,
		ImageURL:  "https://example.com/img1.jpg",
		IsPrimary: true,
	}
	db.Create(&img)

	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user.ID,
		ProductID: prod.ID,
		Quantity:  1,
	}
	db.Create(&cartItem)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/cart", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	result := parseResponseArray(w)
	if len(result) != 1 {
		t.Fatalf("expected 1 cart item, got %d", len(result))
	}

	item := result[0].(map[string]interface{})
	product, ok := item["product"].(map[string]interface{})
	if !ok {
		t.Fatal("expected product to be preloaded")
	}

	images, ok := product["images"].([]interface{})
	if !ok || len(images) == 0 {
		t.Fatal("expected product images to be preloaded")
	}
	imgMap := images[0].(map[string]interface{})
	if imgMap["image_url"] != "https://example.com/img1.jpg" {
		t.Errorf("expected image URL, got %v", imgMap["image_url"])
	}
}

// TestRemoveFromCartNonexistentItem tests removing a cart item that does not exist.
// GORM soft delete does not error on missing rows, so this should still return 200.
func TestRemoveFromCartNonexistentItem(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)
	_, token := seedTestUser(db, "removenonexist@test.com", "customer", nil)

	fakeID := uuid.New()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/cart/%s", fakeID), nil, token))

	// GORM soft delete with Where clause does not return error for 0 rows affected
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["message"] != "Item removed from cart" {
		t.Errorf("expected removal message, got %v", resp["message"])
	}
}

// TestRemoveFromCartWrongUser verifies that a user cannot remove another user's cart item.
func TestRemoveFromCartWrongUser(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user1, _ := seedTestUser(db, "cartowner@test.com", "customer", nil)
	_, token2 := seedTestUser(db, "otheruser@test.com", "customer", nil)
	cat := seedCategory(db, "WrongUserCat")
	prod := seedProduct(db, "Wrong User Prod", cat.ID, 5.99)

	cartItem := models.CartItem{
		ID:        uuid.New(),
		UserID:    user1.ID,
		ProductID: prod.ID,
		Quantity:  1,
	}
	db.Create(&cartItem)

	// User2 tries to delete user1's cart item
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", fmt.Sprintf("/api/cart/%s", cartItem.ID), nil, token2))

	// Soft delete with WHERE user_id matches user2 won't find the item, but also won't error
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 (no rows affected, no error), got %d: %s", w.Code, w.Body.String())
	}

	// Verify that user1's cart item is still there
	var count int64
	db.Model(&models.CartItem{}).Where("id = ?", cartItem.ID).Count(&count)
	if count != 1 {
		t.Error("expected user1's cart item to remain untouched")
	}
}

// TestClearCartEmptyCart tests clearing an already empty cart.
func TestClearCartEmptyCart(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)
	_, token := seedTestUser(db, "emptyclear@test.com", "customer", nil)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", "/api/cart", nil, token))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["message"] != "Cart cleared" {
		t.Errorf("expected 'Cart cleared', got %v", resp["message"])
	}
}

// TestClearCartDoesNotAffectOtherUsers verifies that clearing one user's cart
// does not affect another user's cart items.
func TestClearCartDoesNotAffectOtherUsers(t *testing.T) {
	db := freshDB()
	router := setupCartRouter(db)

	user1, token1 := seedTestUser(db, "clearuser1@test.com", "customer", nil)
	user2, _ := seedTestUser(db, "clearuser2@test.com", "customer", nil)
	cat := seedCategory(db, "ClearOtherCat")
	prod := seedProduct(db, "Clear Other Prod", cat.ID, 5.99)

	db.Create(&models.CartItem{ID: uuid.New(), UserID: user1.ID, ProductID: prod.ID, Quantity: 1})
	db.Create(&models.CartItem{ID: uuid.New(), UserID: user2.ID, ProductID: prod.ID, Quantity: 2})

	// User1 clears their cart
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", "/api/cart", nil, token1))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify user1's cart is empty
	var count1 int64
	db.Model(&models.CartItem{}).Where("user_id = ?", user1.ID).Count(&count1)
	if count1 != 0 {
		t.Errorf("expected 0 items for user1, got %d", count1)
	}

	// Verify user2's cart is untouched
	var count2 int64
	db.Model(&models.CartItem{}).Where("user_id = ?", user2.ID).Count(&count2)
	if count2 != 1 {
		t.Errorf("expected 1 item for user2, got %d", count2)
	}
}

// TestAddToCartNoUserIDInContext tests the unauthorized branch for AddToCart.
func TestAddToCartNoUserIDInContext(t *testing.T) {
	db := freshDB()
	r := gin.New()
	cartHandler := &CartHandler{DB: db}
	r.POST("/api/cart", cartHandler.AddToCart)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest("POST", "/api/cart", map[string]interface{}{
		"product_id": uuid.New().String(),
		"quantity":   1,
	}))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateCartItemNoUserIDInContext tests the unauthorized branch for UpdateCartItem.
func TestUpdateCartItemNoUserIDInContext(t *testing.T) {
	db := freshDB()
	r := gin.New()
	cartHandler := &CartHandler{DB: db}
	r.PUT("/api/cart/:id", cartHandler.UpdateCartItem)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, jsonRequest("PUT", "/api/cart/"+uuid.New().String(), map[string]interface{}{
		"quantity": 2,
	}))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetCartDBError tests the error branch when the DB query fails.
func TestGetCartDBError(t *testing.T) {
	db := freshDB()
	user, token := seedTestUser(db, "carterr@test.com", "customer", nil)
	_ = user

	// Drop the cart_items table to force a DB error
	db.Exec("DROP TABLE cart_items")

	router := setupCartRouter(db)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/cart", nil, token))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Failed to fetch cart" {
		t.Errorf("expected 'Failed to fetch cart', got %v", resp["error"])
	}

	// Recreate the table for subsequent tests
	db.Exec(`CREATE TABLE IF NOT EXISTS "cart_items" (
		"id" TEXT PRIMARY KEY,
		"user_id" TEXT NOT NULL,
		"product_id" TEXT NOT NULL,
		"franchise_id" TEXT,
		"quantity" INTEGER DEFAULT 1,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME,
		CONSTRAINT fk_cart_items_user FOREIGN KEY ("user_id") REFERENCES "users"("id"),
		CONSTRAINT fk_cart_items_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
	)`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_user_product ON "cart_items"("user_id","product_id")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cart_items_deleted_at ON "cart_items"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cart_items_franchise_id ON "cart_items"("franchise_id")`)
}

// TestRemoveFromCartDBError tests the error branch when the DB delete fails.
func TestRemoveFromCartDBError(t *testing.T) {
	db := freshDB()
	_, token := seedTestUser(db, "removeerr@test.com", "customer", nil)

	// Drop the cart_items table to force a DB error
	db.Exec("DROP TABLE cart_items")

	router := setupCartRouter(db)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", "/api/cart/"+uuid.New().String(), nil, token))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Failed to remove item from cart" {
		t.Errorf("expected 'Failed to remove item from cart', got %v", resp["error"])
	}

	// Recreate the table for subsequent tests
	db.Exec(`CREATE TABLE IF NOT EXISTS "cart_items" (
		"id" TEXT PRIMARY KEY,
		"user_id" TEXT NOT NULL,
		"product_id" TEXT NOT NULL,
		"franchise_id" TEXT,
		"quantity" INTEGER DEFAULT 1,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME,
		CONSTRAINT fk_cart_items_user FOREIGN KEY ("user_id") REFERENCES "users"("id"),
		CONSTRAINT fk_cart_items_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
	)`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_user_product ON "cart_items"("user_id","product_id")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cart_items_deleted_at ON "cart_items"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cart_items_franchise_id ON "cart_items"("franchise_id")`)
}

// TestClearCartDBError tests the error branch when the DB delete fails.
func TestClearCartDBError(t *testing.T) {
	db := freshDB()
	_, token := seedTestUser(db, "clearerr@test.com", "customer", nil)

	// Drop the cart_items table to force a DB error
	db.Exec("DROP TABLE cart_items")

	router := setupCartRouter(db)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("DELETE", "/api/cart", nil, token))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Failed to clear cart" {
		t.Errorf("expected 'Failed to clear cart', got %v", resp["error"])
	}

	// Recreate the table for subsequent tests
	db.Exec(`CREATE TABLE IF NOT EXISTS "cart_items" (
		"id" TEXT PRIMARY KEY,
		"user_id" TEXT NOT NULL,
		"product_id" TEXT NOT NULL,
		"franchise_id" TEXT,
		"quantity" INTEGER DEFAULT 1,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME,
		CONSTRAINT fk_cart_items_user FOREIGN KEY ("user_id") REFERENCES "users"("id"),
		CONSTRAINT fk_cart_items_product FOREIGN KEY ("product_id") REFERENCES "products"("id")
	)`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_user_product ON "cart_items"("user_id","product_id")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cart_items_deleted_at ON "cart_items"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_cart_items_franchise_id ON "cart_items"("franchise_id")`)
}
