package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"grabbi-backend/models"
	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestRegisterSuccess(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{
		"email":    "newuser@test.com",
		"password": "password123",
		"name":     "New User",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected token in response")
	}
	user := resp["user"].(map[string]interface{})
	if user["email"] != "newuser@test.com" {
		t.Errorf("expected email newuser@test.com, got %v", user["email"])
	}
	if user["role"] != "customer" {
		t.Errorf("expected role customer, got %v", user["role"])
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	// Create an existing user
	seedTestUser(db, "existing@test.com", "customer", nil)

	body := map[string]string{
		"email":    "existing@test.com",
		"password": "password123",
		"name":     "Duplicate User",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Email already registered" {
		t.Errorf("expected 'Email already registered', got %v", resp["error"])
	}
}

func TestRegisterValidationMissingEmail(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{
		"password": "password123",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterValidationShortPassword(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{
		"email":    "short@test.com",
		"password": "short",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLoginSuccess(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	// Create user
	seedTestUser(db, "login@test.com", "customer", nil)

	body := map[string]string{
		"email":    "login@test.com",
		"password": "password123",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/login", body))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected token in response")
	}
	user := resp["user"].(map[string]interface{})
	if user["email"] != "login@test.com" {
		t.Errorf("expected email login@test.com, got %v", user["email"])
	}
}

func TestLoginWrongPassword(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	seedTestUser(db, "wrongpwd@test.com", "customer", nil)

	body := map[string]string{
		"email":    "wrongpwd@test.com",
		"password": "wrongpassword",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/login", body))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["error"] != "Invalid credentials" {
		t.Errorf("expected 'Invalid credentials', got %v", resp["error"])
	}
}

func TestLoginNonexistentUser(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{
		"email":    "nonexistent@test.com",
		"password": "password123",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/login", body))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetProfileSuccess(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	user, token := seedTestUser(db, "profile@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/auth/profile", nil, token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["email"] != user.Email {
		t.Errorf("expected email %s, got %v", user.Email, resp["email"])
	}
	if resp["role"] != "customer" {
		t.Errorf("expected role customer, got %v", resp["role"])
	}
}

func TestGetProfileUnauthorized(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/profile", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPasswordIsHashed(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{
		"email":    "hash@test.com",
		"password": "password123",
		"name":     "Hash Test",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var user models.User
	db.Where("email = ?", "hash@test.com").First(&user)

	if user.Password == "password123" {
		t.Error("password was stored in plain text")
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("password123"))
	if err != nil {
		t.Error("stored password is not a valid bcrypt hash of the original password")
	}
}

func TestTokenInRegisterResponse(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{
		"email":    "tokentest@test.com",
		"password": "password123",
		"name":     "Token Test",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	token, ok := resp["token"].(string)
	if !ok || token == "" {
		t.Fatal("expected non-empty token string in response")
	}

	// Verify token is valid
	claims, err := utils.ValidateToken(token)
	if err != nil {
		t.Fatalf("token should be valid, got error: %v", err)
	}
	if claims.Email != "tokentest@test.com" {
		t.Errorf("expected email tokentest@test.com in claims, got %s", claims.Email)
	}
	if claims.Role != "customer" {
		t.Errorf("expected role customer in claims, got %s", claims.Role)
	}
}

func TestLoginWithFranchiseUser(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	// Create owner and franchise
	owner, _ := seedTestUser(db, "owner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "Test Franchise", owner.ID)

	// Update the owner with franchise ID
	db.Model(&owner).Update("franchise_id", franchise.ID)

	// Re-seed with franchise ID set
	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	franchiseUser := models.User{
		ID:          uuid.New(),
		Email:       "franchiseuser@test.com",
		Password:    string(hashed),
		Name:        "Franchise User",
		Role:        "franchise_owner",
		FranchiseID: &franchise.ID,
	}
	db.Create(&franchiseUser)

	body := map[string]string{
		"email":    "franchiseuser@test.com",
		"password": "password123",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/login", body))

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	if resp["franchise"] == nil {
		t.Error("expected franchise details in response for franchise user")
	}
}

func TestLoginValidationMissingFields(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	// Missing email
	body := map[string]string{
		"password": "password123",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/login", body))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing email, got %d: %s", w.Code, w.Body.String())
	}

	// Missing password
	body2 := map[string]string{
		"email": "test@test.com",
	}
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, jsonRequest("POST", "/api/auth/login", body2))
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing password, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestRegisterInvalidEmail(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{
		"email":    "not-an-email",
		"password": "password123",
		"name":     "Bad Email User",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid email, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetProfileWithFranchise(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	// Create franchise owner with franchise
	owner, _ := seedTestUser(db, "fowner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "TestStore", owner.ID)
	fID := franchise.ID
	// Update user's franchise_id
	db.Model(&models.User{}).Where("id = ?", owner.ID).Update("franchise_id", fID)
	// Re-generate token with franchise_id
	token, _ := utils.GenerateToken(owner.ID, owner.Email, owner.Role, &fID)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/auth/profile", nil, token)
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["franchise"] == nil {
		t.Error("expected franchise details in profile response")
	}
}

// TestGetProfileUserNotFoundInDB tests the scenario where the token is valid
// but the user has been deleted from the database.
func TestGetProfileUserNotFoundInDB(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	// Create user and get token, then delete the user from DB
	user, token := seedTestUser(db, "deleted@test.com", "customer", nil)
	// Hard delete the user (bypass soft delete)
	db.Unscoped().Delete(&models.User{}, "id = ?", user.ID)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/auth/profile", nil, token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "User not found" {
		t.Errorf("expected 'User not found', got %v", resp["error"])
	}
}

// TestGetProfileNoUserIDInContext tests the unauthorized branch when user_id
// is not present in context (handler called without auth middleware).
func TestGetProfileNoUserIDInContext(t *testing.T) {
	db := freshDB()
	r := gin.New()
	authHandler := &AuthHandler{DB: db}
	r.GET("/api/auth/profile", authHandler.GetProfile)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/auth/profile", nil))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Unauthorized" {
		t.Errorf("expected 'Unauthorized', got %v", resp["error"])
	}
}

// TestRegisterMissingPassword tests registration with missing password field.
func TestRegisterMissingPassword(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{
		"email": "nopwd@test.com",
		"name":  "No Password",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegisterEmptyBody tests registration with an empty JSON body.
func TestRegisterEmptyBody(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	body := map[string]string{}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetProfileWithFranchiseAndStoreHours verifies that store hours are preloaded
// when getting profile for a franchise user.
func TestGetProfileWithFranchiseAndStoreHours(t *testing.T) {
	db := freshDB()
	router := setupAuthRouter(db)

	// Create franchise owner with franchise and store hours
	owner, _ := seedTestUser(db, "hoursowner@test.com", "franchise_owner", nil)
	franchise := seedFranchise(db, "HoursStore", owner.ID)
	seedStoreHours(db, franchise.ID)
	fID := franchise.ID

	// Update user's franchise_id
	db.Model(&models.User{}).Where("id = ?", owner.ID).Update("franchise_id", fID)
	// Re-generate token with franchise_id
	token, _ := utils.GenerateToken(owner.ID, owner.Email, owner.Role, &fID)

	w := httptest.NewRecorder()
	req := authRequest("GET", "/api/auth/profile", nil, token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	resp := parseResponse(w)
	franchise_resp, ok := resp["franchise"].(map[string]interface{})
	if !ok {
		t.Fatal("expected franchise details in profile response")
	}
	storeHours, ok := franchise_resp["store_hours"].([]interface{})
	if !ok {
		t.Fatal("expected store_hours to be preloaded in franchise")
	}
	if len(storeHours) != 7 {
		t.Errorf("expected 7 store hours (Mon-Sun), got %d", len(storeHours))
	}
}

// TestRegisterDBCreateError tests the error branch when DB user creation fails.
func TestRegisterDBCreateError(t *testing.T) {
	db := freshDB()

	// Drop the users table to force a DB error on create
	db.Exec("DROP TABLE users")

	router := setupAuthRouter(db)
	body := map[string]string{
		"email":    "dberror@test.com",
		"password": "password123",
		"name":     "DB Error User",
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, jsonRequest("POST", "/api/auth/register", body))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseResponse(w)
	if resp["error"] != "Failed to create user" {
		t.Errorf("expected 'Failed to create user', got %v", resp["error"])
	}

	// Recreate the users table for subsequent tests
	db.Exec(`CREATE TABLE IF NOT EXISTS "users" (
		"id" TEXT PRIMARY KEY,
		"email" TEXT NOT NULL UNIQUE,
		"password" TEXT NOT NULL,
		"name" TEXT,
		"role" TEXT DEFAULT 'customer',
		"franchise_id" TEXT,
		"loyalty_points" INTEGER DEFAULT 0,
		"phone" TEXT,
		"is_blocked" INTEGER DEFAULT 0,
		"created_at" DATETIME,
		"updated_at" DATETIME,
		"deleted_at" DATETIME
	)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON "users"("deleted_at")`)
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_franchise_id ON "users"("franchise_id")`)
}
