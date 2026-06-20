package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Configuration variables
var (
	MAIN_PATH    = "./cmd/app/main.go"
	DOCS_DIR     = "docs"
	SERVICES_DIR = "internal/services/modules"
	OUTPUT_TYPES = "go,json,yaml"
)

// ANSI Colors
const (
	RESET     = "\033[0m"
	BOLD      = "\033[1m"
	DIM       = "\033[2m"
	UNDERLINE = "\033[4m"

	// Pastel Palette
	P_PURPLE = "\033[38;5;108m"
	B_PURPLE = "\033[1;38;5;108m"
	P_CYAN   = "\033[38;5;117m"
	B_CYAN   = "\033[1;38;5;117m"
	P_GREEN  = "\033[38;5;108m"
	B_GREEN  = "\033[1;38;5;108m"
	P_YELLOW = "\033[93m"
	B_YELLOW = "\033[1;93m"
	P_RED    = "\033[91m"
	B_RED    = "\033[1;91m"
	GRAY     = "\033[38;5;242m"
	WHITE    = "\033[97m"
	B_WHITE  = "\033[1;97m"
)

// Swagger configuration
type SwaggerConfig struct {
	GeneralInfo  bool
	ScanServices bool
	Verbose      bool
	DryRun       bool
}

// SwaggerContext holds the generation state
type SwaggerContext struct {
	Config      SwaggerConfig
	ProjectDir  string
	DocsDir     string
	ServicesDir string
}

// Logger for structured output
type Logger struct {
	verbose bool
}

func (l *Logger) Info(msg string, args ...interface{}) {
	fmt.Printf("%s[INFO]%s %s\n", B_CYAN, RESET, fmt.Sprintf(msg, args...))
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	fmt.Printf("%s[WARN]%s %s\n", B_YELLOW, RESET, fmt.Sprintf(msg, args...))
}

func (l *Logger) Error(msg string, args ...interface{}) {
	fmt.Printf("%s[ERROR]%s %s\n", B_RED, RESET, fmt.Sprintf(msg, args...))
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.verbose {
		fmt.Printf("%s[DEBUG]%s %s\n", GRAY, RESET, fmt.Sprintf(msg, args...))
	}
}

func (l *Logger) Success(msg string, args ...interface{}) {
	fmt.Printf("%s[SUCCESS]%s %s\n", B_GREEN, RESET, fmt.Sprintf(msg, args...))
}

// NewLogger creates a new logger
func NewLogger(verbose bool) *Logger {
	return &Logger{verbose: verbose}
}

// clear console screen
func ClearScreen() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

