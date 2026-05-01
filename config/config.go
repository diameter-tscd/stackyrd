package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

// setupViperDefaults configures viper with default values
func setupViperDefaults() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("app.name", "Golang App")
	viper.SetDefault("app.env", "development")
	viper.SetDefault("app.banner_path", "banner.txt")
	viper.SetDefault("app.startup_delay", 15)   // 15 seconds default
	viper.SetDefault("app.quiet_startup", true) // clean console by default
	viper.SetDefault("app.enable_tui", false)   // TUI enabled by default
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("auth.type", "none")
	// Services config uses a dynamic map - no hardcoded defaults needed
	// Services default to enabled if not specified (see ServicesConfig.IsEnabled)

	viper.SetDefault("redis.enabled", false)
	viper.SetDefault("kafka.enabled", false)
	viper.SetDefault("postgres.enabled", false)
	viper.SetDefault("mongo.enabled", false)
	viper.SetDefault("swagger.enabled", true)
	viper.SetDefault("swagger.base_path", "/swagger")
}

type Config struct {
	App                 AppConfig           `mapstructure:"app"`
	Server              ServerConfig        `mapstructure:"server"`
	Services            ServicesConfig      `mapstructure:"services"`
	Auth                AuthConfig          `mapstructure:"auth"`
	Swagger             SwaggerConfig       `mapstructure:"swagger"`
	Redis               RedisConfig         `mapstructure:"redis"`
	Kafka               KafkaConfig         `mapstructure:"kafka"`
	Postgres            PostgresConfig      `mapstructure:"postgres"`
	PostgresMultiConfig PostgresMultiConfig `mapstructure:"postgres"`
	Mongo               MongoConfig         `mapstructure:"mongo"`
	MongoMultiConfig    MongoMultiConfig    `mapstructure:"mongo"`
	Grafana             GrafanaConfig       `mapstructure:"grafana"`
	Monitoring          MonitoringConfig    `mapstructure:"monitoring"`
	Cron                CronConfig          `mapstructure:"cron"`
	MinIO               MinIOConfig         `mapstructure:"minio"`
	Encryption          EncryptionConfig    `mapstructure:"encryption"`
	Plugins             PluginConfig        `mapstructure:"plugins"`
}

type MonitoringConfig struct {
	UpdatePeriod   time.Duration  `mapstructure:"update_period"`
	Enabled        bool           `mapstructure:"enabled"`
	UploadDir      string         `mapstructure:"upload_dir"`
	Password       string         `mapstructure:"password"`
	Title          string         `mapstructure:"title"`
	Subtitle       string         `mapstructure:"subtitle"`
	MaxPhotoSizeMB int            `mapstructure:"max_photo_size_mb"`
	MinIO          MinIOConfig    `mapstructure:"minio"`
	External       ExternalConfig `mapstructure:"external"`
	ObfuscateAPI   bool           `mapstructure:"obfuscate_api"`
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
	StartupDelay int    `mapstructure:"startup_delay"` // seconds to show TUI boot screen (0 to skip)
	QuietStartup bool   `mapstructure:"quiet_startup"` // suppress console logs at startup (TUI only)
	EnableTUI    bool   `mapstructure:"enable_tui"`    // enable fancy TUI mode (false = traditional console)
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
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
	Type   string `mapstructure:"type"` // e.g., "jwt", "apikey", "none"
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

type MongoConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type MongoConnectionConfig struct {
	Name     string `mapstructure:"name"`
	Enabled  bool   `mapstructure:"enabled"`
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type MongoMultiConfig struct {
	Enabled     bool                    `mapstructure:"enabled"`
	Connections []MongoConnectionConfig `mapstructure:"connections"`
}

type GrafanaConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	URL      string `mapstructure:"url"`
	APIKey   string `mapstructure:"api_key"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type PluginConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Dir     string `mapstructure:"dir"` // directory containing .so plugin files
}

// LoadConfig loads configuration from local file or URL
func LoadConfig() (*Config, error) {
	return LoadConfigWithURL("")
}

// LoadConfigWithURL loads configuration from URL (if provided) or local file
func LoadConfigWithURL(configURL string) (*Config, error) {
	setupViperDefaults()

	if configURL != "" {
		// Load from URL - viper should already have the config loaded from URL
		// by the parameter parsing in main.go
	} else {
		// Standard local file loading
		viper.SetConfigName("config") // name of config file (without extension)
		viper.SetConfigType("yaml")   // REQUIRED if the config file does not have the extension in the name
		viper.AddConfigPath(".")      // optionally look for config in the working directory
		viper.AddConfigPath("./config")

		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, err
			}
			// Config file not found; ignore error if desired or return
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Handle PostgreSQL configuration - both single and multi-connection
	// Check if multi-connection format is provided (has connections array)
	if len(cfg.PostgresMultiConfig.Connections) > 0 {
		// Multi-connection format is provided, use it
		cfg.PostgresMultiConfig.Enabled = true
	} else if cfg.Postgres.Enabled {
		// Single connection format provided, convert to multi-connection format
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

	// Handle MongoDB configuration - both single and multi-connection
	// Check if multi-connection format is provided (has connections array)
	if len(cfg.MongoMultiConfig.Connections) > 0 {
		// Multi-connection format is provided, use it
		cfg.MongoMultiConfig.Enabled = true
	} else if cfg.Mongo.Enabled {
		// Single connection format provided, convert to multi-connection format
		cfg.MongoMultiConfig = MongoMultiConfig{
			Enabled: true,
			Connections: []MongoConnectionConfig{
				{
					Name:     "default",
					Enabled:  true,
					URI:      cfg.Mongo.URI,
					Database: cfg.Mongo.Database,
				},
			},
		}
	}

	return &cfg, nil
}
