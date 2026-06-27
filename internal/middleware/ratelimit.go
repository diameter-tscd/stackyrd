package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func init() {
	RegisterMiddleware("ratelimit", func(cfg *config.Config, logger *logger.Logger) (gin.HandlerFunc, error) {
		if cfg.Redis.Enabled {
			logger.Info("Rate limit using Redis backend")
			client := redis.NewClient(&redis.Options{
				Addr:     cfg.Redis.Address,
				Password: cfg.Redis.Password,
				DB:       cfg.Redis.DB,
			})
			if err := client.Ping(context.Background()).Err(); err != nil {
				return nil, fmt.Errorf("redis rate limiter: failed to connect: %w", err)
			}
			return RedisRateLimitWithConfig(client, 60, time.Minute), nil
		}
		logger.Info("Rate limit using in-memory backend")
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

var (
	rateLimiters    []*RateLimiter
	rateLimitersMu  sync.Mutex
	rateCleanupOnce sync.Once
)

func startRateLimitCleanup() {
	rateCleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				rateLimitersMu.Lock()
				for _, rl := range rateLimiters {
					rl.cleanup()
				}
				rateLimitersMu.Unlock()
			}
		}()
	})
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		window:   window,
	}

	rateLimitersMu.Lock()
	rateLimiters = append(rateLimiters, rl)
	rateLimitersMu.Unlock()
	startRateLimitCleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	now := time.Now()

	rl.mu.RLock()
	expired := make([]string, 0, len(rl.visitors)>>4)
	for ip, v := range rl.visitors {
		if now.Sub(v.lastSeen) > rl.window {
			expired = append(expired, ip)
		}
	}
	rl.mu.RUnlock()

	if len(expired) > 0 {
		rl.mu.Lock()
		for _, ip := range expired {
			delete(rl.visitors, ip)
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) isAllowed(ip string) bool {
	now := time.Now()

	rl.mu.RLock()
	v, exists := rl.visitors[ip]
	// Fast path: existing visitor, still within the window → upgrade to write
	// lock to increment count.  We must verify `exists` before ever touching
	// `v` (a nil dereference here causes a hard panic).
	if exists && now.Sub(v.lastSeen) <= rl.window {
		rl.mu.RUnlock()

		rl.mu.Lock()
		defer rl.mu.Unlock()
		if v.count >= rl.rate {
			return false
		}
		v.count++
		v.lastSeen = now
		return true
	}
	// Slow path: new visitor or window expired — create/reset entry under
	// write lock.  v may be nil here (first visit); only dereference `v`
	// after the Lock/Exists re-check below.
	rl.mu.RUnlock()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock — another goroutine may have
	// raced in and created the entry already.
	if v, exists = rl.visitors[ip]; exists && now.Sub(v.lastSeen) <= rl.window {
		v.count++
		v.lastSeen = now
		return true
	}

	rl.visitors[ip] = &visitor{count: 1, lastSeen: now}
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

// RedisRateLimiter implements a sliding window rate limiter using Redis sorted sets
type RedisRateLimiter struct {
	client *redis.Client
	rate   int
	window time.Duration
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter
func NewRedisRateLimiter(client *redis.Client, rate int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		rate:   rate,
		window: window,
	}
}

func (rl *RedisRateLimiter) isAllowed(ctx context.Context, key string) (bool, error) {
	now := time.Now().UnixMilli()
	windowStart := now - rl.window.Milliseconds()
	redisKey := "ratelimit:" + key

	pipe := rl.client.Pipeline()

	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZCard(ctx, redisKey)
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now), Member: now})
	pipe.Expire(ctx, redisKey, rl.window)

	cmders, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("redis rate limiter: %w", err)
	}

	count := cmders[1].(*redis.IntCmd).Val()
	return count < int64(rl.rate), nil
}

// RedisRateLimitWithConfig creates a Redis-backed rate limit middleware
func RedisRateLimitWithConfig(client *redis.Client, rate int, window time.Duration) gin.HandlerFunc {
	limiter := NewRedisRateLimiter(client, rate, window)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		allowed, err := limiter.isAllowed(c.Request.Context(), ip)
		if err != nil {
			c.Next()
			return
		}
		if !allowed {
			response.Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Please try again later.", map[string]interface{}{
				"retry_after": time.Now().Add(window).Unix(),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
