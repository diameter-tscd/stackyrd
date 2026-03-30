package monitoring

import (
	"fmt"
	"os"
	"stackyard/config"
	"stackyard/pkg/infrastructure"
	"stackyard/pkg/response"
	"stackyard/pkg/utils"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	config                    *config.Config
	statusProvider            StatusProvider
	broadcaster               *LogBroadcaster
	redis                     *infrastructure.RedisManager
	postgres                  *infrastructure.PostgresManager
	postgresConnectionManager *infrastructure.PostgresConnectionManager
	mongo                     *infrastructure.MongoManager
	mongoConnectionManager    *infrastructure.MongoConnectionManager
	kafka                     *infrastructure.KafkaManager
	cron                      *infrastructure.CronManager
	minio                     *infrastructure.MinIOManager
	system                    *infrastructure.SystemManager
	http                      *infrastructure.HttpManager
	services                  []ServiceInfo

	// Dummy Logs
	dummyMu     sync.Mutex
	dummyActive bool
	dummyStop   chan struct{}
}

func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/api/status", h.getStatus)
	g.POST("/api/restart", h.Restart)                      // Maintenance
	g.GET("/api/monitoring/config", h.getMonitoringConfig) // New
	g.GET("/api/config", h.getConfig)
	g.GET("/api/config/raw", h.getRawConfig)     // New
	g.POST("/api/config", h.saveConfig)          // New
	g.POST("/api/config/backup", h.backupConfig) // New
	g.GET("/api/logs", h.streamLogs)
	g.GET("/api/cpu", h.streamCPU)
	g.GET("/api/endpoints", h.getEndpoints)
	g.GET("/api/cron", h.getCronJobs)
	g.POST("/api/postgres/query", h.runPostgresQuery) // New: Raw Query
	g.GET("/api/mongo/info", h.getMongoInfo)          // MongoDB info
	g.POST("/api/mongo/query", h.runMongoQuery)       // MongoDB raw query

	// Utils
	g.GET("/api/logs/dummy/status", h.getDummyStatus)
	g.POST("/api/logs/dummy", h.toggleDummyLogs)

	// Banner
	g.GET("/api/banner", h.getBanner)
	g.POST("/api/banner", h.saveBanner)

	// User Settings
	g.GET("/api/user/settings", h.getUserSettings)
	g.POST("/api/user/settings", h.updateUserSettings)
	g.POST("/api/user/password", h.changePassword)
	g.POST("/api/user/photo", h.uploadPhoto)
	g.DELETE("/api/user/photo", h.deleteUserPhoto)
	// Note: Static route for photos is registered in server.go

	// New Endpoints
	g.GET("/api/redis/keys", h.getRedisKeys)
	g.GET("/api/redis/key/:key", h.getRedisValue)
	g.GET("/api/postgres/queries", h.getPostgresQueries)
	g.GET("/api/postgres/info", h.getPostgresInfo)
	g.GET("/api/kafka/topics", h.getKafkaTopics)
	g.POST("/api/logs/dummy", h.toggleDummyLogs)
}

func (h *Handler) getDummyStatus(c echo.Context) error {
	h.dummyMu.Lock()
	defer h.dummyMu.Unlock()
	return response.Success(c, map[string]bool{"active": h.dummyActive})
}

func (h *Handler) toggleDummyLogs(c echo.Context) error {
	h.dummyMu.Lock()
	defer h.dummyMu.Unlock()

	type Req struct {
		Enable bool `json:"enable"`
	}
	var req Req
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request")
	}

	if req.Enable {
		if h.dummyActive {
			return response.Success(c, nil, "Already active")
		}
		h.dummyActive = true
		h.dummyStop = make(chan struct{})
		go h.runDummyLogs(h.dummyStop)
		return response.Success(c, nil, "Dummy logs enabled")
	} else {
		if !h.dummyActive {
			return response.Success(c, nil, "Already inactive")
		}
		h.dummyActive = false
		close(h.dummyStop)
		return response.Success(c, nil, "Dummy logs disabled")
	}
}

func (h *Handler) getMonitoringConfig(c echo.Context) error {
	return response.Success(c, map[string]string{
		"title":    h.config.Monitoring.Title,
		"subtitle": h.config.Monitoring.Subtitle,
	})
}

