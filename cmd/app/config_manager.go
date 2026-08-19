package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/utils"
	"strings"
)

// ConfigManager handles all configuration loading and validation
type ConfigManager struct {
	configURL string
	port      string
	env       string
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configURL string, overrides ...string) *ConfigManager {
	cm := &ConfigManager{
		configURL: configURL,
	}
	if len(overrides) > 0 {
		cm.port = overrides[0]
	}
	if len(overrides) > 1 {
		cm.env = overrides[1]
	}
	return cm
}

// LoadConfig loads configuration from local file or URL
func (cm *ConfigManager) LoadConfig() (*config.Config, error) {
	var (
		cfg *config.Config
		err error
	)
	if cm.configURL != "" {
		cfg, err = cm.loadConfigFromURL(cm.configURL)
	} else {
		cfg, err = cm.loadConfigFromFile()
	}
	if err != nil {
		return nil, err
	}

	// Apply CLI overrides on top of the loaded config.
	if cm.port != "" {
		cfg.Server.Port = cm.port
	}
	if cm.env != "" {
		cfg.App.Env = cm.env
	}
	return cfg, nil
}

// loadConfigFromURL loads configuration from a URL
func (cm *ConfigManager) loadConfigFromURL(configURL string) (*config.Config, error) {
	fmt.Printf("Loading config from URL: %s\n", configURL)

	// Validate URL format
	if _, err := url.ParseRequestURI(configURL); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrInvalidConfigURLFormat, err)
	}

	// Load and parse config from URL (LoadConfigWithURL owns the fetch + SSRF guard)
	cfg, err := config.LoadConfigWithURL(configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config from URL: %w", err)
	}

	return cfg, nil
}

// loadConfigFromFile loads configuration from local file
func (cm *ConfigManager) loadConfigFromFile() (*config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

// ValidateConfig validates the loaded configuration
func (cm *ConfigManager) ValidateConfig(cfg *config.Config) error {
	if cfg == nil || cfg.Server == (config.ServerConfig{}) {
		return fmt.Errorf("configuration is empty or invalid")
	}
	// Validate port availability
	if err := utils.CheckPortAvailability(cfg.Server.Port); err != nil {
		return fmt.Errorf("%s: %w", ErrPortError, err)
	}
	return nil
}

// LoadBanner loads banner text from file if configured
func (cm *ConfigManager) LoadBanner(cfg *config.Config) (string, error) {
	if cfg.App.BannerPath == "" {
		return "", nil
	}

	// Default banner: read from afero embedded filesystem
	if cfg.App.BannerPath == DefaultBannerPath {
		if infrastructure.Exists("banner") {
			data, err := infrastructure.Read("banner")
			if err == nil {
				return strings.ReplaceAll(string(data), "\r\n", "\n"), nil
			}
		}
	}

	// Custom path: read from OS filesystem
	bannerPath := cfg.App.BannerPath
	if !filepath.IsAbs(bannerPath) {
		bannerPath = filepath.Join(".", bannerPath)
	}
	// Confine relative banner paths to the working directory; reject traversal.
	bannerPath = filepath.Clean(bannerPath)
	if bannerPath == ".." || strings.HasPrefix(bannerPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("banner path escapes working directory")
	}

	banner, err := os.ReadFile(bannerPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warn: banner not found at %s: %v\n", bannerPath, err)
		return "", nil
	}

	return strings.ReplaceAll(string(banner), "\r\n", "\n"), nil
}

// GetServiceConfigs returns a unified list of all service configurations
func (cm *ConfigManager) GetServiceConfigs(cfg *config.Config) []ServiceConfig {
	return []ServiceConfig{
		{Name: ServiceGrafanaName, Enabled: cfg.Grafana.Enabled},
		{Name: ServiceRedisCacheName, Enabled: cfg.Redis.Enabled},
		{Name: ServiceKafkaName, Enabled: cfg.Kafka.Enabled},
		{Name: ServicePostgreSQLName, Enabled: cfg.Postgres.Enabled},
		{Name: ServiceMongoDBName, Enabled: cfg.Mongo.Enabled},
		{Name: ServiceCronName, Enabled: cfg.Cron.Enabled},
	}
}

// CreateServiceQueue creates the service initialization queue for TUI
func (cm *ConfigManager) CreateServiceQueue(cfg *config.Config) []ServiceInit {
	serviceConfigs := cm.GetServiceConfigs(cfg)

	initQueue := []ServiceInit{
		{Name: ServiceConfigName, Enabled: true, InitFunc: nil},
	}

	// Add infrastructure services
	for _, svc := range serviceConfigs {
		initQueue = append(initQueue, ServiceInit{
			Name: svc.Name, Enabled: svc.Enabled, InitFunc: nil,
		})
	}

	initQueue = append(initQueue, ServiceInit{Name: ServiceMiddlewareName, Enabled: true, InitFunc: nil})

	// Add application services
	for name, enabled := range cfg.Services {
		initQueue = append(initQueue, ServiceInit{Name: "Service: " + name, Enabled: enabled, InitFunc: nil})
	}

	// Add monitoring last
	initQueue = append(initQueue, ServiceInit{Name: ServiceMonitoringName, Enabled: true, InitFunc: nil})

	return initQueue
}

// ValidateStartupDelay validates the startup delay configuration
func (cm *ConfigManager) ValidateStartupDelay(delay int) error {
	if delay < MinStartupDelay || delay > MaxStartupDelay {
		return fmt.Errorf("startup delay must be between %d and %d seconds", MinStartupDelay, MaxStartupDelay)
	}
	return nil
}

// ValidatePort validates a port number
func (cm *ConfigManager) ValidatePort(port string) error {
	// Basic validation - port should be numeric and within valid range
	// This is a simple validation; more comprehensive validation could be added
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	return nil
}

// GetDefaultConfig returns a default configuration
func (cm *ConfigManager) GetDefaultConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Name:         DefaultAppName,
			Version:      DefaultVersion,
			Env:          DefaultEnv,
			BannerPath:   DefaultBannerPath,
			StartupDelay: DefaultStartupDelay,
		},
		Server: config.ServerConfig{
			Port: DefaultServerPort,
		},
	}
}
