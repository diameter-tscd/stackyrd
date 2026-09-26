package response

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

// Trace describes one failed request in a form that can be grepped end to
// end: endpoint + handler struct + correlation-id + classified cause.
// It is embedded under error.details.trace so the envelope stays compatible.
type Trace struct {
	TraceID    string `json:"trace_id"`
	Endpoint   string `json:"endpoint"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Handler    string `json:"handler,omitempty"`
	Service    string `json:"service,omitempty"`
	Kind       string `json:"kind"`
	Code       string `json:"code,omitempty"`
	Constraint string `json:"constraint,omitempty"`
	Table      string `json:"table,omitempty"`
	Stack      string `json:"stack,omitempty"`
}

// traceDebug controls verbosity. When false (production default) stack,
// table and raw SQL messages are redacted; trace_id/kind/code stay.
var traceDebug atomic.Bool

// SetTraceDebug toggles verbose trace output. Call once at boot from
// server.go using cfg.App.Debug.
func SetTraceDebug(v bool) {
	traceDebug.Store(v)
}

// GetCorrelationID returns the request correlation ID, checking middleware
// state first, then request headers. Empty only if nothing set it.
func GetCorrelationID(c echo.Context) string {
	if v, ok := c.Get("X-Request-ID").(string); ok && v != "" {
		return v
	}
	if id := c.Response().Header().Get("X-Request-ID"); id != "" {
		return id
	}
	return getCorrelationID(c)
}

// ensureTraceID returns the correlation ID, generating and propagating one
// when the request_id middleware did not run (tests, direct handler calls).
func ensureTraceID(c echo.Context) string {
	if id := GetCorrelationID(c); id != "" {
		return id
	}
	id := "req-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	c.Response().Header().Set("X-Request-ID", id)
	return id
}

// Classify maps an error to (kind, code, constraint, table) for tracing.
// stdlib + already-required drivers only; string matching is the last resort.
func Classify(err error) (kind, code, constraint, table string) {
	if err == nil {
		return "internal", "", "", ""
	}

	var ve interface{ GetFieldErrors() map[string]string }
	if errors.As(err, &ve) {
		return "validation", "VALIDATION_ERROR", "", ""
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		t := pgErr.TableName
		if !traceDebug.Load() {
			t = ""
		}
		return "sql", pgErr.Code, pgErr.ConstraintName, t
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "db", "NOT_FOUND", "", ""
	}

	var writeEx *mongo.WriteException
	if errors.As(err, &writeEx) {
		code := ""
		if len(writeEx.WriteErrors) > 0 {
			code = strconv.Itoa(writeEx.WriteErrors[0].Code)
		}
		return "mongo", code, "", ""
	}
	var cmdErr *mongo.CommandError
	if errors.As(err, &cmdErr) {
		return "mongo", strconv.Itoa(int(cmdErr.Code)), cmdErr.Name, ""
	}
	var bulkErr *mongo.BulkWriteException
	if errors.As(err, &bulkErr) {
		code := ""
		if len(bulkErr.WriteErrors) > 0 {
			code = strconv.Itoa(bulkErr.WriteErrors[0].Code)
		}
		return "mongo", code, "", ""
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return "http", strconv.Itoa(httpErr.Code), "", ""
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "redis"):
		return "redis", "", "", ""
	case strings.Contains(msg, "kafka"):
		return "kafka", "", "", ""
	case strings.Contains(msg, "unauthorized") || strings.Contains(msg, "unauthenticated"):
		return "auth", "UNAUTHORIZED", "", ""
	case strings.Contains(msg, "forbidden") || strings.Contains(msg, "permission denied"):
		return "auth", "FORBIDDEN", "", ""
	}
	return "internal", "", "", ""
}

// sqlStatus maps common SQLSTATE codes to HTTP status + error code.
func sqlStatus(sqlState string) (int, string) {
	switch sqlState {
	case "23505":
		return http.StatusConflict, "SQL_CONFLICT"
	case "23503":
		return http.StatusBadRequest, "SQL_FOREIGN_KEY"
	case "23502":
		return http.StatusBadRequest, "SQL_NOT_NULL"
	case "23514":
		return http.StatusBadRequest, "SQL_CHECK"
	case "22P02", "22P01", "22001", "22003":
		return http.StatusBadRequest, "SQL_BAD_INPUT"
	case "42P01", "42703":
		return http.StatusInternalServerError, "SQL_SCHEMA"
	case "40001", "40P01":
		return http.StatusConflict, "SQL_RETRYABLE"
	default:
		return http.StatusInternalServerError, "SQL_ERROR"
	}
}

// TraceError renders a traced error response. handler is the "Struct.Method"
// name (e.g. "TasksService.Create"), service the wire name. The trace lands
// in error.details.trace and correlation_id == trace.trace_id.
func TraceError(c echo.Context, status int, err error, handler, service string) error {
	traceID := ensureTraceID(c)
	endpoint := c.Path()
	if endpoint == "" {
		endpoint = c.Request().URL.Path
	}
	kind, code, constraint, table := Classify(err)

	if kind == "sql" {
		if s, ec := sqlStatus(code); s != 0 {
			if status == http.StatusInternalServerError {
				status = s
			}
			if code != "" && (ec != "") {
				code = ec + "(" + code + ")"
			}
		}
	}

	message := "Internal server error"
	if err != nil {
		if status < 500 || traceDebug.Load() {
			if line, _, _ := strings.Cut(err.Error(), "\n"); line != "" {
				message = line
			}
		} else if status < 500 {
			if line, _, _ := strings.Cut(err.Error(), "\n"); line != "" {
				message = line
			}
		}
	}

	errCode := code
	if errCode == "" {
		switch {
		case status == http.StatusBadRequest:
			errCode = "BAD_REQUEST"
		case status == http.StatusNotFound:
			errCode = "NOT_FOUND"
		case status == http.StatusConflict:
			errCode = "CONFLICT"
		default:
			errCode = "INTERNAL_ERROR"
		}
	}

	trace := Trace{
		TraceID:    traceID,
		Endpoint:   endpoint,
		Method:     c.Request().Method,
		Path:       c.Request().URL.Path,
		Handler:    handler,
		Service:    service,
		Kind:       kind,
		Code:       code,
		Constraint: constraint,
		Table:      table,
	}
	details := map[string]any{"trace": trace}
	return errorWithCorrelation(c, status, errCode, message, traceID, details)
}

// TracePanic renders a panic recovered by middleware. Stack is included only
// when trace debug is on; otherwise trace_id is the only link to the logs.
func TracePanic(c echo.Context, rec any, stack []byte, handler, service string) error {
	traceID := ensureTraceID(c)
	endpoint := c.Path()
	if endpoint == "" {
		endpoint = c.Request().URL.Path
	}
	trace := Trace{
		TraceID:  traceID,
		Endpoint: endpoint,
		Method:   c.Request().Method,
		Path:     c.Request().URL.Path,
		Handler:  handler,
		Service:  service,
		Kind:     "panic",
		Code:     fmt.Sprintf("%v", rec),
	}
	message := "Internal server error"
	if traceDebug.Load() {
		trace.Stack = string(stack)
		message = fmt.Sprintf("panic: %v", rec)
	}
	details := map[string]any{"trace": trace}
	return errorWithCorrelation(c, http.StatusInternalServerError, "PANIC", message, traceID, details)
}

// errorWithCorrelation is Error() with an explicit correlation ID so traces
// survive when the request_id middleware did not run.
func errorWithCorrelation(c echo.Context, statusCode int, errorCode, message, correlationID string, details map[string]any) error {
	now := time.Now()
	resp := getPooledResponse()
	*resp = Response{}

	var copyDetails map[string]any
	if len(details) > 0 {
		copyDetails = make(map[string]any, len(details))
		for k, v := range details {
			copyDetails[k] = v
		}
	}

	resp.Success = false
	resp.Status = statusCode
	resp.Error = &ErrorDetail{
		Code:    errorCode,
		Message: message,
		Details: copyDetails,
	}
	resp.Timestamp = now.Unix()
	resp.Datetime = now.Format(time.RFC3339)
	resp.CorrelationID = correlationID

	err := c.JSON(statusCode, resp)
	*resp = Response{}
	responsePool.Put(resp)
	return err
}
