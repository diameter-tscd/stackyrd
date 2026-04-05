package main

import (
	"context"
	"embed"
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

//go:embed templates/*
var templatesFS embed.FS

// Configuration variables
var (
	SERVICES_DIR = "internal/services/modules"
	MODULE_NAME  = "stackyrd"
	TESTS_DIR    = "tests/services"
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

// Service patterns
type ServicePattern struct {
	Name        string
	Description string
	Template    string
}

var SERVICE_PATTERNS = []ServicePattern{
	{
		Name:        "Basic CRUD",
		Description: "Standard Create, Read, Update, Delete operations",
		Template:    "basic_crud",
	},
	{
		Name:        "Read-Only",
		Description: "Only list and get operations (no create/update/delete)",
		Template:    "read_only",
	},
	{
		Name:        "Write-Only",
		Description: "Only create and update operations (no list/get)",
		Template:    "write_only",
	},
	{
		Name:        "Event-Driven",
		Description: "Event publishing and subscription handlers",
		Template:    "event_driven",
	},
	{
		Name:        "WebSocket",
		Description: "Real-time WebSocket communication",
		Template:    "websocket",
	},
	{
		Name:        "Batch Processing",
		Description: "Batch operations with worker pool",
		Template:    "batch_processing",
	},
}

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
		Package:     "stackyrd/pkg/infrastructure",
		Type:        "*infrastructure.PostgresManager",
		Description: "PostgreSQL database connection manager",
	},
	{
		Name:        "PostgresConnectionManager",
		Package:     "stackyrd/pkg/infrastructure",
		Type:        "*infrastructure.PostgresConnectionManager",
		Description: "Multi-tenant PostgreSQL connection manager",
	},
	{
		Name:        "MongoConnectionManager",
		Package:     "stackyrd/pkg/infrastructure",
		Type:        "*infrastructure.MongoConnectionManager",
		Description: "Multi-tenant MongoDB connection manager",
	},
	{
		Name:        "RedisManager",
		Package:     "stackyrd/pkg/infrastructure",
		Type:        "*infrastructure.RedisManager",
		Description: "Redis cache manager",
	},
	{
		Name:        "KafkaManager",
		Package:     "stackyrd/pkg/infrastructure",
		Type:        "*infrastructure.KafkaManager",
		Description: "Kafka message queue manager",
	},
	{
		Name:        "MinIOManager",
		Package:     "stackyrd/pkg/infrastructure",
		Type:        "*infrastructure.MinIOManager",
		Description: "MinIO object storage manager",
	},
	{
		Name:        "GrafanaManager",
		Package:     "stackyrd/pkg/infrastructure",
		Type:        "*infrastructure.GrafanaManager",
		Description: "Grafana monitoring dashboard manager",
	},
	{
		Name:        "CronManager",
		Package:     "stackyrd/pkg/infrastructure",
		Type:        "*infrastructure.CronManager",
		Description: "Cron job scheduler manager",
	},
}

// Custom route definition
type CustomRoute struct {
	Method      string
	Path        string
	HandlerName string
	Summary     string
	Description string
}

// Service configuration
type ServiceConfig struct {
	ServiceName     string
	WireName        string
	FileName        string
	Dependencies    []Dependency
	HasDependencies bool
	GenerateTests   bool
	GenerateModel   bool
	ServicePattern  ServicePattern
	CustomRoutes    []CustomRoute
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
		ctx.StructureDir = filepath.Join(projectRoot, "scripts/service")

		logger.Success("Now in project root")
	} else {
		logger.Info("Already in project root")
		ctx.ProjectDir = projectRoot
		ctx.ServicesDir = filepath.Join(projectRoot, SERVICES_DIR)
		ctx.StructureDir = filepath.Join(projectRoot, "scripts/service")
	}

	return nil
}

