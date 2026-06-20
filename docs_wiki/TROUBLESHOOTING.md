# Troubleshooting Guide

Common issues, debugging techniques, and solutions for stackyrd.

## Application Won't Start

### Port Already in Use

```
Error: listen tcp :8080: bind: address already in use
```

**Solution:** Change port via config or flag:

```bash
go run cmd/app/main.go -port 9090
# or in config.yaml
server:
  port: "9090"
```

**Kill existing process:**

```bash
pgrep -x stackyrd | xargs kill
# or
go run scripts/build/build.go  # includes process kill step
```

### Config File Not Found

```
Error: config file not found: config.yaml
```

**Solution:** Run from project root, or specify config path:

```bash
go run cmd/app/main.go -c /path/to/config.yaml
```

### Missing Dependencies

```
Error: module not found
```

**Solution:**

```bash
go mod tidy
go mod download
```

## Services Not Working

### Service Not Registering

**Symptoms:** Endpoint returns 404, or service not in health checks.

**Check:**
1. Service enabled in `config.yaml`:
   ```yaml
   services:
     your_service: true
   ```
2. `init()` function present and properly registered:
   ```go
   func init() {
       registry.RegisterService("your_service", factory)
   }
   ```
3. No build errors in the service file

### Middleware Not Applying

**Check** `config.yaml`:
```yaml
middleware:
  cors: true
  jwt: true
```

**Check** `init()`:
```go
func init() {
    RegisterMiddleware("cors", corsFactory)
}
```

### Route Conflicts

If two services register the same route pattern, the last one wins. Check `Endpoints()` returns unique paths.

## Database Connection Issues

### PostgreSQL

```
Error: failed to connect to postgres: dial tcp 127.0.0.1:5432: connect: connection refused
```

**Check:**
- PostgreSQL running: `docker ps | grep postgres`
- Connection config matches:
  ```yaml
  postgres:
    connections:
      - name: "primary"
        host: "localhost"
        port: 5432
        user: "postgres"
        dbname: "mydb"
  ```

### Redis

```
Error: redis: dial tcp 127.0.0.1:6379: connect: connection refused
```

**Check:**
- Redis running: `docker ps | grep redis`
- Config enabled:
  ```yaml
  redis:
    enabled: true
    address: "localhost:6379"
  ```

## Plugin Issues

See the [Plugin System Guide](../PLUGIN_GUIDE.md#troubleshooting) for detailed plugin troubleshooting.

### Quick Plugin Checks

```bash
# List all plugins
curl -s http://localhost:8080/api/v1/plugins | jq

# Check a specific plugin
curl -s http://localhost:8080/api/v1/plugins/inspector | jq

# Execute a simple ping
curl -s -X POST http://localhost:8080/api/v1/plugins/inspector/execute \
  -H 'Content-Type: application/json' \
  -d '{"args": {"mode": "ping"}}' | jq
```

## TUI Issues

### TUI Not Displaying

**Check:**
- `app.enable_tui: true` in config.yaml
- Terminal supports ANSI escape codes
- Not running in a non-TTY environment (CI, pipe)

### TUI Rendering Glitches

Press `Ctrl+R` to refresh the TUI display.

## Performance Issues

### High Memory Usage

1. Check `/health/resources` endpoint:
   ```bash
   curl -s http://localhost:8080/health/resources | jq
   ```
2. Enable pprof:
   ```go
   import _ "net/http/pprof"
   ```
3. Check goroutine leaks via `/health/resources`

### Slow Response Times

1. Check Prometheus metrics at `/metrics`
2. Look for slow database queries
3. Check circuit breaker states
4. Verify middleware overhead (disable non-essential middleware)

## Debugging Tips

### Enable Verbose Logging

```bash
go run cmd/app/main.go -verbose
```

### Health Check Endpoints

```bash
# Overall health
curl -s http://localhost:8080/health | jq

# Infrastructure components
curl -s http://localhost:8080/health/infrastructure | jq

# Dependencies
curl -s http://localhost:8080/health/dependencies | jq

# Resources
curl -s http://localhost:8080/health/resources | jq
```

### Check Registered Routes

```bash
curl -s http://localhost:8080/health/dependencies | jq '.routes'
```

### Check Config Values

Log the loaded config at startup via `-verbose`.

## Common Error Codes

| HTTP Status | Meaning | Common Cause |
|-------------|---------|--------------|
| 400 | Bad Request | Invalid input, missing required fields |
| 401 | Unauthorized | Missing/invalid API key or JWT |
| 403 | Forbidden | Permission check blocked (DELETE by default) |
| 404 | Not Found | Route not registered, or resource doesn't exist |
| 409 | Conflict | Resource already exists |
| 422 | Validation Error | Request body failed validation |
| 429 | Rate Limited | Too many requests |
| 500 | Internal Error | Unhandled panic or server error |
| 503 | Service Unavailable | Circuit breaker open, dependency down |

## Getting Help

- Check `/health/infrastructure` for component status
- Enable `-verbose` for detailed startup logs
- Review the Plugin troubleshooting section for plugin-specific issues