func (h *Handler) runDummyLogs(stop chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	levels := []string{"INFO", "WARN", "ERROR", "DEBUG"}
	messages := []string{
		"User login successful",
		"Cache miss for key user:123",
		"Database query took 150ms",
		"Background job processing started",
		"Kafka message consumed from topic: orders",
		"Payment gateway timeout",
		"Service health check passed",
		"Redis connection pool refreshing",
	}

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			level := levels[time.Now().UnixNano()%int64(len(levels))]
			msg := messages[time.Now().UnixNano()%int64(len(messages))]

			// Format as zerolog JSON-like output (or whatever format frontend expects)
			// Frontend expects raw text.
			// But broadcaster Write method expects []byte.
			// We can format it nicely.

			timestamp := time.Now().Format(time.RFC3339)
			logLine := fmt.Sprintf(`{"time":"%s","level":"%s","message":"[DUMMY] %s"}`+"\n", timestamp, level, msg)

			// If frontend expects raw string from `data:`, and `h.streamLogs` writes `msg` directly...
			// The broadcaster receives `[]byte` and sends it to channel.
			// `streamLogs` reads `msg` and writes `fmt.Fprintf(c.Response(), "data: %s\n\n", msg)`
			// So `msg` should be the full string line.

			h.broadcaster.Write([]byte(logLine))
		}
	}
}

func (h *Handler) getStatus(c echo.Context) error {
	// Collect status from all sources
	status := h.statusProvider.GetStatus()
	status["redis"] = h.redis.GetStatus()

	// Handle both single and multiple PostgreSQL connections
	if h.postgresConnectionManager != nil || (h.config.PostgresMultiConfig.Enabled && len(h.config.PostgresMultiConfig.Connections) > 0) {
		// For multiple connections, format the status for frontend compatibility
		var pgStatus map[string]interface{}
		if h.postgresConnectionManager != nil {
			pgStatus = h.postgresConnectionManager.GetStatus()
		} else {
			pgStatus = make(map[string]interface{})
		}

		var connectionStatuses = make(map[string]interface{})

		// Include all configured connections, even if they failed to connect
		for _, connCfg := range h.config.PostgresMultiConfig.Connections {
			connName := connCfg.Name
			if connStatus, exists := pgStatus[connName]; exists {
				connectionStatuses[connName] = connStatus
			} else {
				// Connection configured but failed to connect
				connectionStatuses[connName] = map[string]interface{}{
					"connected": false,
				}
			}
		}

		// Check if any connection is connected
		anyConnected := false
		for _, connStatus := range connectionStatuses {
			if statusMap, ok := connStatus.(map[string]interface{}); ok {
				if connected, ok := statusMap["connected"].(bool); ok && connected {
					anyConnected = true
					break
				}
			}
		}

		// Set overall postgres status with connected flag and connection details
		status["postgres"] = map[string]interface{}{
			"connected":   anyConnected,
			"connections": connectionStatuses,
		}
	} else {
		status["postgres"] = h.postgres.GetStatus()
	}

	status["kafka"] = h.kafka.GetStatus()
	status["cron"] = h.cron.GetStatus()

	// Handle both single and multiple MongoDB connections
	if h.mongoConnectionManager != nil || (h.config.MongoMultiConfig.Enabled && len(h.config.MongoMultiConfig.Connections) > 0) {
		// For multiple connections, format the status for frontend compatibility
		var mongoStatus map[string]interface{}
		if h.mongoConnectionManager != nil {
			mongoStatus = h.mongoConnectionManager.GetStatus()
		} else {
			mongoStatus = make(map[string]interface{})
		}

		var connectionStatuses = make(map[string]interface{})

		// Include all configured connections, even if they failed to connect
		for _, connCfg := range h.config.MongoMultiConfig.Connections {
			connName := connCfg.Name
			if connStatus, exists := mongoStatus[connName]; exists {
				connectionStatuses[connName] = connStatus
			} else {
				// Connection configured but failed to connect
				connectionStatuses[connName] = map[string]interface{}{
					"connected": false,
				}
			}
		}

		// Check if any connection is connected
		anyConnected := false
		for _, connStatus := range connectionStatuses {
			if statusMap, ok := connStatus.(map[string]interface{}); ok {
				if connected, ok := statusMap["connected"].(bool); ok && connected {
					anyConnected = true
					break
				}
			}
		}

		// Set overall mongo status with connected flag and connection details
		status["mongo"] = map[string]interface{}{
			"connected":   anyConnected,
			"connections": connectionStatuses,
		}
	} else {
		status["mongo"] = h.mongo.GetStatus()
	}

	// New Infrastructure
	status["storage"] = h.minio.GetStatus()
	status["system"] = h.system.GetStats()
	status["system_info"] = h.system.GetHostInfo()
	status["external"] = h.http.GetStatus()

	status["services"] = h.services
	return response.Success(c, status)
}

