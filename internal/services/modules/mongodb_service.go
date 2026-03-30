package modules

import (
	"context"
	"fmt"
	"stackyard/config"
	"stackyard/pkg/infrastructure"
	"stackyard/pkg/interfaces"
	"stackyard/pkg/logger"
	"stackyard/pkg/registry"
	"stackyard/pkg/response"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
)

// Product represents a product stored in MongoDB
type Product struct {
	Name        string   `json:"name" bson:"name"`
	Description string   `json:"description" bson:"description"`
	Price       float64  `json:"price" bson:"price"`
	Category    string   `json:"category" bson:"category"`
	InStock     bool     `json:"in_stock" bson:"in_stock"`
	Quantity    int      `json:"quantity" bson:"quantity"`
	Tags        []string `json:"tags" bson:"tags"`
}

// ServiceG demonstrates using multiple MongoDB connections with NoSQL operations
// This service shows how to work with different MongoDB databases dynamically
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

func (s *MongoDBService) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/products")

	// Routes with tenant parameter for database selection
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
func (s *MongoDBService) listProductsByTenant(c echo.Context) error {
	tenant := c.Param("tenant")

	// Get the database connection for this tenant
	dbConn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Use async MongoDB operation to avoid blocking main thread
	cursorResult := dbConn.FindAsync(context.Background(), "products", bson.M{})

	// Wait for the async operation to complete
	cursor, err := cursorResult.Wait()
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, err))
	}
	defer cursor.Close(context.Background())

	var products []bson.M
	if err := cursor.All(context.Background(), &products); err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to decode products: %v", err))
	}

	return response.Success(c, products, fmt.Sprintf("Products retrieved from tenant '%s' database", tenant))
}

// createProduct godoc
// @Summary Create product in tenant database
// @Description Create a new product in a specific tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param request body Product true "Product data"
// @Success 201 {object} response.Response "Product created in tenant database"
// @Failure 400 {object} response.Response "Invalid product data"
// @Failure 404 {object} response.Response "Tenant database not found"
// @Failure 500 {object} response.Response "Failed to create product"
// @Router /products/{tenant} [post]
func (s *MongoDBService) createProduct(c echo.Context) error {
	tenant := c.Param("tenant")

	// Get the database connection for this tenant
	dbConn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	var product Product
	if err := c.Bind(&product); err != nil {
		return response.BadRequest(c, "Invalid product data")
	}

	// Validate required fields
	if product.Name == "" {
		return response.BadRequest(c, "Product name is required")
	}
	if product.Price < 0 {
		return response.BadRequest(c, "Product price cannot be negative")
	}

	// Set default values
	if product.Quantity == 0 {
		product.Quantity = 0
	}
	product.InStock = product.Quantity > 0

	// Insert into tenant's database
	result, err := dbConn.InsertOne(context.Background(), "products", product)
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to create product in tenant '%s' database: %v", tenant, err))
	}

	// Create response with the generated ID
	responseData := bson.M{
		"_id":         result.InsertedID,
		"name":        product.Name,
		"description": product.Description,
		"price":       product.Price,
		"category":    product.Category,
		"in_stock":    product.InStock,
		"quantity":    product.Quantity,
		"tags":        product.Tags,
	}

	return response.Created(c, responseData, fmt.Sprintf("Product created in tenant '%s' database", tenant))
}

// getProductByTenant godoc
// @Summary Get product by tenant
// @Description Retrieve a specific product from a tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Product ID"
// @Success 200 {object} response.Response "Product retrieved from tenant database"
// @Failure 400 {object} response.Response "Invalid product ID format"
// @Failure 404 {object} response.Response "Tenant database or product not found"
// @Failure 500 {object} response.Response "Failed to query tenant database"
// @Router /products/{tenant}/{id} [get]
func (s *MongoDBService) getProductByTenant(c echo.Context) error {
	tenant := c.Param("tenant")
	id := c.Param("id")

	// Get the database connection for this tenant
	dbConn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Find product using ObjectID
	objectID, err := infrastructure.StringToObjectID(id)
	if err != nil {
		return response.BadRequest(c, "Invalid product ID format")
	}

	filter := bson.M{"_id": objectID}
	var product bson.M
	err = dbConn.FindOne(context.Background(), "products", filter).Decode(&product)
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			return response.NotFound(c, fmt.Sprintf("Product not found in tenant '%s' database", tenant))
		}
		return response.InternalServerError(c, fmt.Sprintf("Failed to query tenant '%s' database: %v", tenant, err))
	}

	return response.Success(c, product, fmt.Sprintf("Product retrieved from tenant '%s' database", tenant))
}

