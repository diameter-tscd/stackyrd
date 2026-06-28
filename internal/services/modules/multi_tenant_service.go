package modules

import (
	"fmt"
	"strconv"

	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type MultiTenantOrder struct {
	gorm.Model
	TenantID    string  `json:"tenant_id" gorm:"not null;index"`
	CustomerID  uint    `json:"customer_id" gorm:"not null"`
	ProductName string  `json:"product_name" gorm:"not null"`
	Quantity    int     `json:"quantity" gorm:"not null;check:quantity > 0"`
	TotalPrice  float64 `json:"total_price" gorm:"not null;type:decimal(10,2)"`
	Status      string  `json:"status" gorm:"not null;default:'pending'"`
}

type MultiTenantService struct {
	enabled                   bool
	postgresConnectionManager *infrastructure.PostgresConnectionManager
	logger                    *logger.Logger
}

func NewMultiTenantService(
	postgresConnectionManager *infrastructure.PostgresConnectionManager,
	enabled bool,
	logger *logger.Logger,
) *MultiTenantService {
	if enabled && postgresConnectionManager != nil {
		allConnections := postgresConnectionManager.GetAllConnections()
		for tenant, db := range allConnections {
			if db.ORM != nil {
				if err := db.ORM.AutoMigrate(&MultiTenantOrder{}); err != nil {
					logger.Error("Error migrating MultiTenantOrder", err, "tenant", tenant)
				}
			}
		}
	}

	return &MultiTenantService{
		enabled:                   enabled,
		postgresConnectionManager: postgresConnectionManager,
		logger:                    logger,
	}
}

func (s *MultiTenantService) Name() string     { return "Multi-Tenant Service" }
func (s *MultiTenantService) WireName() string { return "multitenant-service" }
func (s *MultiTenantService) Enabled() bool    { return s.enabled }
func (s *MultiTenantService) Endpoints() []string {
	return []string{"/orders/{tenant}", "/orders/{tenant}/{id}"}
}
func (s *MultiTenantService) Get() interface{} { return s }

func (s *MultiTenantService) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/orders")

	sub.GET("/:tenant", s.listOrdersByTenant)
	sub.POST("/:tenant", s.createOrder)
	sub.GET("/:tenant/:id", s.getOrderByTenant)
	sub.PUT("/:tenant/:id", s.updateOrder)
	sub.DELETE("/:tenant/:id", s.deleteOrder)
}

func (s *MultiTenantService) listOrdersByTenant(c echo.Context) error {
	tenant := c.Param("tenant")

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	var orders []MultiTenantOrder
	result := dbConn.ORM.Where("tenant_id = ?", tenant).Order("created_at DESC").Find(&orders)
	if result.Error != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, result.Error))
	}

	return response.Success(c, orders, fmt.Sprintf("Orders retrieved from tenant '%s' database", tenant))
}

func (s *MultiTenantService) createOrder(c echo.Context) error {
	tenant := c.Param("tenant")

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	var order MultiTenantOrder
	if err := request.Bind(c, &order); err != nil {
		return response.BadRequest(c, "Invalid order data")
	}

	order.TenantID = tenant
	order.Status = "pending"

	result := dbConn.ORM.Create(&order)
	if result.Error != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to create order in tenant '%s' database: %v", tenant, result.Error))
	}

	return response.Created(c, order, fmt.Sprintf("Order created in tenant '%s' database", tenant))
}

func (s *MultiTenantService) getOrderByTenant(c echo.Context) error {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	var order MultiTenantOrder
	result := dbConn.ORM.Where("id = ? AND tenant_id = ?", id, tenant).First(&order)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return response.NotFound(c, fmt.Sprintf("Order not found in tenant '%s' database", tenant))
		}
		return response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, result.Error))
	}

	return response.Success(c, order, fmt.Sprintf("Order retrieved from tenant '%s' database", tenant))
}

func (s *MultiTenantService) updateOrder(c echo.Context) error {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	var updateData MultiTenantOrder
	if err := request.Bind(c, &updateData); err != nil {
		return response.BadRequest(c, "Invalid update data")
	}

	var order MultiTenantOrder
	result := dbConn.ORM.Where("id = ? AND tenant_id = ?", id, tenant).First(&order)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return response.NotFound(c, fmt.Sprintf("Order not found in tenant '%s' database", tenant))
		}
		return response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, result.Error))
	}

	updates := make(map[string]interface{})
	if updateData.CustomerID != 0 {
		updates["customer_id"] = updateData.CustomerID
	}
	if updateData.ProductName != "" {
		updates["product_name"] = updateData.ProductName
	}
	if updateData.Quantity != 0 {
		updates["quantity"] = updateData.Quantity
	}
	if updateData.TotalPrice != 0 {
		updates["total_price"] = updateData.TotalPrice
	}
	if updateData.Status != "" {
		updates["status"] = updateData.Status
	}

	if len(updates) == 0 {
		return response.BadRequest(c, "No fields to update")
	}

	result = dbConn.ORM.Model(&order).Updates(updates)
	if result.Error != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to update order in tenant '%s' database: %v", tenant, result.Error))
	}

	return response.Success(c, nil, fmt.Sprintf("Order updated in tenant '%s' database", tenant))
}

func (s *MultiTenantService) deleteOrder(c echo.Context) error {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	result := dbConn.ORM.Where("id = ? AND tenant_id = ?", id, tenant).Delete(&MultiTenantOrder{})
	if result.Error != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to delete order from tenant '%s' database: %v", tenant, result.Error))
	}

	if result.RowsAffected == 0 {
		return response.NotFound(c, fmt.Sprintf("Order not found in tenant '%s' database", tenant))
	}

	return response.Success(c, nil, fmt.Sprintf("Order deleted from tenant '%s' database", tenant))
}

func init() {
	registry.RegisterService("multi_tenant_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		helper := registry.NewServiceHelper(config, logger, deps)

		if !helper.IsServiceEnabled("multi_tenant_service") {
			return nil
		}

		postgresConnectionManager, ok := registry.GetTyped[infrastructure.PostgresConnectionManager](deps, "postgres")
		if !helper.RequireDependency("PostgresConnectionManager", ok) {
			return nil
		}

		return NewMultiTenantService(&postgresConnectionManager, true, logger)
	})
}