func (h *Handler) getConfig(c echo.Context) error {
	return response.Success(c, h.config)
}

func (h *Handler) getEndpoints(c echo.Context) error {
	return response.Success(c, h.services)
}

// ... existing streamLogs and streamCPU ...

func (h *Handler) getRedisKeys(c echo.Context) error {
	if h.redis == nil {
		return response.ServiceUnavailable(c, "Redis not enabled")
	}
	pattern := c.QueryParam("pattern")
	if pattern == "" {
		pattern = "*"
	}
	keys, err := h.redis.ScanKeys(c.Request().Context(), pattern)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, keys)
}

func (h *Handler) Restart(c echo.Context) error {
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(1)
	}()
	return response.Success(c, map[string]string{"status": "restarting", "message": "Service is restarting..."})
}

func (h *Handler) getRedisValue(c echo.Context) error {
	if h.redis == nil {
		return response.ServiceUnavailable(c, "Redis not enabled")
	}
	key := c.Param("key")
	val, err := h.redis.GetValue(c.Request().Context(), key)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, map[string]string{"key": key, "value": val})
}

func (h *Handler) getPostgresQueries(c echo.Context) error {
	// Check if PostgreSQL is enabled in configuration
	if !h.config.Postgres.Enabled && !h.config.PostgresMultiConfig.Enabled {
		return response.ServiceUnavailable(c, "Postgres not enabled")
	}

	// Get requested connection from query param
	connectionName := c.QueryParam("connection")

	// Use connection manager if available, otherwise fallback to single connection
	var postgresManager *infrastructure.PostgresManager
	if h.postgresConnectionManager != nil {
		if connectionName != "" {
			// Use specific connection if requested
			if conn, exists := h.postgresConnectionManager.GetConnection(connectionName); exists {
				postgresManager = conn
			}
			// If specific connection requested but not found, don't fallback
		} else {
			// Use default connection
			if defaultConn, exists := h.postgresConnectionManager.GetDefaultConnection(); exists {
				postgresManager = defaultConn
			}
		}
	} else {
		postgresManager = h.postgres
	}

	// If we have a connection manager but no connection found, try to get any available connection (only if no specific connection requested)
	if postgresManager == nil && h.postgresConnectionManager != nil && connectionName == "" {
		allConnections := h.postgresConnectionManager.GetAllConnections()
		for _, conn := range allConnections {
			postgresManager = conn
			break // Use the first available connection
		}
	}

	if postgresManager == nil {
		return response.Success(c, []interface{}{})
	}

	queries, err := postgresManager.GetRunningQueries(c.Request().Context())
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, queries)
}

func (h *Handler) getPostgresInfo(c echo.Context) error {
	// Check if PostgreSQL is enabled in configuration
	if !h.config.Postgres.Enabled && !h.config.PostgresMultiConfig.Enabled {
		return response.ServiceUnavailable(c, "Postgres not enabled")
	}

	// Get requested connection from query param
	connectionName := c.QueryParam("connection")

	// Use connection manager if available, otherwise fallback to single connection
	var postgresManager *infrastructure.PostgresManager
	if h.postgresConnectionManager != nil {
		if connectionName != "" {
			// Use specific connection if requested
			if conn, exists := h.postgresConnectionManager.GetConnection(connectionName); exists {
				postgresManager = conn
			}
		} else {
			// Use default connection
			if defaultConn, exists := h.postgresConnectionManager.GetDefaultConnection(); exists {
				postgresManager = defaultConn
			}
		}
	} else {
		postgresManager = h.postgres
	}

	// If we have a connection manager but no connection found, try to get any available connection (only if no specific connection requested)
	if postgresManager == nil && h.postgresConnectionManager != nil && connectionName == "" {
		allConnections := h.postgresConnectionManager.GetAllConnections()
		for _, conn := range allConnections {
			postgresManager = conn
			break // Use the first available connection
		}
	}

	if postgresManager == nil {
		return response.Success(c, map[string]interface{}{})
	}

	info, err := postgresManager.GetDBInfo(c.Request().Context())
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	count, _ := postgresManager.GetSessionCount(c.Request().Context())
	info["sessions"] = count

	return response.Success(c, info)
}

