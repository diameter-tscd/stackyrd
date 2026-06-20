package config

import (
	"strings"

	"github.com/spf13/viper"
)

func setupViperDefaults() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("app.name", "Golang App")
	viper.SetDefault("app.env", "development")
	viper.SetDefault("app.banner_path", "banner.txt")
	viper.SetDefault("app.startup_delay", 15)
	viper.SetDefault("app.quiet_startup", true)
	viper.SetDefault("app.enable_tui", false)
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.services_endpoint", "/api/v1")
	viper.SetDefault("auth.type", "none")
	viper.SetDefault("postgres.enabled", false)
	viper.SetDefault("app.debug", false)
}

type Config struct {
	App                 AppConfig           `mapstructure:"app"`
	Server              ServerConfig        `mapstructure:"server"`
	Services            ServicesConfig      `mapstructure:"services"`
	Middleware          MiddlewareConfig    `mapstructure:"middleware"`
	Auth                AuthConfig          `mapstructure:"auth"`
	Postgres            PostgresConfig      `mapstructure:"postgres"`
	PostgresMultiConfig PostgresMultiConfig `mapstructure:"postgres"`
	Encryption          EncryptionConfig    `mapstructure:"encryption"`
}

type MiddlewareConfig map[string]bool

func (m MiddlewareConfig) IsEnabled(middlewareName string) bool {
	if enabled, exists := m[middlewareName]; exists {
		return enabled
	}
	return true
}

type ExternalConfig struct {
	Services []ExternalService `mapstructure:"services"`
}

type ExternalService struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

type EncryptionConfig struct {
	Enabled             bool   `mapstructure:"enabled"`
	Algorithm           string `mapstructure:"algorithm"`
	Key                 string `mapstructure:"key"`
	RotateKeys          bool   `mapstructure:"rotate_keys"`
	KeyRotationInterval string `mapstructure:"key_rotation_interval"`
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
}

type ServerConfig struct {
	Port             string `mapstructure:"port"`
	ServicesEndpoint string `mapstructure:"services_endpoint"`
}

type ServicesConfig map[string]bool

func (s ServicesConfig) IsEnabled(serviceName string) bool {
	if enabled, exists := s[serviceName]; exists {
		return enabled
	}
	return true
}

type AuthConfig struct {
	Type   string `mapstructure:"type"`
	Secret string `mapstructure:"secret"`
}

type PostgresConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
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

type PostgresMultiConfig struct {
	Enabled     bool                       `mapstructure:"enabled"`
	Connections []PostgresConnectionConfig `mapstructure:"connections"`
}

func LoadConfig() (*Config, error) {
	return LoadConfigWithURL("")
}

func LoadConfigWithURL(configURL string) (*Config, error) {
	setupViperDefaults()

	if configURL != "" {
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")

		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, err
			}
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if len(cfg.PostgresMultiConfig.Connections) > 0 {
		cfg.PostgresMultiConfig.Enabled = true
	} else if cfg.Postgres.Enabled {
		cfg.PostgresMultiConfig = PostgresMultiConfig{
			Enabled: true,
			Connections: []PostgresConnectionConfig{
				{
					Name:     "default",
					Enabled:  true,
					Host:     cfg.Postgres.Host,
					Port:     cfg.Postgres.Port,
					User:     cfg.Postgres.User,
					Password: cfg.Postgres.Password,
					DBName:   cfg.Postgres.DBName,
					SSLMode:  cfg.Postgres.SSLMode,
				},
			},
		}
	}

	return &cfg, nil
}
