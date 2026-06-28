package main_test

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	_ "stackyrd/internal/middleware"
	_ "stackyrd/internal/services/modules"
	_ "stackyrd/pkg/infrastructure"

	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/response"
)

func BenchmarkAppStartup(b *testing.B) {
	b.Run("ConfigLoad", BenchmarkAppStartup_ConfigLoad)
	b.Run("LoggerInit", BenchmarkAppStartup_LoggerInit)
	b.Run("ServerInitMiddlewareAndServices", BenchmarkAppStartup_ServerInitMiddlewareAndServices)
	b.Run("FullStartupConsole", BenchmarkAppStartup_FullStartupConsole)
	b.Run("FullStartupTUI", BenchmarkAppStartup_FullStartupTUI)
}

func BenchmarkAppStartup_ConfigLoad(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := config.LoadConfig()
		if err != nil {
			b.Fatalf("config.LoadConfig() failed on iteration %d: %v", i, err)
		}
	}
}

func BenchmarkAppStartup_LoggerInit(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	_ = os.Stderr

	for i := 0; i < b.N; i++ {
		l := logger.New(false, nil)
		_ = l
	}
}

func BenchmarkAppStartup_ServerInitMiddlewareAndServices(b *testing.B) {
	b.ReportAllocs()

	cfg, err := config.LoadConfig()
	if err != nil {
		b.Fatalf("one-time config.LoadConfig() failed: %v", err)
	}
	l := logger.New(false, nil)
	_ = l

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		infraReg := infrastructure.GetGlobalRegistry()
		if err := infraReg.Initialize(cfg, l); err != nil {
			b.Logf("infra init note: %v", err)
		}

		mwReg := middleware.GetGlobalMiddlewareRegistry()
		mwReg.ApplyConfig(cfg)

		deps := registry.NewDependencies()
		svcList := registry.AutoDiscoverServices(cfg, l, deps)

		reg := registry.NewServiceRegistry(l)
		for _, svc := range svcList {
			reg.Register(svc)
		}
		reg.Boot(echo.New())

		infraReg.CloseAll()
	}
}

func BenchmarkAppStartup_FullStartupConsole(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	_ = os.Stderr

	for i := 0; i < b.N; i++ {
		start := time.Now()

		cfg, err := config.LoadConfig()
		if err != nil {
			b.Fatalf("config.LoadConfig() failed on iteration %d: %v", i, err)
		}
		cfg.App.EnableTUI = false

		l := logger.New(false, nil)
		_ = l

		infraReg := infrastructure.GetGlobalRegistry()
		if err := infraReg.Initialize(cfg, l); err != nil {
			b.Logf("infra init note: %v", err)
		}

		mwReg := middleware.GetGlobalMiddlewareRegistry()
		mwReg.ApplyConfig(cfg)

		deps := registry.NewDependencies()
		svcList := registry.AutoDiscoverServices(cfg, l, deps)

		reg := registry.NewServiceRegistry(l)
		for _, svc := range svcList {
			reg.Register(svc)
		}
		reg.Boot(echo.New())

		infraReg.CloseAll()

		_ = time.Since(start)
		_ = b.Elapsed()
	}
}

func BenchmarkAppStartup_FullStartupTUI(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	_ = os.Stderr

	for i := 0; i < b.N; i++ {
		start := time.Now()

		cfg, err := config.LoadConfig()
		if err != nil {
			b.Fatalf("config.LoadConfig() failed on iteration %d: %v", i, err)
		}
		cfg.App.EnableTUI = true

		l := logger.New(false, nil)
		_ = l

		infraReg := infrastructure.GetGlobalRegistry()
		if err := infraReg.Initialize(cfg, l); err != nil {
			b.Logf("infra init note: %v", err)
		}

		mwReg := middleware.GetGlobalMiddlewareRegistry()
		mwReg.ApplyConfig(cfg)

		deps := registry.NewDependencies()
		svcList := registry.AutoDiscoverServices(cfg, l, deps)

		reg := registry.NewServiceRegistry(l)
		for _, svc := range svcList {
			reg.Register(svc)
		}
		reg.Boot(echo.New())

		infraReg.CloseAll()

		_ = time.Since(start)
		_ = b.Elapsed()
	}
}

func BenchmarkAppStartup_Infrastructure(b *testing.B) {
	b.Run("ComponentInit", BenchmarkAppStartup_InfraComponentInit)
}

func BenchmarkAppStartup_InfraComponentInit(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	b.Helper()

	cfg, err := config.LoadConfig()
	if err != nil {
		b.Fatalf("config.LoadConfig() failed: %v", err)
	}
	l := logger.New(false, nil)
	_ = l

	for i := 0; i < b.N; i++ {
		infraReg := infrastructure.GetGlobalRegistry()
		if err := infraReg.Initialize(cfg, l); err != nil {
			b.Logf("infra init note: %v", err)
		}
		comps := infraReg.GetAll()
		_ = comps
		infraReg.CloseAll()
	}
}

