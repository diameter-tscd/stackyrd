package request

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom validators
	validate.RegisterValidation("phone", validatePhone)
	validate.RegisterValidation("username", validateUsername)
}

// Bind binds and validates request data
func Bind(c *gin.Context, req interface{}) error {
	if err := c.ShouldBind(req); err != nil {
		return fmt.Errorf("invalid request format: %w", err)
	}

	if err := Validate(req); err != nil {
		return err
	}

	return nil
}

// Validate validates a struct using validator tags
func Validate(req interface{}) error {
	if err := validate.Struct(req); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			return &ValidationError{
				Errors: FormatValidationErrors(validationErrors),
			}
		}
		return err
	}
	return nil
}

// ValidationError represents validation errors
type ValidationError struct {
	Errors map[string]string
}

func (e *ValidationError) Error() string {
	var messages []string
	for field, msg := range e.Errors {
		messages = append(messages, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(messages, "; ")
}

// GetFieldErrors returns the validation errors map
func (e *ValidationError) GetFieldErrors() map[string]string {
	return e.Errors
}

// FormatValidationErrors formats validator errors into a readable map
func FormatValidationErrors(errs validator.ValidationErrors) map[string]string {
	errors := make(map[string]string)

	for _, err := range errs {
		field := strings.ToLower(err.Field())

		switch err.Tag() {
		case "required":
			errors[field] = fmt.Sprintf("%s is required", err.Field())
		case "email":
			errors[field] = "Invalid email format"
		case "min":
			errors[field] = fmt.Sprintf("%s must be at least %s characters", err.Field(), err.Param())
		case "max":
			errors[field] = fmt.Sprintf("%s must not exceed %s characters", err.Field(), err.Param())
		case "len":
			errors[field] = fmt.Sprintf("%s must be exactly %s characters", err.Field(), err.Param())
		case "gte":
			errors[field] = fmt.Sprintf("%s must be greater than or equal to %s", err.Field(), err.Param())
		case "lte":
			errors[field] = fmt.Sprintf("%s must be less than or equal to %s", err.Field(), err.Param())
		case "phone":
			errors[field] = "Invalid phone number format"
		case "username":
			errors[field] = "Username must be alphanumeric and 3-20 characters"
		case "oneof":
			errors[field] = fmt.Sprintf("%s must be one of: %s", err.Field(), err.Param())
		default:
			errors[field] = fmt.Sprintf("%s failed validation: %s", err.Field(), err.Tag())
		}
	}

	return errors
}

// Custom Validators

// validatePhone validates phone number format
func validatePhone(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	// Simple validation - starts with + or digit, contains only digits, spaces, dashes, parentheses
	matched, _ := regexp.MatchString(`^[\+]?[(]?[0-9]{1,4}[)]?[-\s\.]?[(]?[0-9]{1,4}[)]?[-\s\.]?[0-9]{1,9}$`, phone)
	return matched
}

// validateUsername validates username format (alphanumeric, 3-20 chars)
func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]{3,20}$`, username)
	return matched
}

// Common Request Structs

// IDRequest represents a request with a single ID
type IDRequest struct {
	ID string `uri:"id" validate:"required"`
}

// IDsRequest represents a request with multiple IDs
type IDsRequest struct {
	IDs []string `json:"ids" validate:"required,min=1"`
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query  string            `form:"q" json:"query"`
	Filter map[string]string `form:"filter" json:"filter,omitempty"`
	Page   int               `form:"page" json:"page"`
	Limit  int               `form:"limit" json:"limit"`
}

// GetQuery returns the search query
func (r *SearchRequest) GetQuery() string {
	return strings.TrimSpace(r.Query)
}

// GetPage returns the page number (default: 1)
func (r *SearchRequest) GetPage() int {
	if r.Page < 1 {
		return 1
	}
	return r.Page
}

// GetLimit returns the limit (default: 20, max: 100)
func (r *SearchRequest) GetLimit() int {
	if r.Limit < 1 {
		return 20
	}
	if r.Limit > 100 {
		return 100
	}
	return r.Limit
}

// DateRangeRequest represents a date range filter
type DateRangeRequest struct {
	StartDate string `form:"start_date" json:"start_date"`
	EndDate   string `form:"end_date" json:"end_date"`
}

// Validate validates the date range
func (r *DateRangeRequest) Validate() error {
	if r.StartDate == "" && r.EndDate == "" {
		return nil
	}
	if r.StartDate == "" {
		return errors.New("start_date is required when end_date is provided")
	}
	if r.EndDate == "" {
		return errors.New("end_date is required when start_date is provided")
	}
	// Additional date format validation can be added here
	return nil
}

// SortRequest represents sorting parameters
type SortRequest struct {
	SortBy    string `form:"sort_by" json:"sort_by"`
	SortOrder string `form:"sort_order" json:"sort_order"` // asc or desc
}

// GetSortBy returns the sort field (default: created_at)
func (r *SortRequest) GetSortBy() string {
	if r.SortBy == "" {
		return "created_at"
	}
	return r.SortBy
}

// GetSortOrder returns the sort order (default: desc)
func (r *SortRequest) GetSortOrder() string {
	order := strings.ToLower(r.SortOrder)
	if order == "asc" {
		return "asc"
	}
	return "desc"
}
