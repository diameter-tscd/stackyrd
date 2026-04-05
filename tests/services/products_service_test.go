package services

import (
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

func setupProductsTestRouter(service *modules.ProductsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	group := r.Group("/api/v1")
	service.RegisterRoutes(group)
	return r
}

func TestProductsService_Name(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewProductsService(true, l)
	assert.Equal(t, "Products Service", service.Name())
}

func TestProductsService_WireName(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewProductsService(true, l)
	assert.Equal(t, "products", service.WireName())
}

func TestProductsService_Enabled(t *testing.T) {
	l := logger.New(false, nil)

	service := modules.NewProductsService(true, l)
	assert.True(t, service.Enabled())

	disabledService := modules.NewProductsService(false, l)
	assert.False(t, disabledService.Enabled())
}

func TestProductsService_Endpoints(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewProductsService(true, l)
	endpoints := service.Endpoints()

	assert.Contains(t, endpoints, "/products")
}

func TestProductsService_GetProducts(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewProductsService(true, l)
	router := setupProductsTestRouter(service)

	req, _ := http.NewRequest("GET", "/api/v1/products", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestProductsService_DisabledService(t *testing.T) {
	l := logger.New(false, nil)
	service := modules.NewProductsService(false, l)
	assert.False(t, service.Enabled())
}