func (h *Handler) runPostgresQuery(c echo.Context) error {
	// Check if PostgreSQL is enabled in configuration
	if !h.config.Postgres.Enabled && !h.config.PostgresMultiConfig.Enabled {
		return response.ServiceUnavailable(c, "Postgres not enabled")
	}

	// Get requested connection from query param
	connectionName := c.QueryParam("connection")

	// Use connection manager if available, otherwise fallback to single connection
	var postgresManager *infrastructure.PostgresManager
	if h.postgresConnectionManager != nil {
		if connectionName != "" {
			// Use specific connection if requested
			if conn, exists := h.postgresConnectionManager.GetConnection(connectionName); exists {
				postgresManager = conn
			}
		} else {
			// Use default connection
			if defaultConn, exists := h.postgresConnectionManager.GetDefaultConnection(); exists {
				postgresManager = defaultConn
			}
		}
	} else {
		postgresManager = h.postgres
	}

	// If we have a connection manager but no connection found, try to get any available connection (only if no specific connection requested)
	if postgresManager == nil && h.postgresConnectionManager != nil && connectionName == "" {
		allConnections := h.postgresConnectionManager.GetAllConnections()
		for _, conn := range allConnections {
			postgresManager = conn
			break // Use the first available connection
		}
	}

	if postgresManager == nil {
		return response.ServiceUnavailable(c, "Postgres connection not available")
	}

	type QueryReq struct {
		Query string `json:"query"`
	}
	var req QueryReq
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request")
	}

	if req.Query == "" {
		return response.BadRequest(c, "Query cannot be empty")
	}

	// Safety check: Basic prevention of destructive queries (optional, mainly for safety demo)
	// if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(req.Query)), "SELECT") {
	// 	 return response.Forbidden(c, "Only SELECT queries are allowed in this demo")
	// }

	results, err := postgresManager.ExecuteRawQuery(c.Request().Context(), req.Query)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, results)
}

func (h *Handler) getKafkaTopics(c echo.Context) error {
	// Placeholder: To implement true Kafka monitoring, we need Admin client in KafkaManager.
	// For now return dummy or basic status.
	if h.kafka == nil {
		return response.ServiceUnavailable(c, "Kafka not enabled")
	}
	return response.Success(c, nil, "Kafka monitoring requires Admin API (not implemented yet)")
}

func (h *Handler) getCronJobs(c echo.Context) error {
	if h.cron == nil {
		return response.Success(c, []interface{}{}) // Return empty if disabled
	}
	return response.Success(c, h.cron.GetJobs())
}

func (h *Handler) getBanner(c echo.Context) error {
	path := h.config.App.BannerPath
	if path == "" {
		path = "banner.txt"
	}

	content, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, return empty string or error?
		// User might want to create it.
		if os.IsNotExist(err) {
			return response.Success(c, map[string]string{"content": ""})
		}
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, map[string]string{"content": string(content)})
}

func (h *Handler) saveBanner(c echo.Context) error {
	type Req struct {
		Content string `json:"content"`
	}
	var req Req
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request")
	}

	path := h.config.App.BannerPath
	if path == "" {
		path = "banner.txt"
	}

	// Write file (create if local, 0644)
	err := os.WriteFile(path, []byte(req.Content), 0644)
	if err != nil {
		return response.InternalServerError(c, "Failed to save banner: "+err.Error())
	}

	return response.Success(c, nil, "Banner saved successfully")
}

// Config Handlers
func (h *Handler) getRawConfig(c echo.Context) error {
	content, err := os.ReadFile("config.yaml")
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}
	return response.Success(c, map[string]string{"content": string(content)})
}

func (h *Handler) saveConfig(c echo.Context) error {
	type Req struct {
		Content string `json:"content"`
	}
	var req Req
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request")
	}

	err := os.WriteFile("config.yaml", []byte(req.Content), 0644)
	if err != nil {
		return response.InternalServerError(c, "Failed to save config: "+err.Error())
	}

	return response.Success(c, nil, "Config saved successfully. Restart required to apply changes.")
}

