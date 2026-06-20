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

	"github.com/gin-gonic/gin"
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

// MultiTenantService demonstrates using multiple PostgreSQL connections with GORM
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

func (s *MultiTenantService) RegisterRoutes(g *gin.RouterGroup) {
	sub := g.Group("/orders")

	sub.GET("/:tenant", s.listOrdersByTenant)
	sub.POST("/:tenant", s.createOrder)
	sub.GET("/:tenant/:id", s.getOrderByTenant)
	sub.PUT("/:tenant/:id", s.updateOrder)
	sub.DELETE("/:tenant/:id", s.deleteOrder)
}

// listOrdersByTenant godoc
// @Summary List orders by tenant
// @Description Retrieve all orders from a specific tenant's database
// @Tags orders
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Success 200 {object} response.Response "Orders retrieved from tenant database"
// @Failure 404 {object} response.Response "Tenant database not found"
// @Failure 500 {object} response.Response "Failed to query tenant database"
// @Router /orders/{tenant} [get]
func (s *MultiTenantService) listOrdersByTenant(c *gin.Context) {
	tenant := c.Param("tenant")

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
		return
	}

	var orders []MultiTenantOrder
	result := dbConn.ORM.Where("tenant_id = ?", tenant).Order("created_at DESC").Find(&orders)
	if result.Error != nil {
		response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, result.Error))
		return
	}

	response.Success(c, orders, fmt.Sprintf("Orders retrieved from tenant '%s' database", tenant))
}

// createOrder godoc
// @Summary Create order in tenant database
// @Description Create a new order in a specific tenant's database
// @Tags orders
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param request body MultiTenantOrder true "Order data"
// @Success 201 {object} response.Response "Order created in tenant database"
// @Failure 400 {object} response.Response "Invalid order data"
// @Failure 404 {object} response.Response "Tenant database not found"
// @Failure 500 {object} response.Response "Failed to create order"
// @Router /orders/{tenant} [post]
func (s *MultiTenantService) createOrder(c *gin.Context) {
	tenant := c.Param("tenant")

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
		return
	}

	var order MultiTenantOrder
	if err := request.Bind(c, &order); err != nil {
		response.BadRequest(c, "Invalid order data")
		return
	}

	order.TenantID = tenant
	order.Status = "pending"

	result := dbConn.ORM.Create(&order)
	if result.Error != nil {
		response.InternalServerError(c, fmt.Sprintf("Failed to create order in tenant '%s' database: %v", tenant, result.Error))
		return
	}

	response.Created(c, order, fmt.Sprintf("Order created in tenant '%s' database", tenant))
}

// getOrderByTenant godoc
// @Summary Get order by tenant
// @Description Retrieve a specific order from a tenant's database
// @Tags orders
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Order ID"
// @Success 200 {object} response.Response "Order retrieved from tenant database"
// @Failure 400 {object} response.Response "Invalid order ID"
// @Failure 404 {object} response.Response "Tenant database or order not found"
// @Failure 500 {object} response.Response "Failed to query tenant database"
// @Router /orders/{tenant}/{id} [get]
func (s *MultiTenantService) getOrderByTenant(c *gin.Context) {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
		return
	}

	var order MultiTenantOrder
	result := dbConn.ORM.Where("id = ? AND tenant_id = ?", id, tenant).First(&order)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			response.NotFound(c, fmt.Sprintf("Order not found in tenant '%s' database", tenant))
			return
		}
		response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, result.Error))
		return
	}

	response.Success(c, order, fmt.Sprintf("Order retrieved from tenant '%s' database", tenant))
}

// updateOrder godoc
// @Summary Update order in tenant database
// @Description Update an order in a specific tenant's database
// @Tags orders
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Order ID"
// @Param request body MultiTenantOrder true "Order update data"
// @Success 200 {object} response.Response "Order updated in tenant database"
// @Failure 400 {object} response.Response "Invalid order ID or update data"
// @Failure 404 {object} response.Response "Tenant database or order not found"
// @Failure 500 {object} response.Response "Failed to update order"
// @Router /orders/{tenant}/{id} [put]
func (s *MultiTenantService) updateOrder(c *gin.Context) {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
		return
	}

	var updateData MultiTenantOrder
	if err := request.Bind(c, &updateData); err != nil {
		response.BadRequest(c, "Invalid update data")
		return
	}

	var order MultiTenantOrder
	result := dbConn.ORM.Where("id = ? AND tenant_id = ?", id, tenant).First(&order)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			response.NotFound(c, fmt.Sprintf("Order not found in tenant '%s' database", tenant))
			return
		}
		response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, result.Error))
		return
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
		response.BadRequest(c, "No fields to update")
		return
	}

	result = dbConn.ORM.Model(&order).Updates(updates)
	if result.Error != nil {
		response.InternalServerError(c, fmt.Sprintf("Failed to update order in tenant '%s' database: %v", tenant, result.Error))
		return
	}

	response.Success(c, nil, fmt.Sprintf("Order updated in tenant '%s' database", tenant))
}

// deleteOrder godoc
// @Summary Delete order from tenant database
// @Description Delete an order from a specific tenant's database
// @Tags orders
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Order ID"
// @Success 200 {object} response.Response "Order deleted from tenant database"
// @Failure 400 {object} response.Response "Invalid order ID"
// @Failure 404 {object} response.Response "Tenant database or order not found"
// @Failure 500 {object} response.Response "Failed to delete order"
// @Router /orders/{tenant}/{id} [delete]
func (s *MultiTenantService) deleteOrder(c *gin.Context) {
	tenant := c.Param("tenant")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	dbConn, exists := s.postgresConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
		return
	}

	result := dbConn.ORM.Where("id = ? AND tenant_id = ?", id, tenant).Delete(&MultiTenantOrder{})
	if result.Error != nil {
		response.InternalServerError(c, fmt.Sprintf("Failed to delete order from tenant '%s' database: %v", tenant, result.Error))
		return
	}

	if result.RowsAffected == 0 {
		response.NotFound(c, fmt.Sprintf("Order not found in tenant '%s' database", tenant))
		return
	}

	response.Success(c, nil, fmt.Sprintf("Order deleted from tenant '%s' database", tenant))
}

// Auto-registration function - called when package is imported
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
