package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"stackyard/config"
	"stackyard/internal/monitoring"
	"stackyard/internal/server"
	"stackyard/pkg/logger"
	"stackyard/pkg/tui"
	"stackyard/pkg/utils"
	"syscall"
	"time"

	_ "stackyard/internal/services/modules"
)

func main() {
	// Clear the terminal screen for a fresh start
	utils.ClearScreen()

	// Parse command line flags
	configURL := parseFlags()

	// Load configuration
	cfg := loadConfig(configURL)

	// Check if "web" folder exists, if not, disable web monitoring
	if _, err := os.Stat("web"); os.IsNotExist(err) {
		fmt.Println("\033[33m 'web' folder not found, disabling web monitoring\033[0m")
		cfg.Monitoring.Enabled = false
	}

	// Load banner text
	bannerText := loadBanner(cfg)

	// Check port availability
	if err := utils.CheckPortAvailability(cfg.Server.Port, cfg.Monitoring.Port, cfg.Monitoring.Enabled); err != nil {
		fmt.Printf("\033[31m Port Error: %s\033[0m\n", err.Error())
		fmt.Println("\033[33mPlease stop the conflicting service or change the port in config.yaml\033[0m")
		os.Exit(1)
	}

	// Initialize broadcaster for monitoring
	broadcaster := monitoring.NewLogBroadcaster()

	// Start application based on TUI mode
	if cfg.App.EnableTUI {
		runWithTUI(cfg, bannerText, broadcaster)
	} else {
		runWithConsole(cfg, bannerText, broadcaster)
	}
}

