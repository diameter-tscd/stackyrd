package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"stackyard/config"
	"stackyard/pkg/logger"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// GrafanaManager manages Grafana API interactions
type GrafanaManager struct {
	Client   *retryablehttp.Client
	BaseURL  string
	APIKey   string
	Username string
	Password string
	Pool     *WorkerPool // Async worker pool
	logger   *logger.Logger
}

// grafanaLoggerAdapter adapts our custom logger to go-retryablehttp's LeveledLogger interface
type grafanaLoggerAdapter struct {
	logger *logger.Logger
}

func (a *grafanaLoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	a.logger.Error(msg, nil, keysAndValues...)
}

func (a *grafanaLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	a.logger.Info(msg, keysAndValues...)
}

func (a *grafanaLoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	a.logger.Debug(msg, keysAndValues...)
}

func (a *grafanaLoggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	a.logger.Warn(msg, keysAndValues...)
}

// GrafanaDashboard represents a Grafana dashboard
type GrafanaDashboard struct {
	ID            int                `json:"id,omitempty"`
	UID           string             `json:"uid,omitempty"`
	Title         string             `json:"title"`
	Tags          []string           `json:"tags,omitempty"`
	Timezone      string             `json:"timezone,omitempty"`
	Panels        []GrafanaPanel     `json:"panels,omitempty"`
	Time          GrafanaTimeRange   `json:"time,omitempty"`
	Timepicker    GrafanaTimePicker  `json:"timepicker,omitempty"`
	Templating    GrafanaTemplating  `json:"templating,omitempty"`
	Annotations   GrafanaAnnotations `json:"annotations,omitempty"`
	Refresh       string             `json:"refresh,omitempty"`
	SchemaVersion int                `json:"schemaVersion,omitempty"`
	Version       int                `json:"version,omitempty"`
	Links         []interface{}      `json:"links,omitempty"`
}

// GrafanaPanel represents a dashboard panel
type GrafanaPanel struct {
	ID            int                    `json:"id"`
	Title         string                 `json:"title"`
	Type          string                 `json:"type"`
	GridPos       GrafanaGridPos         `json:"gridPos"`
	Targets       []GrafanaTarget        `json:"targets,omitempty"`
	FieldConfig   GrafanaFieldConfig     `json:"fieldConfig,omitempty"`
	Options       map[string]interface{} `json:"options,omitempty"`
	PluginVersion string                 `json:"pluginVersion,omitempty"`
}

// GrafanaGridPos represents panel position
type GrafanaGridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

// GrafanaTarget represents a query target
type GrafanaTarget struct {
	Expr         string            `json:"expr,omitempty"`
	LegendFormat string            `json:"legendFormat,omitempty"`
	RefID        string            `json:"refId,omitempty"`
	Datasource   GrafanaDatasource `json:"datasource,omitempty"`
}

// GrafanaDatasource represents a data source reference
type GrafanaDatasource struct {
	Type string `json:"type,omitempty"`
	UID  string `json:"uid,omitempty"`
}

// GrafanaTimeRange represents time range settings
type GrafanaTimeRange struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// GrafanaTimePicker represents time picker settings
type GrafanaTimePicker struct {
	RefreshIntervals []string `json:"refresh_intervals,omitempty"`
}

// GrafanaTemplating represents template variables
type GrafanaTemplating struct {
	List []GrafanaTemplateVar `json:"list,omitempty"`
}

// GrafanaTemplateVar represents a template variable
type GrafanaTemplateVar struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Datasource interface{} `json:"datasource,omitempty"`
	Query      string      `json:"query,omitempty"`
	Label      string      `json:"label,omitempty"`
}

// GrafanaAnnotations represents annotation settings
type GrafanaAnnotations struct {
	List []interface{} `json:"list,omitempty"`
}

// GrafanaFieldConfig represents field configuration
type GrafanaFieldConfig struct {
	Defaults  GrafanaFieldDefaults `json:"defaults,omitempty"`
	Overrides []interface{}        `json:"overrides,omitempty"`
}

// GrafanaFieldDefaults represents default field settings
type GrafanaFieldDefaults struct {
	Unit     string                 `json:"unit,omitempty"`
	Decimals *int                   `json:"decimals,omitempty"`
	Custom   map[string]interface{} `json:"custom,omitempty"`
}

