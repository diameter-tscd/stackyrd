package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"stackyrd/internal/middleware"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTraceEcho() *echo.Echo {
	e := echo.New()
	return e
}

func decodeTraceResp(t *testing.T, body []byte) response.Response {
	t.Helper()
	var resp response.Response
	require.NoError(t, json.Unmarshal(body, &resp))
	return resp
}

func TestClassify_SQLUniqueViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key", TableName: "users"}
	kind, code, constraint, _ := response.Classify(pgErr)
	assert.Equal(t, "sql", kind)
	assert.Equal(t, "23505", code)
	assert.Equal(t, "users_email_key", constraint)
}

func TestClassify_Validation(t *testing.T) {
	ve := &request.ValidationError{Errors: map[string]string{"name": "Name is required"}}
	kind, code, _, _ := response.Classify(ve)
	assert.Equal(t, "validation", kind)
	assert.Equal(t, "VALIDATION_ERROR", code)
}

func TestTraceError_SQLMapsTo409WithTrace(t *testing.T) {
	response.SetTraceDebug(true)
	defer response.SetTraceDebug(false)

	e := newTraceEcho()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", nil)
	req.Header.Set("X-Request-ID", "req-test-123")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/tasks")

	pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "tasks_title_key", TableName: "tasks"}
	err := response.TraceError(c, 500, pgErr, "TasksService.Create", "tasks-service")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)

	resp := decodeTraceResp(t, rec.Body.Bytes())
	assert.False(t, resp.Success)
	assert.Equal(t, "req-test-123", resp.CorrelationID)
	require.NotNil(t, resp.Error)
	trace, ok := resp.Error.Details["trace"].(map[string]any)
	require.True(t, ok, "expected details.trace map, got %v", resp.Error.Details)
	assert.Equal(t, "req-test-123", trace["trace_id"])
	assert.Equal(t, "sql", trace["kind"])
	assert.Equal(t, "TasksService.Create", trace["handler"])
	assert.Equal(t, "tasks-service", trace["service"])
	assert.Equal(t, "/api/v1/tasks", trace["endpoint"])
}

func TestTraceError_GeneratesIDWhenMissing(t *testing.T) {
	e := newTraceEcho()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/users/:id")

	err := response.TraceError(c, 500, errors.New("boom"), "UsersService.Get", "users")
	assert.NoError(t, err)
	resp := decodeTraceResp(t, rec.Body.Bytes())
	assert.NotEmpty(t, resp.CorrelationID)
	trace := resp.Error.Details["trace"].(map[string]any)
	assert.Equal(t, resp.CorrelationID, trace["trace_id"])
}

func TestRecoveryMiddleware_ConvertsPanicToTracedJSON(t *testing.T) {
	response.SetTraceDebug(false)
	l := logger.New(true, nil)
	e := newTraceEcho()
	e.Use(middleware.Recovery(l))
	e.GET("/panic", func(c echo.Context) error { panic("kaboom") })

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Request-ID", "req-panic-1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	resp := decodeTraceResp(t, rec.Body.Bytes())
	assert.False(t, resp.Success)
	assert.Equal(t, "req-panic-1", resp.CorrelationID)
	trace := resp.Error.Details["trace"].(map[string]any)
	assert.Equal(t, "panic", trace["kind"])
	_, hasStack := trace["stack"]
	assert.False(t, hasStack, "prod mode must not leak stack")
}
