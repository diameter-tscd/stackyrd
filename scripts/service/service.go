package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Configuration variables
var (
	SERVICES_DIR  = "internal/services/modules"
	MODULE_NAME   = "stackyard"
	STRUCTURE_DIR = "scripts/service"
	TESTS_DIR     = "tests/services"
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

// Available dependencies
type Dependency struct {
	Name        string
	Package     string
	Type        string
	Description string
}

var AVAILABLE_DEPENDENCIES = []Dependency{
	{
		Name:        "PostgresManager",
		Package:     "stackyard/pkg/infrastructure",
		Type:        "*infrastructure.PostgresManager",
		Description: "PostgreSQL database connection manager",
	},
	{
		Name:        "PostgresConnectionManager",
		Package:     "stackyard/pkg/infrastructure",
		Type:        "*infrastructure.PostgresConnectionManager",
		Description: "Multi-tenant PostgreSQL connection manager",
	},
	{
		Name:        "MongoConnectionManager",
		Package:     "stackyard/pkg/infrastructure",
		Type:        "*infrastructure.MongoConnectionManager",
		Description: "Multi-tenant MongoDB connection manager",
	},
	{
		Name:        "RedisManager",
		Package:     "stackyard/pkg/infrastructure",
		Type:        "*infrastructure.RedisManager",
		Description: "Redis cache manager",
	},
	{
		Name:        "KafkaManager",
		Package:     "stackyard/pkg/infrastructure",
		Type:        "*infrastructure.KafkaManager",
		Description: "Kafka message queue manager",
	},
	{
		Name:        "MinIOManager",
		Package:     "stackyard/pkg/infrastructure",
		Type:        "*infrastructure.MinIOManager",
		Description: "MinIO object storage manager",
	},
	{
		Name:        "GrafanaManager",
		Package:     "stackyard/pkg/infrastructure",
		Type:        "*infrastructure.GrafanaManager",
		Description: "Grafana monitoring dashboard manager",
	},
	{
		Name:        "CronManager",
		Package:     "stackyard/pkg/infrastructure",
		Type:        "*infrastructure.CronManager",
		Description: "Cron job scheduler manager",
	},
}

// Service configuration
type ServiceConfig struct {
	ServiceName     string
	WireName        string
	FileName        string
	Dependencies    []Dependency
	HasDependencies bool
	GenerateTests   bool
	Verbose         bool
	DryRun          bool
}

// ServiceContext holds the generation state
type ServiceContext struct {
	Config       ServiceConfig
	ProjectDir   string
	ServicesDir  string
	StructureDir string
	TestsDir     string
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

func (l *Logger) Prompt(msg string, args ...interface{}) {
	fmt.Printf("%s[PROMPT]%s %s", B_YELLOW, RESET, fmt.Sprintf(msg, args...))
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
	cmd.Run()
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
func (ctx *ServiceContext) ensureProjectRoot(logger *Logger) error {
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
		ctx.ServicesDir = filepath.Join(projectRoot, SERVICES_DIR)
		ctx.StructureDir = filepath.Join(projectRoot, STRUCTURE_DIR)

		logger.Success("Now in project root")
	} else {
		logger.Info("Already in project root")
		ctx.ProjectDir = projectRoot
		ctx.ServicesDir = filepath.Join(projectRoot, SERVICES_DIR)
		ctx.StructureDir = filepath.Join(projectRoot, STRUCTURE_DIR)
	}

	return nil
}

// promptServiceName prompts for the service name
func (ctx *ServiceContext) promptServiceName(logger *Logger) error {
	logger.Prompt("Enter service name (e.g., Orders, Inventory): ")

	var serviceName string
	fmt.Scanln(&serviceName)

	if serviceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	// Capitalize first letter
	serviceName = strings.ToUpper(serviceName[:1]) + serviceName[1:]
	ctx.Config.ServiceName = serviceName

	logger.Success("Service name: %s", serviceName)
	return nil
}

// promptWireName prompts for the wire name
func (ctx *ServiceContext) promptWireName(logger *Logger) error {
	// Generate default wire name from service name
	defaultWireName := strings.ToLower(ctx.Config.ServiceName) + "-service"
	logger.Prompt("Enter wire name (default: %s): ", defaultWireName)

	var wireName string
	fmt.Scanln(&wireName)

	if wireName == "" {
		wireName = defaultWireName
	}

	ctx.Config.WireName = wireName

	logger.Success("Wire name: %s", wireName)
	return nil
}

// promptFileName prompts for the file name
func (ctx *ServiceContext) promptFileName(logger *Logger) error {
	// Generate default file name from service name
	defaultFileName := strings.ToLower(ctx.Config.ServiceName) + "_service.go"
	logger.Prompt("Enter file name (default: %s): ", defaultFileName)

	var fileName string
	fmt.Scanln(&fileName)

	if fileName == "" {
		fileName = defaultFileName
	}

	// Ensure .go extension
	if !strings.HasSuffix(fileName, ".go") {
		fileName += ".go"
	}

	ctx.Config.FileName = fileName

	logger.Success("File name: %s", fileName)
	return nil
}

// promptDependencies prompts for dependencies with selection
func (ctx *ServiceContext) promptDependencies(logger *Logger) error {
	logger.Info("Available dependencies:")
	fmt.Println("")

	for i, dep := range AVAILABLE_DEPENDENCIES {
		fmt.Printf("  %s[%d]%s %s%s%s - %s\n", B_CYAN, i+1, RESET, B_WHITE, dep.Name, RESET, dep.Description)
	}

	fmt.Printf("\n  %s[0]%s %sNone%s - No dependencies\n", B_CYAN, RESET, B_WHITE, RESET)
	fmt.Println("")

	logger.Prompt("Enter dependency numbers (comma-separated, e.g., 1,3,5): ")

	var input string
	fmt.Scanln(&input)

	if input == "" || input == "0" {
		logger.Success("No dependencies selected")
		return nil
	}

	// Parse selected dependencies
	selectedIndices := strings.Split(input, ",")
	for _, idxStr := range selectedIndices {
		idxStr = strings.TrimSpace(idxStr)
		var idx int
		if _, err := fmt.Sscanf(idxStr, "%d", &idx); err != nil {
			logger.Warn("Invalid index: %s, skipping", idxStr)
			continue
		}

		if idx < 1 || idx > len(AVAILABLE_DEPENDENCIES) {
			logger.Warn("Index out of range: %d, skipping", idx)
			continue
		}

		dep := AVAILABLE_DEPENDENCIES[idx-1]
		ctx.Config.Dependencies = append(ctx.Config.Dependencies, dep)
		logger.Success("Selected: %s", dep.Name)
	}

	ctx.Config.HasDependencies = len(ctx.Config.Dependencies) > 0

	return nil
}

// promptGenerateTests prompts for test file generation
func (ctx *ServiceContext) promptGenerateTests(logger *Logger) error {
	logger.Prompt("Generate test file? (y/N, default: N): ")

	var input string
	fmt.Scanln(&input)

	if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
		ctx.Config.GenerateTests = true
		logger.Success("Test file will be generated")
	} else {
		ctx.Config.GenerateTests = false
		logger.Info("Skipping test file generation")
	}

	return nil
}

// buildConstructorArgs builds the constructor arguments for tests
func (ctx *ServiceContext) buildConstructorArgs() string {
	if !ctx.Config.HasDependencies {
		return ", nil"
	}

	var args []string
	for range ctx.Config.Dependencies {
		args = append(args, "nil")
	}

	return ", " + strings.Join(args, ", ") + ", nil"
}

// generateTestFile generates the test file
func (ctx *ServiceContext) generateTestFile(logger *Logger) error {
	if !ctx.Config.GenerateTests {
		return nil
	}

	logger.Info("Generating test file...")

	// Read the test structure template
	structurePath := filepath.Join(ctx.StructureDir, "structure_test")
	template, err := os.ReadFile(structurePath)
	if err != nil {
		return fmt.Errorf("failed to read test structure template: %w", err)
	}

	content := string(template)
	content = strings.ReplaceAll(content, "{{SERVICE_NAME}}", ctx.Config.ServiceName)
	content = strings.ReplaceAll(content, "{{SERVICE_NAME_LOWER}}", strings.ToLower(ctx.Config.ServiceName))
	content = strings.ReplaceAll(content, "{{WIRE_NAME}}", ctx.Config.WireName)
	content = strings.ReplaceAll(content, "{{CONSTRUCTOR_ARGS}}", ctx.buildConstructorArgs())

	// Clean up extra newlines
	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")

	// Write the file
	testFileName := strings.ToLower(ctx.Config.ServiceName) + "_service_test.go"
	filePath := filepath.Join(ctx.ProjectDir, TESTS_DIR, testFileName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	logger.Success("Test file generated: %s", testFileName)
	return nil
}

// displayConfiguration displays the service configuration
func (ctx *ServiceContext) displayConfiguration(logger *Logger) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SERVICE CONFIGURATION" + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)

	fmt.Printf(" %sService Name:%s %s\n", B_CYAN, RESET, ctx.Config.ServiceName)
	fmt.Printf(" %sWire Name:%s %s\n", B_CYAN, RESET, ctx.Config.WireName)
	fmt.Printf(" %sFile Name:%s %s\n", B_CYAN, RESET, ctx.Config.FileName)
	fmt.Printf(" %sFile Path:%s %s\n", B_CYAN, RESET, filepath.Join(ctx.ServicesDir, ctx.Config.FileName))

	if len(ctx.Config.Dependencies) > 0 {
		fmt.Printf("\n %sDependencies:%s\n", B_CYAN, RESET)
		for _, dep := range ctx.Config.Dependencies {
			fmt.Printf("   • %s - %s\n", dep.Name, dep.Type)
		}
	} else {
		fmt.Printf("\n %sDependencies:%s None\n", B_CYAN, RESET)
	}

	fmt.Println(GRAY + "======================================================================" + RESET)
}

// askUserForConfirmation asks user to confirm before generation
func (ctx *ServiceContext) askUserForConfirmation(logger *Logger) error {
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

// readStructureTemplate reads the structure template file
func (ctx *ServiceContext) readStructureTemplate(logger *Logger) (string, error) {
	structurePath := filepath.Join(ctx.StructureDir, "structure")
	content, err := os.ReadFile(structurePath)
	if err != nil {
		return "", fmt.Errorf("failed to read structure template: %w", err)
	}
	return string(content), nil
}

// buildImports builds the import statements
func (ctx *ServiceContext) buildImports() string {
	if !ctx.Config.HasDependencies {
		return ""
	}

	// Use a map to deduplicate imports
	importMap := make(map[string]bool)
	for _, dep := range ctx.Config.Dependencies {
		importMap[dep.Package] = true
	}

	// Convert map keys to slice
	var imports []string
	for pkg := range importMap {
		imports = append(imports, fmt.Sprintf(`	"%s"`, pkg))
	}

	// Sort for consistent output
	sort.Strings(imports)

	return strings.Join(imports, "\n")
}

// buildFields builds the struct fields
func (ctx *ServiceContext) buildFields() string {
	if !ctx.Config.HasDependencies {
		return ""
	}

	var fields []string
	for _, dep := range ctx.Config.Dependencies {
		fieldName := strings.ToLower(dep.Name[:1]) + dep.Name[1:]
		fields = append(fields, fmt.Sprintf("\t%s %s", fieldName, dep.Type))
	}

	return strings.Join(fields, "\n")
}

// buildParams builds the constructor parameters
func (ctx *ServiceContext) buildParams() string {
	if !ctx.Config.HasDependencies {
		return ""
	}

	var params []string
	for _, dep := range ctx.Config.Dependencies {
		fieldName := strings.ToLower(dep.Name[:1]) + dep.Name[1:]
		params = append(params, fmt.Sprintf("\t%s %s,", fieldName, dep.Type))
	}

	return strings.Join(params, "\n")
}

// buildAssignments builds the constructor assignments
func (ctx *ServiceContext) buildAssignments() string {
	if !ctx.Config.HasDependencies {
		return ""
	}

	var assignments []string
	for _, dep := range ctx.Config.Dependencies {
		fieldName := strings.ToLower(dep.Name[:1]) + dep.Name[1:]
		assignments = append(assignments, fmt.Sprintf("\t\t%s: %s,", fieldName, fieldName))
	}

	return strings.Join(assignments, "\n")
}

// buildInitFunction builds the init function for auto-registration
func (ctx *ServiceContext) buildInitFunction() string {
	configKey := strings.ToLower(ctx.Config.ServiceName) + "_service"

	var dependencyChecks strings.Builder
	var dependencyParams strings.Builder

	if ctx.Config.HasDependencies {
		dependencyChecks.WriteString(`	if deps == nil {
		logger.Warn("Dependencies not available, skipping Service")
		return nil
	}

`)

		for _, dep := range ctx.Config.Dependencies {
			dependencyChecks.WriteString(fmt.Sprintf(`	if deps.%s == nil {
		logger.Warn("%s not available, skipping Service")
		return nil
	}

`, dep.Name, dep.Name))

			dependencyParams.WriteString(fmt.Sprintf(", deps.%s", dep.Name))
		}
	}

	return fmt.Sprintf(`// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("%s", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		if !config.Services.IsEnabled("%s") {
			return nil
		}
%s		return New%s(true%s, logger)
	})
}`, configKey, configKey, dependencyChecks.String(), ctx.Config.ServiceName, dependencyParams.String())
}

// generateService generates the service Go file
func (ctx *ServiceContext) generateService(logger *Logger) error {
	logger.Info("Generating service file...")

	// Read the structure template
	template, err := ctx.readStructureTemplate(logger)
	if err != nil {
		return err
	}

	// Build replacement values
	imports := ctx.buildImports()
	fields := ctx.buildFields()
	params := ctx.buildParams()
	assignments := ctx.buildAssignments()
	initFunction := ctx.buildInitFunction()
	serviceNameLower := strings.ToLower(ctx.Config.ServiceName)

	// Replace placeholders
	content := template
	content = strings.ReplaceAll(content, "{{SERVICE_NAME}}", ctx.Config.ServiceName)
	content = strings.ReplaceAll(content, "{{SERVICE_NAME_LOWER}}", serviceNameLower)
	content = strings.ReplaceAll(content, "{{WIRE_NAME}}", ctx.Config.WireName)
	content = strings.ReplaceAll(content, "{{IMPORTS}}", imports)
	content = strings.ReplaceAll(content, "{{FIELDS}}", fields)
	content = strings.ReplaceAll(content, "{{PARAMS}}", params)
	content = strings.ReplaceAll(content, "{{ASSIGNMENTS}}", assignments)
	content = strings.ReplaceAll(content, "{{INIT_FUNCTION}}", initFunction)

	// Clean up extra newlines
	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")

	// Write the file
	filePath := filepath.Join(ctx.ServicesDir, ctx.Config.FileName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	logger.Success("Service file generated: %s", filePath)
	return nil
}

// displaySummary displays the generation summary
func (ctx *ServiceContext) displaySummary(logger *Logger) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "GENERATION SUMMARY" + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)

	fmt.Printf(" %s✓%s Service file created: %s\n", B_GREEN, RESET, ctx.Config.FileName)
	fmt.Printf(" %s✓%s Service struct: %s\n", B_GREEN, RESET, ctx.Config.ServiceName)
	fmt.Printf(" %s✓%s Wire name: %s\n", B_GREEN, RESET, ctx.Config.WireName)
	fmt.Printf(" %s✓%s Auto-registration: Enabled\n", B_GREEN, RESET)

	if len(ctx.Config.Dependencies) > 0 {
		fmt.Printf(" %s✓%s Dependencies: %d configured\n", B_GREEN, RESET, len(ctx.Config.Dependencies))
	}

	fmt.Println("")
	fmt.Println(" " + P_CYAN + "Next steps:" + RESET)
	fmt.Println("   1. Add service to config.yaml:")
	fmt.Printf("      services:\n        %s: true\n", strings.ToLower(ctx.Config.ServiceName)+"_service")
	fmt.Println("")
	fmt.Println("   2. Implement business logic in handler methods")
	fmt.Println("   3. Add swagger annotations for API documentation")
	fmt.Println("   4. Test the service endpoints")
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
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

// printBanner prints the application banner
func printBanner() {
	fmt.Println("")
	fmt.Println("   " + P_PURPLE + " /\\ " + RESET)
	fmt.Println("   " + P_PURPLE + "(  )" + RESET + "   " + B_PURPLE + "Service Generator" + RESET + " " + GRAY + "for" + RESET + " " + B_WHITE + "Stackyard" + RESET)
	fmt.Println("   " + P_PURPLE + " \\/ " + RESET)
	fmt.Println(GRAY + "----------------------------------------------------------------------" + RESET)
}

// printSuccess prints the success message
func printSuccess(fileName string) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SUCCESS!" + RESET + " " + P_GREEN + "Service generated:" + RESET + " " + UNDERLINE + B_WHITE + fileName + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)
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

	// Create service context
	ctx := &ServiceContext{
		Config: ServiceConfig{
			Verbose: *verbose,
			DryRun:  *dryRun,
		},
		ProjectDir: projectDir,
	}

	// Create context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	setupSignalHandler(cancel)

	// Execute service generation steps
	steps := []struct {
		name string
		fn   func(*Logger) error
	}{
		{"Finding project root", ctx.ensureProjectRoot},
		{"Prompting for service name", ctx.promptServiceName},
		{"Prompting for wire name", ctx.promptWireName},
		{"Prompting for file name", ctx.promptFileName},
		{"Prompting for dependencies", ctx.promptDependencies},
		{"Prompting for test generation", ctx.promptGenerateTests},
		{"Displaying configuration", func(l *Logger) error {
			ctx.displayConfiguration(l)
			return nil
		}},
		{"Asking for confirmation", ctx.askUserForConfirmation},
		{"Generating service file", ctx.generateService},
		{"Generating test file", ctx.generateTestFile},
		{"Displaying summary", func(l *Logger) error {
			ctx.displaySummary(l)
			return nil
		}},
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
		printSuccess(ctx.Config.FileName)
	}
}
