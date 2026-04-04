package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"stackyrd/config"
	"stackyrd/pkg/utils"
)

// ConfigManager handles all configuration loading and validation
type ConfigManager struct {
	configURL string
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configURL string) *ConfigManager {
	return &ConfigManager{
		configURL: configURL,
	}
}

// LoadConfig loads configuration from local file or URL
func (cm *ConfigManager) LoadConfig() (*config.Config, error) {
	if cm.configURL != "" {
		return cm.loadConfigFromURL(cm.configURL)
	}
	return cm.loadConfigFromFile()
}

// loadConfigFromURL loads configuration from a URL
func (cm *ConfigManager) loadConfigFromURL(configURL string) (*config.Config, error) {
	fmt.Printf("Loading config from URL: %s\n", configURL)

	// Validate URL format
	if _, err := url.ParseRequestURI(configURL); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrInvalidConfigURLFormat, err)
	}

	// Load config from URL
	if err := utils.LoadConfigFromURL(configURL); err != nil {
		return nil, fmt.Errorf("failed to load config from URL: %w", err)
	}

	// Parse the loaded configuration
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
	// Check if web folder exists, if not, disable web monitoring
	if _, err := os.Stat(WebFolderPath); os.IsNotExist(err) {
		fmt.Printf("%s %s%s\n", ColorYellow, ErrWebFolderNotFound, ColorReset)
		cfg.Monitoring.Enabled = false
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

	bannerPath := cfg.App.BannerPath
	if !filepath.IsAbs(bannerPath) {
		bannerPath = filepath.Join(".", bannerPath)
	}

	banner, err := os.ReadFile(bannerPath)
	if err != nil {
		// Return empty string if banner file doesn't exist or can't be read
		return "", nil
	}

	return string(banner), nil
}

// GetServiceConfigs returns a unified list of all service configurations
func (cm *ConfigManager) GetServiceConfigs(cfg *config.Config) []ServiceConfig {
	return []ServiceConfig{
		{Name: ServiceGrafanaName, Enabled: cfg.Grafana.Enabled},
		{Name: ServiceMinIOName, Enabled: cfg.Monitoring.MinIO.Enabled},
		{Name: ServiceRedisCacheName, Enabled: cfg.Redis.Enabled},
		{Name: ServiceKafkaName, Enabled: cfg.Kafka.Enabled},
		{Name: ServicePostgreSQLName, Enabled: cfg.Postgres.Enabled},
		{Name: ServiceMongoDBName, Enabled: cfg.Mongo.Enabled},
		{Name: ServiceCronName, Enabled: cfg.Cron.Enabled},
		{Name: ServiceExternalName, Enabled: len(cfg.Monitoring.External.Services) > 0},
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
	initQueue = append(initQueue, ServiceInit{Name: ServiceMonitoringName, Enabled: cfg.Monitoring.Enabled, InitFunc: nil})

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
		Monitoring: config.MonitoringConfig{
			Port: DefaultMonitoringPort,
		},
	}
}