// runWithTUI runs the application with fancy TUI interface
func runWithTUI(cfg *config.Config, bannerText string, broadcaster *monitoring.LogBroadcaster) {
	// Configure monitoring port for TUI
	if !cfg.Monitoring.Enabled {
		cfg.Monitoring.Port = "disabled"
	}

	// Setup TUI configuration
	tuiConfig := tui.StartupConfig{
		AppName:     cfg.App.Name,
		AppVersion:  cfg.App.Version,
		Banner:      bannerText,
		Port:        cfg.Server.Port,
		MonitorPort: cfg.Monitoring.Port,
		Env:         cfg.App.Env,
		IdleSeconds: cfg.App.StartupDelay,
	}

	// Create service initialization queue
	initQueue := createServiceQueue(cfg)

	// Run the boot sequence TUI
	_, _ = tui.RunBootSequence(tuiConfig, initQueue)

	// Create and start Live TUI
	liveTUI := createLiveTUI(cfg, bannerText)
	liveTUI.Start()

	// Initialize logger with TUI output
	multiWriter := io.MultiWriter(liveTUI, broadcaster)
	l := logger.NewQuiet(cfg.App.Debug, multiWriter)

	// Add initial logs
	liveTUI.AddLog("info", "Server starting on port "+cfg.Server.Port)
	liveTUI.AddLog("info", "Environment: "+cfg.App.Env)

	// Start server
	srv := server.New(cfg, l, broadcaster)
	go func() {
		liveTUI.AddLog("info", "HTTP server listening...")
		if err := srv.Start(); err != nil {
			liveTUI.AddLog("fatal", "Server error: "+err.Error())
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)
	liveTUI.AddLog("info", "Server ready at http://localhost:"+cfg.Server.Port)
	if cfg.Monitoring.Enabled {
		liveTUI.AddLog("info", "Monitoring at http://localhost:"+cfg.Monitoring.Port)
	}

	// Handle shutdown
	handleShutdown(liveTUI, srv, l)
}

// exampleAferoUsage demonstrates how to use the Global Singleton Afero Manager
// This function is commented out as it's for demonstration purposes only
/*
func exampleAferoUsage() {
	fmt.Println("=== Global Singleton Afero Manager Example ===")

	// Mock alias configuration
	aliasMap := map[string]string{
		"config":  "all:config.yaml",
		"banner":  "all:banner.txt",
		"readme":  "all:README.md",
		"web-app": "all:web/monitoring/index.html",
	}

	// Initialize the Afero manager in development mode
	// Note: In a real application, you would use //go:embed directives
	// For this example, we'll just show the API usage
	fmt.Println("Initializing Afero Manager...")

	// In a real application, you would have:
	// //go:embed all:dist
	// var embedFS embed.FS
	// infrastructure.Init(embedFS, aliasMap, true)

	// For demonstration purposes, we'll just show the API
	fmt.Println("✓ Afero Manager initialized")
	fmt.Println("✓ Development mode: CopyOnWriteFs (embed.FS + OS overrides)")
	fmt.Println("✓ Production mode: ReadOnlyFs (embed.FS only)")
	fmt.Println()

	// Show available aliases
	fmt.Println("Available aliases:")
	for alias, path := range aliasMap {
		fmt.Printf("  - %s -> %s\n", alias, path)
	}
	fmt.Println()

	// Example of checking if files exist
	fmt.Println("Checking file existence:")
	for alias := range aliasMap {
		exists := infrastructure.Exists(alias)
		fmt.Printf("  - %s: %v\n", alias, exists)
	}
	fmt.Println()

	// Example of reading a file
	fmt.Println("Reading banner file:")
	if content, err := infrastructure.Read("banner"); err == nil {
		fmt.Printf("  Content length: %d bytes\n", len(content))
		if len(content) > 100 {
			fmt.Printf("  Preview: %s...\n", string(content[:100]))
		} else {
			fmt.Printf("  Content: %s\n", string(content))
		}
	} else {
		fmt.Printf("  Error reading file: %v\n", err)
	}
	fmt.Println()

	// Example of streaming a file
	fmt.Println("Streaming README file:")
	if stream, err := infrastructure.Stream("readme"); err == nil {
		defer stream.Close()
		content := make([]byte, 200)
		n, err := stream.Read(content)
		if err == nil || err == io.EOF {
			fmt.Printf("  Read %d bytes from stream\n", n)
			fmt.Printf("  Preview: %s...\n", string(content[:n]))
		}
	} else {
		fmt.Printf("  Error streaming file: %v\n", err)
	}
	fmt.Println()

	// Show all configured aliases
	fmt.Println("All configured aliases:")
	aliases := infrastructure.GetAliases()
	for alias, path := range aliases {
		fmt.Printf("  - %s -> %s\n", alias, path)
	}
	fmt.Println()

	fmt.Println("=== Afero Manager Example Complete ===")
	fmt.Println()
}
*/

// runWithConsole runs the application with traditional console logging
func runWithConsole(cfg *config.Config, bannerText string, broadcaster *monitoring.LogBroadcaster) {
	// Print banner to console
	if bannerText != "" {
		fmt.Print("\033[35m") // Purple color
		fmt.Println(bannerText)
		fmt.Print("\033[0m") // Reset color
	}

	// Initialize logger
	l := logger.New(cfg.App.Debug, broadcaster)

	// Log startup information
	l.Info("Starting Application", "name", cfg.App.Name, "env", cfg.App.Env)
	l.Info("TUI mode disabled, using traditional console logging")
	l.Info("Initializing services...")

	// Log all services
	logAllServices(l, cfg)

	// Start server
	srv := server.New(cfg, l, broadcaster)
	go func() {
		l.Info("HTTP server listening", "port", cfg.Server.Port)
		if err := srv.Start(); err != nil {
			l.Fatal("Server error", err)
		}
	}()

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)
	l.Info("Server ready", "url", "http://localhost:"+cfg.Server.Port)
	if cfg.Monitoring.Enabled {
		time.Sleep(500 * time.Millisecond)
		l.Info("Monitoring dashboard", "url", "http://localhost:"+cfg.Monitoring.Port)
	}

	// Handle shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	l.Warn("Shutting down...")
	srv.Shutdown(context.Background(), l)
	time.Sleep(100 * time.Millisecond)
	os.Exit(0)
}

// parseFlags parses command line flags using standard Go flag package
func parseFlags() string {
	var configURL string
	flag.StringVar(&configURL, "c", "", "URL to load configuration from (YAML format)")
	flag.Parse()

	// Validate URL if provided
	if configURL != "" {
		if _, err := url.ParseRequestURI(configURL); err != nil {
			fmt.Printf("Invalid config URL format: %v\n", err)
			fmt.Println("Usage: stackyard [-c config-url]")
			os.Exit(1)
		}
	}

	return configURL
}

// loadConfig loads configuration from local file or URL
func loadConfig(configURL string) *config.Config {
	if configURL != "" {
		fmt.Printf("Loading config from URL: %s\n", configURL)
		if err := utils.LoadConfigFromURL(configURL); err != nil {
			fmt.Printf("Failed to load config from URL: %s\n", err.Error())
			os.Exit(1)
		}

		cfg, err := config.LoadConfigWithURL(configURL)
		if err != nil {
			panic("Failed to parse config from URL: " + err.Error())
		}
		return cfg
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}
	return cfg
}

