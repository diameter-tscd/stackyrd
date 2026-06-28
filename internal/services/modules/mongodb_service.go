package modules

import (
	"fmt"

	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

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

	sub.GET("/:tenant", s.listProductsByTenant)
	sub.POST("/:tenant", s.createProduct)
	sub.GET("/:tenant/:id", s.getProductByTenant)
	sub.PUT("/:tenant/:id", s.updateProduct)
	sub.DELETE("/:tenant/:id", s.deleteProduct)
	sub.GET("/:tenant/search", s.searchProducts)
	sub.GET("/:tenant/analytics", s.getProductAnalytics)
}

func (s *MongoDBService) listProductsByTenant(c echo.Context) error {
	tenant := c.Param("tenant")
	if tenant == "" {
		return response.BadRequest(c, "Tenant identifier is required")
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
	}

	ctx := c.Request().Context()
	cursor, err := conn.Find(ctx, "products", bson.M{})
	if err != nil {
		s.logger.Error("Failed to query products", err, "tenant", tenant)
		return response.InternalServerError(c, "Failed to query tenant database")
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			s.logger.Error("Failed to close MongoDB cursor", err)
		}
	}()

	var products []Product
	if err := cursor.All(ctx, &products); err != nil {
		s.logger.Error("Failed to decode products", err)
		return response.InternalServerError(c, "Failed to decode products")
	}

	return response.Success(c, products, fmt.Sprintf("Products retrieved from tenant '%s'", tenant))
}

func (s *MongoDBService) createProduct(c echo.Context) error {
	tenant := c.Param("tenant")
	if tenant == "" {
		return response.BadRequest(c, "Tenant identifier is required")
	}

	var product Product
	if err := request.Bind(c, &product); err != nil {
		return response.BadRequest(c, "Invalid product data")
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
	}

	ctx := c.Request().Context()
	result, err := conn.InsertOne(ctx, "products", product)
	if err != nil {
		s.logger.Error("Failed to create product", err, "tenant", tenant)
		return response.InternalServerError(c, "Failed to create product")
	}

	return response.Created(c, map[string]interface{}{
		"id":      result.InsertedID,
		"tenant":  tenant,
		"product": product,
	}, fmt.Sprintf("Product created in tenant '%s'", tenant))
}

func (s *MongoDBService) getProductByTenant(c echo.Context) error {
	tenant := c.Param("tenant")
	id := c.Param("id")

	if tenant == "" || id == "" {
		return response.BadRequest(c, "Tenant and product ID are required")
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid product ID format")
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
	}

	ctx := c.Request().Context()
	var product Product
	err = conn.FindOne(ctx, "products", bson.M{"_id": objectID}).Decode(&product)
	if err != nil {
		return response.NotFound(c, "Product not found")
	}

	return response.Success(c, product, "Product retrieved successfully")
}

func (s *MongoDBService) updateProduct(c echo.Context) error {
	tenant := c.Param("tenant")
	id := c.Param("id")

	if tenant == "" || id == "" {
		return response.BadRequest(c, "Tenant and product ID are required")
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid product ID format")
	}

	var product Product
	if err := request.Bind(c, &product); err != nil {
		return response.BadRequest(c, "Invalid product data")
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
	}

	ctx := c.Request().Context()
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
		return response.InternalServerError(c, "Failed to update product")
	}

	if result.MatchedCount == 0 {
		return response.NotFound(c, "Product not found")
	}

	return response.Success(c, nil, "Product updated successfully")
}

func (s *MongoDBService) deleteProduct(c echo.Context) error {
	tenant := c.Param("tenant")
	id := c.Param("id")

	if tenant == "" || id == "" {
		return response.BadRequest(c, "Tenant and product ID are required")
	}

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return response.BadRequest(c, "Invalid product ID format")
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
	}

	ctx := c.Request().Context()
	result, err := conn.DeleteOne(ctx, "products", bson.M{"_id": objectID})
	if err != nil {
		s.logger.Error("Failed to delete product", err, "tenant", tenant)
		return response.InternalServerError(c, "Failed to delete product")
	}

	if result.DeletedCount == 0 {
		return response.NotFound(c, "Product not found")
	}

	return response.Success(c, nil, "Product deleted successfully")
}

func (s *MongoDBService) searchProducts(c echo.Context) error {
	tenant := c.Param("tenant")
	if tenant == "" {
		return response.BadRequest(c, "Tenant identifier is required")
	}

	query := c.QueryParam("q")

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
	}

	ctx := c.Request().Context()
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
		return response.InternalServerError(c, "Failed to search products")
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			s.logger.Error("Failed to close MongoDB cursor", err)
		}
	}()

	var products []Product
	if err := cursor.All(ctx, &products); err != nil {
		s.logger.Error("Failed to decode products", err)
		return response.InternalServerError(c, "Failed to decode products")
	}

	return response.Success(c, products, fmt.Sprintf("Found %d products", len(products)))
}

func (s *MongoDBService) getProductAnalytics(c echo.Context) error {
	tenant := c.Param("tenant")
	if tenant == "" {
		return response.BadRequest(c, "Tenant identifier is required")
	}

	conn, exists := s.mongoConnectionManager.GetConnection(tenant)
	if !exists {
		return response.NotFound(c, fmt.Sprintf("Tenant database '%s' not found", tenant))
	}

	ctx := c.Request().Context()
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
		return response.InternalServerError(c, "Failed to get analytics")
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			s.logger.Error("Failed to close MongoDB cursor", err)
		}
	}()

	var analytics []bson.M
	if err := cursor.All(ctx, &analytics); err != nil {
		s.logger.Error("Failed to decode analytics", err)
		return response.InternalServerError(c, "Failed to decode analytics")
	}

	return response.Success(c, analytics, "Analytics retrieved successfully")
}

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
