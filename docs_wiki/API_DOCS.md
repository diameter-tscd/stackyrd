# API Documentation

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

## Request Binding

```go
type CreateUserRequest struct {
    Username string `json:"username" validate:"required,min=3,max=20"`
    Email    string `json:"email" validate:"required,email"`
}

func (s *YourService) create(c *gin.Context) {
    var req CreateUserRequest
    if err := request.Bind(c, &req); err != nil {
        if validationErr, ok := err.(*request.ValidationError); ok {
            response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
            return
        }
        response.BadRequest(c, err.Error())
        return
    }
    response.Created(c, req)
}
```
