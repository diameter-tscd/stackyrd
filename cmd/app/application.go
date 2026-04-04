package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"stackyrd/config"
	"stackyrd/internal/server"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/tui"
	"stackyrd/pkg/utils"
	"syscall"
	"time"
)

// Application represents the main application with all its dependencies
type Application struct {
	configManager *ConfigManager
	config        *config.Config
	logger        *logger.Logger
	bannerText    string
}

// NewApplication creates a new application instance
func NewApplication(configManager *ConfigManager) *Application {
	return &Application{
		configManager: configManager,
	}
}

// Run executes the application lifecycle
func (app *Application) Run() error {
	// Clear the terminal screen for a fresh start
	utils.ClearScreen()

	// Execute initialization steps
	steps := []AppStep{
		{"Loading configuration", app.loadConfigStep},
		{"Validating configuration", app.validateConfigStep},
		{"Loading banner", app.loadBannerStep},
		{"Checking port availability", app.checkPortStep},
		{"Initializing logger", app.initLoggerStep},
		{"Starting application", app.startAppStep},
	}

	ctx := &AppContext{
		Timestamp: time.Now().Format("20060102_150405"),
		ConfigURL: app.configManager.configURL,
	}

	if err := executeSteps(ctx, steps); err != nil {
		return fmt.Errorf("%s: %w", ErrStepFailed, err)
	}

	return nil
}

// executeSteps executes the provided steps in sequence with error handling
func executeSteps(ctx *AppContext, steps []AppStep) error {
	for i, step := range steps {
		stepNum := fmt.Sprintf("%d/%d", i+1, len(steps))
		fmt.Printf("[%s] %s\n", stepNum, step.Name)

		if err := step.Fn(ctx); err != nil {
			return fmt.Errorf("step failed: %w", err)
		}
	}
	return nil
}

// Step functions for the initialization process

// loadConfigStep loads configuration from local file or URL
func (app *Application) loadConfigStep(ctx *AppContext) error {
	cfg, err := app.configManager.LoadConfig()
	if err != nil {
		return err
	}
	app.config = cfg
	return nil
}

// validateConfigStep validates the loaded configuration
func (app *Application) validateConfigStep(ctx *AppContext) error {
	return app.configManager.ValidateConfig(app.config)
}

// loadBannerStep loads banner text from file if configured
func (app *Application) loadBannerStep(ctx *AppContext) error {
	bannerText, err := app.configManager.LoadBanner(app.config)
	if err != nil {
		return err
	}
	app.bannerText = bannerText
	return nil
}

// checkPortStep checks port availability
func (app *Application) checkPortStep(ctx *AppContext) error {
	return utils.CheckPortAvailability(app.config.Server.Port)
}

// initLoggerStep initializes the logger
func (app *Application) initLoggerStep(ctx *AppContext) error {
	if app.config.App.EnableTUI {
		// For TUI mode, logger will be initialized later when we have the broadcaster
		return nil
	}

	// For console mode, create a regular logger
	app.logger = logger.New(app.config.App.Debug, nil)
	app.logger.Info("Starting Application", "name", app.config.App.Name, "env", app.config.App.Env)
	app.logger.Info("TUI mode disabled, using traditional console logging")
	app.logger.Info("Initializing services...")

	return nil
}

// startAppStep starts the application based on TUI mode
func (app *Application) startAppStep(ctx *AppContext) error {
	if app.config.App.EnableTUI {
		app.runWithTUI()
	} else {
		app.runWithConsole()
	}
	return nil
}

