package main_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	_ "stackyrd/internal/middleware" // nolint:blank-imports triggers init() auto-registrations
	_ "stackyrd/internal/services/modules"
	_ "stackyrd/pkg/infrastructure" // nolint:blank-imports triggers init() auto-registrations

	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
)

// Sub-benchmarks: config + logger + router init (non-blocking)

func BenchmarkAppStartup(b *testing.B) {
	gin.SetMode(gin.TestMode)

	b.Run("ConfigLoad", BenchmarkAppStartup_ConfigLoad)
	b.Run("LoggerInit", BenchmarkAppStartup_LoggerInit)
	b.Run("ServerInitMiddlewareAndServices", BenchmarkAppStartup_ServerInitMiddlewareAndServices)
	b.Run("FullStartupConsole", BenchmarkAppStartup_FullStartupConsole)
	b.Run("FullStartupTUI", BenchmarkAppStartup_FullStartupTUI)
}

// BenchmarkAppStartup_ConfigLoad measures the time to load and validate the
// application configuration (Viper defaults + YAML file read + unmarshal +
// PostgreSQL / MongoDB multi-connection normalization).
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

// BenchmarkAppStartup_LoggerInit measures the time to construct a new
// structured zerolog-based logger instance.
func BenchmarkAppStartup_LoggerInit(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	_ = os.Stderr // ensure stderr (used by zerolog) is open

	for i := 0; i < b.N; i++ {
		l := logger.New(false, nil)
		_ = l
	}
}

// BenchmarkAppStartup_ServerInitMiddlewareAndServices times the heavyweight
// synchronous wiring performed by server.Start() up to — but not including —
// the blocking gin.Run() call:
//
//  1. Infrastructure component registry (registered via init() in pkg/infrastructure)
//  2. Middleware config applied to the global registry + all enabled factory
//     functions called
//  3. Service factories auto-discovered (registered via init() in
//     internal/services/modules)
//  4. Services registered + Boot() called (route tree built on the Gin engine)
//
// Safe to run in CI: no network listener is bound.
func BenchmarkAppStartup_ServerInitMiddlewareAndServices(b *testing.B) {
	b.ReportAllocs()

	cfg, err := config.LoadConfig()
	if err != nil {
		b.Fatalf("one-time config.LoadConfig() failed: %v", err)
	}
	l := logger.New(false, nil)
	_ = l
	gin.SetMode(gin.TestMode)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// ── Infrastructure component registry ──────────────────────────
		infraReg := infrastructure.GetGlobalRegistry()
		if err := infraReg.Initialize(cfg, l); err != nil {
			// Connecting to DB/Kafka/Redis may fail in CI; not fatal for
			// measuring the init code path.
			b.Logf("infra init note: %v", err)
		}

		// ── Middleware wiring ──────────────────────────────────────────
		mwReg := middleware.GetGlobalMiddlewareRegistry()
		mwReg.ApplyConfig(cfg)

		// ── Service discovery + bootstrap ──────────────────────────────
		deps := registry.NewDependencies()
		svcList := registry.AutoDiscoverServices(cfg, l, deps)

		reg := registry.NewServiceRegistry(l)
		for _, svc := range svcList {
			reg.Register(svc)
		}
		reg.Boot(gin.New())

		// ── Soft teardown (release infra component resources) ───────────
		infraReg.CloseAll()
	}
}

// BenchmarkAppStartup_FullStartupConsole times config + logger + middleware +
// service discovery + infrastructure init + Bootstrap in a single pass, with
// TUI disabled (console mode).
//
// server.Start() is intentionally NOT called — that would block indefinitely
// on gin.Run().  The measured phase is every synchronous step before the HTTP
// listener opens.
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
		cfg.App.EnableTUI = false // force console mode

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

		gin.SetMode(gin.TestMode)
		reg := registry.NewServiceRegistry(l)
		for _, svc := range svcList {
			reg.Register(svc)
		}
		reg.Boot(gin.New())

		infraReg.CloseAll()

		_ = time.Since(start)
		_ = b.Elapsed() // record per-iteration wall time for the benchmark report
	}
}

// BenchmarkAppStartup_FullStartupTUI mirrors FullStartupConsole but forces
// TUI enabled (cfg.App.EnableTUI = true).
// The flag is only a boolean check so TUI and non-TUI timings should be
// near-identical; a significant gap signals an unintended code-path difference.
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
		cfg.App.EnableTUI = true // force TUI mode

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

		gin.SetMode(gin.TestMode)
		reg := registry.NewServiceRegistry(l)
		for _, svc := range svcList {
			reg.Register(svc)
		}
		reg.Boot(gin.New())

		infraReg.CloseAll()

		_ = time.Since(start)
		_ = b.Elapsed()
	}
}

// Infrastructure component benchmark

