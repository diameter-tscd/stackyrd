package services

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"stackyrd/internal/services/modules"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter(service *modules.UsersService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	group := r.Group("/api/v1")
	service.RegisterRoutes(group)
	return r
}

func TestUsersService_Name(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	assert.Equal(t, "Users Service", service.Name())
}

func TestUsersService_WireName(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	assert.Equal(t, "users", service.WireName())
}

func TestUsersService_Enabled(t *testing.T) {
	l := logger.New(false, nil)

	// Test enabled service
	service := modules.NewUsersService(true, l)
	assert.True(t, service.Enabled())

	// Test disabled service
	disabledService := modules.NewUsersService(false, l)
	assert.False(t, disabledService.Enabled())
}

func TestUsersService_Endpoints(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	endpoints := service.Endpoints()

	assert.Contains(t, endpoints, "/users")
	assert.Contains(t, endpoints, "/users/:id")
}

func TestUsersService_ListUsers(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	req, _ := http.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestUsersService_ListUsersWithPagination(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	req, _ := http.NewRequest("GET", "/api/v1/users?page=1&per_page=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestUsersService_GetUser(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	req, _ := http.NewRequest("GET", "/api/v1/users/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestUsersService_GetUserNotFound(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	req, _ := http.NewRequest("GET", "/api/v1/users/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUsersService_CreateUser(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	user := map[string]interface{}{
		"name":     "Test User",
		"email":    "test@example.com",
		"phone":    "+1234567890",
		"username": "testuser",
		"age":      25,
	}
	body, _ := json.Marshal(user)

	req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestUsersService_CreateUserValidation(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	// Missing required fields
	user := map[string]interface{}{
		"name": "Test User",
	}
	body, _ := json.Marshal(user)

	req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestUsersService_UpdateUser(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	user := map[string]interface{}{
		"name":     "Updated User",
		"email":    "updated@example.com",
		"phone":    "+0987654321",
		"username": "updateduser",
		"age":      30,
	}
	body, _ := json.Marshal(user)

	req, _ := http.NewRequest("PUT", "/api/v1/users/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUsersService_UpdateUserNotFound(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	// Send complete valid data but with non-existent user ID
	user := map[string]interface{}{
		"name":     "Updated User",
		"email":    "updated@example.com",
		"phone":    "+1234567890",
		"username": "updateduser",
		"age":      30,
	}
	body, _ := json.Marshal(user)

	req, _ := http.NewRequest("PUT", "/api/v1/users/999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUsersService_DeleteUserBlocked(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(true, l)
	router := setupTestRouter(service)

	req, _ := http.NewRequest("DELETE", "/api/v1/users/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// DELETE should return 404 because it's not registered
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUsersService_DisabledService(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewUsersService(false, l)
	assert.False(t, service.Enabled())
}
