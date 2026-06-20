package modules

import (
	"stackyrd/config"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
)

type ProductsService struct {
	enabled bool
	logger  *logger.Logger
}

type ProductItem struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func NewProductsService(enabled bool, logger *logger.Logger) *ProductsService {
	return &ProductsService{
		enabled: enabled,
		logger:  logger,
	}
}

func (s *ProductsService) Name() string {
	return "Products Service"
}

func (s *ProductsService) WireName() string {
	return "products"
}

func (s *ProductsService) Enabled() bool {
	return s.enabled
}

func (s *ProductsService) Endpoints() []string {
	return []string{
		"/products",
	}
}

func (s *ProductsService) Get() interface{} {
	return s
}

func (s *ProductsService) RegisterRoutes(g *gin.RouterGroup) {
	sub := g.Group("/products")
	{
		sub.GET("", s.getProducts)
	}
}

// Mock database
var products = []ProductItem{
	{ID: 1, Name: "Laptop", Price: 999.99},
	{ID: 2, Name: "Mouse", Price: 29.99},
	{ID: 3, Name: "Keyboard", Price: 79.99},
}

func (s *ProductsService) getProducts(c *gin.Context) {
	response.Success(c, products, "Products retrieved successfully")
}

// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("products_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewProductsService(config.Services.IsEnabled("products_service"), logger)
	})
}
