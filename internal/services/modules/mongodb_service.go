package modules

import (
	"context"
	"fmt"

	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Product represents a product stored in MongoDB
type Product struct {
	ID          primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string             `json:"name" bson:"name"`
	Description string             `json:"description" bson:"description"`
	Price       float64            `json:"price" bson:"price"`
	Category    string             `json:"category" bson:"category"`
	InStock     bool               `json:"in_stock" bson:"in_stock"`
	Quantity    int                `json:"quantity" bson:"quantity"`
	Tags        []string           `json:"tags" bson:"tags"`
}

// MongoDBService demonstrates using multiple MongoDB connections with NoSQL operations
type MongoDBService struct {
	enabled                bool
	mongoConnectionManager *infrastructure.MongoConnectionManager
	logger                 *logger.Logger
}

func NewMongoDBService(
	mongoConnectionManager *infrastructure.MongoConnectionManager,
	enabled bool,
	logger *logger.Logger,
) *MongoDBService {
	return &MongoDBService{
		enabled:                enabled,
		mongoConnectionManager: mongoConnectionManager,
		logger:                 logger,
	}
}

func (s *MongoDBService) Name() string     { return "MongoDB Service" }
func (s *MongoDBService) WireName() string { return "mongodb-service" }
func (s *MongoDBService) Enabled() bool    { return s.enabled }
func (s *MongoDBService) Endpoints() []string {
	return []string{"/products/{tenant}", "/products/{tenant}/{id}"}
}
func (s *MongoDBService) Get() interface{} { return s }

func (s *MongoDBService) RegisterRoutes(g *gin.RouterGroup) {
	sub := g.Group("/products")

	sub.GET("/:tenant", s.listProductsByTenant)
	sub.POST("/:tenant", s.createProduct)
	sub.GET("/:tenant/:id", s.getProductByTenant)
	sub.PUT("/:tenant/:id", s.updateProduct)
	sub.DELETE("/:tenant/:id", s.deleteProduct)
	sub.GET("/:tenant/search", s.searchProducts)
	sub.GET("/:tenant/analytics", s.getProductAnalytics)
}

// listProductsByTenant godoc
// @Summary List products by tenant
// @Description Retrieve all products from a specific tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Success 200 {object} response.Response "Products retrieved from tenant database"
// @Failure 404 {object} response.Response "Tenant database not found"
// @Failure 500 {object} response.Response "Failed to query tenant database"
// @Router /products/{tenant} [get]
func (s *MongoDBService) listProductsByTenant(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		response.BadRequest(c, "Tenant identifier is required")
		return
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
		return
	}

	ctx := context.Background()
	cursor, err := conn.Find(ctx, "products", bson.M{})
	if err != nil {
		s.logger.Error("Failed to query products", err, "tenant", tenant)
		response.InternalServerError(c, "Failed to query tenant database")
		return
	}
	defer cursor.Close(ctx)

	var products []Product
	if err := cursor.All(ctx, &products); err != nil {
		s.logger.Error("Failed to decode products", err)
		response.InternalServerError(c, "Failed to decode products")
		return
	}

	response.Success(c, products, fmt.Sprintf("Products retrieved from tenant '%s'", tenant))
}

// createProduct godoc
// @Summary Create a product for tenant
// @Description Create a new product in a specific tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param request body Product true "Product data"
// @Success 201 {object} response.Response "Product created successfully"
// @Failure 400 {object} response.Response "Invalid product data"
// @Failure 404 {object} response.Response "Tenant database not found"
// @Router /products/{tenant} [post]
func (s *MongoDBService) createProduct(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		response.BadRequest(c, "Tenant identifier is required")
		return
	}

	var product Product
	if err := request.Bind(c, &product); err != nil {
		response.BadRequest(c, "Invalid product data")
		return
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
		return
	}

	ctx := context.Background()
	result, err := conn.InsertOne(ctx, "products", product)
	if err != nil {
		s.logger.Error("Failed to create product", err, "tenant", tenant)
		response.InternalServerError(c, "Failed to create product")
		return
	}

	response.Created(c, map[string]interface{}{
		"id":      result.InsertedID,
		"tenant":  tenant,
		"product": product,
	}, fmt.Sprintf("Product created in tenant '%s'", tenant))
}

// getProductByTenant godoc
// @Summary Get product by tenant and ID
// @Description Retrieve a specific product from a tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Product ID"
// @Success 200 {object} response.Response "Product retrieved successfully"
// @Failure 400 {object} response.Response "Invalid product ID"
// @Failure 404 {object} response.Response "Product or tenant not found"
// @Router /products/{tenant}/{id} [get]
func (s *MongoDBService) getProductByTenant(c *gin.Context) {
	tenant := c.Param("tenant")
	id := c.Param("id")

	if tenant == "" || id == "" {
		response.BadRequest(c, "Tenant and product ID are required")
		return
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
		return
	}

	ctx := context.Background()
	var product Product
	err = conn.FindOne(ctx, "products", bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		response.NotFound(c, "Product not found")
		return
	}

	response.Success(c, product, "Product retrieved successfully")
}

