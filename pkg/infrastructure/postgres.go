package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"stackyard/config"
	"stackyard/pkg/logger"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresManager struct {
	DB   *sql.DB
	ORM  *gorm.DB
	Pool *WorkerPool // Async worker pool
}

type PostgresConnectionManager struct {
	connections map[string]*PostgresManager
	mu          sync.RWMutex
}

// Name returns the display name of the component
func (p *PostgresManager) Name() string {
	return "PostgreSQL"
}

// Name returns the display name of the component
func (m *PostgresConnectionManager) Name() string {
	return "PostgreSQL Connection Manager"
}

func NewPostgresDB(cfg config.PostgresConfig) (*PostgresManager, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	// Open raw SQL connection
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// Initialize GORM with the existing SQL connection
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GORM: %w", err)
	}

	// Initialize worker pool for async operations
	pool := NewWorkerPool(15) // Moderate pool for DB operations
	pool.Start()

	return &PostgresManager{
		DB:   sqlDB,
		ORM:  gormDB,
		Pool: pool,
	}, nil
}

func NewPostgresConnectionManager(cfg config.PostgresMultiConfig) (*PostgresConnectionManager, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	manager := &PostgresConnectionManager{
		connections: make(map[string]*PostgresManager),
	}

	for _, connCfg := range cfg.Connections {
		if !connCfg.Enabled {
			continue
		}

		// Convert connection config to single config for backward compatibility
		singleCfg := config.PostgresConfig{
			Enabled:  connCfg.Enabled,
			Host:     connCfg.Host,
			Port:     connCfg.Port,
			User:     connCfg.User,
			Password: connCfg.Password,
			DBName:   connCfg.DBName,
			SSLMode:  connCfg.SSLMode,
		}

		db, err := NewPostgresDB(singleCfg)
		if err != nil {
			// Log error but continue with other connections
			// Don't fail the entire manager initialization
			continue
		}

		if db != nil {
			manager.connections[connCfg.Name] = db
		}
	}

	return manager, nil
}

// GetConnection returns a specific named connection
func (m *PostgresConnectionManager) GetConnection(name string) (*PostgresManager, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, exists := m.connections[name]
	return conn, exists
}

// GetDefaultConnection returns the first connection or nil if none exist
func (m *PostgresConnectionManager) GetDefaultConnection() (*PostgresManager, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, conn := range m.connections {
		return conn, true
	}
	return nil, false
}

// GetAllConnections returns all connections
func (m *PostgresConnectionManager) GetAllConnections() map[string]*PostgresManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Create a copy to avoid race conditions
	copy := make(map[string]*PostgresManager, len(m.connections))
	for k, v := range m.connections {
		copy[k] = v
	}
	return copy
}

// GetStatus returns status for all connections
func (m *PostgresConnectionManager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status := make(map[string]interface{})

	for name, conn := range m.connections {
		status[name] = conn.GetStatus()
	}

	return status
}

// Close closes all connections (implements InfrastructureComponent)
func (m *PostgresConnectionManager) Close() error {
	return m.CloseAll()
}

// CloseAll closes all connections
func (m *PostgresConnectionManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error
	for name, conn := range m.connections {
		if err := conn.DB.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close connection '%s': %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing connections: %v", errors)
	}
	return nil
}

func (p *PostgresManager) GetStatus() map[string]interface{} {
	stats := make(map[string]interface{})
	if p == nil || p.DB == nil {
		stats["connected"] = false
		return stats
	}

	err := p.DB.Ping()
	stats["connected"] = err == nil

	// DB Stats
	dbStats := p.DB.Stats()
	stats["open_connections"] = dbStats.OpenConnections
	stats["in_use"] = dbStats.InUse
	stats["idle"] = dbStats.Idle
	stats["wait_count"] = dbStats.WaitCount
	stats["wait_duration_ms"] = dbStats.WaitDuration.Milliseconds()

	return stats
}

// Query executes a query that returns rows, typically a SELECT.
func (p *PostgresManager) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.DB.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
func (p *PostgresManager) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.DB.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning any rows.
func (p *PostgresManager) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.DB.ExecContext(ctx, query, args...)
}

// Select is a semantic alias for Query.
func (p *PostgresManager) Select(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.Query(ctx, query, args...)
}

