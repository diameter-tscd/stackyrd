package infrastructure

import (
	"context"
	"fmt"
	"stackyard/config"
	"stackyard/pkg/logger"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type MongoManager struct {
	Client   *mongo.Client
	Database *mongo.Database
	Pool     *WorkerPool // Async worker pool
}

// Name returns the display name of the component
func (m *MongoManager) Name() string {
	return "MongoDB"
}

type MongoConnectionManager struct {
	connections map[string]*MongoManager
	mu          sync.RWMutex
}

// Name returns the display name of the component
func (m *MongoConnectionManager) Name() string {
	return "MongoDB Connection Manager"
}

func NewMongoDB(cfg config.MongoConfig, l *logger.Logger) (*MongoManager, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	l.Info("Connecting to MongoDB", "uri", cfg.URI, "database", cfg.Database)

	// Create context with timeout for connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set client options with timeout configurations
	clientOptions := options.Client().
		ApplyURI(cfg.URI).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(5 * time.Second).
		SetSocketTimeout(10 * time.Second).
		SetMaxConnIdleTime(30 * time.Second).
		SetHeartbeatInterval(10 * time.Second).
		SetReadPreference(readpref.PrimaryPreferred())

	// Connect to MongoDB with timeout
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		l.Error("Failed to connect to MongoDB", err, "timeout", "10s")
		return nil, fmt.Errorf("failed to connect to MongoDB (timeout: 10s): %w", err)
	}

	// Ping the database with timeout
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		// Close connection on ping failure
		client.Disconnect(context.Background())
		l.Error("Failed to ping MongoDB", err, "timeout", "5s")
		return nil, fmt.Errorf("failed to ping MongoDB (timeout: 5s): %w", err)
	}

	l.Info("Successfully connected to MongoDB", "database", cfg.Database)

	// Get database
	database := client.Database(cfg.Database)

	// Initialize worker pool for async operations
	pool := NewWorkerPool(12) // Moderate pool for document operations
	pool.Start()

	return &MongoManager{
		Client:   client,
		Database: database,
		Pool:     pool,
	}, nil
}

func NewMongoConnectionManager(cfg config.MongoMultiConfig, l *logger.Logger) (*MongoConnectionManager, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	l.Info("Initializing MongoDB connection manager", "connections", len(cfg.Connections))

	manager := &MongoConnectionManager{
		connections: make(map[string]*MongoManager),
	}

	for _, connCfg := range cfg.Connections {
		if !connCfg.Enabled {
			continue
		}

		// Convert connection config to single config for backward compatibility
		singleCfg := config.MongoConfig{
			Enabled:  connCfg.Enabled,
			URI:      connCfg.URI,
			Database: connCfg.Database,
		}

		db, err := NewMongoDB(singleCfg, l)
		if err != nil {
			// Log error but continue with other connections
			l.Error("Failed to create MongoDB connection", err, "name", connCfg.Name)
			continue
		}

		if db != nil {
			manager.connections[connCfg.Name] = db
			l.Info("MongoDB connection established", "name", connCfg.Name, "database", connCfg.Database)
		}
	}

	l.Info("MongoDB connection manager initialized", "active_connections", len(manager.connections))
	return manager, nil
}

// GetConnection returns a specific named connection
func (m *MongoConnectionManager) GetConnection(name string) (*MongoManager, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, exists := m.connections[name]
	return conn, exists
}

// GetDefaultConnection returns the first connection or nil if none exist
func (m *MongoConnectionManager) GetDefaultConnection() (*MongoManager, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, conn := range m.connections {
		return conn, true
	}
	return nil, false
}

// GetAllConnections returns all connections
func (m *MongoConnectionManager) GetAllConnections() map[string]*MongoManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Create a copy to avoid race conditions
	copy := make(map[string]*MongoManager, len(m.connections))
	for k, v := range m.connections {
		copy[k] = v
	}
	return copy
}

// GetStatus returns status for all connections
func (m *MongoConnectionManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := make(map[string]interface{})

	for name, conn := range m.connections {
		status[name] = conn.GetStatus()
	}

	return status
}

// Close closes all connections (implements InfrastructureComponent)
func (m *MongoConnectionManager) Close() error {
	return m.CloseAll()
}