// GrafanaDataSource represents a Grafana data source
type GrafanaDataSource struct {
	ID                int                    `json:"id,omitempty"`
	UID               string                 `json:"uid,omitempty"`
	Name              string                 `json:"name"`
	Type              string                 `json:"type"`
	URL               string                 `json:"url,omitempty"`
	Access            string                 `json:"access,omitempty"`
	Database          string                 `json:"database,omitempty"`
	User              string                 `json:"user,omitempty"`
	Password          string                 `json:"password,omitempty"`
	BasicAuth         bool                   `json:"basicAuth,omitempty"`
	BasicAuthUser     string                 `json:"basicAuthUser,omitempty"`
	BasicAuthPassword string                 `json:"basicAuthPassword,omitempty"`
	JSONData          map[string]interface{} `json:"jsonData,omitempty"`
	SecureJSONData    map[string]interface{} `json:"secureJsonData,omitempty"`
	ReadOnly          bool                   `json:"readOnly,omitempty"`
}

// GrafanaAnnotation represents an annotation
type GrafanaAnnotation struct {
	ID          int                    `json:"id,omitempty"`
	DashboardID int                    `json:"dashboardId,omitempty"`
	PanelID     int                    `json:"panelId,omitempty"`
	Time        int64                  `json:"time,omitempty"`
	TimeEnd     int64                  `json:"timeEnd,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Text        string                 `json:"text"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// Name returns the display name of the component
func (gm *GrafanaManager) Name() string {
	return "Grafana"
}

// NewGrafanaManager creates a new Grafana manager
func NewGrafanaManager(cfg config.GrafanaConfig, logger *logger.Logger) (*GrafanaManager, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	logger.Info("Initializing Grafana manager", "url", cfg.URL)

	// Create HTTP client with retry logic
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = time.Second
	client.RetryWaitMax = 5 * time.Second
	client.HTTPClient.Timeout = 30 * time.Second

	// Set custom logger for go-retryablehttp
	client.Logger = &grafanaLoggerAdapter{logger: logger}

	// Add authentication if provided
	if cfg.APIKey != "" {
		client.RequestLogHook = func(logger retryablehttp.Logger, req *http.Request, retryNumber int) {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		logger.Debug("Using API key authentication")
	} else if cfg.Username != "" {
		logger.Debug("Using basic authentication", "username", cfg.Username)
	}

	manager := &GrafanaManager{
		Client:   client,
		BaseURL:  strings.TrimSuffix(cfg.URL, "/"),
		APIKey:   cfg.APIKey,
		Username: cfg.Username,
		Password: cfg.Password,
		logger:   logger,
	}

	// Test connection
	if err := manager.testConnection(); err != nil {
		logger.Error("Grafana connection test failed", err)
		return nil, fmt.Errorf("failed to connect to Grafana: %w", err)
	}

	logger.Info("Grafana connection test successful")

	// Initialize worker pool for async operations
	pool := NewWorkerPool(5) // Default 5 workers
	pool.Start()

	manager.Pool = pool
	logger.Info("Grafana manager initialized with worker pool")

	return manager, nil
}

// testConnection tests the connection to Grafana
func (gm *GrafanaManager) testConnection() error {
	req, err := retryablehttp.NewRequest("GET", gm.BaseURL+"/api/health", nil)
	if err != nil {
		return err
	}

	resp, err := gm.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		gm.logger.Error("Grafana health check failed", nil, "status", resp.StatusCode)
		return fmt.Errorf("Grafana health check failed with status: %d", resp.StatusCode)
	}

	return nil
}

// CreateDashboard creates a new dashboard
func (gm *GrafanaManager) CreateDashboard(ctx context.Context, dashboard GrafanaDashboard) (*GrafanaDashboard, error) {
	gm.logger.Info("Creating Grafana dashboard", "title", dashboard.Title)

	payload := map[string]interface{}{
		"dashboard": dashboard,
		"overwrite": false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		gm.logger.Error("Failed to marshal dashboard", err, "title", dashboard.Title)
		return nil, fmt.Errorf("failed to marshal dashboard: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, "POST", gm.BaseURL+"/api/dashboards/db", bytes.NewReader(jsonData))
	if err != nil {
		gm.logger.Error("Failed to create HTTP request", err)
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := gm.Client.Do(req)
	if err != nil {
		gm.logger.Error("HTTP request failed", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		gm.logger.Error("Grafana API returned error", nil, "status", resp.StatusCode, "response", string(body))
		return nil, fmt.Errorf("failed to create dashboard: %s (status: %d)", string(body), resp.StatusCode)
	}

	var result struct {
		ID      int    `json:"id"`
		UID     string `json:"uid"`
		URL     string `json:"url"`
		Status  string `json:"status"`
		Version int    `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		gm.logger.Error("Failed to decode response", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	dashboard.ID = result.ID
	dashboard.UID = result.UID
	dashboard.Version = result.Version

	gm.logger.Info("Dashboard created successfully", "title", dashboard.Title, "uid", dashboard.UID, "id", dashboard.ID)
	return &dashboard, nil
}

// UpdateDashboard updates an existing dashboard
func (gm *GrafanaManager) UpdateDashboard(ctx context.Context, dashboard GrafanaDashboard) (*GrafanaDashboard, error) {
	payload := map[string]interface{}{
		"dashboard": dashboard,
		"overwrite": true,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dashboard: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, "POST", gm.BaseURL+"/api/dashboards/db", bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := gm.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update dashboard: %s (status: %d)", string(body), resp.StatusCode)
	}

	var result struct {
		ID      int    `json:"id"`
		UID     string `json:"uid"`
		URL     string `json:"url"`
		Status  string `json:"status"`
		Version int    `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	dashboard.ID = result.ID
	dashboard.UID = result.UID
	dashboard.Version = result.Version

	return &dashboard, nil
}

// GetDashboard retrieves a dashboard by UID
func (gm *GrafanaManager) GetDashboard(ctx context.Context, uid string) (*GrafanaDashboard, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", gm.BaseURL+"/api/dashboards/uid/"+uid, nil)
	if err != nil {
		return nil, err
	}

	resp, err := gm.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("dashboard not found: %s", uid)
		}
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get dashboard: %s (status: %d)", string(body), resp.StatusCode)
	}

	var result struct {
		Dashboard GrafanaDashboard `json:"dashboard"`
		Meta      struct {
			Type        string `json:"type"`
			CanSave     bool   `json:"canSave"`
			CanEdit     bool   `json:"canEdit"`
			CanAdmin    bool   `json:"canAdmin"`
			CanStar     bool   `json:"canStar"`
			Slug        string `json:"slug"`
			URL         string `json:"url"`
			Expires     string `json:"expires"`
			Created     string `json:"created"`
			Updated     string `json:"updated"`
			UpdatedBy   string `json:"updatedBy"`
			CreatedBy   string `json:"createdBy"`
			Version     int    `json:"version"`
			HasACL      bool   `json:"hasACL"`
			IsFolder    bool   `json:"isFolder"`
			FolderID    int    `json:"folderId"`
			FolderTitle string `json:"folderTitle"`
			FolderURL   string `json:"folderUrl"`
		} `json:"meta"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode dashboard: %w", err)
	}

	return &result.Dashboard, nil
}

// DeleteDashboard deletes a dashboard by UID
func (gm *GrafanaManager) DeleteDashboard(ctx context.Context, uid string) error {
	req, err := retryablehttp.NewRequestWithContext(ctx, "DELETE", gm.BaseURL+"/api/dashboards/uid/"+uid, nil)
	if err != nil {
		return err
	}

	resp, err := gm.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete dashboard: %s (status: %d)", string(body), resp.StatusCode)
	}

	return nil
}

// ListDashboards lists all dashboards
func (gm *GrafanaManager) ListDashboards(ctx context.Context) ([]GrafanaDashboard, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", gm.BaseURL+"/api/search?type=dash-db", nil)
	if err != nil {
		return nil, err
	}

	resp, err := gm.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list dashboards: %s (status: %d)", string(body), resp.StatusCode)
	}

	var dashboards []struct {
		ID          int      `json:"id"`
		UID         string   `json:"uid"`
		Title       string   `json:"title"`
		URI         string   `json:"uri"`
		URL         string   `json:"url"`
		Slug        string   `json:"slug"`
		Type        string   `json:"type"`
		Tags        []string `json:"tags"`
		IsStarred   bool     `json:"isStarred"`
		FolderID    int      `json:"folderId"`
		FolderTitle string   `json:"folderTitle"`
		FolderURL   string   `json:"folderUrl"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&dashboards); err != nil {
		return nil, fmt.Errorf("failed to decode dashboards: %w", err)
	}

	result := make([]GrafanaDashboard, len(dashboards))
	for i, d := range dashboards {
		result[i] = GrafanaDashboard{
			ID:    d.ID,
			UID:   d.UID,
			Title: d.Title,
			Tags:  d.Tags,
		}
	}

	return result, nil
}

// CreateDataSource creates a new data source
func (gm *GrafanaManager) CreateDataSource(ctx context.Context, ds GrafanaDataSource) (*GrafanaDataSource, error) {
	jsonData, err := json.Marshal(ds)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data source: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, "POST", gm.BaseURL+"/api/datasources", bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := gm.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create data source: %s (status: %d)", string(body), resp.StatusCode)
	}

	var result GrafanaDataSource
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode data source: %w", err)
	}

	return &result, nil
}

// CreateAnnotation creates a new annotation
func (gm *GrafanaManager) CreateAnnotation(ctx context.Context, annotation GrafanaAnnotation) (*GrafanaAnnotation, error) {
	jsonData, err := json.Marshal(annotation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal annotation: %w", err)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, "POST", gm.BaseURL+"/api/annotations", bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := gm.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create annotation: %s (status: %d)", string(body), resp.StatusCode)
	}

	var result GrafanaAnnotation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode annotation: %w", err)
	}

	return &result, nil
}

