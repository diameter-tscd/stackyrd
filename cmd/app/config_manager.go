package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"stackyrd-nano/config"
	"stackyrd-nano/pkg/utils"
)

type ConfigManager struct {
	configURL string
}

func NewConfigManager(configURL string) *ConfigManager {
	return &ConfigManager{
		configURL: configURL,
	}
}

func (cm *ConfigManager) LoadConfig() (*config.Config, error) {
	if cm.configURL != "" {
		return cm.loadConfigFromURL(cm.configURL)
	}
	return cm.loadConfigFromFile()
}

func (cm *ConfigManager) loadConfigFromURL(configURL string) (*config.Config, error) {
	fmt.Printf("Loading config from URL: %s\n", configURL)

	if _, err := url.ParseRequestURI(configURL); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrInvalidConfigURLFormat, err)
	}

	if err := utils.LoadConfigFromURL(configURL); err != nil {
		return nil, fmt.Errorf("failed to load config from URL: %w", err)
	}

	cfg, err := config.LoadConfigWithURL(configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config from URL: %w", err)
	}

	return cfg, nil
}

func (cm *ConfigManager) loadConfigFromFile() (*config.Config, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

func (cm *ConfigManager) ValidateConfig(cfg *config.Config) error {
	if err := utils.CheckPortAvailability(cfg.Server.Port); err != nil {
		return fmt.Errorf("%s: %w", ErrPortError, err)
	}
	return nil
}

func (cm *ConfigManager) LoadBanner(cfg *config.Config) (string, error) {
	if cfg.App.BannerPath == "" {
		return "", nil
	}

	if embeddedBanner != "" {
		return embeddedBanner, nil
	}

	bannerPath := cfg.App.BannerPath
	if !filepath.IsAbs(bannerPath) {
		bannerPath = filepath.Join(".", bannerPath)
	}

	banner, err := os.ReadFile(bannerPath)
	if err != nil {
		return "", nil
	}

	return string(banner), nil
}

func (cm *ConfigManager) GetServiceConfigs(cfg *config.Config) []ServiceConfig {
	return []ServiceConfig{
		{Name: ServicePostgreSQLName, Enabled: cfg.Postgres.Enabled},
	}
}

func (cm *ConfigManager) CreateServiceQueue(cfg *config.Config) []ServiceInit {
	serviceConfigs := cm.GetServiceConfigs(cfg)

	initQueue := []ServiceInit{
		{Name: ServiceConfigName, Enabled: true, InitFunc: nil},
	}

	for _, svc := range serviceConfigs {
		initQueue = append(initQueue, ServiceInit{
			Name: svc.Name, Enabled: svc.Enabled, InitFunc: nil,
		})
	}

	initQueue = append(initQueue, ServiceInit{Name: ServiceMiddlewareName, Enabled: true, InitFunc: nil})

	for name, enabled := range cfg.Services {
		initQueue = append(initQueue, ServiceInit{Name: "Service: " + name, Enabled: enabled, InitFunc: nil})
	}

	initQueue = append(initQueue, ServiceInit{Name: ServiceMonitoringName, InitFunc: nil})

	return initQueue
}

func (cm *ConfigManager) ValidateStartupDelay(delay int) error {
	if delay < MinStartupDelay || delay > MaxStartupDelay {
		return fmt.Errorf("startup delay must be between %d and %d seconds", MinStartupDelay, MaxStartupDelay)
	}
	return nil
}

func (cm *ConfigManager) ValidatePort(port string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	return nil
}

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
