package examples

import (
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
)

// Example 1: Simple Success Response
func exampleSuccess(c *gin.Context) {
	data := map[string]string{
		"message": "Hello World",
		"version": "1.0.0",
	}
	response.Success(c, data, "Request successful")
}

// Example 2: Paginated Response
func examplePagination(c *gin.Context) {
	var pagination response.PaginationRequest
	if err := c.ShouldBindQuery(&pagination); err != nil {
		response.BadRequest(c, "Invalid pagination parameters")
		return
	}

	// Get data (example)
	items := []map[string]string{
		{"id": "1", "name": "Item 1"},
		{"id": "2", "name": "Item 2"},
		{"id": "3", "name": "Item 3"},
	}

	total := int64(100)
	meta := response.CalculateMeta(pagination.GetPage(), pagination.GetPerPage(), total)

	response.SuccessWithMeta(c, items, meta)
}

// Example 3: Request Validation
type CreateItemRequest struct {
	Name        string `json:"name" validate:"required,min=3,max=100"`
	Description string `json:"description" validate:"max=500"`
	Price       int    `json:"price" validate:"required,gte=0"`
	Category    string `json:"category" validate:"required,oneof=electronics books clothing"`
}

func exampleValidation(c *gin.Context) {
	var req CreateItemRequest

	// Bind and validate
	if err := request.Bind(c, &req); err != nil {
		if validationErr, ok := err.(*request.ValidationError); ok {
			response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	// Process valid request
	item := map[string]interface{}{
		"id":          "123",
		"name":        req.Name,
		"description": req.Description,
		"price":       req.Price,
		"category":    req.Category,
	}

	response.Created(c, item, "Item created successfully")
}

// Example 4: Error Handling
func exampleErrors(c *gin.Context) {
	id := c.Param("id")

	// Not found error
	if id == "999" {
		response.NotFound(c, "Item not found")
		return
	}

	// Unauthorized error
	if c.GetHeader("Authorization") == "" {
		response.Unauthorized(c, "Authentication required")
		return
	}

	// Forbidden error
	if !hasPermission() {
		response.Forbidden(c, "Insufficient permissions")
		return
	}

	// Success
	item := map[string]string{"id": id, "name": "Example Item"}
	response.Success(c, item)
}

func hasPermission() bool {
	return false // Example
}

// Example 5: Search with Filters
func exampleSearch(c *gin.Context) {
	var search request.SearchRequest
	c.ShouldBindQuery(&search)

	// Use helper methods
	query := search.GetQuery()
	page := search.GetPage()
	limit := search.GetLimit()

	results := []map[string]string{
		{"id": "1", "title": "Result 1"},
		{"id": "2", "title": "Result 2"},
	}

	meta := response.CalculateMeta(page, limit, 50, map[string]interface{}{
		"query":        query,
		"filter_count": len(search.Filter),
	})

	response.SuccessWithMeta(c, results, meta, "Search completed")
}

// Example 6: Custom Error with Details
func exampleCustomError(c *gin.Context) {
	details := map[string]interface{}{
		"field":         "email",
		"reason":        "already exists",
		"suggested_fix": "Use a different email or login",
	}

	response.Conflict(c, "Email already registered", details)
}

// Example 7: No Content Response
func exampleDelete(c *gin.Context) {
	id := c.Param("id")

	// Delete item
	_ = id // Simulate deletion

	response.NoContent(c)
}
