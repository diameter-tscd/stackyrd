package modules

import (
	"strconv"

	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
)

type GrafanaService struct {
	grafanaManager *infrastructure.GrafanaManager
	enabled        bool
	logger         *logger.Logger
}

func NewGrafanaService(grafanaManager *infrastructure.GrafanaManager, enabled bool, logger *logger.Logger) *GrafanaService {
	return &GrafanaService{
		grafanaManager: grafanaManager,
		enabled:        enabled,
		logger:         logger,
	}
}

func (s *GrafanaService) Name() string     { return "Grafana Service" }
func (s *GrafanaService) WireName() string { return "grafana-service" }
func (s *GrafanaService) Enabled() bool    { return s.enabled }
func (s *GrafanaService) Get() interface{} { return s }
func (s *GrafanaService) Endpoints() []string {
	return []string{"/grafana/dashboards", "/grafana/datasources", "/grafana/annotations", "/grafana/health"}
}

func (s *GrafanaService) RegisterRoutes(g *echo.Group) {
	grafana := g.Group("/grafana")

	dashboards := grafana.Group("/dashboards")
	dashboards.POST("", s.createDashboard)
	dashboards.PUT("/:uid", s.updateDashboard)
	dashboards.GET("/:uid", s.getDashboard)
	dashboards.DELETE("/:uid", s.deleteDashboard)
	dashboards.GET("", s.listDashboards)

	datasources := grafana.Group("/datasources")
	datasources.POST("", s.createDataSource)

	annotations := grafana.Group("/annotations")
	annotations.POST("", s.createAnnotation)

	grafana.GET("/health", s.getHealth)
}

func (s *GrafanaService) createDashboard(c echo.Context) error {
	var dashboard infrastructure.GrafanaDashboard
	if err := c.Bind(&dashboard); err != nil {
		return response.BadRequest(c, "Invalid dashboard data")
	}

	result, err := s.grafanaManager.CreateDashboard(c.Request().Context(), dashboard)
	if err != nil {
		s.logger.Error("Failed to create Grafana dashboard", err)
		return response.InternalServerError(c, "Failed to create dashboard")
	}

	return response.Created(c, result, "Dashboard created successfully")
}

func (s *GrafanaService) updateDashboard(c echo.Context) error {
	uid := c.Param("uid")
	if uid == "" {
		return response.BadRequest(c, "Dashboard UID is required")
	}

	var dashboard infrastructure.GrafanaDashboard
	if err := c.Bind(&dashboard); err != nil {
		return response.BadRequest(c, "Invalid dashboard data")
	}

	dashboard.UID = uid

	result, err := s.grafanaManager.UpdateDashboard(c.Request().Context(), dashboard)
	if err != nil {
		s.logger.Error("Failed to update Grafana dashboard", err, "uid", uid)
		return response.InternalServerError(c, "Failed to update dashboard")
	}

	return response.Success(c, result, "Dashboard updated successfully")
}

func (s *GrafanaService) getDashboard(c echo.Context) error {
	uid := c.Param("uid")
	if uid == "" {
		return response.BadRequest(c, "Dashboard UID is required")
	}

	dashboard, err := s.grafanaManager.GetDashboard(c.Request().Context(), uid)
	if err != nil {
		s.logger.Error("Failed to get Grafana dashboard", err, "uid", uid)
		return response.NotFound(c, "Dashboard not found")
	}

	return response.Success(c, dashboard, "Dashboard retrieved successfully")
}

func (s *GrafanaService) deleteDashboard(c echo.Context) error {
	uid := c.Param("uid")
	if uid == "" {
		return response.BadRequest(c, "Dashboard UID is required")
	}

	err := s.grafanaManager.DeleteDashboard(c.Request().Context(), uid)
	if err != nil {
		s.logger.Error("Failed to delete Grafana dashboard", err, "uid", uid)
		return response.InternalServerError(c, "Failed to delete dashboard")
	}

	return response.Success(c, nil, "Dashboard deleted successfully")
}

func (s *GrafanaService) listDashboards(c echo.Context) error {
	page := 1
	perPage := 50

	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if perPageStr := c.QueryParam("per_page"); perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	dashboards, err := s.grafanaManager.ListDashboards(c.Request().Context())
	if err != nil {
		s.logger.Error("Failed to list Grafana dashboards", err)
		return response.InternalServerError(c, "Failed to list dashboards")
	}

	start := (page - 1) * perPage
	end := start + perPage

	if start >= len(dashboards) {
		dashboards = []infrastructure.GrafanaDashboard{}
	} else if end > len(dashboards) {
		dashboards = dashboards[start:]
	} else {
		dashboards = dashboards[start:end]
	}

	meta := response.CalculateMeta(page, perPage, int64(len(dashboards)))
	return response.SuccessWithMeta(c, dashboards, meta, "Dashboards retrieved successfully")
}

func (s *GrafanaService) createDataSource(c echo.Context) error {
	var ds infrastructure.GrafanaDataSource
	if err := c.Bind(&ds); err != nil {
		return response.BadRequest(c, "Invalid data source data")
	}

	result, err := s.grafanaManager.CreateDataSource(c.Request().Context(), ds)
	if err != nil {
		s.logger.Error("Failed to create Grafana data source", err)
		return response.InternalServerError(c, "Failed to create data source")
	}

	return response.Created(c, result, "Data source created successfully")
}

func (s *GrafanaService) createAnnotation(c echo.Context) error {
	var annotation infrastructure.GrafanaAnnotation
	if err := c.Bind(&annotation); err != nil {
		return response.BadRequest(c, "Invalid annotation data")
	}

	result, err := s.grafanaManager.CreateAnnotation(c.Request().Context(), annotation)
	if err != nil {
		s.logger.Error("Failed to create Grafana annotation", err)
		return response.InternalServerError(c, "Failed to create annotation")
	}

	return response.Created(c, result, "Annotation created successfully")
}

func (s *GrafanaService) getHealth(c echo.Context) error {
	health, err := s.grafanaManager.GetHealth(c.Request().Context())
	if err != nil {
		s.logger.Error("Failed to get Grafana health", err)
		return response.ServiceUnavailable(c, "Grafana is not available")
	}

	return response.Success(c, health, "Grafana health check successful")
}

func init() {
	registry.RegisterService("grafana_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		helper := registry.NewServiceHelper(config, logger, deps)

		if !helper.IsServiceEnabled("grafana_service") {
			return nil
		}

		grafanaManager, ok := registry.GetTyped[infrastructure.GrafanaManager](deps, "grafana")
		if !helper.RequireDependency("GrafanaManager", ok) {
			return nil
		}

		return NewGrafanaService(&grafanaManager, true, logger)
	})
}