func (h *Handler) backupConfig(c echo.Context) error {
	input, err := os.ReadFile("config.yaml")
	if err != nil {
		return response.InternalServerError(c, "Failed to read config: "+err.Error())
	}

	backupName := fmt.Sprintf("config.yaml.bak.%d", time.Now().Unix())
	err = os.WriteFile(backupName, input, 0644)
	if err != nil {
		return response.InternalServerError(c, "Failed to create backup: "+err.Error())
	}

	return response.Success(c, nil, "Backup created: "+backupName)
}

func (h *Handler) streamLogs(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")

	logs := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(logs)

	for {
		select {
		case msg := <-logs:
			fmt.Fprintf(c.Response(), "data: %s\n\n", msg)
			c.Response().Flush()
		case <-c.Request().Context().Done():
			return nil
		}
	}
}

func (h *Handler) streamCPU(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats, _ := utils.GetSystemStats()
			fmt.Fprintf(c.Response(), "data: %.2f\n\n", stats["cpu_percent"])
			c.Response().Flush()
		case <-c.Request().Context().Done():
			return nil
		}
	}
}

func (h *Handler) getMongoInfo(c echo.Context) error {
	// Check if MongoDB is enabled in configuration
	if !h.config.Mongo.Enabled && !h.config.MongoMultiConfig.Enabled {
		return response.ServiceUnavailable(c, "MongoDB not enabled")
	}

	// Get requested connection from query param
	connectionName := c.QueryParam("connection")

	// Use connection manager if available, otherwise fallback to single connection
	var mongoManager *infrastructure.MongoManager
	if h.mongoConnectionManager != nil {
		if connectionName != "" {
			// Use specific connection if requested
			if conn, exists := h.mongoConnectionManager.GetConnection(connectionName); exists {
				mongoManager = conn
			}
		} else {
			// Use default connection
			if defaultConn, exists := h.mongoConnectionManager.GetDefaultConnection(); exists {
				mongoManager = defaultConn
			}
		}
	} else {
		mongoManager = h.mongo
	}

	// If we have a connection manager but no connection found, try to get any available connection (only if no specific connection requested)
	if mongoManager == nil && h.mongoConnectionManager != nil && connectionName == "" {
		allConnections := h.mongoConnectionManager.GetAllConnections()
		for _, conn := range allConnections {
			mongoManager = conn
			break // Use the first available connection
		}
	}

	if mongoManager == nil {
		return response.Success(c, map[string]interface{}{})
	}

	info, err := mongoManager.GetDBInfo(c.Request().Context())
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, info)
}

func (h *Handler) runMongoQuery(c echo.Context) error {
	// Check if MongoDB is enabled in configuration
	if !h.config.Mongo.Enabled && !h.config.MongoMultiConfig.Enabled {
		return response.ServiceUnavailable(c, "MongoDB not enabled")
	}

	// Get requested connection from query param
	connectionName := c.QueryParam("connection")

	// Use connection manager if available, otherwise fallback to single connection
	var mongoManager *infrastructure.MongoManager
	if h.mongoConnectionManager != nil {
		if connectionName != "" {
			// Use specific connection if requested
			if conn, exists := h.mongoConnectionManager.GetConnection(connectionName); exists {
				mongoManager = conn
			}
		} else {
			// Use default connection
			if defaultConn, exists := h.mongoConnectionManager.GetDefaultConnection(); exists {
				mongoManager = defaultConn
			}
		}
	} else {
		mongoManager = h.mongo
	}

	// If we have a connection manager but no connection found, try to get any available connection (only if no specific connection requested)
	if mongoManager == nil && h.mongoConnectionManager != nil && connectionName == "" {
		allConnections := h.mongoConnectionManager.GetAllConnections()
		for _, conn := range allConnections {
			mongoManager = conn
			break // Use the first available connection
		}
	}

	if mongoManager == nil {
		return response.ServiceUnavailable(c, "MongoDB connection not available")
	}

	type QueryReq struct {
		Collection string                 `json:"collection"`
		Query      map[string]interface{} `json:"query"`
	}
	var req QueryReq
	if err := c.Bind(&req); err != nil {
		return response.BadRequest(c, "Invalid request")
	}

	if req.Collection == "" {
		return response.BadRequest(c, "Collection cannot be empty")
	}

	if req.Query == nil {
		req.Query = map[string]interface{}{} // Empty query to find all documents
	}

	results, err := mongoManager.ExecuteRawQuery(c.Request().Context(), req.Collection, req.Query)
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, results)
}
