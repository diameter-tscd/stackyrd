# API Documentation

Documentation generated automatically from code annotations using swaggo/swag via the project's own script.

## Quick Start

```bash
# Generate docs (auto-installs swag CLI if missing)
go run scripts/swagger/swagger.go

# Dry-run (analyze only, no generation)
go run scripts/swagger/swagger.go -dry-run
```

The script scans `internal/services/modules/`, analyzes annotations, and generates `docs.go`, `swagger.json`, and `swagger.yaml` into `docs/`.

Swagger UI is served when `swagger.enabled: true` in `config.yaml`:
```yaml
swagger:
  enabled: true
  base_path: "/swagger"
```

Access at: `http://localhost:8080/swagger/index.html`

## Annotations

Add annotations directly above handler functions:

```go
// @Summary List users
// @Description Get paginated list of users
// @Tags users
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} response.Response{data=[]User} "Success"
// @Failure 400 {object} response.Response "Bad request"
// @Router /users [get]
func (s *UsersService) ListUsers(c *gin.Context) {
    // handler
}
```

## Common Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `@Summary` | Brief title | `@Summary List users` |
| `@Description` | Detailed explanation | `@Description Get users with pagination` |
| `@Tags` | Group endpoints | `@Tags users` |
| `@Param` | Request params | `@Param page query int false "Page"` |
| `@Success` | Success response | `@Success 200 {object} response.Response` |
| `@Failure` | Error response | `@Failure 404 {object} response.Response` |
| `@Router` | Path + method | `@Router /users [get]` |

## Global Annotations

Defined in `cmd/app/main.go`:

```go
// @title stackyrd API
// @version 1.0
// @description stackyrd API Documentation - A modular Go API framework
// @host localhost:8080
// @BasePath /api/v1
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
```

## API Response Format

All responses follow the `response.Response` structure:

```json
{
  "success": true,
  "status": 200,
  "message": "Operation completed",
  "data": {},
  "error": null,
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  },
  "timestamp": 1748963400,
  "datetime": "2026-06-03T15:10:00+07:00",
  "correlation_id": "req-1748963400123456789"
}
```

Error response:

```json
{
  "success": false,
  "status": 400,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input",
    "details": {
      "field": "email",
      "reason": "required"
    }
  },
  "timestamp": 1748963400,
  "datetime": "2026-06-03T15:10:00+07:00",
  "correlation_id": "req-1748963400123456789"
}
```

## Response Helpers

| Function | HTTP Status | Usage |
|----------|-------------|-------|
| `response.Success(c, data)` | 200 | Standard success |
| `response.Success(c, data, "message")` | 200 | Success with message |
| `response.SuccessWithMeta(c, data, meta)` | 200 | Paginated response |
| `response.Created(c, data)` | 201 | Resource created |
| `response.NoContent(c)` | 204 | No content (delete) |
| `response.BadRequest(c, "msg")` | 400 | Bad request |
| `response.Unauthorized(c)` | 401 | Unauthorized |
| `response.Forbidden(c)` | 403 | Forbidden |
| `response.NotFound(c)` | 404 | Not found |
| `response.Conflict(c, "msg")` | 409 | Conflict |
| `response.ValidationError(c, "msg", details)` | 422 | Validation failure |
| `response.InternalServerError(c)` | 500 | Internal error |
| `response.ServiceUnavailable(c)` | 503 | Service unavailable |
| `response.Error(c, code, errCode, msg)` | custom | Custom error |

## Struct Documentation

```go
// User represents a user in the system
type User struct {
    ID       string `json:"id" example:"usr_123" description:"User ID"`
    Username string `json:"username" example:"john" description:"Username"`
    Email    string `json:"email" example:"john@test.com" description:"Email"`
}
```

## Generation

```bash
# Using project script (recommended)
go run scripts/swagger/swagger.go

# Direct swag CLI (alternative)
swag init -g cmd/app/main.go -o docs --outputTypes go,json,yaml

# Verify
ls docs/  # docs.go, swagger.json, swagger.yaml
```