// CloseAll closes all connections
func (m *MongoConnectionManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error
	for name, conn := range m.connections {
		if err := conn.Client.Disconnect(context.Background()); err != nil {
			errors = append(errors, fmt.Errorf("failed to close connection '%s': %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing connections: %v", errors)
	}
	return nil
}

func (m *MongoManager) GetStatus() map[string]interface{} {
	stats := make(map[string]interface{})
	if m == nil || m.Client == nil {
		stats["connected"] = false
		return stats
	}

	// Ping to check connection
	err := m.Client.Ping(context.Background(), nil)
	stats["connected"] = err == nil

	if err != nil {
		return stats
	}

	// Get database stats
	dbStats := m.Database.RunCommand(context.Background(), map[string]interface{}{"dbStats": 1})
	if dbStats.Err() == nil {
		var result map[string]interface{}
		if err := dbStats.Decode(&result); err == nil {
			stats["db_name"] = result["db"]
			stats["collections"] = result["collections"]
			stats["objects"] = result["objects"]
			stats["data_size"] = result["dataSize"]
			stats["storage_size"] = result["storageSize"]
			stats["indexes"] = result["indexes"]
			stats["index_size"] = result["indexSize"]
		}
	}

	return stats
}

// Collection returns a collection from the database
func (m *MongoManager) Collection(name string) *mongo.Collection {
	return m.Database.Collection(name)
}

// InsertOne inserts a single document
func (m *MongoManager) InsertOne(ctx context.Context, collection string, document interface{}) (*mongo.InsertOneResult, error) {
	coll := m.Database.Collection(collection)
	return coll.InsertOne(ctx, document)
}

// InsertMany inserts multiple documents
func (m *MongoManager) InsertMany(ctx context.Context, collection string, documents []interface{}) (*mongo.InsertManyResult, error) {
	coll := m.Database.Collection(collection)
	return coll.InsertMany(ctx, documents)
}

// FindOne finds a single document
func (m *MongoManager) FindOne(ctx context.Context, collection string, filter interface{}) *mongo.SingleResult {
	coll := m.Database.Collection(collection)
	return coll.FindOne(ctx, filter)
}

// Find finds multiple documents
func (m *MongoManager) Find(ctx context.Context, collection string, filter interface{}) (*mongo.Cursor, error) {
	coll := m.Database.Collection(collection)
	return coll.Find(ctx, filter)
}

// UpdateOne updates a single document
func (m *MongoManager) UpdateOne(ctx context.Context, collection string, filter interface{}, update interface{}) (*mongo.UpdateResult, error) {
	coll := m.Database.Collection(collection)
	return coll.UpdateOne(ctx, filter, update)
}

// UpdateMany updates multiple documents
func (m *MongoManager) UpdateMany(ctx context.Context, collection string, filter interface{}, update interface{}) (*mongo.UpdateResult, error) {
	coll := m.Database.Collection(collection)
	return coll.UpdateMany(ctx, filter, update)
}

// DeleteOne deletes a single document
func (m *MongoManager) DeleteOne(ctx context.Context, collection string, filter interface{}) (*mongo.DeleteResult, error) {
	coll := m.Database.Collection(collection)
	return coll.DeleteOne(ctx, filter)
}

// DeleteMany deletes multiple documents
func (m *MongoManager) DeleteMany(ctx context.Context, collection string, filter interface{}) (*mongo.DeleteResult, error) {
	coll := m.Database.Collection(collection)
	return coll.DeleteMany(ctx, filter)
}

// CountDocuments counts documents in a collection
func (m *MongoManager) CountDocuments(ctx context.Context, collection string, filter interface{}) (int64, error) {
	coll := m.Database.Collection(collection)
	return coll.CountDocuments(ctx, filter)
}

// Aggregate performs aggregation operations
func (m *MongoManager) Aggregate(ctx context.Context, collection string, pipeline interface{}) (*mongo.Cursor, error) {
	coll := m.Database.Collection(collection)
	return coll.Aggregate(ctx, pipeline)
}

// ListCollections returns all collection names
func (m *MongoManager) ListCollections(ctx context.Context) ([]string, error) {
	collections, err := m.Database.ListCollectionNames(ctx, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return collections, nil
}

// CreateCollection creates a new collection
func (m *MongoManager) CreateCollection(ctx context.Context, name string) error {
	return m.Database.CreateCollection(ctx, name)
}

// DropCollection drops a collection
func (m *MongoManager) DropCollection(ctx context.Context, name string) error {
	coll := m.Database.Collection(name)
	return coll.Drop(ctx)
}

// GetDBInfo returns database information
func (m *MongoManager) GetDBInfo(ctx context.Context) (map[string]interface{}, error) {
	// Get database stats
	command := map[string]interface{}{"dbStats": 1}
	result := m.Database.RunCommand(ctx, command)

	var stats map[string]interface{}
	if err := result.Decode(&stats); err != nil {
		return nil, err
	}

	// Get server status
	serverStatus := m.Client.Database("admin").RunCommand(ctx, map[string]interface{}{"serverStatus": 1})
	var serverInfo map[string]interface{}
	if serverStatus.Err() == nil {
		serverStatus.Decode(&serverInfo)
	}

	// Get list of collections
	collections, err := m.ListCollections(ctx)
	if err != nil {
		collections = []string{}
	}

	info := map[string]interface{}{
		"database":    m.Database.Name(),
		"collections": collections,
		"stats":       stats,
	}

	if serverInfo != nil {
		info["server_info"] = serverInfo
	}

	return info, nil
}

// ExecuteRawQuery executes a raw MongoDB query and returns results as a slice of maps
func (m *MongoManager) ExecuteRawQuery(ctx context.Context, collection string, query map[string]interface{}) ([]map[string]interface{}, error) {
	cursor, err := m.Find(ctx, collection, query)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	for cursor.Next(ctx) {
		var result map[string]interface{}
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, cursor.Err()
}

// StringToObjectID converts a string to MongoDB ObjectID
func StringToObjectID(id string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(id)
}

// StringToFloat converts a string to float64
func StringToFloat(s string) float64 {
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val
	}
	return 0.0
}

// StringToStringSlice converts a comma-separated string to []string
func StringToStringSlice(s string) []string {
	if s == "" {
		return []string{}
	}
	// Simple split by comma, trim spaces
	result := strings.Split(s, ",")
	for i, v := range result {
		result[i] = strings.TrimSpace(v)
	}
	return result
}

// Async MongoDB Operations

// InsertOneAsync asynchronously inserts a single document
func (m *MongoManager) InsertOneAsync(ctx context.Context, collection string, document interface{}) *AsyncResult[*mongo.InsertOneResult] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.InsertOneResult, error) {
		return m.InsertOne(ctx, collection, document)
	})
}