// GetHealth returns Grafana health status
func (gm *GrafanaManager) GetHealth(ctx context.Context) (map[string]interface{}, error) {
	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", gm.BaseURL+"/api/health", nil)
	if err != nil {
		return nil, err
	}

	resp, err := gm.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	return result, nil
}

// GetStatus returns the current status of the Grafana manager
func (gm *GrafanaManager) GetStatus() map[string]interface{} {
	stats := make(map[string]interface{})
	if gm == nil {
		stats["connected"] = false
		return stats
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := gm.GetHealth(ctx)
	if err != nil {
		stats["connected"] = false
		stats["error"] = err.Error()
		return stats
	}

	stats["connected"] = true
	stats["url"] = gm.BaseURL
	stats["version"] = health["version"]
	stats["database"] = health["database"]

	// Add pool stats
	if gm.Pool != nil {
		// Worker pool stats would go here if exposed
		stats["pool_active"] = true
	}

	return stats
}

// Async Operations

// CreateDashboardAsync asynchronously creates a dashboard
func (gm *GrafanaManager) CreateDashboardAsync(ctx context.Context, dashboard GrafanaDashboard) *AsyncResult[*GrafanaDashboard] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*GrafanaDashboard, error) {
		return gm.CreateDashboard(ctx, dashboard)
	})
}

