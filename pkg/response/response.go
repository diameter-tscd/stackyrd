package response

import (
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type Response struct {
	Success       bool         `json:"success"`
	Status        int          `json:"status"`
	Message       string       `json:"message,omitempty"`
	Data          any  `json:"data,omitempty"`
	Error         *ErrorDetail `json:"error,omitempty"`
	Meta          *Meta        `json:"meta,omitempty"`
	Timestamp     int64        `json:"timestamp"`
	Datetime      string       `json:"datetime"`
	CorrelationID string       `json:"correlation_id,omitempty"`
}

type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type Meta struct {
	Page       int                    `json:"page,omitempty"`
	PerPage    int                    `json:"per_page,omitempty"`
	Total      int64                  `json:"total,omitempty"`
	TotalPages int                    `json:"total_pages,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

type PaginationRequest struct {
	Page    int    `form:"page" json:"page"`
	PerPage int    `form:"per_page" json:"per_page"`
	Sort    string `form:"sort" json:"sort,omitempty"`
	Order   string `form:"order" json:"order,omitempty"`
}

func (p *PaginationRequest) GetPage() int {
	if p.Page < 1 {
		return 1
	}
	return p.Page
}

func (p *PaginationRequest) GetPerPage() int {
	if p.PerPage < 1 {
		return 10
	}
	if p.PerPage > 100 {
		return 100
	}
	return p.PerPage
}

func (p *PaginationRequest) GetOffset() int {
	page := p.GetPage()
	perPage := p.GetPerPage()
	if page > math.MaxInt/perPage {
		page = math.MaxInt / perPage
	}
	return (page - 1) * perPage
}

func (p *PaginationRequest) GetOrder() string {
	if p.Order == "" {
		return "desc"
	}
	return p.Order
}

// sync.Pool for Response structs to reduce allocations
var responsePool = sync.Pool{
	New: func() any { return &Response{} },
}

func getPooledResponse() *Response {
	resp, ok := responsePool.Get().(*Response)
	if !ok || resp == nil {
		return &Response{}
	}
	return resp
}

func Success(c echo.Context, data any, message ...string) error {
	resp := getPooledResponse()
	// Reset to zero values
	*resp = Response{}

	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	now := time.Now()
	resp.Success = true
	resp.Status = http.StatusOK
	resp.Message = msg
	resp.Data = data
	resp.Timestamp = now.Unix()
	resp.Datetime = now.Format(time.RFC3339)
	resp.CorrelationID = getCorrelationID(c)

	err := c.JSON(http.StatusOK, resp)
	responsePool.Put(resp)
	return err
}

func SuccessWithMeta(c echo.Context, data any, meta *Meta, message ...string) error {
	resp := getPooledResponse()
	*resp = Response{}

	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	now := time.Now()
	resp.Success = true
	resp.Status = http.StatusOK
	resp.Message = msg
	resp.Data = data
	resp.Meta = meta
	resp.Timestamp = now.Unix()
	resp.Datetime = now.Format(time.RFC3339)
	resp.CorrelationID = getCorrelationID(c)

	err := c.JSON(http.StatusOK, resp)
	responsePool.Put(resp)
	return err
}

func Created(c echo.Context, data any, message ...string) error {
	resp := getPooledResponse()
	*resp = Response{}

	msg := "Resource created successfully"
	if len(message) > 0 {
		msg = message[0]
	}

	now := time.Now()
	resp.Success = true
	resp.Status = http.StatusCreated
	resp.Message = msg
	resp.Data = data
	resp.Timestamp = now.Unix()
	resp.Datetime = now.Format(time.RFC3339)
	resp.CorrelationID = getCorrelationID(c)

	err := c.JSON(http.StatusCreated, resp)
	responsePool.Put(resp)
	return err
}

func NoContent(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

func BadRequest(c echo.Context, message string, details ...map[string]any) error {
	return Error(c, http.StatusBadRequest, "BAD_REQUEST", message, details...)
}

func Unauthorized(c echo.Context, message ...string) error {
	msg := "Unauthorized access"
	if len(message) > 0 {
		msg = message[0]
	}
	return Error(c, http.StatusUnauthorized, "UNAUTHORIZED", msg)
}

func Forbidden(c echo.Context, message ...string) error {
	msg := "Access forbidden"
	if len(message) > 0 {
		msg = message[0]
	}
	return Error(c, http.StatusForbidden, "FORBIDDEN", msg)
}

func NotFound(c echo.Context, message ...string) error {
	msg := "Resource not found"
	if len(message) > 0 {
		msg = message[0]
	}
	return Error(c, http.StatusNotFound, "NOT_FOUND", msg)
}

func Conflict(c echo.Context, message string, details ...map[string]any) error {
	return Error(c, http.StatusConflict, "CONFLICT", message, details...)
}

func ValidationError(c echo.Context, message string, details map[string]string) error {
	errorDetails := make(map[string]any, len(details))
	for k, v := range details {
		errorDetails[k] = v
	}
	return Error(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", message, errorDetails)
}

func InternalServerError(c echo.Context, message ...string) error {
	msg := "Internal server error"
	if len(message) > 0 {
		msg = message[0]
	}
	return Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", msg)
}

func ServiceUnavailable(c echo.Context, message ...string) error {
	msg := "Service temporarily unavailable"
	if len(message) > 0 {
		msg = message[0]
	}
	return Error(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", msg)
}

func Error(c echo.Context, statusCode int, errorCode string, message string, details ...map[string]any) error {
	var errorDetails map[string]any
	if len(details) > 0 {
		errorDetails = details[0]
	}

	now := time.Now()
	resp := getPooledResponse()
	*resp = Response{}

	errorDetailsCopy := make(map[string]any, len(errorDetails))
	for k, v := range errorDetails {
		errorDetailsCopy[k] = v
	}

	resp.Success = false
	resp.Status = statusCode
	resp.Error = &ErrorDetail{
		Code:    errorCode,
		Message: message,
		Details: errorDetailsCopy,
	}
	resp.Timestamp = now.Unix()
	resp.Datetime = now.Format(time.RFC3339)
	resp.CorrelationID = getCorrelationID(c)

	err := c.JSON(statusCode, resp)
	responsePool.Put(resp)
	return err
}

func getCorrelationID(c echo.Context) string {
	if id := c.Request().Header.Get("X-Request-ID"); id != "" {
		return id
	}
	if id := c.Request().Header.Get("X-Correlation-ID"); id != "" {
		return id
	}
	return ""
}

func CalculateMeta(page, perPage int, total int64, extra ...map[string]any) *Meta {
	if perPage < 1 {
		perPage = 1
	}
	if page < 1 {
		page = 1
	}
	if total > math.MaxInt {
		total = math.MaxInt
	}
	totalPages := int(total) / perPage
	if total%int64(perPage) > 0 {
		totalPages++
	}

	meta := &Meta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}

	if len(extra) > 0 {
		meta.Extra = extra[0]
	}

	return meta
}
