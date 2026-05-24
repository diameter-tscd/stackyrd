package main_test

import (
	"context"
	_ "stackyrd/internal/services/modules" // nolint:blank-imports triggers init() registrations
	"io"
	"sync"
	"testing"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Config tests
// ---------------------------------------------------------------------------

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
	assert.True(t, cfg.Services.IsEnabled("users_service"))
}

func TestConfig_ServicesDisabled(t *testing.T) {
	c := config.ServicesConfig{"users_service": false}
	assert.False(t, c.IsEnabled("users_service"))
	assert.True(t, c.IsEnabled("unset_service")) // defaults to true
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
	assert.True(t, c.IsEnabled("unset_middleware")) // defaults to true
}

func TestConfig_InfraDefaults(t *testing.T) {
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)
	assert.False(t, cfg.Redis.Enabled)
	assert.False(t, cfg.Kafka.Enabled)
	assert.False(t, cfg.Mongo.Enabled)
	assert.False(t, cfg.Cron.Enabled)
}

// ---------------------------------------------------------------------------
// PaginationRequest tests
// PaginationRequest is defined in pkg/response — zero deps, pure value-object.
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Registry tests
// ---------------------------------------------------------------------------

func TestRegistry_RegisterAndRetrieve(t *testing.T) {
	l := logger.New(false, nil)
	cfg, err := config.LoadConfig()
	assert.NoError(t, err)

	reg := registry.NewServiceRegistry(l)
	err = reg.RegisterServiceWithDependencies(cfg, l, registry.NewDependencies(), "users_service")
	assert.NoError(t, err)
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

func TestRegistry_EmptyServiceList(t *testing.T) {
	reg := registry.NewServiceRegistry(logger.New(false, nil))
	assert.Empty(t, reg.GetServices())
}

func TestRegistry_KnownFactoriesExist(t *testing.T) {
	// AutoDiscoverServices populates factories via init() in the module packages.
	// getServiceFactories returns the snapshot; it must contain at least the
	// names auto-registered by the module init() calls above.
	factories := registry.GetServiceFactories()
	assert.NotEmpty(t, factories,
		"service factories should be populated by module init() imports")
	_, hasUsers := factories["users_service"]
	assert.True(t, hasUsers, "users_service must be auto-registered via init()")
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

// ---------------------------------------------------------------------------
// Dependencies container tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Self-contained mock helpers
// (avoid relying on pkg/testing to prevent import-name shadow issues)
// ---------------------------------------------------------------------------

type simpleMockConfig struct {
	services map[string]bool
}

func (m *simpleMockConfig) IsServiceEnabled(name string) bool {
	if v, ok := m.services[name]; ok { return v }
	return false
}
func (m *simpleMockConfig) SetServiceEnabled(name string, enabled bool) {
	if m.services == nil { m.services = make(map[string]bool) }
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

func (m *simpleMockLogger) Debug(msg string, args ...interface{}) { m.mu.Lock(); m.logs = append(m.logs, mockLogEntry{"DEBUG", msg, args}); m.mu.Unlock() }
func (m *simpleMockLogger) Info(msg string, args ...interface{})  { m.mu.Lock(); m.logs = append(m.logs, mockLogEntry{"INFO", msg, args}); m.mu.Unlock() }
func (m *simpleMockLogger) Warn(msg string, args ...interface{})  { m.mu.Lock(); m.logs = append(m.logs, mockLogEntry{"WARN", msg, args}); m.mu.Unlock() }
func (m *simpleMockLogger) Error(msg string, args ...interface{}) { m.mu.Lock(); m.logs = append(m.logs, mockLogEntry{"ERROR", msg, args}); m.mu.Unlock() }
func (m *simpleMockLogger) Fatal(msg string, args ...interface{}) { m.mu.Lock(); m.logs = append(m.logs, mockLogEntry{"FATAL", msg, args}); m.mu.Unlock() }
func (m *simpleMockLogger) GetLogs() []mockLogEntry {
	m.mu.RLock(); out := make([]mockLogEntry, len(m.logs)); copy(out, m.logs); m.mu.RUnlock()
	return out
}
func (m *simpleMockLogger) Clear()                       { m.mu.Lock(); m.logs = m.logs[:0]; m.mu.Unlock() }

type simpleMockRedisManager struct {
	mu      sync.RWMutex
	storage map[string]interface{}
}

func (m *simpleMockRedisManager) Set(_ context.Context, key string, value interface{}, _ time.Duration) error {
	m.mu.Lock()
	if m.storage == nil { m.storage = make(map[string]interface{}) }
	m.storage[key] = value
	m.mu.Unlock()
	return nil
}
func (m *simpleMockRedisManager) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if v, ok := m.storage[key]; ok {
		if s, ok := v.(string); ok { return s, nil }
	}
	return "", nil
}
func (m *simpleMockRedisManager) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	if m.storage != nil { delete(m.storage, key) }
	m.mu.Unlock()
	return nil
}
func (*simpleMockRedisManager) Close() error { return nil }

type simpleMockKafkaManager struct {
	mu       sync.RWMutex
	messages []mockKafkaMessage
}
type mockKafkaMessage struct {
	Topic string
	Value []byte
}

func (m *simpleMockKafkaManager) Publish(topic string, value []byte) error {
	m.mu.Lock(); m.messages = append(m.messages, mockKafkaMessage{topic, value}); m.mu.Unlock(); return nil
}
func (m *simpleMockKafkaManager) GetMessages() []mockKafkaMessage {
	m.mu.RLock(); out := make([]mockKafkaMessage, len(m.messages)); copy(out, m.messages); m.mu.RUnlock()
	return out
}
func (*simpleMockKafkaManager) Close() error { return nil }

type simpleMockCronManager struct {
	mu   sync.RWMutex
	jobs map[string]func()
}

func (m *simpleMockCronManager) AddJob(name string, _ string, cmd func()) error {
	m.mu.Lock()
	if m.jobs == nil { m.jobs = make(map[string]func()) }
	m.jobs[name] = cmd
	m.mu.Unlock()
	return nil
}
func (m *simpleMockCronManager) RemoveJob(name string) error {
	m.mu.Lock()
	if m.jobs != nil { delete(m.jobs, name) }
	m.mu.Unlock()
	return nil
}
func (*simpleMockCronManager) Close() error { return nil }

type simpleMockFileReader struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func (m *simpleMockFileReader) Read(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if d, ok := m.files[path]; ok {
		out := make([]byte, len(d)); copy(out, d); return out, nil
	}
	return nil, io.EOF
}
func (m *simpleMockFileReader) AddFile(path string, content []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.files == nil { m.files = make(map[string][]byte) }
	d := make([]byte, len(content)); copy(d, content); m.files[path] = d
}

// ---------------------------------------------------------------------------
// Twig mocks
// ---------------------------------------------------------------------------

type simpleMockPostgresManager struct{}
func (*simpleMockPostgresManager) Close() error { return nil }

type simpleMockMongoManager struct{}
func (*simpleMockMongoManager) Close() error { return nil }

// ---------------------------------------------------------------------------
// Assertions against self-contained mocks
// ---------------------------------------------------------------------------

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
	ml.Debug("d", "k", 1); ml.Info("i", "k", 2); ml.Warn("w"); ml.Error("e"); ml.Fatal("f")
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
	ml.Info("msg"); ml.Clear()
	assert.Empty(t, ml.GetLogs())
}

