package response

import (
	"net/http"
	"time"

	"fmt"
	"github.com/labstack/echo/v4"
	"sync/atomic"
)

type Response struct {
	Success       bool         `json:"success"`
	Status        int          `json:"status"`
	Message       string       `json:"message,omitempty"`
	Data          interface{}  `json:"data,omitempty"`
	Error         *ErrorDetail `json:"error,omitempty"`
	Meta          *Meta        `json:"meta,omitempty"`
	Timestamp     int64        `json:"timestamp"`
	Datetime      string       `json:"datetime"`
	CorrelationID string       `json:"correlation_id"`
}

type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type Meta struct {
	Page       int                    `json:"page,omitempty"`
	PerPage    int                    `json:"per_page,omitempty"`
	Total      int64                  `json:"total,omitempty"`
	TotalPages int                    `json:"total_pages,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
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
	return (p.GetPage() - 1) * p.GetPerPage()
}

func (p *PaginationRequest) GetOrder() string {
	if p.Order == "" {
		return "desc"
	}
	return p.Order
}

func Success(c echo.Context, data interface{}, message ...string) error {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	now := time.Now()
	return c.JSON(http.StatusOK, Response{
		Success:       true,
		Status:        http.StatusOK,
		Message:       msg,
		Data:          data,
		Timestamp:     now.Unix(),
		Datetime:      time.Unix(now.Unix(), 0).Format(time.RFC3339),
		CorrelationID: getCorrelationID(c),
	})
}

func SuccessWithMeta(c echo.Context, data interface{}, meta *Meta, message ...string) error {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	now := time.Now()
	return c.JSON(http.StatusOK, Response{
		Success:       true,
		Status:        http.StatusOK,
		Message:       msg,
		Data:          data,
		Meta:          meta,
		Timestamp:     now.Unix(),
		Datetime:      time.Unix(now.Unix(), 0).Format(time.RFC3339),
		CorrelationID: getCorrelationID(c),
	})
}

func Created(c echo.Context, data interface{}, message ...string) error {
	msg := "Resource created successfully"
	if len(message) > 0 {
		msg = message[0]
	}

	now := time.Now()
	return c.JSON(http.StatusCreated, Response{
		Success:       true,
		Status:        http.StatusCreated,
		Message:       msg,
		Data:          data,
		Timestamp:     now.Unix(),
		Datetime:      time.Unix(now.Unix(), 0).Format(time.RFC3339),
		CorrelationID: getCorrelationID(c),
	})
}

func NoContent(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

func BadRequest(c echo.Context, message string, details ...map[string]interface{}) error {
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

func Conflict(c echo.Context, message string, details ...map[string]interface{}) error {
	return Error(c, http.StatusConflict, "CONFLICT", message, details...)
}

func ValidationError(c echo.Context, message string, details map[string]string) error {
	errorDetails := make(map[string]interface{})
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

func Error(c echo.Context, statusCode int, errorCode string, message string, details ...map[string]interface{}) error {
	var errorDetails map[string]interface{}
	if len(details) > 0 {
		errorDetails = details[0]
	}

	now := time.Now()
	return c.JSON(statusCode, Response{
		Success: false,
		Status:  statusCode,
		Error: &ErrorDetail{
			Code:    errorCode,
			Message: message,
			Details: errorDetails,
		},
		Timestamp:     now.Unix(),
		Datetime:      time.Unix(now.Unix(), 0).Format(time.RFC3339),
		CorrelationID: getCorrelationID(c),
	})
}

func getCorrelationID(c echo.Context) string {
	id := c.Request().Header.Get("X-Request-ID")
	if id == "" {
		id = c.Request().Header.Get("X-Correlation-ID")
	}

	if id == "" {
		id = genUUID()
	}
	return id
}

var uuidCounter uint64

func genUUID() string {
	hi := atomic.AddUint64(&uuidCounter, 1)
	lo := atomic.AddUint64(&uuidCounter, 1)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(hi>>32), uint16(hi), uint16(lo>>48), uint16(lo>>32), uint32(lo))
}

func CalculateMeta(page, perPage int, total int64, extra ...map[string]interface{}) *Meta {
	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
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