// findProjectRoot searches up the directory tree for go.mod
func findProjectRoot(startDir string) (string, error) {
	current := startDir

	for {
		goModPath := filepath.Join(current, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("go.mod not found in directory tree")
}

// ensureProjectRoot finds the project root and changes to it
func (ctx *SwaggerContext) ensureProjectRoot(logger *Logger) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	logger.Info("Starting from: %s", currentDir)

	projectRoot, err := findProjectRoot(currentDir)
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	if projectRoot != currentDir {
		logger.Info("Changing to project root: %s", projectRoot)
		if err := os.Chdir(projectRoot); err != nil {
			return fmt.Errorf("failed to change directory to %s: %w", projectRoot, err)
		}

		ctx.ProjectDir = projectRoot
		ctx.DocsDir = filepath.Join(projectRoot, DOCS_DIR)
		ctx.ServicesDir = filepath.Join(projectRoot, SERVICES_DIR)

		logger.Success("Now in project root")
	} else {
		logger.Info("Already in project root")
		ctx.ProjectDir = projectRoot
		ctx.DocsDir = filepath.Join(projectRoot, DOCS_DIR)
		ctx.ServicesDir = filepath.Join(projectRoot, SERVICES_DIR)
	}

	return nil
}

// checkSwagInstalled checks if swag CLI is installed
func (ctx *SwaggerContext) checkSwagInstalled(logger *Logger) error {
	logger.Info("Checking if swag CLI is installed...")

	cmd := exec.Command("swag", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("swag CLI not found. Installing...")
		if err := ctx.installSwag(logger); err != nil {
			return fmt.Errorf("failed to install swag: %w", err)
		}
		logger.Success("swag CLI installed")
	} else {
		logger.Success("swag CLI found: %s", strings.TrimSpace(string(output)))
	}

	return nil
}

// installSwag installs swag using go install
func (ctx *SwaggerContext) installSwag(logger *Logger) error {
	cmd := exec.Command("go", "install", "github.com/swaggo/swag/cmd/swag@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// APIEndpoint represents a discovered API endpoint
type APIEndpoint struct {
	Method      string
	Path        string
	Summary     string
	Description string
	Tags        []string
	Service     string
}

// ServiceInfo represents service information
type ServiceInfo struct {
	Name        string
	FileName    string
	Endpoints   []APIEndpoint
	Structs     []string
	HasSwagTags bool
}

// SwaggerTagInfo represents swagger annotation information
type SwaggerTagInfo struct {
	Name        string
	Description string
}

// analyzeAPIEndpoints scans for API endpoints and annotations
func (ctx *SwaggerContext) analyzeAPIEndpoints(logger *Logger) ([]ServiceInfo, error) {
	logger.Info("Analyzing API endpoints and annotations...")

	services := []ServiceInfo{}

	// Read all service files
	files, err := os.ReadDir(ctx.ServicesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read services directory: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".go") {
			continue
		}

		filePath := filepath.Join(ctx.ServicesDir, file.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			logger.Warn("Failed to read %s: %v", file.Name(), err)
			continue
		}

		serviceInfo := ctx.analyzeServiceFile(file.Name(), string(content), logger)
		services = append(services, serviceInfo)
	}

	return services, nil
}

// analyzeServiceFile analyzes a single service file
func (ctx *SwaggerContext) analyzeServiceFile(fileName, content string, logger *Logger) ServiceInfo {
	serviceName := strings.TrimSuffix(fileName, ".go")
	serviceName = strings.ReplaceAll(serviceName, "_", " ")
	serviceName = strings.Title(serviceName)

	info := ServiceInfo{
		Name:     serviceName,
		FileName: fileName,
	}

	// Find swagger annotations
	swagPattern := regexp.MustCompile(`//\s*@(Summary|Description|Tags|Router|Param|Success|Failure)\s+(.+)`)
	matches := swagPattern.FindAllStringSubmatch(content, -1)

	if len(matches) > 0 {
		info.HasSwagTags = true
	}

	// Find endpoints
	routerPattern := regexp.MustCompile(`//\s*@Router\s+([^\s]+)\s+\[(\w+)\]`)
	routerMatches := routerPattern.FindAllStringSubmatch(content, -1)

	for _, match := range routerMatches {
		endpoint := APIEndpoint{
			Path:   match[1],
			Method: match[2],
		}

		// Find associated summary
		summaryPattern := regexp.MustCompile(`//\s*@Summary\s+(.+)`)
		summaryMatch := summaryPattern.FindStringSubmatch(content)
		if summaryMatch != nil {
			endpoint.Summary = summaryMatch[1]
		}

		// Find associated description
		descPattern := regexp.MustCompile(`//\s*@Description\s+(.+)`)
		descMatch := descPattern.FindStringSubmatch(content)
		if descMatch != nil {
			endpoint.Description = descMatch[1]
		}

		// Find tags
		tagsPattern := regexp.MustCompile(`//\s*@Tags\s+(.+)`)
		tagsMatch := tagsPattern.FindStringSubmatch(content)
		if tagsMatch != nil {
			endpoint.Tags = strings.Split(tagsMatch[1], ",")
		}

		endpoint.Service = serviceName
		info.Endpoints = append(info.Endpoints, endpoint)
	}

	// Find struct definitions
	structPattern := regexp.MustCompile(`type\s+(\w+)\s+struct`)
	structMatches := structPattern.FindAllStringSubmatch(content, -1)

	for _, match := range structMatches {
		info.Structs = append(info.Structs, match[1])
	}

	return info
}

// displayAnalysis displays the analysis results
func (ctx *SwaggerContext) displayAnalysis(services []ServiceInfo, logger *Logger) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SWAGGER ANALYSIS RESULTS" + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)

	totalEndpoints := 0
	totalStructs := 0
	servicesWithAnnotations := 0

	for _, service := range services {
		if len(service.Endpoints) > 0 {
			servicesWithAnnotations++
		}

		fmt.Println("")
		fmt.Printf("%s%s%s\n", B_CYAN, service.Name, RESET)
		fmt.Printf("  %sFile:%s %s\n", GRAY, RESET, service.FileName)

		if service.HasSwagTags {
			fmt.Printf("  %sAnnotations:%s %s✓ Found%s\n", GRAY, RESET, B_GREEN, RESET)
		} else {
			fmt.Printf("  %sAnnotations:%s %s✗ Not found%s\n", GRAY, RESET, P_RED, RESET)
		}

		if len(service.Endpoints) > 0 {
			fmt.Printf("  %sEndpoints:%s %d\n", GRAY, RESET, len(service.Endpoints))
			for _, endpoint := range service.Endpoints {
				fmt.Printf("    • %s %s %s %s\n", B_WHITE, endpoint.Method, RESET, endpoint.Path)
				if endpoint.Summary != "" {
					fmt.Printf("      %s%s%s\n", DIM, endpoint.Summary, RESET)
				}
			}
			totalEndpoints += len(service.Endpoints)
		} else {
			fmt.Printf("  %sEndpoints:%s None\n", GRAY, RESET)
		}

		if len(service.Structs) > 0 {
			fmt.Printf("  %sStructs:%s %d\n", GRAY, RESET, len(service.Structs))
			for _, s := range service.Structs {
				fmt.Printf("    • %s\n", s)
			}
			totalStructs += len(service.Structs)
		}
	}

	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Printf(" %sTotal Services:%s %d\n", B_PURPLE, RESET, len(services))
	fmt.Printf(" %sServices with Annotations:%s %d\n", B_PURPLE, RESET, servicesWithAnnotations)
	fmt.Printf(" %sTotal Endpoints:%s %d\n", B_PURPLE, RESET, totalEndpoints)
	fmt.Printf(" %sTotal Structs:%s %d\n", B_PURPLE, RESET, totalStructs)
	fmt.Println(GRAY + "======================================================================" + RESET)
}

// generateSwagger generates swagger documentation
func (ctx *SwaggerContext) generateSwagger(logger *Logger) error {
	logger.Info("Generating Swagger documentation...")

	// Build swag command
	args := []string{"init"}
	args = append(args, "-g", MAIN_PATH)
	args = append(args, "-o", DOCS_DIR)
	args = append(args, "--outputTypes", OUTPUT_TYPES)

	if ctx.Config.Verbose {
		args = append(args, "-v")
	}

	logger.Debug("Running: swag %s", strings.Join(args, " "))

	cmd := exec.Command("swag", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("swagger generation failed: %w", err)
	}

	logger.Success("Swagger documentation generated")
	return nil
}

// verifyOutput verifies the generated files
func (ctx *SwaggerContext) verifyOutput(logger *Logger) error {
	logger.Info("Verifying generated files...")

	expectedFiles := []string{
		"docs.go",
		"swagger.json",
		"swagger.yaml",
	}

	for _, file := range expectedFiles {
		filePath := filepath.Join(ctx.DocsDir, file)
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Errorf("expected file not found: %s", file)
		}
		logger.Success("Found: %s", file)
	}

	return nil
}