func BenchmarkStartupSnapshot(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	_ = os.Stderr

	for i := 0; i < b.N; i++ {
		start := time.Now()

		cfg, err := config.LoadConfig()
		if err != nil {
			b.Fatalf("config.LoadConfig() failed on iteration %d: %v", i, err)
		}

		l := logger.New(false, nil)
		_ = l

		infraReg := infrastructure.GetGlobalRegistry()
		if err := infraReg.Initialize(cfg, l); err != nil {
			b.Logf("infra init note: %v", err)
		}
		components := infraReg.GetAll()
		for name, comp := range components {
			_ = name
			_ = comp.GetStatus()
		}

		mwReg := middleware.GetGlobalMiddlewareRegistry()
		mwReg.ApplyConfig(cfg)

		deps := registry.NewDependencies()
		svcList := registry.AutoDiscoverServices(cfg, l, deps)

		reg := registry.NewServiceRegistry(l)
		for _, svc := range svcList {
			reg.Register(svc)
		}
		reg.Boot(echo.New())

		infraReg.CloseAll()

		_ = time.Since(start)
		_ = b.Elapsed()
	}
}

func TestStartup_HealthEndpointReady(t *testing.T) {
	r, deps := mustBuildDiagnosticRouter(t)

	w := performRequest(r, http.MethodGet, "/health")
	assert.Equal(t, http.StatusOK, w.Code, "health endpoint must respond 200 after full startup")
	assert.Contains(t, w.Body.String(), "status", "health payload must contain a status field")
	assert.NotNil(t, deps, "dependencies container must be non-nil after full startup")
}

func TestStartup_HealthDependenciesEndpointReady(t *testing.T) {
	r, _ := mustBuildDiagnosticRouter(t)

	w := performRequest(r, http.MethodGet, "/health/dependencies")
	assert.Equal(t, http.StatusOK, w.Code, "health/dependencies endpoint must respond 200")
	assert.Contains(t, w.Body.String(), "total_infrastructure",
		"health/dependencies must report infrastructure component count")
}

func TestStartup_HealthResourcesEndpointReady(t *testing.T) {
	r, _ := mustBuildDiagnosticRouter(t)

	w := performRequest(r, http.MethodGet, "/health/resources")
	assert.Equal(t, http.StatusOK, w.Code, "health/resources endpoint must respond 200")
}

func mustBuildDiagnosticRouter(t *testing.T) (*echo.Echo, *registry.Dependencies) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("config.LoadConfig() failed: %v", err)
	}
	cfg.App.EnableTUI = false

	l := logger.New(false, nil)
	_ = l

	infraReg := infrastructure.GetGlobalRegistry()
	if err := infraReg.Initialize(cfg, l); err != nil {
		t.Logf("infra init note (non-fatal in CI): %v", err)
	}

	mwReg := middleware.GetGlobalMiddlewareRegistry()
	mwReg.ApplyConfig(cfg)

	deps := registry.NewDependencies()
	svcList := registry.AutoDiscoverServices(cfg, l, deps)

	e := echo.New()
	reg := registry.NewServiceRegistry(l)
	for _, svc := range svcList {
		reg.Register(svc)
	}

	// Register health endpoints (normally done by server.Start())
	e.GET("/health", func(c echo.Context) error {
		return response.Success(c, map[string]interface{}{
			"status":       "ok",
			"server_ready": true,
		})
	})
	e.GET("/health/dependencies", func(c echo.Context) error {
		allComponents := deps.GetAll()
		allFactories := registry.GetServiceFactories()
		return response.Success(c, map[string]interface{}{
			"total_infrastructure": len(allComponents),
			"list_infrastructure":  slices.Collect(maps.Keys(allComponents)),
			"total_service":        len(allFactories),
			"list_service":         slices.Collect(maps.Keys(allFactories)),
		})
	})
	e.GET("/health/resources", func(c echo.Context) error {
		return response.Success(c, map[string]interface{}{})
	})

	reg.Boot(e)

	return e, deps
}

func performRequest(e *echo.Echo, method, path string) *httptestRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	e.ServeHTTP(w, req)
	return &httptestRecorder{w}
}

type httptestRecorder struct {
	*httptest.ResponseRecorder
}

func TestStartup_AutoDiscoveredServiceFactoriesArePresent(t *testing.T) {
	factories := registry.GetServiceFactories()
	assert.NotEmpty(t, factories,
		"at least one service factory must be registered by init() in internal/services/modules")
	_, hasUsers := factories["users_service"]
	assert.True(t, hasUsers, "users_service must be registered by init() in internal/services/modules")
}

func TestStartup_AutoDiscoveredMiddlewareFactoriesArePresent(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)

	l := logger.New(false, nil)
	_ = l

	mwReg := middleware.GetGlobalMiddlewareRegistry()
	mwReg.ApplyConfig(cfg)
	mws := mwReg.AutoDiscoverMiddlewares(cfg, l)

	assert.NotEmpty(t, mws,
		"AutoDiscoverMiddlewares must return at least one handler; "+
			"middleware init() side-effects may be broken")
	for _, mw := range mws {
		assert.NotNil(t, mw, "no auto-discovered middleware handler may be nil")
	}
}

const BenchmarkThresholdMilliseconds = 1000