// updateProduct godoc
// @Summary Update a product for tenant
// @Description Update an existing product in a tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Product ID"
// @Param request body Product true "Updated product data"
// @Success 200 {object} response.Response "Product updated successfully"
// @Failure 400 {object} response.Response "Invalid data"
// @Failure 404 {object} response.Response "Product or tenant not found"
// @Router /products/{tenant}/{id} [put]
func (s *MongoDBService) updateProduct(c *gin.Context) {
	tenant := c.Param("tenant")
	id := c.Param("id")

	if tenant == "" || id == "" {
		response.BadRequest(c, "Tenant and product ID are required")
		return
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	var product Product
	if err := request.Bind(c, &product); err != nil {
		response.BadRequest(c, "Invalid product data")
		return
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
		return
	}

	ctx := context.Background()
	update := bson.M{
		"$set": bson.M{
			"name":        product.Name,
			"description": product.Description,
			"price":       product.Price,
			"category":    product.Category,
			"in_stock":    product.InStock,
			"quantity":    product.Quantity,
			"tags":        product.Tags,
		},
	}

	result, err := conn.UpdateOne(ctx, "products", bson.M{"_id": objectID}, update)
	if err != nil {
		s.logger.Error("Failed to update product", err, "tenant", tenant)
		response.InternalServerError(c, "Failed to update product")
		return
	}

	if result.MatchedCount == 0 {
		response.NotFound(c, "Product not found")
		return
	}

	response.Success(c, nil, "Product updated successfully")
}

// deleteProduct godoc
// @Summary Delete a product for tenant
// @Description Delete a product from a tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Product ID"
// @Success 200 {object} response.Response "Product deleted successfully"
// @Failure 400 {object} response.Response "Invalid product ID"
// @Failure 404 {object} response.Response "Product or tenant not found"
// @Router /products/{tenant}/{id} [delete]
func (s *MongoDBService) deleteProduct(c *gin.Context) {
	tenant := c.Param("tenant")
	id := c.Param("id")

	if tenant == "" || id == "" {
		response.BadRequest(c, "Tenant and product ID are required")
		return
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		response.BadRequest(c, "Invalid product ID format")
		return
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
		return
	}

	ctx := context.Background()
	result, err := conn.DeleteOne(ctx, "products", bson.M{"_id": objectID})
	if err != nil {
		s.logger.Error("Failed to delete product", err, "tenant", tenant)
		response.InternalServerError(c, "Failed to delete product")
		return
	}

	if result.DeletedCount == 0 {
		response.NotFound(c, "Product not found")
		return
	}

	response.Success(c, nil, "Product deleted successfully")
}

// searchProducts godoc
// @Summary Search products for tenant
// @Description Search products in a tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param q query string false "Search query"
// @Success 200 {object} response.Response "Search results"
// @Failure 400 {object} response.Response "Missing tenant"
// @Router /products/{tenant}/search [get]
func (s *MongoDBService) searchProducts(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		response.BadRequest(c, "Tenant identifier is required")
		return
	}

	query := c.Query("q")

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
		return
	}

	ctx := context.Background()
	var filter bson.M
	if query != "" {
		filter = bson.M{
			"$or": []bson.M{
				{"name": bson.M{"$regex": query, "$options": "i"}},
				{"description": bson.M{"$regex": query, "$options": "i"}},
				{"category": bson.M{"$regex": query, "$options": "i"}},
			},
		}
	} else {
		filter = bson.M{}
	}

	cursor, err := conn.Find(ctx, "products", filter)
	if err != nil {
		s.logger.Error("Failed to search products", err)
		response.InternalServerError(c, "Failed to search products")
		return
	}
	defer cursor.Close(ctx)

	var products []Product
	if err := cursor.All(ctx, &products); err != nil {
		s.logger.Error("Failed to decode products", err)
		response.InternalServerError(c, "Failed to decode products")
		return
	}

	response.Success(c, products, fmt.Sprintf("Found %d products", len(products)))
}

// getProductAnalytics godoc
// @Summary Get product analytics for tenant
// @Description Get aggregated product analytics from a tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Success 200 {object} response.Response "Analytics data"
// @Failure 400 {object} response.Response "Missing tenant"
// @Router /products/{tenant}/analytics [get]
func (s *MongoDBService) getProductAnalytics(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		response.BadRequest(c, "Tenant identifier is required")
		return
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
		return
	}

	ctx := context.Background()
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":        "$category",
				"count":      bson.M{"$sum": 1},
				"avgPrice":   bson.M{"$avg": "$price"},
				"totalValue": bson.M{"$sum": bson.M{"$multiply": []interface{}{"$price", "$quantity"}}},
			},
		},
	}

	cursor, err := conn.Aggregate(ctx, "products", pipeline)
	if err != nil {
		s.logger.Error("Failed to get analytics", err)
		response.InternalServerError(c, "Failed to get analytics")
		return
	}
	defer cursor.Close(ctx)

	var analytics []bson.M
	if err := cursor.All(ctx, &analytics); err != nil {
		s.logger.Error("Failed to decode analytics", err)
		response.InternalServerError(c, "Failed to decode analytics")
		return
	}

	response.Success(c, analytics, "Analytics retrieved successfully")
}

// Auto-registration function
func init() {
	registry.RegisterService("mongodb_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		helper := registry.NewServiceHelper(config, logger, deps)

		if !helper.IsServiceEnabled("mongodb_service") {
			return nil
		}

		mongoManager, ok := registry.GetTyped[infrastructure.MongoConnectionManager](deps, "mongo")
		if !helper.RequireDependency("MongoConnectionManager", ok) {
			return nil
		}

		return NewMongoDBService(&mongoManager, true, logger)
	})
}
