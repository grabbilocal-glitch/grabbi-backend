package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimitEntry struct {
	tokens    float64
	lastCheck time.Time
}

type RateLimiter struct {
	mu         sync.Mutex
	clients    map[string]*rateLimitEntry
	maxTokens  float64
	refillRate float64 // tokens per second
}

// NewRateLimiter creates a rate limiter.
// maxRequests is the burst size, perDuration is the window over which maxRequests are allowed.
func NewRateLimiter(maxRequests int, perDuration time.Duration) *RateLimiter {
	rl := &RateLimiter{
		clients:    make(map[string]*rateLimitEntry),
		maxTokens:  float64(maxRequests),
		refillRate: float64(maxRequests) / perDuration.Seconds(),
	}

	// Start cleanup goroutine to remove stale entries every 5 minutes
	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, entry := range rl.clients {
			// Remove entries that haven't been seen in 10 minutes
			if now.Sub(entry.lastCheck) > 10*time.Minute {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.clients[clientIP]

	if !exists {
		rl.clients[clientIP] = &rateLimitEntry{
			tokens:    rl.maxTokens - 1,
			lastCheck: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(entry.lastCheck).Seconds()
	entry.tokens += elapsed * rl.refillRate
	if entry.tokens > rl.maxTokens {
		entry.tokens = rl.maxTokens
	}
	entry.lastCheck = now

	if entry.tokens >= 1 {
		entry.tokens--
		return true
	}

	return false
}

// Middleware returns a gin middleware that rate limits requests.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		if !rl.allow(clientIP) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests. Please try again later."})
			c.Abort()
			return
		}
		c.Next()
	}
}
