package modules

import (
	"fmt"
	"stackyard/config"
	"stackyard/pkg/infrastructure"
	"stackyard/pkg/interfaces"
	"stackyard/pkg/logger"
	"stackyard/pkg/registry"
	"stackyard/pkg/response"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// MultiTenantOrder represents an order that can exist in different databases
type MultiTenantOrder struct {
	gorm.Model
	TenantID    string  `json:"tenant_id" gorm:"not null;index"`
	CustomerID  uint    `json:"customer_id" gorm:"not null"`
	ProductName string  `json:"product_name" gorm:"not null"`
	Quantity    int     `json:"quantity" gorm:"not null;check:quantity > 0"`
	TotalPrice  float64 `json:"total_price" gorm:"not null;type:decimal(10,2)"`
	Status      string  `json:"status" gorm:"not null;default:'pending'"`
}

// ServiceF demonstrates using multiple PostgreSQL connections with GORM
// This service shows how to work with different databases dynamically using ORM
type ServiceF struct {
	enabled                   bool
	postgresConnectionManager *infrastructure.PostgresConnectionManager
	logger                    *logger.Logger
}

func NewServiceF(
	postgresConnectionManager *infrastructure.PostgresConnectionManager,
	enabled bool,
	logger *logger.Logger,
) *ServiceF {
	// Auto-migrate the schema for each connected database
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

	return &ServiceF{
		enabled:                   enabled,
		postgresConnectionManager: postgresConnectionManager,
		logger:                    logger,
	}
}

func (s *ServiceF) Name() string        { return "Service F (Multi-Tenant Orders - GORM)" }
func (s *ServiceF) Enabled() bool       { return s.enabled }
func (s *ServiceF) Endpoints() []string { return []string{"/orders/{tenant}", "/orders/{tenant}/{id}"} }

func (s *ServiceF) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/orders")

	// Routes with tenant parameter for database selection
	sub.GET("/:tenant", s.listOrdersByTenant)
	sub.POST("/:tenant", s.createOrder)
	sub.GET("/:tenant/:id", s.getOrderByTenant)
	sub.PUT("/:tenant/:id", s.updateOrder)
	sub.DELETE("/:tenant/:id", s.deleteOrder)
}

// listOrdersByTenant lists orders from a specific tenant database
func (s *ServiceF) listOrdersByTenant(c echo.Context) error {
	tenant := c.Param("tenant")

	// Get the database connection for this tenant
	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Query orders from the tenant's database using GORM
	var orders []MultiTenantOrder
	result := dbConn.ORM.Where("tenant_id = ?", tenant).Order("created_at DESC").Find(&orders)
	if result.Error != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, result.Error))
	}

	return response.Success(c, orders, fmt.Sprintf("Orders retrieved from tenant '%s' database", tenant))
}

// createOrder creates a new order in the specified tenant database
func (s *ServiceF) createOrder(c echo.Context) error {
	tenant := c.Param("tenant")

	// Get the database connection for this tenant
	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	var order MultiTenantOrder
	if err := c.Bind(&order); err != nil {
		return response.BadRequest(c, "Invalid order data")
	}

	// Set the tenant ID
	order.TenantID = tenant
	order.Status = "pending" // Default status

	// Create in the tenant's database using GORM
	result := dbConn.ORM.Create(&order)
	if result.Error != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to create order in tenant '%s' database: %v", tenant, result.Error))
	}

	return response.Created(c, order, fmt.Sprintf("Order created in tenant '%s' database", tenant))
}

// getOrderByTenant retrieves a specific order from a tenant database
func (s *ServiceF) getOrderByTenant(c echo.Context) error {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	// Get the database connection for this tenant
	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Find order using GORM
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

// updateOrder updates an order in the specified tenant database
func (s *ServiceF) updateOrder(c echo.Context) error {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	// Get the database connection for this tenant
	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	var updateData MultiTenantOrder
	if err := c.Bind(&updateData); err != nil {
		return response.BadRequest(c, "Invalid update data")
	}

	// Find and update the order using GORM
	var order MultiTenantOrder
	result := dbConn.ORM.Where("id = ? AND tenant_id = ?", id, tenant).First(&order)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return response.NotFound(c, fmt.Sprintf("Order not found in tenant '%s' database", tenant))
		}
		return response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, result.Error))
	}

	// Update only non-zero fields
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

// deleteOrder deletes an order from the specified tenant database
func (s *ServiceF) deleteOrder(c echo.Context) error {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return response.BadRequest(c, "Invalid order ID")
	}

	// Get the database connection for this tenant
	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Delete order using GORM
	result := dbConn.ORM.Where("id = ? AND tenant_id = ?", id, tenant).Delete(&MultiTenantOrder{})
	if result.Error != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to delete order from tenant '%s' database: %v", tenant, result.Error))
	}

	if result.RowsAffected == 0 {
		return response.NotFound(c, fmt.Sprintf("Order not found in tenant '%s' database", tenant))
	}

	return response.Success(c, nil, fmt.Sprintf("Order deleted from tenant '%s' database", tenant))
}

// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("service_f", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		if !config.Services.IsEnabled("service_f") {
			return nil
		}
		if deps == nil || deps.PostgresConnectionManager == nil {
			logger.Warn("PostgreSQL connections not available, skipping Service F")
			return nil
		}
		return NewServiceF(deps.PostgresConnectionManager, true, logger)
	})
}