// printBanner prints the application banner
func printBanner() {
	fmt.Println("")
	fmt.Println("   " + P_PURPLE + " /\\ " + RESET)
	fmt.Println("   " + P_PURPLE + "(  )" + RESET + "   " + B_PURPLE + "Swagger Generator" + RESET + " " + GRAY + "for" + RESET + " " + B_WHITE + "stackyrd" + RESET)
	fmt.Println("   " + P_PURPLE + " \\/ " + RESET)
	fmt.Println(GRAY + "----------------------------------------------------------------------" + RESET)
}

// printSuccess prints the success message
func printSuccess(docsDir string) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SUCCESS!" + RESET + " " + P_GREEN + "Swagger docs at:" + RESET + " " + UNDERLINE + B_WHITE + docsDir + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println("")
	fmt.Println(" " + P_CYAN + "Generated files:" + RESET)
	fmt.Println("   • docs/docs.go")
	fmt.Println("   • docs/swagger.json")
	fmt.Println("   • docs/swagger.yaml")
	fmt.Println("")
	fmt.Println(" " + P_CYAN + "Next steps:" + RESET)
	fmt.Println("   1. Add gin-swagger middleware to your server")
	fmt.Println("   2. Access Swagger UI at /swagger/index.html")
	fmt.Println("")
}