// updateProduct godoc
// @Summary Update product in tenant database
// @Description Update a product in a specific tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Product ID"
// @Param request body map[string]interface{} true "Product update data"
// @Success 200 {object} response.Response "Product updated in tenant database"
// @Failure 400 {object} response.Response "Invalid product ID format or update data"
// @Failure 404 {object} response.Response "Tenant database or product not found"
// @Failure 500 {object} response.Response "Failed to update product"
// @Router /products/{tenant}/{id} [put]
func (s *MongoDBService) updateProduct(c echo.Context) error {
	tenant := c.Param("tenant")
	id := c.Param("id")

	// Get the database connection for this tenant
	dbConn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Parse ObjectID
	objectID, err := infrastructure.StringToObjectID(id)
	if err != nil {
		return response.BadRequest(c, "Invalid product ID format")
	}

	// Get update data
	var updateData bson.M
	if err := c.Bind(&updateData); err != nil {
		return response.BadRequest(c, "Invalid update data")
	}

	// Remove _id from update data if present
	delete(updateData, "_id")

	// Update in_stock based on quantity if quantity is being updated
	if quantity, ok := updateData["quantity"].(float64); ok {
		updateData["in_stock"] = quantity > 0
	}

	// Update product
	filter := bson.M{"_id": objectID}
	update := bson.M{"$set": updateData}

	result, err := dbConn.UpdateOne(context.Background(), "products", filter, update)
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to update product in tenant '%s' database: %v", tenant, err))
	}

	if result.MatchedCount == 0 {
		return response.NotFound(c, fmt.Sprintf("Product not found in tenant '%s' database", tenant))
	}

	return response.Success(c, bson.M{"modified_count": result.ModifiedCount}, fmt.Sprintf("Product updated in tenant '%s' database", tenant))
}

// deleteProduct godoc
// @Summary Delete product from tenant database
// @Description Delete a product from a specific tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param id path string true "Product ID"
// @Success 200 {object} response.Response "Product deleted from tenant database"
// @Failure 400 {object} response.Response "Invalid product ID format"
// @Failure 404 {object} response.Response "Tenant database or product not found"
// @Failure 500 {object} response.Response "Failed to delete product"
// @Router /products/{tenant}/{id} [delete]
func (s *MongoDBService) deleteProduct(c echo.Context) error {
	tenant := c.Param("tenant")
	id := c.Param("id")

	// Get the database connection for this tenant
	dbConn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Parse ObjectID
	objectID, err := infrastructure.StringToObjectID(id)
	if err != nil {
		return response.BadRequest(c, "Invalid product ID format")
	}

	// Delete product
	filter := bson.M{"_id": objectID}
	result, err := dbConn.DeleteOne(context.Background(), "products", filter)
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to delete product from tenant '%s' database: %v", tenant, err))
	}

	if result.DeletedCount == 0 {
		return response.NotFound(c, fmt.Sprintf("Product not found in tenant '%s' database", tenant))
	}

	return response.Success(c, bson.M{"deleted_count": result.DeletedCount}, fmt.Sprintf("Product deleted from tenant '%s' database", tenant))
}