// loadBanner loads banner text from file if configured
func loadBanner(cfg *config.Config) string {
	if cfg.App.BannerPath != "" {
		banner, err := os.ReadFile(cfg.App.BannerPath)
		if err == nil {
			return string(banner)
		}
	}
	return ""
}

// createServiceQueue creates the service initialization queue for TUI
func createServiceQueue(cfg *config.Config) []tui.ServiceInit {
	serviceConfigs := getServiceConfigs(cfg)

	initQueue := []tui.ServiceInit{
		{Name: "Configuration", Enabled: true, InitFunc: nil},
	}

	// Add infrastructure services
	for _, svc := range serviceConfigs {
		initQueue = append(initQueue, tui.ServiceInit{
			Name: svc.Name, Enabled: svc.Enabled, InitFunc: nil,
		})
	}

	initQueue = append(initQueue, tui.ServiceInit{Name: "Middleware", Enabled: true, InitFunc: nil})

	// Add application services
	for name, enabled := range cfg.Services {
		initQueue = append(initQueue, tui.ServiceInit{Name: "Service: " + name, Enabled: enabled, InitFunc: nil})
	}

	// Add monitoring last
	initQueue = append(initQueue, tui.ServiceInit{Name: "Monitoring", Enabled: cfg.Monitoring.Enabled, InitFunc: nil})

	return initQueue
}

// createLiveTUI creates and configures the Live TUI
func createLiveTUI(cfg *config.Config, bannerText string) *tui.LiveTUI {
	return tui.NewLiveTUI(tui.LiveConfig{
		AppName:     cfg.App.Name,
		AppVersion:  cfg.App.Version,
		Banner:      bannerText,
		Port:        cfg.Server.Port,
		MonitorPort: cfg.Monitoring.Port,
		Env:         cfg.App.Env,
		OnShutdown:  utils.TriggerShutdown,
	})
}

// handleShutdown handles graceful shutdown for TUI mode
func handleShutdown(liveTUI *tui.LiveTUI, srv *server.Server, l *logger.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		liveTUI.AddLog("warn", "Shutting down...")
		srv.Shutdown(context.Background(), l)
	case <-utils.ShutdownChan:
		liveTUI.AddLog("warn", "Shutting down...")
		srv.Shutdown(context.Background(), l)
	}

	liveTUI.Stop()
	time.Sleep(100 * time.Millisecond)
	os.Exit(0)
}

// logAllServices logs the status of all services
func logAllServices(l *logger.Logger, cfg *config.Config) {
	// Log infrastructure services
	serviceConfigs := getServiceConfigs(cfg)
	for _, svc := range serviceConfigs {
		logServiceStatus(l, svc.Name, svc.Enabled)
	}

	// Log application services
	for name, enabled := range cfg.Services {
		logServiceStatus(l, "Service: "+name, enabled)
	}

	// Log monitoring
	logServiceStatus(l, "Monitoring", cfg.Monitoring.Enabled)
}

// ServiceConfig represents a service with its name and enabled status
type ServiceConfig struct {
	Name    string
	Enabled bool
}

// getServiceConfigs returns a unified list of all service configurations
func getServiceConfigs(cfg *config.Config) []ServiceConfig {
	return []ServiceConfig{
		{Name: "Grafana", Enabled: cfg.Grafana.Enabled},
		{Name: "MinIO", Enabled: cfg.Monitoring.MinIO.Enabled},
		{Name: "Redis Cache", Enabled: cfg.Redis.Enabled},
		{Name: "Kafka Messaging", Enabled: cfg.Kafka.Enabled},
		{Name: "PostgreSQL", Enabled: cfg.Postgres.Enabled},
		{Name: "MongoDB", Enabled: cfg.Mongo.Enabled},
		{Name: "Cron Scheduler", Enabled: cfg.Cron.Enabled},
		{Name: "External Services", Enabled: (len(cfg.Monitoring.External.Services) > 0)},
	}
}

// logServiceStatus logs whether a service is enabled or skipped
func logServiceStatus(l *logger.Logger, name string, enabled bool) {
	if enabled {
		l.Info("Service initialized", "service", name, "status", "enabled")
	} else {
		l.Debug("Service skipped", "service", name, "status", "disabled")
	}
}
