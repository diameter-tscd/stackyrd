package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/samber/oops"
)

func init() {
	RegisterMiddleware("ratelimit", func(cfg *config.Config, logger *logger.Logger) (echo.MiddlewareFunc, error) {
		if cfg.Redis.Enabled {
			logger.Info("Rate limit using Redis backend")
			client := redis.NewClient(&redis.Options{
				Addr:     cfg.Redis.Address,
				Password: cfg.Redis.Password,
				DB:       cfg.Redis.DB,
			})
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			pingErr := client.Ping(pingCtx).Err()
			pingCancel()
			if pingErr != nil {
				return nil, oops.In("ratelimit-middleware").Tags("redis", "middleware-init").With("addr", cfg.Redis.Address).Wrapf(pingErr, "redis rate limiter: failed to connect")
			}
			return RedisRateLimitWithConfig(logger, client, 60, time.Minute), nil
		}
		logger.Info("Rate limit using in-memory backend")
		return RateLimit(), nil
	})
}

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
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[ip]
	if exists && now.Sub(v.lastSeen) <= rl.window {
		if v.count >= rl.rate {
			return false
		}
		v.count++
		v.lastSeen = now
		return true
	}

	rl.visitors[ip] = &visitor{count: 1, lastSeen: now}
	return true
}

func RateLimit() echo.MiddlewareFunc {
	return RateLimitWithConfig(60, time.Minute)
}

func RateLimitWithConfig(rate int, window time.Duration) echo.MiddlewareFunc {
	limiter := NewRateLimiter(rate, window)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()

			if !limiter.isAllowed(ip) {
				return response.Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Please try again later.", map[string]any{
					"retry_after": time.Now().Add(window).Unix(),
				})
			}

			return next(c)
		}
	}
}

func RateLimitPerUser(rate int, window time.Duration) echo.MiddlewareFunc {
	limiter := NewRateLimiter(rate, window)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, ok := c.Get("user_id").(string)
			if !ok || userID == "" {
				return next(c)
			}

			if !limiter.isAllowed(userID) {
				return response.Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Please try again later.", map[string]any{
					"retry_after": time.Now().Add(window).Unix(),
				})
			}

			return next(c)
		}
	}
}

type RedisRateLimiter struct {
	client *redis.Client
	rate   int
	window time.Duration
}

func NewRedisRateLimiter(client *redis.Client, rate int, window time.Duration) *RedisRateLimiter {
	return &RedisRateLimiter{
		client: client,
		rate:   rate,
		window: window,
	}
}

// redisSeq disambiguates same-millisecond requests so no two members in the
// rate-limit zset collide (a colliding member would undercount the window).
var redisSeq atomic.Int64

func (rl *RedisRateLimiter) isAllowed(ctx context.Context, key string) (bool, error) {
	now := time.Now().UnixMilli()
	windowStart := now - rl.window.Milliseconds()
	redisKey := "ratelimit:" + key

	pipe := rl.client.Pipeline()

	pipe.ZRemRangeByScore(ctx, redisKey, "0", fmt.Sprintf("%d", windowStart))
	pipe.ZAdd(ctx, redisKey, redis.Z{Score: float64(now), Member: fmt.Sprintf("%d-%d", now, redisSeq.Add(1))})
	pipe.ZCard(ctx, redisKey)
	pipe.Expire(ctx, redisKey, rl.window)

	cmders, err := pipe.Exec(ctx)
	if err != nil {
		return false, oops.In("ratelimit-middleware").Tags("redis").With("key", key).Wrapf(err, "redis rate limiter: pipeline exec failed")
	}

	countCmd, ok := cmders[2].(*redis.IntCmd)
	if !ok {
		return false, oops.In("ratelimit-middleware").Tags("redis").With("key", key).Errorf("redis rate limiter: unexpected pipeline result type")
	}
	return countCmd.Val() <= int64(rl.rate), nil
}

func RedisRateLimitWithConfig(log *logger.Logger, client *redis.Client, rate int, window time.Duration) echo.MiddlewareFunc {
	limiter := NewRedisRateLimiter(client, rate, window)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			allowed, err := limiter.isAllowed(c.Request().Context(), ip)
			if err != nil {
				// Redis outage: fail open and let the request through, but log loudly.
				log.Warn("Rate limiter failed open", "error", err.Error(), "ip", ip)
				return next(c)
			}
			if !allowed {
				return response.Error(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Rate limit exceeded. Please try again later.", map[string]any{
					"retry_after": time.Now().Add(window).Unix(),
				})
			}
			return next(c)
		}
	}
}