// InsertManyAsync asynchronously inserts multiple documents
func (m *MongoManager) InsertManyAsync(ctx context.Context, collection string, documents []interface{}) *AsyncResult[*mongo.InsertManyResult] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.InsertManyResult, error) {
		return m.InsertMany(ctx, collection, documents)
	})
}

// FindOneAsync asynchronously finds a single document
func (m *MongoManager) FindOneAsync(ctx context.Context, collection string, filter interface{}) *AsyncResult[*mongo.SingleResult] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.SingleResult, error) {
		result := m.FindOne(ctx, collection, filter)
		return result, nil // Note: SingleResult cannot be directly returned from async
	})
}

// FindAsync asynchronously finds multiple documents
func (m *MongoManager) FindAsync(ctx context.Context, collection string, filter interface{}) *AsyncResult[*mongo.Cursor] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.Cursor, error) {
		return m.Find(ctx, collection, filter)
	})
}

// UpdateOneAsync asynchronously updates a single document
func (m *MongoManager) UpdateOneAsync(ctx context.Context, collection string, filter interface{}, update interface{}) *AsyncResult[*mongo.UpdateResult] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.UpdateResult, error) {
		return m.UpdateOne(ctx, collection, filter, update)
	})
}

// UpdateManyAsync asynchronously updates multiple documents
func (m *MongoManager) UpdateManyAsync(ctx context.Context, collection string, filter interface{}, update interface{}) *AsyncResult[*mongo.UpdateResult] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.UpdateResult, error) {
		return m.UpdateMany(ctx, collection, filter, update)
	})
}

// DeleteOneAsync asynchronously deletes a single document
func (m *MongoManager) DeleteOneAsync(ctx context.Context, collection string, filter interface{}) *AsyncResult[*mongo.DeleteResult] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.DeleteResult, error) {
		return m.DeleteOne(ctx, collection, filter)
	})
}

// DeleteManyAsync asynchronously deletes multiple documents
func (m *MongoManager) DeleteManyAsync(ctx context.Context, collection string, filter interface{}) *AsyncResult[*mongo.DeleteResult] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.DeleteResult, error) {
		return m.DeleteMany(ctx, collection, filter)
	})
}

