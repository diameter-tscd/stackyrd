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
type ServiceG struct {
	enabled                bool
	mongoConnectionManager *infrastructure.MongoConnectionManager
	logger                 *logger.Logger
}

func NewServiceG(
	mongoConnectionManager *infrastructure.MongoConnectionManager,
	enabled bool,
	logger *logger.Logger,
) *ServiceG {
	return &ServiceG{
		enabled:                enabled,
		mongoConnectionManager: mongoConnectionManager,
		logger:                 logger,
	}
}

func (s *ServiceG) Name() string  { return "Service G (MongoDB Products)" }
func (s *ServiceG) Enabled() bool { return s.enabled }
func (s *ServiceG) Endpoints() []string {
	return []string{"/products/{tenant}", "/products/{tenant}/{id}"}
}

func (s *ServiceG) RegisterRoutes(g *echo.Group) {
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

// listProductsByTenant lists products from a specific tenant database
func (s *ServiceG) listProductsByTenant(c echo.Context) error {
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

// createProduct creates a new product in the specified tenant database
func (s *ServiceG) createProduct(c echo.Context) error {
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

// getProductByTenant retrieves a specific product from a tenant database
func (s *ServiceG) getProductByTenant(c echo.Context) error {
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

// updateProduct updates a product in the specified tenant database
func (s *ServiceG) updateProduct(c echo.Context) error {
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

// deleteProduct deletes a product from the specified tenant database
func (s *ServiceG) deleteProduct(c echo.Context) error {
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

// searchProducts performs advanced search on products
func (s *ServiceG) searchProducts(c echo.Context) error {
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

// getProductAnalytics provides analytics for products in a tenant
func (s *ServiceG) getProductAnalytics(c echo.Context) error {
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
	registry.RegisterService("service_g", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		if !config.Services.IsEnabled("service_g") {
			return nil
		}
		if deps == nil || deps.MongoConnectionManager == nil {
			logger.Warn("MongoDB connections not available, skipping Service G")
			return nil
		}
		return NewServiceG(deps.MongoConnectionManager, true, logger)
	})
}
