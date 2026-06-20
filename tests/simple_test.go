package main_test

import (
	"io"
	"sync"
	"testing"

	"stackyrd-nano/config"
	"stackyrd-nano/pkg/logger"
	"stackyrd-nano/pkg/registry"
	"stackyrd-nano/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestConfig_Defaults(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.App)
	assert.Equal(t, "development", cfg.App.Env)
	assert.False(t, cfg.App.Debug)
	assert.False(t, cfg.App.EnableTUI)
}

func TestConfig_AppStruct(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)
	assert.Equal(t, "development", cfg.App.Env)
	assert.Equal(t, "Golang App", cfg.App.Name)
	assert.Equal(t, "8080", cfg.Server.Port)
}

func TestConfig_ServerPort(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)
	assert.NotEmpty(t, cfg.Server.Port, "server port should not be empty")
}

func TestConfig_ServicesIsEnabled(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)
	assert.True(t, cfg.Services.IsEnabled("any_service"))
}

func TestConfig_ServicesDisabled(t *testing.T) {
	c := config.ServicesConfig{"users_service": false}
	assert.False(t, c.IsEnabled("users_service"))
	assert.True(t, c.IsEnabled("unset_service"))
}

func TestConfig_AuthTypeDefault(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)
	assert.Equal(t, "none", cfg.Auth.Type)
}

func TestConfig_MiddlewareDefaults(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)
	assert.True(t, cfg.Middleware.IsEnabled("any_middleware"))
	assert.True(t, cfg.Middleware.IsEnabled("cors"))
}

func TestConfig_MiddlewareDisabled(t *testing.T) {
	c := config.MiddlewareConfig{"ratelimit": false}
	assert.False(t, c.IsEnabled("ratelimit"))
	assert.True(t, c.IsEnabled("unset_middleware"))
}

func TestPaginationRequest_Defaults(t *testing.T) {
	pr := response.PaginationRequest{}
	assert.Equal(t, 1, pr.GetPage())
	assert.Equal(t, 10, pr.GetPerPage())
	assert.Equal(t, 0, pr.GetOffset())
	assert.Equal(t, "desc", pr.GetOrder())
}

func TestPaginationRequest_InvalidValues(t *testing.T) {
	pr := response.PaginationRequest{Page: 0, PerPage: 0, Order: ""}
	assert.Equal(t, 1, pr.GetPage())
	assert.Equal(t, 10, pr.GetPerPage())
	assert.Equal(t, "desc", pr.GetOrder())
}

func TestPaginationRequest_Clamping(t *testing.T) {
	pr := response.PaginationRequest{Page: 999, PerPage: 10_000}
	assert.Equal(t, 999, pr.GetPage())
	assert.Equal(t, 100, pr.GetPerPage())
}

func TestPaginationRequest_Offset(t *testing.T) {
	cases := []struct{ page, perPage, want int }{
		{1, 10, 0},
		{2, 10, 10},
		{3, 10, 20},
		{1, 50, 0},
		{2, 50, 50},
	}
	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			pr := response.PaginationRequest{Page: c.page, PerPage: c.perPage}
			assert.Equal(t, c.want, pr.GetOffset())
		})
	}
}

func TestRegistry_EmptyServiceList(t *testing.T) {
	reg := registry.NewServiceRegistry(logger.New(false, nil))
	assert.Empty(t, reg.GetServices())
}

func TestRegistry_RegisterMissingFactory(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)
	reg := registry.NewServiceRegistry(logger.New(false, nil))

	err = reg.RegisterServiceWithDependencies(cfg, logger.New(false, nil),
		registry.NewDependencies(), "nonexistent_service")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service factory not found")
}

func TestRegistry_ServiceDiscoveredEmpty(t *testing.T) {
	assert.Nil(t, registry.GetService("nonexistent-service"))
}

func TestRegistry_RegisterNilFactory(t *testing.T) {
	registry.RegisterService("nil_factory_test", nil)
	_, exists := registry.GetServiceFactories()["nil_factory_test"]
	assert.False(t, exists, "nil factory must not be stored")
}

func TestRegistry_BootEmpty(t *testing.T) {
	reg := registry.NewServiceRegistry(logger.New(false, nil))
	gin.SetMode(gin.TestMode)
	assert.NotPanics(t, func() { reg.Boot(gin.New()) })
}

func TestDependencies_SetGet(t *testing.T) {
	deps := registry.NewDependencies()
	deps.Set("key", "val")
	v, ok := deps.Get("key")
	assert.True(t, ok)
	assert.Equal(t, "val", v)
}

func TestDependencies_GetAll(t *testing.T) {
	deps := registry.NewDependencies()
	deps.Set("a", 1)
	deps.Set("b", 2)
	out := deps.GetAll()
	assert.Len(t, out, 2)
	assert.Equal(t, 1, out["a"])
	assert.Equal(t, 2, out["b"])
}

