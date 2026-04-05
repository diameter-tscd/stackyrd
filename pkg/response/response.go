package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Response represents the standard API response structure
type Response struct {
	Success       bool         `json:"success"`
	Status        int          `json:"status"` // HTTP Status Code
	Message       string       `json:"message,omitempty"`
	Data          interface{}  `json:"data,omitempty"`
	Error         *ErrorDetail `json:"error,omitempty"`
	Meta          *Meta        `json:"meta,omitempty"`
	Timestamp     int64        `json:"timestamp"`      // Unix Timestamp
	Datetime      string       `json:"datetime"`       // ISO8601 Datetime
	CorrelationID string       `json:"correlation_id"` // Request ID for tracking
}

// ErrorDetail represents detailed error information
type ErrorDetail struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Meta represents metadata for the response (pagination, etc.)
type Meta struct {
	Page       int                    `json:"page,omitempty"`
	PerPage    int                    `json:"per_page,omitempty"`
	Total      int64                  `json:"total,omitempty"`
	TotalPages int                    `json:"total_pages,omitempty"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// PaginationRequest represents standard pagination parameters
type PaginationRequest struct {
	Page    int    `form:"page" json:"page"`
	PerPage int    `form:"per_page" json:"per_page"`
	Sort    string `form:"sort" json:"sort,omitempty"`
	Order   string `form:"order" json:"order,omitempty"` // asc, desc
}

// GetPage returns the page number (default: 1)
func (p *PaginationRequest) GetPage() int {
	if p.Page < 1 {
		return 1
	}
	return p.Page
}

// GetPerPage returns the per_page limit (default: 10, max: 100)
func (p *PaginationRequest) GetPerPage() int {
	if p.PerPage < 1 {
		return 10
	}
	if p.PerPage > 100 {
		return 100
	}
	return p.PerPage
}

// GetOffset calculates the offset for database queries
func (p *PaginationRequest) GetOffset() int {
	return (p.GetPage() - 1) * p.GetPerPage()
}

// GetOrder returns the order direction (default: desc)
func (p *PaginationRequest) GetOrder() string {
	if p.Order == "" {
		return "desc"
	}
	return p.Order
}

// Success sends a successful response
func Success(c *gin.Context, data interface{}, message ...string) {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	c.JSON(http.StatusOK, Response{
		Success:       true,
		Status:        http.StatusOK,
		Message:       msg,
		Data:          data,
		Timestamp:     time.Now().Unix(),
		Datetime:      time.Now().Format(time.RFC3339),
		CorrelationID: getCorrelationID(c),
	})
}

// SuccessWithMeta sends a successful response with metadata
func SuccessWithMeta(c *gin.Context, data interface{}, meta *Meta, message ...string) {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	c.JSON(http.StatusOK, Response{
		Success:       true,
		Status:        http.StatusOK,
		Message:       msg,
		Data:          data,
		Meta:          meta,
		Timestamp:     time.Now().Unix(),
		Datetime:      time.Now().Format(time.RFC3339),
		CorrelationID: getCorrelationID(c),
	})
}

// Created sends a 201 Created response
func Created(c *gin.Context, data interface{}, message ...string) {
	msg := "Resource created successfully"
	if len(message) > 0 {
		msg = message[0]
	}

	c.JSON(http.StatusCreated, Response{
		Success:       true,
		Status:        http.StatusCreated,
		Message:       msg,
		Data:          data,
		Timestamp:     time.Now().Unix(),
		Datetime:      time.Now().Format(time.RFC3339),
		CorrelationID: getCorrelationID(c),
	})
}

// NoContent sends a 204 No Content response
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// BadRequest sends a 400 Bad Request error response
func BadRequest(c *gin.Context, message string, details ...map[string]interface{}) {
	Error(c, http.StatusBadRequest, "BAD_REQUEST", message, details...)
}

// Unauthorized sends a 401 Unauthorized error response
func Unauthorized(c *gin.Context, message ...string) {
	msg := "Unauthorized access"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(c, http.StatusUnauthorized, "UNAUTHORIZED", msg)
}

// Forbidden sends a 403 Forbidden error response
func Forbidden(c *gin.Context, message ...string) {
	msg := "Access forbidden"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(c, http.StatusForbidden, "FORBIDDEN", msg)
}

// NotFound sends a 404 Not Found error response
func NotFound(c *gin.Context, message ...string) {
	msg := "Resource not found"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(c, http.StatusNotFound, "NOT_FOUND", msg)
}

// Conflict sends a 409 Conflict error response
func Conflict(c *gin.Context, message string, details ...map[string]interface{}) {
	Error(c, http.StatusConflict, "CONFLICT", message, details...)
}

// ValidationError sends a 422 Unprocessable Entity error response
func ValidationError(c *gin.Context, message string, details map[string]string) {
	// Convert map[string]string to map[string]interface{} for the error details
	errorDetails := make(map[string]interface{})
	for k, v := range details {
		errorDetails[k] = v
	}
	Error(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", message, errorDetails)
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(c *gin.Context, message ...string) {
	msg := "Internal server error"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", msg)
}

// ServiceUnavailable sends a 503 Service Unavailable error response
func ServiceUnavailable(c *gin.Context, message ...string) {
	msg := "Service temporarily unavailable"
	if len(message) > 0 {
		msg = message[0]
	}
	Error(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", msg)
}

// Error sends a generic error response with custom status code
func Error(c *gin.Context, statusCode int, errorCode string, message string, details ...map[string]interface{}) {
	var errorDetails map[string]interface{}
	if len(details) > 0 {
		errorDetails = details[0]
	}

	c.JSON(statusCode, Response{
		Success: false,
		Status:  statusCode,
		Error: &ErrorDetail{
			Code:    errorCode,
			Message: message,
			Details: errorDetails,
		},
		Timestamp:     time.Now().Unix(),
		Datetime:      time.Now().Format(time.RFC3339),
		CorrelationID: getCorrelationID(c),
	})
}

// getCorrelationID extracts or generates the correlation ID
func getCorrelationID(c *gin.Context) string {
	// Try standard request ID
	id := c.GetHeader("X-Request-ID")
	if id == "" {
		id = c.GetHeader("X-Correlation-ID")
	}

	// If still empty, generate a new one
	if id == "" {
		id = uuid.New().String()
	}
	return id
}

// CalculateMeta creates pagination metadata
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