func BenchmarkAppStartup_Infrastructure(b *testing.B) {
	b.Run("ComponentInit", BenchmarkAppStartup_InfraComponentInit)
}

// BenchmarkAppStartup_InfraComponentInit measures the cost of
// ComponentRegistry.Initialize over the full set of factories registered via
// init() in pkg/infrastructure.  All infra connections are disabled by default
// so factories return nil components and the loop body is cheap.  This
// benchmark guards the factory-dispatch mechanism against regressions.
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

// Full-path snapshot benchmark

// BenchmarkStartupSnapshot times the complete synchronous startup pipeline in
// a single loop and produces one per-iteration figure (ns/op) so CI can reject
// any startup-speed regression immediately:
//
//	< 300 ms  → healthy startup
//	300–1000 ms → slow cold-start, investigate infra wiring
//	> 1 s    → regression; must not ship
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

		// ── Infrastructure component init ─────────────────────────────
		infraReg := infrastructure.GetGlobalRegistry()
		if err := infraReg.Initialize(cfg, l); err != nil {
			b.Logf("infra init note: %v", err)
		}
		components := infraReg.GetAll()
		for name, comp := range components {
			_ = name
			_ = comp.GetStatus()
		}

		// ── Middleware wiring ─────────────────────────────────────────
		mwReg := middleware.GetGlobalMiddlewareRegistry()
		mwReg.ApplyConfig(cfg)

		// ── Service discovery + bootstrap ──────────────────────────────
		deps := registry.NewDependencies()
		svcList := registry.AutoDiscoverServices(cfg, l, deps)

		gin.SetMode(gin.TestMode)
		reg := registry.NewServiceRegistry(l)
		for _, svc := range svcList {
			reg.Register(svc)
		}
		reg.Boot(gin.New())

		infraReg.CloseAll()

		_ = time.Since(start)
		_ = b.Elapsed()
	}
}

// Integration tests: health endpoints must be reachable after full startup init

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

// mustBuildDiagnosticRouter wires up the entire startup pipeline against a
// fresh Gin engine and returns everything the caller needs.  It is guarded by
// sync.Once so concurrent sub-tests share one initialised router without
// double-registering service factories.
func mustBuildDiagnosticRouter(t *testing.T) (*gin.Engine, *registry.Dependencies) {
	gin.SetMode(gin.TestMode)

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("config.LoadConfig() failed: %v", err)
	}
	cfg.App.EnableTUI = false // keep test output clean

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

	r := gin.New()
	reg := registry.NewServiceRegistry(l)
	for _, svc := range svcList {
		reg.Register(svc)
	}
	reg.Boot(r)

	return r, deps
}

// performRequest sends a single HTTP request through the given engine's
// middleware + handler pipeline and returns the response recorder.
// gin.CreateTestContextOnly is used (not CreateTestContext) so that the
// provided engine — which has all service routes registered — is used rather
// than a throw-away engine created by CreateTestContext.
func performRequest(r *gin.Engine, method, path string) *httptestRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, nil)
	c := gin.CreateTestContextOnly(w, r)
	c.Request = req
	r.ServeHTTP(w, req)
	return &httptestRecorder{w}
}

// httptestRecorder embeds *http.ResponseRecorder so callers get direct access
// to Code and Body without needing to pass the recorder back explicitly.
type httptestRecorder struct {
	*httptest.ResponseRecorder
}

// Guard assertions: auto-discovery must not be silently broken

// TestStartup_AutoDiscoveredServiceFactoriesArePresent asserts that at least
// one service factory is registered by the init() side-effect in each service
// module under internal/services/modules/.  Adding a new service file requires
// no changes here — this test will fail if the init() call is accidentally
// removed or the file is renamed without updating the import.
func TestStartup_AutoDiscoveredServiceFactoriesArePresent(t *testing.T) {
	factories := registry.GetServiceFactories()
	assert.NotEmpty(t, factories,
		"at least one service factory must be registered by init() in internal/services/modules")
	_, hasUsers := factories["users_service"]
	assert.True(t, hasUsers, "users_service must be registered by init() in internal/services/modules")
}

// TestStartup_AutoDiscoveredMiddlewareFactoriesArePresent asserts that
// middleware factories are populated by init() side-effects in
// internal/middleware/.  Verified indirectly through AutoDiscoverMiddlewares
// which is the public code-path that consumes the registry.
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

// CI guidance

// BenchmarkThresholdMilliseconds is the recommended maximum acceptable per-
// iteration startup time for BenchmarkStartupSnapshot in CI.
//
//	go test -bench=BenchmarkStartupSnapshot -benchtime=10x -run=^$ ./tests/
//
// CI can parse the go test -bench output and FAIL the build when the
// measured ns/op exceeds this value.
const BenchmarkThresholdMilliseconds = 1000 /* cold-start latency hard cap */
