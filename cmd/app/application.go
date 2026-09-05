package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"stackyrd/config"
	"stackyrd/internal/server"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/tui"
	"stackyrd/pkg/utils"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
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
	if app.configManager == nil {
		return fmt.Errorf("application config manager is nil")
	}

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
		_, _ = fmt.Printf("[%s] %s\n", stepNum, step.Name)

		if err := step.Fn(ctx); err != nil {
			return fmt.Errorf("step failed: %w", err)
		}
	}
	utils.ClearScreen()
	return nil
}

// Step functions for the initialization process

// loadConfigStep loads configuration from local file or URL
func (app *Application) loadConfigStep(ctx *AppContext) error {
	cfg, err := app.configManager.LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
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
		return fmt.Errorf("load banner: %w", err)
	}
	app.bannerText = bannerText
	return nil
}

// checkPortStep checks port availability
func (app *Application) checkPortStep(ctx *AppContext) error {
	return utils.CheckPortAvailability(app.config.Server.Port)
}

func (app *Application) applyTheme() {
	tui.SetThemeName(app.config.App.Theme)
	logger.SetThemeColorFunc(tui.TC)
}

func (app *Application) initLoggerStep(ctx *AppContext) error {
	app.applyTheme()
	if app.config.App.EnableTUI {
		return nil
	}

	app.logger = logger.New(app.config.App.Debug, utils.DashboardWriter)
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

func (app *Application) runWithTUI() {
	app.applyTheme()

	// Setup TUI configuration
	tuiConfig := tui.StartupConfig{
		AppName:     app.config.App.Name,
		AppVersion:  app.config.App.Version,
		Banner:      app.bannerText,
		Port:        app.config.Server.Port,
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

	broadcaster := io.MultiWriter(liveTUI, utils.DashboardWriter)
	app.logger = logger.NewQuiet(app.config.App.Debug, broadcaster)

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

func (app *Application) runWithConsole() {
	app.applyTheme()
	if app.bannerText != "" {
		banner := lipgloss.NewStyle().Foreground(lipgloss.Color(tui.TC("primary"))).Render(app.bannerText)
		_, _ = fmt.Println(banner)
	}
	app.printConsoleSystemInfo()

	app.logger = logger.New(app.config.App.Debug, utils.DashboardWriter)

	// Log startup information
	app.logger.Info("Starting Application", "name", app.config.App.Name, "env", app.config.App.Env)
	app.logger.Info("TUI mode disabled, using traditional console logging")
	app.logger.Info("Initializing services...")

	// Log all services
	app.logAllServices()

	// Start server
	srv := server.New(app.config, app.logger)
	srvErr := make(chan error, 1)
	go func() {
		app.logger.Info("HTTP server listening", "port", app.config.Server.Port)
		srvErr <- srv.Start()
	}()

	// Wait for server to start
	time.Sleep(StartupDelay)
	app.logger.Info("Server ready", "url", "http://localhost:"+app.config.Server.Port)

	// Handle shutdown
	app.handleConsoleShutdown(srv, srvErr)
}

// createLiveTUI creates and configures the runtime dashboard TUI
func (app *Application) createLiveTUI() *tui.TerminalTUI {
	return tui.NewTerminalTUI(tui.LiveConfig{
		AppName:    app.config.App.Name,
		AppVersion: app.config.App.Version,
		Banner:     app.bannerText,
		Port:       app.config.Server.Port,
		Env:        app.config.App.Env,
		OnShutdown: utils.TriggerShutdown,
		TUI:        app.config.App.TUI,
	})
}

// handleShutdown handles graceful shutdown for TUI mode
func (app *Application) handleShutdown(liveTUI *tui.TerminalTUI, srv *server.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	select {
	case <-sigChan:
		liveTUI.AddLog(LogLevelWarn, "Shutting down...")
		if err := srv.Shutdown(context.Background(), app.logger); err != nil {
			liveTUI.AddLog(LogLevelError, "Server shutdown error: "+err.Error())
		}
	case <-utils.ShutdownChan:
		liveTUI.AddLog(LogLevelWarn, "Shutting down...")
		if err := srv.Shutdown(context.Background(), app.logger); err != nil {
			liveTUI.AddLog(LogLevelError, "Server shutdown error: "+err.Error())
		}
	}

	liveTUI.Stop()
	time.Sleep(ShutdownDelay)
}

// handleConsoleShutdown handles graceful shutdown for console mode
func (app *Application) handleConsoleShutdown(srv *server.Server, srvErr <-chan error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	select {
	case err := <-srvErr:
		// Server exited on its own (fatal listener error): report, don't call os.Exit.
		if err != nil {
			app.logger.Error("Server error", err)
		}
		return
	case <-sigChan:
	}

	app.logger.Warn("Shutting down...")
	if err := srv.Shutdown(context.Background(), app.logger); err != nil {
		app.logger.Error("Server shutdown error", err)
	}
	time.Sleep(ShutdownDelay)
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

func (app *Application) printConsoleSystemInfo() {
	hostname := "unknown"
	if info, err := utils.GetNetworkInfo(); err == nil {
		hostname = info["hostname"]
	}
	cpuModel := ""
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		cpuModel = info[0].ModelName
		if len(cpuModel) > 48 {
			cpuModel = cpuModel[:45] + "..."
		}
	}
	cpuPercent := 0.0
	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		cpuPercent = pct[0]
	}
	memUsed, memTotal, memPct := uint64(0), uint64(0), 0.0
	if vm, err := mem.VirtualMemory(); err == nil {
		memUsed = vm.Used / 1024 / 1024
		memTotal = vm.Total / 1024 / 1024
		memPct = vm.UsedPercent
	}
	appMem := utils.GetMemSelf()
	goroutines := runtime.NumGoroutine()
	ncpu := runtime.NumCPU()

	secStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tui.TC("dim"))).Bold(true)
	lblStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tui.TC("text")))
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tui.TC("text")))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tui.TC("dim")))
	priStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tui.TC("primary"))).Bold(true)

	cpuBar := tui.ProgressBar(cpuPercent, 14, false)
	memBar := tui.ProgressBar(memPct, 14, false)

	var b strings.Builder
	b.WriteString(secStyle.Render("⟡ System"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf(" %s %s  %s %s\n",
		lblStyle.Render("OS"), valStyle.Render(fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)),
		lblStyle.Render("Host"), valStyle.Render(hostname)))
	if cpuModel != "" {
		b.WriteString(fmt.Sprintf(" %s %s\n", lblStyle.Render("CPU"), valStyle.Render(cpuModel)))
	}
	b.WriteString(fmt.Sprintf(" %s %s %s  %s %s %s\n",
		lblStyle.Render("Cores"), valStyle.Render(fmt.Sprintf("%d", ncpu)),
		dimStyle.Render("│"),
		lblStyle.Render("Goroutines"), valStyle.Render(fmt.Sprintf("%d", goroutines)),
		dimStyle.Render("│"),
	))
	b.WriteString(fmt.Sprintf(" %s %s %s  %s %s %s / %s GiB\n",
		lblStyle.Render("CPU"), cpuBar, priStyle.Render(fmt.Sprintf("%5.1f%%", cpuPercent)),
		lblStyle.Render("RAM"), memBar, valStyle.Render(fmt.Sprintf("%.1f", float64(memUsed)/1024)), dimStyle.Render(fmt.Sprintf("%.1f", float64(memTotal)/1024))))
	b.WriteString(fmt.Sprintf(" %s %s  %s %s %s\n",
		lblStyle.Render("AppMem"), valStyle.Render(fmt.Sprintf("%d MiB", appMem)),
		dimStyle.Render("│"),
		lblStyle.Render("PID"), valStyle.Render(fmt.Sprintf("%d", os.Getpid()))))
	b.WriteString(dimStyle.Render(strings.Repeat("─", 52)))
	_, _ = fmt.Println(b.String())
	_, _ = fmt.Println(dimStyle.Render(fmt.Sprintf(" Port %s  Env %s  Theme %s", app.config.Server.Port, app.config.App.Env, app.config.App.Theme)))
}

func (app *Application) logServiceStatus(name string, enabled bool) {
	if enabled {
		app.logger.Info("Service initialized", "service", name, "status", ServiceStatusEnabled.String())
	} else {
		app.logger.Debug("Service skipped", "service", name, "status", ServiceStatusDisabled.String())
	}
}