func TestMockRedisManager_SetGetDelete(t *testing.T) {
	rm := &simpleMockRedisManager{}
	assert.NoError(t, rm.Set(t.Context(), "k1", "v1", 0))
	v, err := rm.Get(t.Context(), "k1")
	assert.NoError(t, err); assert.Equal(t, "v1", v)
	assert.NoError(t, rm.Delete(t.Context(), "k1"))
	v, _ = rm.Get(t.Context(), "k1"); assert.Empty(t, v)
}

func TestMockKafkaManager_PublishGetMessages(t *testing.T) {
	km := &simpleMockKafkaManager{}
	assert.NoError(t, km.Publish("topic1", []byte("hello")))
	msgs := km.GetMessages()
	assert.Len(t, msgs, 1)
	assert.Equal(t, "topic1", msgs[0].Topic)
	assert.Equal(t, []byte("hello"), msgs[0].Value)
}

func TestMockCronManager_AddRemoveJob(t *testing.T) {
	cm := &simpleMockCronManager{}
	assert.NoError(t, cm.AddJob("j", "* * * * *", func() {}))
	assert.NoError(t, cm.RemoveJob("j"))
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

func TestMockMongoManager_Close(t *testing.T) {
	assert.NoError(t, (&simpleMockMongoManager{}).Close())
}
