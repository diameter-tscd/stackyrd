# Caching Guide

In-memory generic cache with TTL support and automatic expiration cleanup.

## Overview

The `pkg/cache/` package provides a thread-safe, generic in-memory cache. It supports TTL-based expiration with a background cleanup goroutine. Unlike Redis-backed caches, it requires no external dependencies and is suitable for single-instance applications.

## Basic Usage

```go
import "stackyrd-nano/pkg/cache"

// Create a cache for any type
c := cache.New[User]()

// Don't forget to close when done (stops the cleanup goroutine)
defer c.Close()
```

### Set with TTL

```go
// Set with 5-minute TTL
c.Set("user:123", User{ID: "123", Name: "Alice"}, 5*time.Minute)

// Set with no expiry
c.Set("static:config", configData, 0)
```

### Get

```go
user, ok := c.Get("user:123")
if !ok {
    // cache miss — load from database
}
```

### Delete

```go
c.Delete("user:123")
```

## Full Example

```go
type User struct {
    ID   string
    Name string
}

func getUser(id string) User {
    c := cache.New[User]()
    defer c.Close()

    // Try cache
    if user, ok := c.Get(id); ok {
        return user
    }

    // Miss — load from database
    user := loadFromDB(id)
    c.Set(id, user, 5*time.Minute)
    return user
}
```

## Configuration

The cache requires no external configuration. TTL is specified per call:

| TTL Value | Behavior |
|-----------|----------|
| `> 0` | Item expires after the duration |
| `0` | Item never expires (must be deleted manually) |

## Cleanup

Expired items are removed by a background goroutine that runs every 5 minutes. This can also be triggered manually:

```go
c.Cleanup() // force cleanup of expired items
```

## Best Practices

- **Always set a TTL** for data that changes — avoids stale reads
- **Use type parameters** to get compile-time type safety: `cache.New[User]()`
- **Close the cache** when it's no longer needed to stop the cleanup goroutine
- **Use TTL=0** only for truly static data that lives for the application lifetime
- **Not suitable for distributed systems** — each instance has its own isolated cache