// promptServiceName prompts for the service name
func (ctx *ServiceContext) promptServiceName(logger *Logger) error {
	for {
		logger.Prompt("Enter service name (e.g., Orders, Inventory): ")

		var serviceName string
		fmt.Scanln(&serviceName)

		if serviceName == "" {
			logger.Error("Service name cannot be empty")
			continue
		}

		// Capitalize first letter
		serviceName = strings.ToUpper(serviceName[:1]) + serviceName[1:]

		// Check for duplicates
		exists, err := ctx.checkServiceExists(serviceName)
		if err != nil {
			logger.Error("Error checking for existing service: %v", err)
			return err
		}

		if exists {
			logger.Warn("A service with name '%s' already exists. Please choose a different name.", serviceName)
			continue
		}

		ctx.Config.ServiceName = serviceName
		logger.Success("Service name: %s", serviceName)
		return nil
	}
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

// promptServicePattern prompts for service pattern selection
func (ctx *ServiceContext) promptServicePattern(logger *Logger) error {
	logger.Info("Select service pattern:")
	fmt.Println("")

	for i, pattern := range SERVICE_PATTERNS {
		fmt.Printf("  %s[%d]%s %s%s%s - %s\n", B_CYAN, i+1, RESET, B_WHITE, pattern.Name, RESET, pattern.Description)
	}

	fmt.Println("")

	logger.Prompt("Enter pattern number (default: 1): ")

	var input string
	fmt.Scanln(&input)

	if input == "" {
		ctx.Config.ServicePattern = SERVICE_PATTERNS[0]
	} else {
		var idx int
		if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(SERVICE_PATTERNS) {
			logger.Warn("Invalid pattern, using Basic CRUD")
			idx = 1
		}
		ctx.Config.ServicePattern = SERVICE_PATTERNS[idx-1]
	}

	logger.Success("Selected pattern: %s", ctx.Config.ServicePattern.Name)
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

// promptGenerateModel prompts for database model generation
func (ctx *ServiceContext) promptGenerateModel(logger *Logger) error {
	logger.Prompt("Generate database model (GORM)? (y/N, default: N): ")

	var input string
	fmt.Scanln(&input)

	if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
		ctx.Config.GenerateModel = true
		logger.Success("Database model will be generated")
	} else {
		ctx.Config.GenerateModel = false
		logger.Info("Skipping database model generation")
	}

	return nil
}

// promptCustomRoutes prompts for custom routes
func (ctx *ServiceContext) promptCustomRoutes(logger *Logger) error {
	logger.Prompt("Add custom routes? (y/N, default: N): ")

	var input string
	fmt.Scanln(&input)

	if strings.ToLower(input) != "y" && strings.ToLower(input) != "yes" {
		logger.Info("No custom routes added")
		return nil
	}

	for {
		route := CustomRoute{}

		logger.Prompt("Enter route path (e.g., /search, /bulk): ")
		fmt.Scanln(&route.Path)
		if route.Path == "" {
			break
		}

		logger.Prompt("Enter HTTP method (GET/POST/PUT/DELETE): ")
		fmt.Scanln(&route.Method)
		route.Method = strings.ToUpper(route.Method)

		logger.Prompt("Enter handler summary (e.g., Search items): ")
		fmt.Scanln(&route.Summary)

		logger.Prompt("Enter handler description: ")
		fmt.Scanln(&route.Description)

		// Generate handler name from path
		route.HandlerName = strings.ToLower(route.Method) + strings.ToUpper(route.Path[1:2]) + route.Path[2:]

		ctx.Config.CustomRoutes = append(ctx.Config.CustomRoutes, route)
		logger.Success("Added route: %s %s", route.Method, route.Path)

		logger.Prompt("Add another route? (y/N): ")
		fmt.Scanln(&input)
		if strings.ToLower(input) != "y" && strings.ToLower(input) != "yes" {
			break
		}
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

// displayConfiguration displays the service configuration
func (ctx *ServiceContext) displayConfiguration(logger *Logger) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SERVICE CONFIGURATION" + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)

	fmt.Printf(" %sService Name:%s %s\n", B_CYAN, RESET, ctx.Config.ServiceName)
	fmt.Printf(" %sWire Name:%s %s\n", B_CYAN, RESET, ctx.Config.WireName)
	fmt.Printf(" %sFile Name:%s %s\n", B_CYAN, RESET, ctx.Config.FileName)
	fmt.Printf(" %sPattern:%s %s\n", B_CYAN, RESET, ctx.Config.ServicePattern.Name)
	fmt.Printf(" %sFile Path:%s %s\n", B_CYAN, RESET, filepath.Join(ctx.ServicesDir, ctx.Config.FileName))

	if len(ctx.Config.Dependencies) > 0 {
		fmt.Printf("\n %sDependencies:%s\n", B_CYAN, RESET)
		for _, dep := range ctx.Config.Dependencies {
			fmt.Printf("   • %s - %s\n", dep.Name, dep.Type)
		}
	} else {
		fmt.Printf("\n %sDependencies:%s None\n", B_CYAN, RESET)
	}

	if len(ctx.Config.CustomRoutes) > 0 {
		fmt.Printf("\n %sCustom Routes:%s\n", B_CYAN, RESET)
		for _, route := range ctx.Config.CustomRoutes {
			fmt.Printf("   • %s %s\n", route.Method, route.Path)
		}
	}

	fmt.Printf("\n %sGenerate Tests:%s %v\n", B_CYAN, RESET, ctx.Config.GenerateTests)
	fmt.Printf(" %sGenerate Model:%s %v\n", B_CYAN, RESET, ctx.Config.GenerateModel)

	fmt.Println(GRAY + "======================================================================" + RESET)
}

// askUserForConfirmation asks user to confirm before generation
func (ctx *ServiceContext) askUserForConfirmation(logger *Logger) error {
	if ctx.Config.DryRun {
		logger.Info("Dry run mode - skipping generation")
		return nil
	}

	// Check for method duplication
	conflicts, err := ctx.checkMethodDuplication(logger)
	if err != nil {
		logger.Warn("Error checking method duplication: %v", err)
	}

	if len(conflicts) > 0 {
		logger.Error("Method duplication detected!")
		fmt.Println("")
		fmt.Println(" The following methods already exist in other services:")
		for _, conflict := range conflicts {
			fmt.Printf("   %s⚠%s %s\n", B_RED, RESET, conflict)
		}
		fmt.Println("")
		logger.Warn("Please choose a different service name or modify your custom routes.")
		os.Exit(1)
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

// readTemplate reads a template from embedded filesystem
func (ctx *ServiceContext) readTemplate(templateName string) (string, error) {
	path := fmt.Sprintf("templates/%s.tmpl", templateName)
	content, err := templatesFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templateName, err)
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
		dependencyChecks.WriteString(`		helper := registry.NewServiceHelper(config, logger, deps)
		
		if !helper.IsServiceEnabled("` + configKey + `") {
			return nil
		}
		
`)

		for _, dep := range ctx.Config.Dependencies {
			varName := strings.ToLower(dep.Name[:1]) + dep.Name[1:]

			dependencyChecks.WriteString(fmt.Sprintf(`		%s, ok := registry.GetTyped[%s](deps, "%s")
		if !ok {
			logger.Warn("%s not available, skipping service")
			return nil
		}
		
`, varName, dep.Type, strings.ToLower(dep.Name[:1])+dep.Name[1:], dep.Name))

			dependencyParams.WriteString(fmt.Sprintf(", %s", varName))
		}
	} else {
		dependencyChecks.WriteString(`		helper := registry.NewServiceHelper(config, logger, deps)
		
		if !helper.IsServiceEnabled("` + configKey + `") {
			return nil
		}
		
`)
	}

	return fmt.Sprintf(`// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("%s", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
%s		return New%s(true%s, logger)
	})
}`, configKey, dependencyChecks.String(), ctx.Config.ServiceName, dependencyParams.String())
}

// buildSwaggerAnnotations builds Swagger annotations for routes
func (ctx *ServiceContext) buildSwaggerAnnotations() string {
	var annotations strings.Builder
	serviceNameLower := strings.ToLower(ctx.Config.ServiceName)

	// Standard CRUD annotations based on pattern
	switch ctx.Config.ServicePattern.Template {
	case "basic_crud":
		annotations.WriteString(ctx.buildSwaggerAnnotation("GET", "", "List "+serviceNameLower, "Get a list of all "+serviceNameLower, serviceNameLower, "200", "array"))
		annotations.WriteString(ctx.buildSwaggerAnnotation("GET", "/:id", "Get "+serviceNameLower[:len(serviceNameLower)-1], "Get a specific "+serviceNameLower[:len(serviceNameLower)-1]+" by ID", serviceNameLower, "200", "object"))
		annotations.WriteString(ctx.buildSwaggerAnnotation("POST", "", "Create "+serviceNameLower[:len(serviceNameLower)-1], "Create a new "+serviceNameLower[:len(serviceNameLower)-1], serviceNameLower, "201", "object"))
		annotations.WriteString(ctx.buildSwaggerAnnotation("PUT", "/:id", "Update "+serviceNameLower[:len(serviceNameLower)-1], "Update an existing "+serviceNameLower[:len(serviceNameLower)-1], serviceNameLower, "200", "object"))
		annotations.WriteString(ctx.buildSwaggerAnnotation("DELETE", "/:id", "Delete "+serviceNameLower[:len(serviceNameLower)-1], "Delete a "+serviceNameLower[:len(serviceNameLower)-1], serviceNameLower, "204", ""))

	case "read_only":
		annotations.WriteString(ctx.buildSwaggerAnnotation("GET", "", "List "+serviceNameLower, "Get a list of all "+serviceNameLower, serviceNameLower, "200", "array"))
		annotations.WriteString(ctx.buildSwaggerAnnotation("GET", "/:id", "Get "+serviceNameLower[:len(serviceNameLower)-1], "Get a specific "+serviceNameLower[:len(serviceNameLower)-1]+" by ID", serviceNameLower, "200", "object"))

	case "write_only":
		annotations.WriteString(ctx.buildSwaggerAnnotation("POST", "", "Create "+serviceNameLower[:len(serviceNameLower)-1], "Create a new "+serviceNameLower[:len(serviceNameLower)-1], serviceNameLower, "201", "object"))
		annotations.WriteString(ctx.buildSwaggerAnnotation("PUT", "/:id", "Update "+serviceNameLower[:len(serviceNameLower)-1], "Update an existing "+serviceNameLower[:len(serviceNameLower)-1], serviceNameLower, "200", "object"))

	case "event_driven":
		annotations.WriteString(ctx.buildSwaggerAnnotation("POST", "/publish", "Publish event", "Publish an event to the "+serviceNameLower, serviceNameLower, "200", "object"))
		annotations.WriteString(ctx.buildSwaggerAnnotation("GET", "/subscribe", "Subscribe to events", "Subscribe to "+serviceNameLower+" events", serviceNameLower, "200", "stream"))

	case "websocket":
		annotations.WriteString(ctx.buildSwaggerAnnotation("GET", "/ws", "WebSocket connection", "Establish WebSocket connection for "+serviceNameLower, serviceNameLower, "101", "websocket"))

	case "batch_processing":
		annotations.WriteString(ctx.buildSwaggerAnnotation("POST", "/batch", "Batch process", "Process multiple items in batch", serviceNameLower, "200", "object"))
		annotations.WriteString(ctx.buildSwaggerAnnotation("GET", "/batch/status", "Get batch status", "Get status of batch processing", serviceNameLower, "200", "object"))
	}

	// Custom route annotations
	for _, route := range ctx.Config.CustomRoutes {
		annotations.WriteString(ctx.buildSwaggerAnnotation(route.Method, route.Path, route.Summary, route.Description, serviceNameLower, "200", "object"))
	}

	return annotations.String()
}

func (ctx *ServiceContext) buildSwaggerAnnotation(method, path, summary, description, tag, successCode, successType string) string {
	produces := "application/json"
	if successType == "stream" {
		produces = "text/event-stream"
	} else if successType == "websocket" {
		produces = "text/plain"
	}

	annotation := fmt.Sprintf(`// @Summary %s
// @Description %s
// @Tags %s
// @Accept json
// @Produce %s
`, summary, description, tag, produces)

	if path != "" && strings.Contains(path, ":id") {
		annotation += fmt.Sprintf(`// @Param id path int true "Item ID"
`)
	}

	if method == "POST" || method == "PUT" {
		annotation += fmt.Sprintf(`// @Param request body interface{} true "Request body"
`)
	}

	if successType != "" {
		annotation += fmt.Sprintf(`// @Success %s {object} response.Response "Success"
`, successCode)
	}

	annotation += fmt.Sprintf(`// @Failure 400 {object} response.Response "Bad request"
// @Failure 500 {object} response.Response "Internal server error"
// @Router /%s%s [%s]
`, strings.ToLower(ctx.Config.ServiceName), path, strings.ToLower(method))

	return annotation + "\n"
}

// generateService generates the service Go file
func (ctx *ServiceContext) generateService(logger *Logger) error {
	logger.Info("Generating service file...")

	// Read the template based on service pattern
	template, err := ctx.readTemplate(ctx.Config.ServicePattern.Template)
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
	swaggerAnnotations := ctx.buildSwaggerAnnotations()

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
	content = strings.ReplaceAll(content, "{{SWAGGER_ANNOTATIONS}}", swaggerAnnotations)

	// Handle model generation
	if ctx.Config.GenerateModel {
		modelContent := ctx.generateModelCode()
		content = strings.ReplaceAll(content, "{{MODEL_CODE}}", modelContent)
	} else {
		content = strings.ReplaceAll(content, "{{MODEL_CODE}}", "")
	}

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

// generateModelCode generates GORM model code
func (ctx *ServiceContext) generateModelCode() string {
	modelName := ctx.Config.ServiceName[:len(ctx.Config.ServiceName)-1] // Remove 's' from service name
	serviceNameLower := strings.ToLower(ctx.Config.ServiceName)

	return fmt.Sprintf(`// %s represents the database model for %s
type %s struct {
	ID        uint   `+"`gorm:\"primaryKey\" json:\"id\"`"+`
	Name      string `+"`gorm:\"size:255;not null\" json:\"name\"`"+`
	CreatedAt time.Time `+"`json:\"created_at\"`"+`
	UpdatedAt time.Time `+"`json:\"updated_at\"`"+`
}

func (%s) TableName() string {
	return \"%s\"
}`, modelName, serviceNameLower, modelName, modelName, serviceNameLower)
}

// checkServiceExists checks if a service with the given name already exists
func (ctx *ServiceContext) checkServiceExists(serviceName string) (bool, error) {
	// Convert service name to file name format
	fileName := strings.ToLower(serviceName) + "_service.go"
	filePath := filepath.Join(ctx.ServicesDir, fileName)

	// Check if file exists
	if _, err := os.Stat(filePath); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// checkMethodDuplication checks if any methods in the new service would conflict with existing services
func (ctx *ServiceContext) checkMethodDuplication(logger *Logger) ([]string, error) {
	var conflicts []string

	// Get all service files in the modules directory
	entries, err := os.ReadDir(ctx.ServicesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Directory doesn't exist yet
		}
		return nil, err
	}

	// Collect all existing method names
	existingMethods := make(map[string]string) // method -> file

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_service.go") {
			continue
		}

		filePath := filepath.Join(ctx.ServicesDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Extract method names from the file
		methods := extractMethodNames(string(content))
		for _, method := range methods {
			if _, exists := existingMethods[method]; !exists {
				existingMethods[method] = entry.Name()
			}
		}
	}

	// Get methods that will be generated based on pattern
	newMethods := ctx.getPatternMethods()

	// Check for conflicts
	for _, method := range newMethods {
		if existingFile, exists := existingMethods[method]; exists {
			conflicts = append(conflicts, fmt.Sprintf("%s (already in %s)", method, existingFile))
		}
	}

	// Check custom routes for conflicts
	for _, route := range ctx.Config.CustomRoutes {
		handlerName := route.HandlerName
		if existingFile, exists := existingMethods[handlerName]; exists {
			conflicts = append(conflicts, fmt.Sprintf("%s (already in %s)", handlerName, existingFile))
		}
	}

	return conflicts, nil
}

// extractMethodNames extracts public method names from Go source code
func extractMethodNames(content string) []string {
	var methods []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for function definitions: func (s *ServiceName) MethodName(
		if strings.HasPrefix(line, "func (") {
			// Extract method name between ) and (
			parts := strings.Split(line, ") ")
			if len(parts) < 2 {
				continue
			}

			secondPart := parts[1]
			// Get the method name (before the opening parenthesis)
			if idx := strings.Index(secondPart, "("); idx > 0 {
				methodName := secondPart[:idx]
				// Only include exported methods (starting with uppercase)
				if len(methodName) > 0 && methodName[0] >= 'A' && methodName[0] <= 'Z' {
					methods = append(methods, methodName)
				}
			}
		}
	}

	return methods
}

// getPatternMethods returns the method names that will be generated for the current pattern
func (ctx *ServiceContext) getPatternMethods() []string {
	serviceName := ctx.Config.ServiceName
	var methods []string

	switch ctx.Config.ServicePattern.Template {
	case "basic_crud":
		methods = []string{
			"List" + serviceName,
			"Get" + serviceName[:len(serviceName)-1],
			"Create" + serviceName[:len(serviceName)-1],
			"Update" + serviceName[:len(serviceName)-1],
			"Delete" + serviceName[:len(serviceName)-1],
		}
	case "read_only":
		methods = []string{
			"List" + serviceName,
			"Get" + serviceName[:len(serviceName)-1],
		}
	case "write_only":
		methods = []string{
			"Create" + serviceName[:len(serviceName)-1],
			"Update" + serviceName[:len(serviceName)-1],
		}
	case "event_driven":
		methods = []string{
			"Publish" + serviceName,
			"Subscribe" + serviceName,
		}
	case "websocket":
		methods = []string{
			"HandleWebSocket" + serviceName,
		}
	case "batch_processing":
		methods = []string{
			"BatchProcess" + serviceName,
			"GetBatchStatus" + serviceName,
		}
	}

	return methods
}

// generateTestFile generates the test file
func (ctx *ServiceContext) generateTestFile(logger *Logger) error {
	if !ctx.Config.GenerateTests {
		return nil
	}

	logger.Info("Generating test file...")

	// Read the test template
	template, err := ctx.readTemplate("test")
	if err != nil {
		return err
	}

	content := template
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

// displaySummary displays the generation summary
func (ctx *ServiceContext) displaySummary(logger *Logger) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "GENERATION SUMMARY" + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)

	fmt.Printf(" %s✓%s Service file created: %s\n", B_GREEN, RESET, ctx.Config.FileName)
	fmt.Printf(" %s✓%s Service struct: %s\n", B_GREEN, RESET, ctx.Config.ServiceName)
	fmt.Printf(" %s✓%s Wire name: %s\n", B_GREEN, RESET, ctx.Config.WireName)
	fmt.Printf(" %s✓%s Service pattern: %s\n", B_GREEN, RESET, ctx.Config.ServicePattern.Name)
	fmt.Printf(" %s✓%s Auto-registration: Enabled\n", B_GREEN, RESET)
	fmt.Printf(" %s✓%s Swagger annotations: Generated\n", B_GREEN, RESET)

	if len(ctx.Config.Dependencies) > 0 {
		fmt.Printf(" %s✓%s Dependencies: %d configured\n", B_GREEN, RESET, len(ctx.Config.Dependencies))
	}

	if ctx.Config.GenerateModel {
		fmt.Printf(" %s✓%s Database model: Generated\n", B_GREEN, RESET)
	}

	if ctx.Config.GenerateTests {
		fmt.Printf(" %s✓%s Test file: Generated\n", B_GREEN, RESET)
	}

	fmt.Println("")
	fmt.Println(" " + P_CYAN + "Next steps:" + RESET)
	fmt.Println("   1. Add service to config.yaml:")
	fmt.Printf("      services:\n        %s: true\n", strings.ToLower(ctx.Config.ServiceName)+"_service")
	fmt.Println("")
	fmt.Println("   2. Implement business logic in handler methods")
	fmt.Println("   3. Regenerate Swagger docs: go run scripts/swagger/swagger.go")
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
	fmt.Println("   " + P_PURPLE + "(  )" + RESET + "   " + B_PURPLE + "Service Generator" + RESET + " " + GRAY + "for" + RESET + " " + B_WHITE + "stackyrd" + RESET)
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
		{"Selecting service pattern", ctx.promptServicePattern},
		{"Prompting for dependencies", ctx.promptDependencies},
		{"Prompting for test generation", ctx.promptGenerateTests},
		{"Prompting for database model", ctx.promptGenerateModel},
		{"Prompting for custom routes", ctx.promptCustomRoutes},
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
