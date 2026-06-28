package main_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/logger"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func testEchoContext(method, path string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	c := e.NewContext(req, rec)
	return c, rec
}

func TestMiddleware_CORSAllowAll(t *testing.T) {
	mw := middleware.CORSAllowAll()

	c, rec := testEchoContext(http.MethodGet, "/")
	c.Request().Header.Set("Origin", "http://example.com")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)

	assert.Equal(t, "http://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestMiddleware_CORSBlockedOrigin(t *testing.T) {
	mw := middleware.CORSWithConfig([]string{"http://trusted.com"})

	c, rec := testEchoContext(http.MethodGet, "/")
	c.Request().Header.Set("Origin", "http://evil.com")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestMiddleware_CORSPreflight(t *testing.T) {
	mw := middleware.CORSAllowAll()

	c, rec := testEchoContext(http.MethodOptions, "/")
	c.Request().Header.Set("Origin", "http://example.com")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestMiddleware_CORSSubdomainMatch(t *testing.T) {
	mw := middleware.CORSWithConfig([]string{"*.example.com"})

	c, rec := testEchoContext(http.MethodGet, "/")
	c.Request().Header.Set("Origin", "http://sub.example.com")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)

	assert.Equal(t, "http://sub.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestMiddleware_JWTRequiredValid(t *testing.T) {
	secret := "test-secret"
	token, err := middleware.GenerateToken("u1", "testuser", "test@test.com", "admin", secret, time.Hour)
	assert.NoError(t, err)

	mw := middleware.JWTRequired(secret)

	c, _ := testEchoContext(http.MethodGet, "/")
	c.Request().Header.Set("Authorization", "Bearer "+token)

	var called bool
	handler := mw(func(c echo.Context) error { called = true; return nil })
	_ = handler(c)
	assert.True(t, called)
}

func TestMiddleware_JWTRequiredInvalid(t *testing.T) {
	mw := middleware.JWTRequired("secret")

	c, rec := testEchoContext(http.MethodGet, "/")
	c.Request().Header.Set("Authorization", "Bearer invalid-token")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_JWTRequiredMissing(t *testing.T) {
	mw := middleware.JWTRequired("secret")

	c, rec := testEchoContext(http.MethodGet, "/")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_JWTSetsClaims(t *testing.T) {
	secret := "test-secret"
	token, err := middleware.GenerateToken("u1", "testuser", "test@test.com", "admin", secret, time.Hour)
	assert.NoError(t, err)

	mw := middleware.JWTRequired(secret)

	c, _ := testEchoContext(http.MethodGet, "/")
	c.Request().Header.Set("Authorization", "Bearer "+token)

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)

	uid := c.Get("user_id")
	uname := c.Get("username")
	role := c.Get("role")
	assert.Equal(t, "u1", uid)
	assert.Equal(t, "testuser", uname)
	assert.Equal(t, "admin", role)
}

func TestMiddleware_RequireRoleAllowed(t *testing.T) {
	mw := middleware.RequireRole("admin")

	c, _ := testEchoContext(http.MethodGet, "/")
	c.Set("role", "admin")

	var called bool
	handler := mw(func(c echo.Context) error { called = true; return nil })
	_ = handler(c)

	assert.True(t, called)
}

func TestMiddleware_RequireRoleForbidden(t *testing.T) {
	mw := middleware.RequireRole("admin")

	c, rec := testEchoContext(http.MethodGet, "/")
	c.Set("role", "user")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestMiddleware_RateLimit(t *testing.T) {
	mw := middleware.RateLimitWithConfig(3, time.Minute)

	for i := 0; i < 3; i++ {
		c, rec := testEchoContext(http.MethodGet, "/")
		c.Request().Header.Set("X-Forwarded-For", "10.0.0.1")

		handler := mw(func(c echo.Context) error { return nil })
		_ = handler(c)
		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code, "iteration %d", i)
	}

	c, rec := testEchoContext(http.MethodGet, "/")
	c.Request().Header.Set("X-Forwarded-For", "10.0.0.1")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestMiddleware_SecurityHeaders(t *testing.T) {
	mw := middleware.Security()

	c, rec := testEchoContext(http.MethodGet, "/")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)

	assert.Equal(t, "default-src 'self'", rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
}

func TestMiddleware_SecurityPermissive(t *testing.T) {
	mw := middleware.SecurityPermissive()

	c, rec := testEchoContext(http.MethodGet, "/")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

func TestMiddleware_Audit(t *testing.T) {
	l := logger.New(false, nil)

	c, _ := testEchoContext(http.MethodGet, "/test")

	handler := middleware.AuditWithConfig(l)(func(c echo.Context) error { return nil })
	_ = handler(c)
}

func TestMiddleware_AuditSkipsHealth(t *testing.T) {
	l := logger.New(false, nil)

	c, _ := testEchoContext(http.MethodGet, "/health")

	handler := middleware.AuditSkipHealthCheck(l)(func(c echo.Context) error { return nil })
	_ = handler(c)
}

func TestMiddleware_EncryptionDisabled(t *testing.T) {
	cfg := &config.Config{
		Encryption: config.EncryptionConfig{
			Enabled: false,
		},
	}
	l := logger.New(false, nil)
	mw := middleware.EncryptionMiddleware(cfg, l)

	c, _ := testEchoContext(http.MethodGet, "/")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)
}

func TestMiddleware_GzipEncoding(t *testing.T) {
	mw := middleware.GzipMiddleware()

	c, rec := testEchoContext(http.MethodGet, "/")
	c.Request().Header.Set("Accept-Encoding", "gzip")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

func TestMiddleware_GzipNoEncoding(t *testing.T) {
	mw := middleware.GzipMiddleware()

	c, rec := testEchoContext(http.MethodGet, "/")

	handler := mw(func(c echo.Context) error { return nil })
	_ = handler(c)
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
}

func TestMiddleware_GenerateToken(t *testing.T) {
	token, err := middleware.GenerateToken("u1", "user", "u@t.com", "admin", "secret", time.Hour)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := jwt.ParseWithClaims(token, &middleware.JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	assert.NoError(t, err)
	assert.True(t, parsed.Valid)

	claims, ok := parsed.Claims.(*middleware.JWTClaims)
	assert.True(t, ok)
	assert.Equal(t, "u1", claims.UserID)
	assert.Equal(t, "admin", claims.Role)
}

func TestMiddleware_GetUserID(t *testing.T) {
	c, _ := testEchoContext(http.MethodGet, "/")

	assert.Empty(t, middleware.GetUserID(c))

	c.Set("user_id", "u42")
	assert.Equal(t, "u42", middleware.GetUserID(c))
}

func TestMiddleware_GetUsername(t *testing.T) {
	c, _ := testEchoContext(http.MethodGet, "/")

	assert.Empty(t, middleware.GetUsername(c))

	c.Set("username", "testman")
	assert.Equal(t, "testman", middleware.GetUsername(c))
}

func TestMiddleware_GetUserRole(t *testing.T) {
	c, _ := testEchoContext(http.MethodGet, "/")

	assert.Empty(t, middleware.GetUserRole(c))

	c.Set("role", "editor")
	assert.Equal(t, "editor", middleware.GetUserRole(c))
}