// runWithTUI runs the application with fancy TUI interface
func (app *Application) runWithTUI() {
	// Configure monitoring port for TUI
	if !app.config.Monitoring.Enabled {
		app.config.Monitoring.Port = "disabled"
	}

	// Setup TUI configuration
	tuiConfig := tui.StartupConfig{
		AppName:     app.config.App.Name,
		AppVersion:  app.config.App.Version,
		Banner:      app.bannerText,
		Port:        app.config.Server.Port,
		MonitorPort: app.config.Monitoring.Port,
		Env:         app.config.App.Env,
		IdleSeconds: app.config.App.StartupDelay,
	}

	// Create service initialization queue
	initQueue := app.configManager.CreateServiceQueue(app.config)

	// Convert to tui.ServiceInit
	tuiInitQueue := make([]tui.ServiceInit, len(initQueue))
	for i, svc := range initQueue {
		tuiInitQueue[i] = tui.ServiceInit{
			Name:     svc.Name,
			Enabled:  svc.Enabled,
			InitFunc: svc.InitFunc,
		}
	}

	// Run the boot sequence TUI
	_, _ = tui.RunBootSequence(tuiConfig, tuiInitQueue)

	// Create and start Live TUI
	liveTUI := app.createLiveTUI()
	liveTUI.Start()

	// Initialize logger with TUI output
	app.logger = logger.NewQuiet(app.config.App.Debug, liveTUI)

	// Add initial logs
	liveTUI.AddLog(LogLevelInfo, "Server starting on port "+app.config.Server.Port)
	liveTUI.AddLog(LogLevelInfo, "Environment: "+app.config.App.Env)

	// Start server
	srv := server.New(app.config, app.logger)
	go func() {
		liveTUI.AddLog(LogLevelInfo, "HTTP server listening...")
		if err := srv.Start(); err != nil {
			liveTUI.AddLog(LogLevelFatal, "Server error: "+err.Error())
		}
	}()

	// Wait for server to start
	time.Sleep(StartupDelay)
	liveTUI.AddLog(LogLevelInfo, "Server ready at http://localhost:"+app.config.Server.Port)

	// Handle shutdown
	app.handleShutdown(liveTUI, srv)
}

// runWithConsole runs the application with traditional console logging
func (app *Application) runWithConsole() {
	// Print banner to console
	if app.bannerText != "" {
		fmt.Print(ColorPurple)
		fmt.Println(app.bannerText)
		fmt.Print(ColorReset)
	}

	// Initialize logger
	app.logger = logger.New(app.config.App.Debug, nil)

	// Log startup information
	app.logger.Info("Starting Application", "name", app.config.App.Name, "env", app.config.App.Env)
	app.logger.Info("TUI mode disabled, using traditional console logging")
	app.logger.Info("Initializing services...")

	// Log all services
	app.logAllServices()

	// Start server
	srv := server.New(app.config, app.logger)
	go func() {
		app.logger.Info("HTTP server listening", "port", app.config.Server.Port)
		if err := srv.Start(); err != nil {
			app.logger.Fatal("Server error", err)
		}
	}()

	// Wait for server to start
	time.Sleep(StartupDelay)
	app.logger.Info("Server ready", "url", "http://localhost:"+app.config.Server.Port)

	// Handle shutdown
	app.handleConsoleShutdown(srv)
}

// createLiveTUI creates and configures the Live TUI
func (app *Application) createLiveTUI() *tui.LiveTUI {
	return tui.NewLiveTUI(tui.LiveConfig{
		AppName:     app.config.App.Name,
		AppVersion:  app.config.App.Version,
		Banner:      app.bannerText,
		Port:        app.config.Server.Port,
		MonitorPort: app.config.Monitoring.Port,
		Env:         app.config.App.Env,
		OnShutdown:  utils.TriggerShutdown,
	})
}

// handleShutdown handles graceful shutdown for TUI mode
func (app *Application) handleShutdown(liveTUI *tui.LiveTUI, srv *server.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		liveTUI.AddLog(LogLevelWarn, "Shutting down...")
		srv.Shutdown(context.Background(), app.logger)
	case <-utils.ShutdownChan:
		liveTUI.AddLog(LogLevelWarn, "Shutting down...")
		srv.Shutdown(context.Background(), app.logger)
	}

	liveTUI.Stop()
	time.Sleep(ShutdownDelay)
	os.Exit(0)
}

// handleConsoleShutdown handles graceful shutdown for console mode
func (app *Application) handleConsoleShutdown(srv *server.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	app.logger.Warn("Shutting down...")
	srv.Shutdown(context.Background(), app.logger)
	time.Sleep(ShutdownDelay)
	os.Exit(0)
}

// logAllServices logs the status of all services
func (app *Application) logAllServices() {
	// Log infrastructure services
	serviceConfigs := app.configManager.GetServiceConfigs(app.config)
	for _, svc := range serviceConfigs {
		app.logServiceStatus(svc.Name, svc.Enabled)
	}

	// Log application services
	for name, enabled := range app.config.Services {
		app.logServiceStatus("Service: "+name, enabled)
	}

}

// logServiceStatus logs whether a service is enabled or skipped
func (app *Application) logServiceStatus(name string, enabled bool) {
	if enabled {
		app.logger.Info("Service initialized", "service", name, "status", ServiceStatusEnabled.String())
	} else {
		app.logger.Debug("Service skipped", "service", name, "status", ServiceStatusDisabled.String())
	}
}