// Insert executes an INSERT statement and returns the number of rows affected.
func (p *PostgresManager) Insert(ctx context.Context, query string, args ...interface{}) (int64, error) {
	res, err := p.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ExecuteRawQuery executes a raw SQL query and returns the results as a slice of maps
func (p *PostgresManager) ExecuteRawQuery(ctx context.Context, query string) ([]map[string]interface{}, error) {
	if p.DB == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	rows, err := p.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Initialize with make to ensure empty slice [] instead of nil
	results := make([]map[string]interface{}, 0)

	for rows.Next() {
		// Create a slice of interface{} to hold values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Create a map for the current row
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Handle byte arrays (common for strings in some drivers)
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		results = append(results, rowMap)
	}

	return results, nil
}

// Update executes an UPDATE statement and returns the number of rows affected.
func (p *PostgresManager) Update(ctx context.Context, query string, args ...interface{}) (int64, error) {
	res, err := p.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Delete executes a DELETE statement and returns the number of rows affected.
func (p *PostgresManager) Delete(ctx context.Context, query string, args ...interface{}) (int64, error) {
	res, err := p.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Monitoring Helpers

type PGQuery struct {
	Pid      int    `json:"pid"`
	User     string `json:"user"`
	DB       string `json:"db"`
	State    string `json:"state"`
	Duration string `json:"duration"`
	Query    string `json:"query"`
}

func (p *PostgresManager) GetRunningQueries(ctx context.Context) ([]PGQuery, error) {
	rows, err := p.DB.QueryContext(ctx, `
		SELECT pid, usename, datname, state, (now() - query_start) as duration, query 
		FROM pg_stat_activity 
		WHERE state != 'idle' AND pid <> pg_backend_pid()
		ORDER BY duration DESC LIMIT 50;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queries []PGQuery
	for rows.Next() {
		var q PGQuery
		var user, db, state, query sql.NullString
		var duration sql.NullString
		if err := rows.Scan(&q.Pid, &user, &db, &state, &duration, &query); err != nil {
			continue
		}
		q.User = user.String
		q.DB = db.String
		q.State = state.String
		q.Duration = duration.String
		q.Query = query.String
		queries = append(queries, q)
	}
	return queries, nil
}

func (p *PostgresManager) GetSessionCount(ctx context.Context) (int, error) {
	var count int
	err := p.DB.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity").Scan(&count)
	return count, err
}

func (p *PostgresManager) GetDBInfo(ctx context.Context) (map[string]interface{}, error) {
	var version, dbName, user, sslMode string

	// Fetch Version
	if err := p.DB.QueryRowContext(ctx, "SELECT version()").Scan(&version); err != nil {
		return nil, err
	}

	// Fetch DB Size (formatted)
	var size string
	if err := p.DB.QueryRowContext(ctx, "SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&size); err != nil {
		return nil, err
	}

	// Fetch DB Name
	if err := p.DB.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName); err != nil {
		return nil, err
	}

	// Fetch Current User
	if err := p.DB.QueryRowContext(ctx, "SELECT current_user").Scan(&user); err != nil {
		return nil, err
	}

	// Fetch SSL Status
	// Note: checks if usage of SSL is active for this backend
	err := p.DB.QueryRowContext(ctx, "SELECT COALESCE((SELECT 'enable' FROM pg_stat_ssl WHERE pid = pg_backend_pid() AND ssl = true), 'disable')").Scan(&sslMode)
	if err != nil {
		sslMode = "unknown"
	}

	return map[string]interface{}{
		"version":  version,
		"size":     size,
		"db_name":  dbName,
		"user":     user,
		"ssl_mode": sslMode,
	}, nil
}

// Async Postgres Operations

// QueryAsync asynchronously executes a query that returns rows.
func (p *PostgresManager) QueryAsync(ctx context.Context, query string, args ...interface{}) *AsyncResult[*sql.Rows] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*sql.Rows, error) {
		return p.Query(ctx, query, args...)
	})
}

// QueryRowAsync asynchronously executes a query that returns at most one row.
func (p *PostgresManager) QueryRowAsync(ctx context.Context, query string, args ...interface{}) *AsyncResult[*sql.Row] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*sql.Row, error) {
		row := p.QueryRow(ctx, query, args...)
		return row, nil // Note: sql.Row cannot be directly returned from async, this is a limitation
	})
}

// ExecAsync asynchronously executes a query without returning rows.
func (p *PostgresManager) ExecAsync(ctx context.Context, query string, args ...interface{}) *AsyncResult[sql.Result] {
	return ExecuteAsync(ctx, func(ctx context.Context) (sql.Result, error) {
		return p.Exec(ctx, query, args...)
	})
}

// InsertAsync asynchronously executes an INSERT statement.
func (p *PostgresManager) InsertAsync(ctx context.Context, query string, args ...interface{}) *AsyncResult[int64] {
	return ExecuteAsync(ctx, func(ctx context.Context) (int64, error) {
		return p.Insert(ctx, query, args...)
	})
}

// UpdateAsync asynchronously executes an UPDATE statement.
func (p *PostgresManager) UpdateAsync(ctx context.Context, query string, args ...interface{}) *AsyncResult[int64] {
	return ExecuteAsync(ctx, func(ctx context.Context) (int64, error) {
		return p.Update(ctx, query, args...)
	})
}

// DeleteAsync asynchronously executes a DELETE statement.
func (p *PostgresManager) DeleteAsync(ctx context.Context, query string, args ...interface{}) *AsyncResult[int64] {
	return ExecuteAsync(ctx, func(ctx context.Context) (int64, error) {
		return p.Delete(ctx, query, args...)
	})
}

// ExecuteRawQueryAsync asynchronously executes a raw SQL query.
func (p *PostgresManager) ExecuteRawQueryAsync(ctx context.Context, query string) *AsyncResult[[]map[string]interface{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) ([]map[string]interface{}, error) {
		return p.ExecuteRawQuery(ctx, query)
	})
}

// GetRunningQueriesAsync asynchronously gets running queries.
func (p *PostgresManager) GetRunningQueriesAsync(ctx context.Context) *AsyncResult[[]PGQuery] {
	return ExecuteAsync(ctx, func(ctx context.Context) ([]PGQuery, error) {
		return p.GetRunningQueries(ctx)
	})
}

// GetSessionCountAsync asynchronously gets session count.
func (p *PostgresManager) GetSessionCountAsync(ctx context.Context) *AsyncResult[int] {
	return ExecuteAsync(ctx, func(ctx context.Context) (int, error) {
		return p.GetSessionCount(ctx)
	})
}

// GetDBInfoAsync asynchronously gets database information.
func (p *PostgresManager) GetDBInfoAsync(ctx context.Context) *AsyncResult[map[string]interface{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (map[string]interface{}, error) {
		return p.GetDBInfo(ctx)
	})
}

// GORM Async Operations

// GORMCreateAsync asynchronously creates a record using GORM.
func (p *PostgresManager) GORMCreateAsync(ctx context.Context, value interface{}) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := p.ORM.WithContext(ctx).Create(value).Error
		return struct{}{}, err
	})
}

// GORMFindAsync asynchronously finds records using GORM.
func (p *PostgresManager) GORMFindAsync(ctx context.Context, dest interface{}, conds ...interface{}) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := p.ORM.WithContext(ctx).Find(dest, conds...).Error
		return struct{}{}, err
	})
}

// GORMFirstAsync asynchronously finds first record using GORM.
func (p *PostgresManager) GORMFirstAsync(ctx context.Context, dest interface{}, conds ...interface{}) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := p.ORM.WithContext(ctx).First(dest, conds...).Error
		return struct{}{}, err
	})
}

// GORMUpdateAsync asynchronously updates records using GORM.
func (p *PostgresManager) GORMUpdateAsync(ctx context.Context, model interface{}, updates interface{}, conds ...interface{}) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := p.ORM.WithContext(ctx).Model(model).Where(conds[0], conds[1:]...).Updates(updates).Error
		return struct{}{}, err
	})
}

// GORMDeleteAsync asynchronously deletes records using GORM.
func (p *PostgresManager) GORMDeleteAsync(ctx context.Context, value interface{}, conds ...interface{}) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := p.ORM.WithContext(ctx).Delete(value, conds...).Error
		return struct{}{}, err
	})
}

// Batch Operations

// ExecuteBatchAsync asynchronously executes multiple queries.
func (p *PostgresManager) ExecuteBatchAsync(ctx context.Context, queries []string, args [][]interface{}) *BatchAsyncResult[sql.Result] {
	if len(queries) != len(args) {
		// Create a batch result with an error
		result := NewBatchAsyncResult[sql.Result](len(queries))
		for i := range result.Results {
			result.Results[i].Complete(nil, fmt.Errorf("queries and args length mismatch"))
		}
		result.Complete()
		return result
	}

	operations := make([]AsyncOperation[sql.Result], len(queries))
	for i, query := range queries {
		query, args := query, args[i] // Capture loop variables
		operations[i] = func(ctx context.Context) (sql.Result, error) {
			return p.Exec(ctx, query, args...)
		}
	}

	return ExecuteBatchAsync(ctx, operations)
}

// Worker Pool Operations

// SubmitAsyncJob submits an async job to the worker pool.
func (p *PostgresManager) SubmitAsyncJob(job func()) {
	if p.Pool != nil {
		p.Pool.Submit(job)
	} else {
		// Fallback to direct execution if pool not available
		go job()
	}
}

// Close closes the Postgres manager and its worker pool.
func (p *PostgresManager) Close() error {
	if p.Pool != nil {
		p.Pool.Close()
	}
	if p.DB != nil {
		return p.DB.Close()
	}
	return nil
}

func init() {
	RegisterComponent("postgres", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.Postgres.Enabled && !cfg.PostgresMultiConfig.Enabled {
			return nil, nil
		}
		if cfg.PostgresMultiConfig.Enabled {
			return NewPostgresConnectionManager(cfg.PostgresMultiConfig)
		}
		return NewPostgresDB(cfg.Postgres)
	})
}
