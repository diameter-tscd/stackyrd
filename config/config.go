package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// cache control for config reloading
var (
	configCache     *Config
	configCacheMu   sync.RWMutex
	configCacheTime time.Time
	configCacheTTL  = 5 * time.Minute
)

func setupViperDefaults() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("app.theme", "default")

	viper.SetDefault("app.name", "Golang App")
	viper.SetDefault("app.env", "development")
	viper.SetDefault("app.banner_path", "banner.txt")
	viper.SetDefault("app.startup_delay", 15)
	viper.SetDefault("app.quiet_startup", true)
	viper.SetDefault("app.enable_tui", false)
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.services_endpoint", "/api/v1")
	viper.SetDefault("auth.type", "none")

	viper.SetDefault("redis.enabled", false)
	viper.SetDefault("kafka.enabled", false)
	viper.SetDefault("postgres.enabled", false)
	viper.SetDefault("mongo.enabled", false)
	viper.SetDefault("swagger.enabled", false)
	viper.SetDefault("app.debug", false)
	viper.SetDefault("swagger.base_path", "/swagger")
	viper.SetDefault("metrics.enabled", false)
	viper.SetDefault("metrics.path", "/metrics")
	viper.SetDefault("webhook.enabled", false)
	viper.SetDefault("webhook.timeout_seconds", 30)
	viper.SetDefault("webhook.max_retries", 3)
	viper.SetDefault("webhook.endpoint", "/api/v1/webhook")
}

type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Server     ServerConfig     `mapstructure:"server"`
	Services   ServicesConfig   `mapstructure:"services"`
	Middleware MiddlewareConfig `mapstructure:"middleware"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Swagger    SwaggerConfig    `mapstructure:"swagger"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Postgres   PostgresConfig   `mapstructure:"postgres"`
	Mongo      MongoConfig      `mapstructure:"mongo"`
	Webhook    WebhookConfig    `mapstructure:"webhook"`
	Metrics    MetricsConfig    `mapstructure:"metrics"`
	Grafana    GrafanaConfig    `mapstructure:"grafana"`
	Cron       CronConfig       `mapstructure:"cron"`
	MinIO      MinIOConfig      `mapstructure:"minio"`
	Encryption EncryptionConfig `mapstructure:"encryption"`
}

// MiddlewareConfig is a dynamic map of middleware names to their enabled status.
type MiddlewareConfig map[string]bool

// IsEnabled checks if a middleware is enabled. Returns true by default if not specified.
func (m MiddlewareConfig) IsEnabled(middlewareName string) bool {
	if enabled, exists := m[middlewareName]; exists {
		return enabled
	}
	return true // Default to enabled if not specified
}

type WebhookConfig struct {
	Enabled    bool              `mapstructure:"enabled"`
	URL        string            `mapstructure:"url"`
	Secret     string            `mapstructure:"secret"`
	Timeout    int               `mapstructure:"timeout_seconds"`
	MaxRetries int               `mapstructure:"max_retries"`
	Headers    map[string]string `mapstructure:"headers"`
	Endpoint   string            `mapstructure:"endpoint"`
}

type MinIOConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	UseSSL          bool   `mapstructure:"use_ssl"`
	BucketName      string `mapstructure:"bucket_name"`
}

type ExternalConfig struct {
	Services []ExternalService `mapstructure:"services"`
}

type ExternalService struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

type CronConfig struct {
	Enabled bool              `mapstructure:"enabled"`
	Jobs    map[string]string `mapstructure:"jobs"`
}

type EncryptionConfig struct {
	Enabled             bool   `mapstructure:"enabled"`
	Algorithm           string `mapstructure:"algorithm"`
	Key                 string `mapstructure:"key"`
	RotateKeys          bool   `mapstructure:"rotate_keys"`
	KeyRotationInterval string `mapstructure:"key_rotation_interval"`
}

type SwaggerConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BasePath string `mapstructure:"base_path"`
}

type AppConfig struct {
	Name         string `mapstructure:"name"`
	Version      string `mapstructure:"version"`
	Debug        bool   `mapstructure:"debug"`
	Env          string `mapstructure:"env"`
	BannerPath   string `mapstructure:"banner_path"`
	StartupDelay int    `mapstructure:"startup_delay"`
	QuietStartup bool   `mapstructure:"quiet_startup"`
	EnableTUI    bool   `mapstructure:"enable_tui"`
	Theme        string `mapstructure:"theme"`
}

type ServerConfig struct {
	Port             string `mapstructure:"port"`
	ServicesEndpoint string `mapstructure:"services_endpoint"`
}

// ServicesConfig is a dynamic map of service names to their enabled status.
type ServicesConfig map[string]bool

// IsEnabled checks if a service is enabled. Returns true by default if not specified.
func (s ServicesConfig) IsEnabled(serviceName string) bool {
	if enabled, exists := s[serviceName]; exists {
		return enabled
	}
	return true // Default to enabled if not specified
}

type AuthConfig struct {
	Type   string `mapstructure:"type"`
	Secret string `mapstructure:"secret"`
}

type RedisConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type KafkaConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
	GroupID string   `mapstructure:"group_id"`
}

type PostgresConnectionConfig struct {
	Name     string `mapstructure:"name"`
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type PostgresConfig struct {
	Enabled     bool                       `mapstructure:"enabled"`
	Connections []PostgresConnectionConfig `mapstructure:"connections"`
}

type MongoConnectionConfig struct {
	Name     string `mapstructure:"name"`
	Enabled  bool   `mapstructure:"enabled"`
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type MongoConfig struct {
	Enabled     bool                    `mapstructure:"enabled"`
	Connections []MongoConnectionConfig `mapstructure:"connections"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type GrafanaConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	URL      string `mapstructure:"url"`
	APIKey   string `mapstructure:"api_key"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// LoadConfig loads configuration from local file or URL
func LoadConfig() (*Config, error) {
	configCacheMu.RLock()
	if configCache != nil && time.Since(configCacheTime) < configCacheTTL {
		cached := configCache
		configCacheMu.RUnlock()
		return cached, nil
	}
	configCacheMu.RUnlock()
	return loadFromSource()
}

// ForceReloadConfig bypasses the cache and reloads from source
func ForceReloadConfig() (*Config, error) {
	return loadFromSource()
}

func loadFromSource() (*Config, error) {
	setupViperDefaults()

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	configCacheMu.Lock()
	configCache = &cfg
	configCacheTime = time.Now()
	configCacheMu.Unlock()

	return &cfg, nil
}

// LoadConfigWithURL loads configuration from URL (if provided) or local file
func LoadConfigWithURL(configURL string) (*Config, error) {
	if configURL != "" {
		return loadFromSource()
	}
	return LoadConfig()
}

// SaveTheme persists app.theme back to the local config file so a runtime
// theme change survives a restart. It edits only the theme line, preserving the
// rest of the file byte-for-byte. Remote-URL configs have no file to write.
func SaveTheme(name string) error {
	viper.Set("app.theme", name)

	path := viper.ConfigFileUsed()
	if path == "" {
		return errors.New("no local config file to persist to")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "theme:") {
			continue
		}
		indent := line[:len(line)-len(trimmed)]
		comment := ""
		if c := strings.Index(trimmed, "#"); c >= 0 {
			comment = " " + trimmed[c:]
			trimmed = trimmed[:c]
		}
		lines[i] = indent + "theme: " + fmt.Sprintf("%q", name) + comment
		replaced = true
		break
	}
	if !replaced {
		return errors.New("config has no app.theme key")
	}

	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), mode)
}
