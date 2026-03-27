package modules

import (
	"stackyard/config"
	"stackyard/pkg/interfaces"
	"stackyard/pkg/logger"
	"stackyard/pkg/registry"
	"stackyard/pkg/response"

	"github.com/labstack/echo/v4"
)

const (
	SERVICE_NAME = "products-service"
)

type ProductsService struct {
	enabled bool
}

func NewProductsService(enabled bool) *ProductsService {
	return &ProductsService{enabled: enabled}
}

func (s *ProductsService) Name() string        { return "Products Service" }
func (s *ProductsService) WireName() string    { return SERVICE_NAME }
func (s *ProductsService) Enabled() bool       { return s.enabled }
func (s *ProductsService) Endpoints() []string { return []string{"/products"} }
func (s *ProductsService) Get() interface{}    { return s }

func (s *ProductsService) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/products")
	sub.GET("", func(c echo.Context) error {
		return response.Success(c, map[string]string{"message": "Hello from Service B - Products"})
	})
}

// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService(SERVICE_NAME, func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewProductsService(config.Services.IsEnabled(SERVICE_NAME))
	})
}