func TestDependencies_MissingKey(t *testing.T) {
	_, ok := registry.NewDependencies().Get("missing")
	assert.False(t, ok)
}

type simpleMockConfig struct {
	services map[string]bool
}

func (m *simpleMockConfig) IsServiceEnabled(name string) bool {
	if v, ok := m.services[name]; ok {
		return v
	}
	return false
}
func (m *simpleMockConfig) SetServiceEnabled(name string, enabled bool) {
	if m.services == nil {
		m.services = make(map[string]bool)
	}
	m.services[name] = enabled
}

type simpleMockLogger struct {
	mu   sync.RWMutex
	logs []mockLogEntry
}
type mockLogEntry struct {
	Level   string
	Message string
	Args    []interface{}
}

func (m *simpleMockLogger) Debug(msg string, args ...interface{}) {
	m.mu.Lock()
	m.logs = append(m.logs, mockLogEntry{"DEBUG", msg, args})
	m.mu.Unlock()
}
func (m *simpleMockLogger) Info(msg string, args ...interface{}) {
	m.mu.Lock()
	m.logs = append(m.logs, mockLogEntry{"INFO", msg, args})
	m.mu.Unlock()
}
func (m *simpleMockLogger) Warn(msg string, args ...interface{}) {
	m.mu.Lock()
	m.logs = append(m.logs, mockLogEntry{"WARN", msg, args})
	m.mu.Unlock()
}
func (m *simpleMockLogger) Error(msg string, args ...interface{}) {
	m.mu.Lock()
	m.logs = append(m.logs, mockLogEntry{"ERROR", msg, args})
	m.mu.Unlock()
}
func (m *simpleMockLogger) Fatal(msg string, args ...interface{}) {
	m.mu.Lock()
	m.logs = append(m.logs, mockLogEntry{"FATAL", msg, args})
	m.mu.Unlock()
}
func (m *simpleMockLogger) GetLogs() []mockLogEntry {
	m.mu.RLock()
	out := make([]mockLogEntry, len(m.logs))
	copy(out, m.logs)
	m.mu.RUnlock()
	return out
}
func (m *simpleMockLogger) Clear() { m.mu.Lock(); m.logs = m.logs[:0]; m.mu.Unlock() }

type simpleMockFileReader struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func (m *simpleMockFileReader) Read(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d, ok := m.files[path]; ok {
		out := make([]byte, len(d))
		copy(out, d)
		return out, nil
	}
	return nil, io.EOF
}
func (m *simpleMockFileReader) AddFile(path string, content []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil {
		m.files = make(map[string][]byte)
	}
	d := make([]byte, len(content))
	copy(d, content)
	m.files[path] = d
}

type simpleMockPostgresManager struct{}

func (*simpleMockPostgresManager) Close() error { return nil }

func TestMockConfig_Defaults(t *testing.T) {
	mc := &simpleMockConfig{}
	assert.False(t, mc.IsServiceEnabled("anything"))
}

func TestMockConfig_Toggle(t *testing.T) {
	mc := &simpleMockConfig{}
	mc.SetServiceEnabled("s", true)
	assert.True(t, mc.IsServiceEnabled("s"))
	mc.SetServiceEnabled("s", false)
	assert.False(t, mc.IsServiceEnabled("s"))
}

func TestMockLogger_LogLevels(t *testing.T) {
	ml := &simpleMockLogger{}
	ml.Debug("d", "k", 1)
	ml.Info("i", "k", 2)
	ml.Warn("w")
	ml.Error("e")
	ml.Fatal("f")
	logs := ml.GetLogs()
	assert.Len(t, logs, 5)
	assert.Equal(t, "DEBUG", logs[0].Level)
	assert.Equal(t, "INFO", logs[1].Level)
	assert.Equal(t, "WARN", logs[2].Level)
	assert.Equal(t, "ERROR", logs[3].Level)
	assert.Equal(t, "FATAL", logs[4].Level)
}

func TestMockLogger_Clear(t *testing.T) {
	ml := &simpleMockLogger{}
	ml.Info("msg")
	ml.Clear()
	assert.Empty(t, ml.GetLogs())
}

func TestMockFileReader_ReadAddFile(t *testing.T) {
	fr := &simpleMockFileReader{}
	_, err := fr.Read("missing.txt")
	assert.ErrorIs(t, err, io.EOF)
	fr.AddFile("hello.txt", []byte("hello world"))
	data, err := fr.Read("hello.txt")
	assert.NoError(t, err)
	assert.Equal(t, "hello world", string(data))
}

func TestMockPostgresManager_Close(t *testing.T) {
	assert.NoError(t, (&simpleMockPostgresManager{}).Close())
}
