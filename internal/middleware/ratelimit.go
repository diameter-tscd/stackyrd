package middleware

import (
	"net/http"
	"sync"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
)

func init() {
	// Register RateLimit middleware
	RegisterMiddleware("ratelimit", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		// Default: 60 requests per minute
		return RateLimit(), nil
	})
}

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int
	window   time.Duration
}

type visitor struct {
	count    int
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   window,
	}

	// Cleanup old visitors periodically
	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastSeen) > rl.window {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) isAllowed(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[ip]

	if !exists || now.Sub(v.lastSeen) > rl.window {
		rl.visitors[ip] = &visitor{count: 1, lastSeen: now}
		return true
	}

	if v.count >= rl.rate {
		return false
	}

	v.count++
	v.lastSeen = now
	return true
}

// RateLimit middleware with default settings (60 requests per minute)
func RateLimit() gin.HandlerFunc {
	return RateLimitWithConfig(60, time.Minute)
}

// RateLimitWithConfig middleware with custom settings
func RateLimitWithConfig(rate int, window time.Duration) gin.HandlerFunc {
	limiter := NewRateLimiter(rate, window)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !limiter.isAllowed(ip) {
			response.Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Please try again later.", map[string]interface{}{
				"retry_after": time.Now().Add(window).Unix(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitPerUser middleware based on user ID (requires JWT)
func RateLimitPerUser(rate int, window time.Duration) gin.HandlerFunc {
	limiter := NewRateLimiter(rate, window)

	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		if !limiter.isAllowed(userID.(string)) {
			response.Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Please try again later.", map[string]interface{}{
				"retry_after": time.Now().Add(window).Unix(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