// searchProducts godoc
// @Summary Search products in tenant database
// @Description Search products with various filters in a tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Param name query string false "Search by product name"
// @Param category query string false "Filter by category"
// @Param in_stock query boolean false "Filter by stock availability"
// @Param min_price query number false "Minimum price filter"
// @Param max_price query number false "Maximum price filter"
// @Param tags query string false "Filter by tags (comma-separated)"
// @Success 200 {object} response.Response "Products found"
// @Failure 404 {object} response.Response "Tenant database not found"
// @Failure 500 {object} response.Response "Failed to search products"
// @Router /products/{tenant}/search [get]
func (s *MongoDBService) searchProducts(c echo.Context) error {
	tenant := c.Param("tenant")

	// Get the database connection for this tenant
	dbConn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Build search filter from query parameters
	filter := bson.M{}

	if name := c.QueryParam("name"); name != "" {
		filter["name"] = bson.M{"$regex": name, "$options": "i"}
	}

	if category := c.QueryParam("category"); category != "" {
		filter["category"] = category
	}

	if inStock := c.QueryParam("in_stock"); inStock != "" {
		if inStock == "true" {
			filter["in_stock"] = true
		} else if inStock == "false" {
			filter["in_stock"] = false
		}
	}

	if minPrice := c.QueryParam("min_price"); minPrice != "" {
		if minPriceFloat := infrastructure.StringToFloat(minPrice); minPriceFloat >= 0 {
			if priceFilter, exists := filter["price"]; exists {
				if priceMap, ok := priceFilter.(bson.M); ok {
					priceMap["$gte"] = minPriceFloat
				}
			} else {
				filter["price"] = bson.M{"$gte": minPriceFloat}
			}
		}
	}

	if maxPrice := c.QueryParam("max_price"); maxPrice != "" {
		if maxPriceFloat := infrastructure.StringToFloat(maxPrice); maxPriceFloat > 0 {
			if priceFilter, exists := filter["price"]; exists {
				if priceMap, ok := priceFilter.(bson.M); ok {
					priceMap["$lte"] = maxPriceFloat
				}
			} else {
				filter["price"] = bson.M{"$lte": maxPriceFloat}
			}
		}
	}

	if tags := c.QueryParam("tags"); tags != "" {
		filter["tags"] = bson.M{"$in": infrastructure.StringToStringSlice(tags)}
	}

	// Execute search
	cursor, err := dbConn.Find(context.Background(), "products", filter)
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to search products in tenant '%s' database: %v", tenant, err))
	}
	defer cursor.Close(context.Background())

	var products []bson.M
	if err := cursor.All(context.Background(), &products); err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to decode search results: %v", err))
	}

	return response.Success(c, products, fmt.Sprintf("Found %d products in tenant '%s' database", len(products), tenant))
}

// getProductAnalytics godoc
// @Summary Get product analytics
// @Description Get analytics for products in a tenant's database
// @Tags products
// @Accept json
// @Produce json
// @Param tenant path string true "Tenant identifier"
// @Success 200 {object} response.Response "Product analytics for tenant database"
// @Failure 404 {object} response.Response "Tenant database not found"
// @Failure 500 {object} response.Response "Failed to aggregate product analytics"
// @Router /products/{tenant}/analytics [get]
func (s *MongoDBService) getProductAnalytics(c echo.Context) error {
	tenant := c.Param("tenant")

	// Get the database connection for this tenant
	dbConn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found or not connected", tenant))
	}

	// Aggregation pipeline for analytics
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":            "$category",
				"total_products": bson.M{"$sum": 1},
				"avg_price":      bson.M{"$avg": "$price"},
				"min_price":      bson.M{"$min": "$price"},
				"max_price":      bson.M{"$max": "$price"},
				"total_quantity": bson.M{"$sum": "$quantity"},
				"in_stock_count": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{"$in_stock", 1, 0},
					},
				},
			},
		},
		{
			"$sort": bson.M{"total_products": -1},
		},
	}

	cursor, err := dbConn.Aggregate(context.Background(), "products", pipeline)
	if err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to aggregate product analytics for tenant '%s': %v", tenant, err))
	}
	defer cursor.Close(context.Background())

	var analytics []bson.M
	if err := cursor.All(context.Background(), &analytics); err != nil {
		return response.InternalServerError(c, fmt.Sprintf("Failed to decode analytics results: %v", err))
	}

	// Get overall statistics
	totalProducts, _ := dbConn.CountDocuments(context.Background(), "products", bson.M{})
	inStockProducts, _ := dbConn.CountDocuments(context.Background(), "products", bson.M{"in_stock": true})

	result := bson.M{
		"total_products":     totalProducts,
		"in_stock_products":  inStockProducts,
		"out_of_stock":       totalProducts - inStockProducts,
		"category_breakdown": analytics,
	}

	return response.Success(c, result, fmt.Sprintf("Product analytics for tenant '%s' database", tenant))
}

// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("mongodb_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		helper := registry.NewServiceHelper(config, logger, deps)

		if !helper.IsServiceEnabled("mongodb_service") {
			return nil
		}

		mongoConnectionManager, ok := helper.GetMongoConnection()
		if !helper.RequireDependency("MongoConnectionManager", ok) {
			return nil
		}

		return NewMongoDBService(mongoConnectionManager, true, logger)
	})
}
