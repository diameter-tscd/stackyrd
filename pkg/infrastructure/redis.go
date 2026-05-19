package infrastructure

import (
	"context"
	"fmt"
	"stackyrd/config"
	"stackyrd/pkg/logger"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisManager struct {
	Client *redis.Client
	Pool   *WorkerPool // Async worker pool — lazily initialised on first async call
	once   sync.Once

	// statusCache avoids re-running Ping + PoolStats on every /health call.
	statusCache  map[string]interface{}
	statusExpiry time.Time
	statusMu     sync.Mutex
}

// Name returns the display name of the component
func (r *RedisManager) Name() string {
	return "Redis"
}

func NewRedisClient(cfg config.RedisConfig) (*RedisManager, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:            cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize:     25,
		MinIdleConns: 5,
		PoolTimeout:  4 * time.Second,
	})

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisManager{
		Client: client,
		// Pool is nil until the first async call — avoids allocating 10 goroutines
		// for services that only use the sync API.
	}, nil
}

// startPool lazily initialises the worker pool on first async use.
func (r *RedisManager) startPool() {
	r.once.Do(func() {
		pool := NewWorkerPool(10)
		pool.Start()
		r.Pool = pool
	})
}

// Set adds a key-value pair to Redis with a TTL.
func (r *RedisManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.Client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a value by key.
func (r *RedisManager) Get(ctx context.Context, key string) (string, error) {
	return r.Client.Get(ctx, key).Result()
}

// Delete removes a key from Redis.
func (r *RedisManager) Delete(ctx context.Context, key string) error {
	return r.Client.Del(ctx, key).Err()
}

// Replace updates a key only if it exists (XX).
func (r *RedisManager) Replace(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.Client.SetXX(ctx, key, value, ttl).Err()
}

func (r *RedisManager) GetStatus() map[string]interface{} {
	stats := make(map[string]interface{})
	if r == nil || r.Client == nil {
		stats["connected"] = false
		return stats
	}

	// Fast path: return cached result when still within TTL.
	r.statusMu.Lock()
	if time.Now().Before(r.statusExpiry) && r.statusCache != nil {
		cached := r.statusCache
		r.statusMu.Unlock()
		return cached
	}
	r.statusMu.Unlock()

	// Slow path: actually ping the server.
	addr := r.Client.Options().Addr
	db := r.Client.Options().DB

	pong, err := r.Client.Ping(context.Background()).Result()
	stats["connected"] = err == nil
	stats["ping"] = pong
	stats["addr"] = addr
	stats["db"] = db

	pool := r.Client.PoolStats()
	stats["pool_hits"] = pool.Hits
	stats["pool_misses"] = pool.Misses
	stats["pool_timeouts"] = pool.Timeouts
	stats["pool_total_conns"] = pool.TotalConns
	stats["pool_idle_conns"] = pool.IdleConns

	r.statusMu.Lock()
	r.statusCache = stats
	r.statusExpiry = time.Now().Add(2 * time.Second)
	r.statusMu.Unlock()

	return stats
}

// GetInfo retrieves Redis server info.
func (r *RedisManager) GetInfo(ctx context.Context) (string, error) {
	return r.Client.Info(ctx).Result()
}

// ScanKeys returns a list of keys matching the pattern. Limit to 100 for safety.
func (r *RedisManager) ScanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := r.Client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

// GetValue returns the value of a specific key for monitoring.
// It assumes string for simplicity, but could be extended.
func (r *RedisManager) GetValue(ctx context.Context, key string) (string, error) {
	val, err := r.Client.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

// Async Redis Operations

// SetAsync asynchronously sets a key-value pair to Redis with a TTL.
func (r *RedisManager) SetAsync(ctx context.Context, key string, value interface{}, ttl time.Duration) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := r.Set(ctx, key, value, ttl)
		return struct{}{}, err
	})
}

