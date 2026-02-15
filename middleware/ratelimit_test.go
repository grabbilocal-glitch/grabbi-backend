package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterWithinLimit(t *testing.T) {
	rl := NewRateLimiter(5, 1*time.Minute)
	for i := 0; i < 5; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiterOverLimit(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Minute)
	rl.allow("1.2.3.4") // 1
	rl.allow("1.2.3.4") // 2
	if rl.allow("1.2.3.4") { // 3 - should be blocked
		t.Fatal("should be rate limited")
	}
}

func TestRateLimiterTokenRefill(t *testing.T) {
	// Use a very short duration so tokens refill quickly
	rl := NewRateLimiter(1, 50*time.Millisecond)
	rl.allow("1.2.3.4") // consume token
	if rl.allow("1.2.3.4") { // should fail
		t.Fatal("should be rate limited immediately")
	}
	time.Sleep(60 * time.Millisecond) // wait for refill
	if !rl.allow("1.2.3.4") {
		t.Fatal("token should have refilled")
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	rl := NewRateLimiter(1, 1*time.Minute)
	rl.allow("1.1.1.1")
	if !rl.allow("2.2.2.2") {
		t.Fatal("different IP should have its own bucket")
	}
}

func TestRateLimiterMiddleware429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(1, 1*time.Minute)

	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// First request should pass
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/test", nil))
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	// Second request should be rate limited
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/test", nil))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}
}
