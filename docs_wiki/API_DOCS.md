# API Documentation

Documentation generated automatically from code annotations using swaggo/swag.

## Quick Start

```bash
# Install swag CLI
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs
swag init -g cmd/app/main.go -o docs
```

## Annotations

Add annotations directly above handler functions:

```go
// GetUsers godoc
// @Summary List users
// @Description Get paginated users
// @Tags users
// @Accept json
// @Produce json
// @Param page query int false "Page" default(1)
// @Success 200 {object} response.Response{data=[]User} "Success"
// @Failure 400 {object} response.Response "Bad request"
// @Router /users [get]
func (s *UsersService) GetUsers(c *gin.Context) error {
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
| `@Success` | Success response | `@Success 200 {object} User "OK"` |
| `@Failure` | Error response | `@Failure 404 {object} response.Response` |
| `@Router` | Path + method | `@Router /users [get]` |

## Struct Documentation

```go
// User represents a user
type User struct {
    ID       string `json:"id" example:"usr_123" description:"User ID"`
    Username string `json:"username" example:"john" description:"Username"`
    Email    string `json:"email" example:"john@test.com" description:"Email"`
}
```

## Generation

```bash
# Basic generation
swag init -g cmd/app/main.go -o docs

# Verify
ls docs/  # docs.go, swagger.json, swagger.yaml
```

## Serving Swagger UI

```go
import (
    "github.com/gin-gonic/gin"
    ginSwagger "github.com/swaggo/gin-swagger"
    "github.com/swaggo/files"
)

func setupSwagger(r *gin.Engine) {
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
```

Access at: `http://localhost:8080/swagger/index.html`