// setupSignalHandler sets up graceful shutdown on interrupt
func setupSignalHandler(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal. Exiting...")
		cancel()
		os.Exit(1)
	}()
}

// askUserForConfirmation asks user to confirm before generation
func (ctx *SwaggerContext) askUserForConfirmation(logger *Logger) error {
	if ctx.Config.DryRun {
		logger.Info("Dry run mode - skipping generation")
		return nil
	}

	fmt.Printf("%sProceed with generation? (Y/n, timeout 10s): %s", B_YELLOW, RESET)

	inputChan := make(chan string, 1)

	go func() {
		var choice string
		fmt.Scanln(&choice)
		inputChan <- choice
	}()

	select {
	case choice := <-inputChan:
		if strings.ToLower(choice) == "n" || strings.ToLower(choice) == "no" {
			logger.Info("Generation cancelled by user")
			os.Exit(0)
		}
		logger.Success("Proceeding with generation")
	case <-time.After(10 * time.Second):
		logger.Info("Timeout reached. Proceeding with generation")
	}

	return nil
}

// main function
func main() {
	ClearScreen()

	// Parse command line flags
	var (
		verbose = flag.Bool("verbose", false, "Enable verbose logging")
		dryRun  = flag.Bool("dry-run", false, "Only analyze, don't generate")
	)
	flag.Parse()

	// Initialize logger
	logger := NewLogger(*verbose)

	// Print banner
	printBanner()

	// Get project directory
	projectDir, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get current directory: %v", err)
		os.Exit(1)
	}

	// Create swagger context
	ctx := &SwaggerContext{
		Config: SwaggerConfig{
			GeneralInfo:  true,
			ScanServices: true,
			Verbose:      *verbose,
			DryRun:       *dryRun,
		},
		ProjectDir: projectDir,
	}

	// Create context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	setupSignalHandler(cancel)

	// Execute swagger generation steps
	steps := []struct {
		name string
		fn   func(*Logger) error
	}{
		{"Finding project root", ctx.ensureProjectRoot},
		{"Checking swag CLI", ctx.checkSwagInstalled},
		{"Analyzing API endpoints", func(l *Logger) error {
			services, err := ctx.analyzeAPIEndpoints(l)
			if err != nil {
				return err
			}
			ctx.displayAnalysis(services, l)
			return nil
		}},
		{"Asking for confirmation", ctx.askUserForConfirmation},
		{"Generating swagger docs", ctx.generateSwagger},
		{"Verifying output", ctx.verifyOutput},
	}

	for i, step := range steps {
		stepNum := fmt.Sprintf("%d/%d", i+1, len(steps))
		fmt.Printf("%s[%s]%s %s%s%s\n", B_PURPLE, stepNum, RESET, P_CYAN, step.name, RESET)

		if err := step.fn(logger); err != nil {
			logger.Error("Step failed: %v", err)
			os.Exit(1)
		}
	}

	// Print success message
	if !ctx.Config.DryRun {
		printSuccess(ctx.DocsDir)
	}
}
