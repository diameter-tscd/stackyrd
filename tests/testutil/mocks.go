package testutil

import (
	"context"
	"io"
	"sync"
	"time"
)

// MockService implements a mock service for testing
type MockService struct {
	name      string
	enabled   bool
	endpoints []string
}

// NewMockService creates a new mock service
func NewMockService(name string, enabled bool, endpoints []string) *MockService {
	return &MockService{
		name:      name,
		enabled:   enabled,
		endpoints: endpoints,
	}
}

func (m *MockService) Name() string        { return m.name }
func (m *MockService) WireName() string    { return "mock-" + m.name }
func (m *MockService) Enabled() bool       { return m.enabled }
func (m *MockService) Endpoints() []string { return m.endpoints }
func (m *MockService) Get() interface{}    { return m }

func (m *MockService) RegisterRoutes(g interface{}) {}

// MockLogger implements a mock logger for testing
type MockLogger struct {
	mu   sync.RWMutex
	logs []LogEntry
}

// LogEntry represents a single log entry
type LogEntry struct {
	Level   string
	Message string
	Args    []interface{}
}

// NewMockLogger creates a new mock logger
func NewMockLogger() *MockLogger {
	return &MockLogger{
		logs: make([]LogEntry, 0),
	}
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "DEBUG", Message: msg, Args: args})
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "INFO", Message: msg, Args: args})
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "WARN", Message: msg, Args: args})
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "ERROR", Message: msg, Args: args})
}

func (m *MockLogger) Fatal(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "FATAL", Message: msg, Args: args})
}

func (m *MockLogger) GetLogs() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]LogEntry, len(m.logs))
	copy(result, m.logs)
	return result
}

func (m *MockLogger) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = m.logs[:0]
}

// MockRedisManager implements a mock Redis manager for testing
type MockRedisManager struct {
	mu      sync.RWMutex
	storage map[string]interface{}
}

// NewMockRedisManager creates a new mock Redis manager
func NewMockRedisManager() *MockRedisManager {
	return &MockRedisManager{
		storage: make(map[string]interface{}),
	}
}

func (m *MockRedisManager) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storage[key] = value
	return nil
}

func (m *MockRedisManager) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if val, ok := m.storage[key]; ok {
		if str, ok := val.(string); ok {
			return str, nil
		}
	}
	return "", nil
}

func (m *MockRedisManager) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.storage, key)
	return nil
}

func (m *MockRedisManager) Close() error {
	return nil
}

// MockPostgresManager implements a mock PostgreSQL manager for testing
type MockPostgresManager struct {
	storage map[string]interface{}
}

// NewMockPostgresManager creates a new mock PostgreSQL manager
func NewMockPostgresManager() *MockPostgresManager {
	return &MockPostgresManager{
		storage: make(map[string]interface{}),
	}
}

func (m *MockPostgresManager) Close() error {
	return nil
}

// MockMongoManager implements a mock MongoDB manager for testing
type MockMongoManager struct {
	storage map[string]interface{}
}

// NewMockMongoManager creates a new mock MongoDB manager
func NewMockMongoManager() *MockMongoManager {
	return &MockMongoManager{
		storage: make(map[string]interface{}),
	}
}

func (m *MockMongoManager) Close() error {
	return nil
}

// MockKafkaManager implements a mock Kafka manager for testing
type MockKafkaManager struct {
	mu       sync.RWMutex
	messages []MockMessage
}

// MockMessage represents a Kafka message
type MockMessage struct {
	Topic string
	Value []byte
}

// NewMockKafkaManager creates a new mock Kafka manager
func NewMockKafkaManager() *MockKafkaManager {
	return &MockKafkaManager{
		messages: make([]MockMessage, 0),
	}
}

func (m *MockKafkaManager) Publish(topic string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, MockMessage{Topic: topic, Value: value})
	return nil
}

func (m *MockKafkaManager) GetMessages() []MockMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]MockMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

func (m *MockKafkaManager) Close() error {
	return nil
}

// MockCronManager implements a mock Cron manager for testing
type MockCronManager struct {
	mu   sync.RWMutex
	jobs map[string]func()
}

// NewMockCronManager creates a new mock Cron manager
func NewMockCronManager() *MockCronManager {
	return &MockCronManager{
		jobs: make(map[string]func()),
	}
}

func (m *MockCronManager) AddJob(name string, schedule string, cmd func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[name] = cmd
	return nil
}

func (m *MockCronManager) RemoveJob(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, name)
	return nil
}

func (m *MockCronManager) Close() error {
	return nil
}

// MockFileReader implements a mock file reader for testing
type MockFileReader struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewMockFileReader creates a new mock file reader
func NewMockFileReader() *MockFileReader {
	return &MockFileReader{
		files: make(map[string][]byte),
	}
}

func (m *MockFileReader) Read(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, io.EOF
}

func (m *MockFileReader) AddFile(path string, content []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = content
}

// MockConfig implements a mock configuration for testing
type MockConfig struct {
	Services map[string]bool
	Redis    struct {
		Enabled bool
	}
	Postgres struct {
		Enabled bool
	}
	Mongo struct {
		Enabled bool
	}
	Kafka struct {
		Enabled bool
	}
	Cron struct {
		Enabled bool
	}
}

// NewMockConfig creates a new mock configuration
func NewMockConfig() *MockConfig {
	return &MockConfig{
		Services: make(map[string]bool),
	}
}

func (m *MockConfig) IsServiceEnabled(name string) bool {
	if enabled, ok := m.Services[name]; ok {
		return enabled
	}
	return false
}

func (m *MockConfig) SetServiceEnabled(name string, enabled bool) {
	m.Services[name] = enabled
}

// TestSuite provides a comprehensive test suite setup
type TestSuite struct {
	Logger     *MockLogger
	Redis      *MockRedisManager
	Postgres   *MockPostgresManager
	Mongo      *MockMongoManager
	Kafka      *MockKafkaManager
	Cron       *MockCronManager
	FileReader *MockFileReader
	Config     *MockConfig
}

// NewTestSuite creates a new test suite with all mocks
func NewTestSuite() *TestSuite {
	return &TestSuite{
		Logger:     NewMockLogger(),
		Redis:      NewMockRedisManager(),
		Postgres:   NewMockPostgresManager(),
		Mongo:      NewMockMongoManager(),
		Kafka:      NewMockKafkaManager(),
		Cron:       NewMockCronManager(),
		FileReader: NewMockFileReader(),
		Config:     NewMockConfig(),
	}
}

// Cleanup cleans up all test suite resources
func (ts *TestSuite) Cleanup() {
	ts.Logger.Clear()
}
