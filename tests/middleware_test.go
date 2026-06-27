package main_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware_CORSAllowAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.CORSAllowAll()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Origin", "http://example.com")

	mw(c)

	assert.Equal(t, "http://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
}

func TestMiddleware_CORSBlockedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.CORSWithConfig([]string{"http://trusted.com"})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Origin", "http://evil.com")

	mw(c)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestMiddleware_CORSPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.CORSAllowAll()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodOptions, "/", nil)
	c.Request.Header.Set("Origin", "http://example.com")

	mw(c)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestMiddleware_CORSSubdomainMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.CORSWithConfig([]string{"*.example.com"})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Origin", "http://sub.example.com")

	mw(c)

	assert.Equal(t, "http://sub.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestMiddleware_JWTRequiredValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	token, err := middleware.GenerateToken("u1", "testuser", "test@test.com", "admin", secret, time.Hour)
	assert.NoError(t, err)

	mw := middleware.JWTRequired(secret)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)

	mw(c)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_JWTRequiredInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.JWTRequired("secret")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer invalid-token")

	mw(c)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_JWTRequiredMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.JWTRequired("secret")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	mw(c)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddleware_JWTSetsClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "test-secret"
	token, err := middleware.GenerateToken("u1", "testuser", "test@test.com", "admin", secret, time.Hour)
	assert.NoError(t, err)

	mw := middleware.JWTRequired(secret)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)

	mw(c)

	uid, _ := c.Get("user_id")
	uname, _ := c.Get("username")
	role, _ := c.Get("role")
	assert.Equal(t, "u1", uid)
	assert.Equal(t, "testuser", uname)
	assert.Equal(t, "admin", role)
}

func TestMiddleware_RequireRoleAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.RequireRole("admin")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("role", "admin")

	var called bool
	next := func(c *gin.Context) { called = true }
	mw(c)
	next(c)

	assert.True(t, called)
}

func TestMiddleware_RequireRoleForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.RequireRole("admin")

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("role", "user")

	mw(c)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestMiddleware_RateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.RateLimitWithConfig(3, time.Minute)

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.Header.Set("X-Forwarded-For", "10.0.0.1")

		mw(c)
		assert.NotEqual(t, http.StatusTooManyRequests, rec.Code, "iteration %d", i)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("X-Forwarded-For", "10.0.0.1")

	mw(c)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestMiddleware_SecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.Security()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	mw(c)

	assert.Equal(t, "default-src 'self'", rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rec.Header().Get("Referrer-Policy"))
}

func TestMiddleware_SecurityPermissive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.SecurityPermissive()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	mw(c)
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
}

func TestMiddleware_Audit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := logger.New(false, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	handler := middleware.AuditWithConfig(l)
	handler(c)
}

func TestMiddleware_AuditSkipsHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := logger.New(false, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/health", nil)

	handler := middleware.AuditSkipHealthCheck(l)
	handler(c)
}

func TestMiddleware_EncryptionDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Encryption: config.EncryptionConfig{
			Enabled: false,
		},
	}
	l := logger.New(false, nil)
	mw := middleware.EncryptionMiddleware(cfg, l)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	mw(c)
}

func TestMiddleware_GzipEncoding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.GzipMiddleware()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Accept-Encoding", "gzip")

	mw(c)
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

func TestMiddleware_GzipNoEncoding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := middleware.GzipMiddleware()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	mw(c)
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
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	assert.Empty(t, middleware.GetUserID(c))

	c.Set("user_id", "u42")
	assert.Equal(t, "u42", middleware.GetUserID(c))
}

func TestMiddleware_GetUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	assert.Empty(t, middleware.GetUsername(c))

	c.Set("username", "testman")
	assert.Equal(t, "testman", middleware.GetUsername(c))
}

func TestMiddleware_GetUserRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	assert.Empty(t, middleware.GetUserRole(c))

	c.Set("role", "editor")
	assert.Equal(t, "editor", middleware.GetUserRole(c))
}