// GetAsync asynchronously retrieves a value by key.
func (r *RedisManager) GetAsync(ctx context.Context, key string) *AsyncResult[string] {
	return ExecuteAsync(ctx, func(ctx context.Context) (string, error) {
		return r.Get(ctx, key)
	})
}

// DeleteAsync asynchronously removes a key from Redis.
func (r *RedisManager) DeleteAsync(ctx context.Context, key string) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := r.Delete(ctx, key)
		return struct{}{}, err
	})
}

// ReplaceAsync asynchronously updates a key only if it exists (XX).
func (r *RedisManager) ReplaceAsync(ctx context.Context, key string, value interface{}, ttl time.Duration) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := r.Replace(ctx, key, value, ttl)
		return struct{}{}, err
	})
}

// GetInfoAsync asynchronously retrieves Redis server info.
func (r *RedisManager) GetInfoAsync(ctx context.Context) *AsyncResult[string] {
	return ExecuteAsync(ctx, func(ctx context.Context) (string, error) {
		return r.GetInfo(ctx)
	})
}

// ScanKeysAsync asynchronously returns a list of keys matching the pattern.
func (r *RedisManager) ScanKeysAsync(ctx context.Context, pattern string) *AsyncResult[[]string] {
	return ExecuteAsync(ctx, func(ctx context.Context) ([]string, error) {
		return r.ScanKeys(ctx, pattern)
	})
}

// GetValueAsync asynchronously returns the value of a specific key.
func (r *RedisManager) GetValueAsync(ctx context.Context, key string) *AsyncResult[string] {
	return ExecuteAsync(ctx, func(ctx context.Context) (string, error) {
		return r.GetValue(ctx, key)
	})
}

// Batch Operations

// SetBatchAsync asynchronously sets multiple key-value pairs.
func (r *RedisManager) SetBatchAsync(ctx context.Context, kvPairs map[string]interface{}, ttl time.Duration) *BatchAsyncResult[struct{}] {
	operations := make([]AsyncOperation[struct{}], 0, len(kvPairs))

	for key, value := range kvPairs {
		key, value := key, value // Capture loop variables
		operations = append(operations, func(ctx context.Context) (struct{}, error) {
			return struct{}{}, r.Set(ctx, key, value, ttl)
		})
	}

	return ExecuteBatchAsync(ctx, operations, 30)
}

// GetBatchAsync asynchronously gets multiple values by keys.
func (r *RedisManager) GetBatchAsync(ctx context.Context, keys []string) *BatchAsyncResult[string] {
	operations := make([]AsyncOperation[string], len(keys))

	for i, key := range keys {
		key := key // Capture loop variable
		operations[i] = func(ctx context.Context) (string, error) {
			return r.Get(ctx, key)
		}
	}

	return ExecuteBatchAsync(ctx, operations, 30)
}

// DeleteBatchAsync asynchronously deletes multiple keys.
func (r *RedisManager) DeleteBatchAsync(ctx context.Context, keys []string) *BatchAsyncResult[struct{}] {
	operations := make([]AsyncOperation[struct{}], len(keys))

	for i, key := range keys {
		key := key // Capture loop variable
		operations[i] = func(ctx context.Context) (struct{}, error) {
			return struct{}{}, r.Delete(ctx, key)
		}
	}

	return ExecuteBatchAsync(ctx, operations, 30)
}

// Worker Pool Operations

// SubmitAsyncJob submits an async job to the worker pool.
func (r *RedisManager) SubmitAsyncJob(job func()) {
	r.startPool()
	if r.Pool != nil {
		r.Pool.Submit(job)
	} else {
		// Fallback to direct execution if pool not available
		go job()
	}
}

// Close closes the Redis manager and its worker pool.
func (r *RedisManager) Close() error {
	if r.Pool != nil {
		r.Pool.Close()
	}
	if r.Client != nil {
		return r.Client.Close()
	}
	return nil
}

func init() {
	RegisterComponent("redis", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.Redis.Enabled {
			return nil, nil
		}
		return NewRedisClient(cfg.Redis)
	})
}
