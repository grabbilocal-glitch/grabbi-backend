package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func init() {
	gin.SetMode(gin.TestMode)
	os.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests")
}

func setupTestRouter() *gin.Engine {
	r := gin.New()

	// Protected endpoint for testing AuthMiddleware
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	protected.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		role, _ := c.Get("user_role")
		c.JSON(http.StatusOK, gin.H{
			"user_id": userID,
			"role":    role,
		})
	})

	// Admin endpoint for testing AdminMiddleware
	admin := r.Group("/api/admin")
	admin.Use(AuthMiddleware())
	admin.Use(AdminMiddleware())
	admin.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "admin access granted"})
	})

	// Franchise endpoint for testing FranchiseMiddleware
	franchise := r.Group("/api/franchise")
	franchise.Use(AuthMiddleware())
	franchise.Use(FranchiseMiddleware())
	franchise.GET("/test", func(c *gin.Context) {
		franchiseID, _ := c.Get("franchise_id")
		c.JSON(http.StatusOK, gin.H{"franchise_id": franchiseID})
	})

	// Franchise owner endpoint for testing FranchiseOwnerMiddleware
	franchiseOwner := r.Group("/api/franchise-owner")
	franchiseOwner.Use(AuthMiddleware())
	franchiseOwner.Use(FranchiseOwnerMiddleware())
	franchiseOwner.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "owner access granted"})
	})

	return r
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	router := setupTestRouter()

	userID := uuid.New()
	token, err := utils.GenerateToken(userID, "test@test.com", "customer", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddlewareMalformedToken(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddlewareExpiredToken(t *testing.T) {
	router := setupTestRouter()

	// Create an expired token manually
	secret := os.Getenv("JWT_SECRET")
	claims := utils.Claims{
		UserID: uuid.New(),
		Email:  "expired@test.com",
		Role:   "customer",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "grabbi-backend",
		},
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, _ := tokenObj.SignedString([]byte(secret))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminMiddlewareAllowsAdmin(t *testing.T) {
	router := setupTestRouter()

	token, _ := utils.GenerateToken(uuid.New(), "admin@test.com", "admin", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminMiddlewareBlocksCustomer(t *testing.T) {
	router := setupTestRouter()

	token, _ := utils.GenerateToken(uuid.New(), "customer@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseMiddlewareAllowsFranchiseOwner(t *testing.T) {
	router := setupTestRouter()

	fID := uuid.New()
	token, _ := utils.GenerateToken(uuid.New(), "owner@test.com", "franchise_owner", &fID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseMiddlewareBlocksCustomer(t *testing.T) {
	router := setupTestRouter()

	token, _ := utils.GenerateToken(uuid.New(), "customer@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddlewareInvalidFormatNoBearer(t *testing.T) {
	router := setupTestRouter()

	token, _ := utils.GenerateToken(uuid.New(), "test@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/test", nil)
	// Missing "Bearer " prefix
	req.Header.Set("Authorization", token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseOwnerMiddlewareAllowsOwner(t *testing.T) {
	router := setupTestRouter()

	fID := uuid.New()
	token, _ := utils.GenerateToken(uuid.New(), "owner@test.com", "franchise_owner", &fID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise-owner/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseOwnerMiddlewareBlocksStaff(t *testing.T) {
	router := setupTestRouter()

	fID := uuid.New()
	token, _ := utils.GenerateToken(uuid.New(), "staff@test.com", "franchise_staff", &fID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise-owner/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseOwnerMiddlewareBlocksCustomer(t *testing.T) {
	router := setupTestRouter()

	token, _ := utils.GenerateToken(uuid.New(), "customer@test.com", "customer", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise-owner/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseOwnerMiddlewareBlocksNoRole(t *testing.T) {
	router := setupTestRouter()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise-owner/test", nil)
	// No Authorization header
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseMiddlewareAllowsStaff(t *testing.T) {
	router := setupTestRouter()

	fID := uuid.New()
	token, _ := utils.GenerateToken(uuid.New(), "staff@test.com", "franchise_staff", &fID)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFranchiseMiddlewareBlocksNoFranchiseID(t *testing.T) {
	router := setupTestRouter()

	// franchise_owner role but nil franchise_id
	token, _ := utils.GenerateToken(uuid.New(), "owner-nofid@test.com", "franchise_owner", nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/franchise/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}