// UpdateDashboardAsync asynchronously updates a dashboard
func (gm *GrafanaManager) UpdateDashboardAsync(ctx context.Context, dashboard GrafanaDashboard) *AsyncResult[*GrafanaDashboard] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*GrafanaDashboard, error) {
		return gm.UpdateDashboard(ctx, dashboard)
	})
}

// GetDashboardAsync asynchronously retrieves a dashboard
func (gm *GrafanaManager) GetDashboardAsync(ctx context.Context, uid string) *AsyncResult[*GrafanaDashboard] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*GrafanaDashboard, error) {
		return gm.GetDashboard(ctx, uid)
	})
}

// DeleteDashboardAsync asynchronously deletes a dashboard
func (gm *GrafanaManager) DeleteDashboardAsync(ctx context.Context, uid string) *AsyncResult[struct{}] {
	return ExecuteAsync(ctx, func(ctx context.Context) (struct{}, error) {
		err := gm.DeleteDashboard(ctx, uid)
		return struct{}{}, err
	})
}

// ListDashboardsAsync asynchronously lists dashboards
func (gm *GrafanaManager) ListDashboardsAsync(ctx context.Context) *AsyncResult[[]GrafanaDashboard] {
	return ExecuteAsync(ctx, func(ctx context.Context) ([]GrafanaDashboard, error) {
		return gm.ListDashboards(ctx)
	})
}

// CreateDataSourceAsync asynchronously creates a data source
func (gm *GrafanaManager) CreateDataSourceAsync(ctx context.Context, ds GrafanaDataSource) *AsyncResult[*GrafanaDataSource] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*GrafanaDataSource, error) {
		return gm.CreateDataSource(ctx, ds)
	})
}

// CreateAnnotationAsync asynchronously creates an annotation
func (gm *GrafanaManager) CreateAnnotationAsync(ctx context.Context, annotation GrafanaAnnotation) *AsyncResult[*GrafanaAnnotation] {
	return ExecuteAsync(ctx, func(ctx context.Context) (*GrafanaAnnotation, error) {
		return gm.CreateAnnotation(ctx, annotation)
	})
}

// SubmitAsyncJob submits an async job to the worker pool
func (gm *GrafanaManager) SubmitAsyncJob(job func()) {
	if gm.Pool != nil {
		gm.Pool.Submit(job)
	} else {
		// Fallback to direct execution if pool not available
		go job()
	}
}

// Close closes the Grafana manager and its worker pool
func (gm *GrafanaManager) Close() error {
	if gm.Pool != nil {
		gm.Pool.Close()
	}
	return nil
}

func init() {
	RegisterComponent("grafana", func(cfg *config.Config, l *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.Grafana.Enabled {
			return nil, nil
		}
		return NewGrafanaManager(cfg.Grafana, l)
	})
}