// CountDocumentsAsync asynchronously counts documents in a collection
func (m *MongoManager) CountDocumentsAsync(ctx context.Context, collection string, filter interface{}) *AsyncResult[int64] {
	return ExecuteAsync(ctx, func(ctx context.Context) (int64, error) {
		return m.CountDocuments(ctx, collection, filter)
	})
}

// AggregateAsync asynchronously performs aggregation operations
func (m *MongoManager) AggregateAsync(ctx context.Context, collection string, pipeline interface{}) *AsyncResult[*mongo.Cursor] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*mongo.Cursor, error) {
		return m.Aggregate(ctx, collection, pipeline)
	})
}

// ListCollectionsAsync asynchronously returns all collection names
func (m *MongoManager) ListCollectionsAsync(ctx context.Context) *AsyncResult[[]string] {
	return ExecuteAsync(ctx, func(ctx context.Context) ([]string, error) {
		return m.ListCollections(ctx)
	})
}

// CreateCollectionAsync asynchronously creates a new collection
func (m *MongoManager) CreateCollectionAsync(ctx context.Context, name string) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := m.CreateCollection(ctx, name)
		return struct{}{}, err
	})
}

// DropCollectionAsync asynchronously drops a collection
func (m *MongoManager) DropCollectionAsync(ctx context.Context, name string) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := m.DropCollection(ctx, name)
		return struct{}{}, err
	})
}

// GetDBInfoAsync asynchronously returns database information
func (m *MongoManager) GetDBInfoAsync(ctx context.Context) *AsyncResult[map[string]interface{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (map[string]interface{}, error) {
		return m.GetDBInfo(ctx)
	})
}

// ExecuteRawQueryAsync asynchronously executes a raw MongoDB query
func (m *MongoManager) ExecuteRawQueryAsync(ctx context.Context, collection string, query map[string]interface{}) *AsyncResult[[]map[string]interface{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) ([]map[string]interface{}, error) {
		return m.ExecuteRawQuery(ctx, collection, query)
	})
}

// Batch Operations

// InsertBatchAsync asynchronously inserts multiple documents across different collections
func (m *MongoManager) InsertBatchAsync(ctx context.Context, inserts []struct {
	Collection string
	Document   interface{}
}) *BatchAsyncResult[*mongo.InsertOneResult] {
	operations := make([]AsyncOperation[*mongo.InsertOneResult], len(inserts))

	for i, insert := range inserts {
		insert := insert // Capture loop variable
		operations[i] = func(ctx context.Context) (*mongo.InsertOneResult, error) {
			return m.InsertOne(ctx, insert.Collection, insert.Document)
		}
	}

	return ExecuteBatchAsync(ctx, operations)
}

// UpdateBatchAsync asynchronously updates multiple documents
func (m *MongoManager) UpdateBatchAsync(ctx context.Context, updates []struct {
	Collection string
	Filter     interface{}
	Update     interface{}
}) *BatchAsyncResult[*mongo.UpdateResult] {
	operations := make([]AsyncOperation[*mongo.UpdateResult], len(updates))

	for i, update := range updates {
		update := update // Capture loop variable
		operations[i] = func(ctx context.Context) (*mongo.UpdateResult, error) {
			return m.UpdateOne(ctx, update.Collection, update.Filter, update.Update)
		}
	}

	return ExecuteBatchAsync(ctx, operations)
}

// Worker Pool Operations

// SubmitAsyncJob submits an async job to the worker pool.
func (m *MongoManager) SubmitAsyncJob(job func()) {
	if m.Pool != nil {
		m.Pool.Submit(job)
	} else {
		// Fallback to direct execution if pool not available
		go job()
	}
}

// Close closes the MongoDB manager and its worker pool.
func (m *MongoManager) Close() error {
	if m.Pool != nil {
		m.Pool.Close()
	}
	if m.Client != nil {
		return m.Client.Disconnect(context.Background())
	}
	return nil
}

func init() {
	RegisterComponent("mongo", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.Mongo.Enabled && !cfg.MongoMultiConfig.Enabled {
			return nil, nil
		}
		if cfg.MongoMultiConfig.Enabled {
			return NewMongoConnectionManager(cfg.MongoMultiConfig, log)
		}
		return NewMongoDB(cfg.Mongo, log)
	})
}
