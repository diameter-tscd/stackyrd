package main

import (
	"bytes"
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
	"github.com/charmbracelet/lipgloss"
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

var ServicePatterns = []ServicePattern{
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
	ServiceName    string
	WireName       string
	FileName       string
	GenerateTests  bool
	GenerateModel  bool
	ServicePattern ServicePattern
	CustomRoutes   []CustomRoute
	Verbose        bool
	DryRun         bool
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
	writer  io.Writer
}

func (l *Logger) Info(msg string, args ...interface{}) {
	fmt.Fprintf(l.writer, "%s[INFO]%s %s\n", B_CYAN, RESET, fmt.Sprintf(msg, args...))
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	fmt.Fprintf(l.writer, "%s[WARN]%s %s\n", B_YELLOW, RESET, fmt.Sprintf(msg, args...))
}

func (l *Logger) Error(msg string, args ...interface{}) {
	fmt.Fprintf(l.writer, "%s[ERROR]%s %s\n", B_RED, RESET, fmt.Sprintf(msg, args...))
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.verbose {
		fmt.Fprintf(l.writer, "%s[DEBUG]%s %s\n", GRAY, RESET, fmt.Sprintf(msg, args...))
	}
}

func (l *Logger) Success(msg string, args ...interface{}) {
	fmt.Fprintf(l.writer, "%s[SUCCESS]%s %s\n", B_GREEN, RESET, fmt.Sprintf(msg, args...))
}

func (l *Logger) Prompt(msg string, args ...interface{}) {
	fmt.Fprintf(l.writer, "%s[PROMPT]%s %s", B_YELLOW, RESET, fmt.Sprintf(msg, args...))
}

// NewLogger creates a new logger
func NewLogger(verbose bool) *Logger {
	return &Logger{verbose: verbose, writer: os.Stdout}
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

// promptServiceName prompts for the service name (CLI mode)
func (ctx *ServiceContext) promptServiceName(logger *Logger) error {
	for {
		logger.Prompt("Enter service name (e.g., Orders, Inventory): ")

		var serviceName string
		_, _ = fmt.Scanln(&serviceName)

		if serviceName == "" {
			logger.Error("Service name cannot be empty")
			continue
		}

		serviceName = strings.ToUpper(serviceName[:1]) + serviceName[1:]

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

// promptWireName prompts for the wire name (CLI mode)
func (ctx *ServiceContext) promptWireName(logger *Logger) error {
	defaultWireName := strings.ToLower(ctx.Config.ServiceName) + "-service"
	logger.Prompt("Enter wire name (default: %s): ", defaultWireName)

	var wireName string
	_, _ = fmt.Scanln(&wireName)

	if wireName == "" {
		wireName = defaultWireName
	}

	ctx.Config.WireName = wireName

	logger.Success("Wire name: %s", wireName)
	return nil
}

// promptFileName prompts for the file name (CLI mode)
func (ctx *ServiceContext) promptFileName(logger *Logger) error {
	defaultFileName := strings.ToLower(ctx.Config.ServiceName) + "_service.go"
	logger.Prompt("Enter file name (default: %s): ", defaultFileName)

	var fileName string
	_, _ = fmt.Scanln(&fileName)

	if fileName == "" {
		fileName = defaultFileName
	}

	if !strings.HasSuffix(fileName, ".go") {
		fileName += ".go"
	}

	ctx.Config.FileName = fileName

	logger.Success("File name: %s", fileName)
	return nil
}

// promptServicePattern prompts for service pattern selection (CLI mode)
func (ctx *ServiceContext) promptServicePattern(logger *Logger) error {
	logger.Info("Select service pattern:")
	fmt.Println("")

	for i, pattern := range ServicePatterns {
		fmt.Printf("  %s[%d]%s %s%s%s - %s\n", B_CYAN, i+1, RESET, B_WHITE, pattern.Name, RESET, pattern.Description)
	}

	fmt.Println("")

	logger.Prompt("Enter pattern number (default: 1): ")

	var input string
	_, _ = fmt.Scanln(&input)

	if input == "" {
		ctx.Config.ServicePattern = ServicePatterns[0]
	} else {
		var idx int
		if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(ServicePatterns) {
			logger.Warn("Invalid pattern, using Basic CRUD")
			idx = 1
		}
		ctx.Config.ServicePattern = ServicePatterns[idx-1]
	}

	logger.Success("Selected pattern: %s", ctx.Config.ServicePattern.Name)
	return nil
}

// promptGenerateTests prompts for test file generation (CLI mode)
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

// promptGenerateModel prompts for database model generation (CLI mode)
func (ctx *ServiceContext) promptGenerateModel(logger *Logger) error {
	logger.Prompt("Generate database model (GORM)? (y/N, default: N): ")

	var input string
	_, _ = fmt.Scanln(&input)

	if strings.ToLower(input) == "y" || strings.ToLower(input) == "yes" {
		ctx.Config.GenerateModel = true
		logger.Success("Database model will be generated")
	} else {
		ctx.Config.GenerateModel = false
		logger.Info("Skipping database model generation")
	}

	return nil
}

// promptCustomRoutes prompts for custom routes (CLI mode)
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
	return ", nil"
}

// displayConfiguration displays the service configuration
func (ctx *ServiceContext) displayConfiguration(logger *Logger) {
	fmt.Fprintln(logger.writer, "")
	fmt.Fprintln(logger.writer, GRAY+"======================================================================"+RESET)
	fmt.Fprintln(logger.writer, " "+B_PURPLE+"SERVICE CONFIGURATION"+RESET)
	fmt.Fprintln(logger.writer, GRAY+"======================================================================"+RESET)

	fmt.Fprintf(logger.writer, " %sService Name:%s %s\n", B_CYAN, RESET, ctx.Config.ServiceName)
	fmt.Fprintf(logger.writer, " %sWire Name:%s %s\n", B_CYAN, RESET, ctx.Config.WireName)
	fmt.Fprintf(logger.writer, " %sFile Name:%s %s\n", B_CYAN, RESET, ctx.Config.FileName)
	fmt.Fprintf(logger.writer, " %sPattern:%s %s\n", B_CYAN, RESET, ctx.Config.ServicePattern.Name)
	fmt.Fprintf(logger.writer, " %sFile Path:%s %s\n", B_CYAN, RESET, filepath.Join(ctx.ServicesDir, ctx.Config.FileName))

	if len(ctx.Config.CustomRoutes) > 0 {
		fmt.Fprintf(logger.writer, "\n %sCustom Routes:%s\n", B_CYAN, RESET)
		for _, route := range ctx.Config.CustomRoutes {
			fmt.Fprintf(logger.writer, "   \u2022 %s %s\n", route.Method, route.Path)
		}
	}

	fmt.Fprintf(logger.writer, "\n %sGenerate Tests:%s %v\n", B_CYAN, RESET, ctx.Config.GenerateTests)
	fmt.Fprintf(logger.writer, " %sGenerate Model:%s %v\n", B_CYAN, RESET, ctx.Config.GenerateModel)

	fmt.Fprintln(logger.writer, GRAY+"======================================================================"+RESET)
}

// askUserForConfirmation asks user to confirm before generation (CLI mode)
func (ctx *ServiceContext) askUserForConfirmation(logger *Logger) error {
	if ctx.Config.DryRun {
		logger.Info("Dry run mode - skipping generation")
		return nil
	}

	conflicts, err := ctx.checkMethodDuplication(logger)
	if err != nil {
		logger.Warn("Error checking method duplication: %v", err)
	}

	if len(conflicts) > 0 {
		logger.Error("Method duplication detected!")
		fmt.Println("")
		fmt.Println(" The following methods already exist in other services:")
		for _, conflict := range conflicts {
			fmt.Printf("   %s\u26a0%s %s\n", B_RED, RESET, conflict)
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
	return ""
}

// buildFields builds the struct fields
func (ctx *ServiceContext) buildFields() string {
	return ""
}

// buildParams builds the constructor parameters
func (ctx *ServiceContext) buildParams() string {
	return ""
}

// buildAssignments builds the constructor assignments
func (ctx *ServiceContext) buildAssignments() string {
	return ""
}

// buildInitFunction builds the init function for auto-registration
func (ctx *ServiceContext) buildInitFunction() string {
	configKey := strings.ToLower(ctx.Config.ServiceName) + "_service"

	return fmt.Sprintf(`func init() {
	registry.RegisterService("%s", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		helper := registry.NewServiceHelper(config, logger, deps)

		if !helper.IsServiceEnabled("%s") {
			return nil
		}

		return New%s(true, logger)
	})
}`, configKey, configKey, ctx.Config.ServiceName)
}

// buildSwaggerAnnotations builds Swagger annotations for routes
func (ctx *ServiceContext) buildSwaggerAnnotations() string {
	var annotations strings.Builder
	serviceNameLower := strings.ToLower(ctx.Config.ServiceName)

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

	for _, route := range ctx.Config.CustomRoutes {
		annotations.WriteString(ctx.buildSwaggerAnnotation(route.Method, route.Path, route.Summary, route.Description, serviceNameLower, "200", "object"))
	}

	return annotations.String()
}

func (ctx *ServiceContext) buildSwaggerAnnotation(method, path, summary, description, tag, successCode, successType string) string {
	produces := "application/json"
	switch successType {
	case "stream":
		produces = "text/event-stream"
	case "websocket":
		produces = "text/plain"
	}

	annotation := fmt.Sprintf(`// @Summary %s
// @Description %s
// @Tags %s
// @Accept json
// @Produce %s
`, summary, description, tag, produces)

	if path != "" && strings.Contains(path, ":id") {
		annotation += `// @Param id path int true "Item ID"
`
	}

	if method == "POST" || method == "PUT" {
		annotation += `// @Param request body interface{} true "Request body"
`
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

	template, err := ctx.readTemplate(ctx.Config.ServicePattern.Template)
	if err != nil {
		return err
	}

	imports := ctx.buildImports()
	fields := ctx.buildFields()
	params := ctx.buildParams()
	assignments := ctx.buildAssignments()
	initFunction := ctx.buildInitFunction()
	serviceNameLower := strings.ToLower(ctx.Config.ServiceName)
	swaggerAnnotations := ctx.buildSwaggerAnnotations()

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

	if ctx.Config.GenerateModel {
		modelContent := ctx.generateModelCode()
		content = strings.ReplaceAll(content, "{{MODEL_CODE}}", modelContent)
	} else {
		content = strings.ReplaceAll(content, "{{MODEL_CODE}}", "")
	}

	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")

	filePath := filepath.Join(ctx.ServicesDir, ctx.Config.FileName)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	logger.Success("Service file generated: %s", filePath)
	return nil
}

// generateModelCode generates GORM model code
func (ctx *ServiceContext) generateModelCode() string {
	modelName := ctx.Config.ServiceName[:len(ctx.Config.ServiceName)-1]
	serviceNameLower := strings.ToLower(ctx.Config.ServiceName)

	return fmt.Sprintf(`// %s represents the database model for %s
type %s struct {
	ID        uint   `+"`gorm:\"primaryKey\" json:\"id\"`"+`
	Name      string `+"`gorm:\"size:255;not null\" json:\"name\"`"+`
	CreatedAt time.Time `+"`json:\"created_at\"`"+`
	UpdatedAt time.Time `+"`json:\"updated_at\"`"+`
}

func (%s) TableName() string {
	return "%s"
}`, modelName, serviceNameLower, modelName, modelName, serviceNameLower)
}

// checkServiceExists checks if a service with the given name already exists
func (ctx *ServiceContext) checkServiceExists(serviceName string) (bool, error) {
	fileName := strings.ToLower(serviceName) + "_service.go"
	filePath := filepath.Join(ctx.ServicesDir, fileName)

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

	entries, err := os.ReadDir(ctx.ServicesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	existingMethods := make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_service.go") {
			continue
		}

		filePath := filepath.Join(ctx.ServicesDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		methods := extractMethodNames(string(content))
		for _, method := range methods {
			if _, exists := existingMethods[method]; !exists {
				existingMethods[method] = entry.Name()
			}
		}
	}

	newMethods := ctx.getPatternMethods()

	for _, method := range newMethods {
		if existingFile, exists := existingMethods[method]; exists {
			conflicts = append(conflicts, fmt.Sprintf("%s (already in %s)", method, existingFile))
		}
	}

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
		if strings.HasPrefix(line, "func (") {
			parts := strings.Split(line, ") ")
			if len(parts) < 2 {
				continue
			}

			secondPart := parts[1]
			if idx := strings.Index(secondPart, "("); idx > 0 {
				methodName := secondPart[:idx]
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

	template, err := ctx.readTemplate("test")
	if err != nil {
		return err
	}

	content := template
	content = strings.ReplaceAll(content, "{{SERVICE_NAME}}", ctx.Config.ServiceName)
	content = strings.ReplaceAll(content, "{{SERVICE_NAME_LOWER}}", strings.ToLower(ctx.Config.ServiceName))
	content = strings.ReplaceAll(content, "{{WIRE_NAME}}", ctx.Config.WireName)
	content = strings.ReplaceAll(content, "{{CONSTRUCTOR_ARGS}}", ctx.buildConstructorArgs())

	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")

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
	fmt.Fprintln(logger.writer, "")
	fmt.Fprintln(logger.writer, GRAY+"======================================================================"+RESET)
	fmt.Fprintln(logger.writer, " "+B_PURPLE+"GENERATION SUMMARY"+RESET)
	fmt.Fprintln(logger.writer, GRAY+"======================================================================"+RESET)

	fmt.Fprintf(logger.writer, " %s\u2713%s Service file created: %s\n", B_GREEN, RESET, ctx.Config.FileName)
	fmt.Fprintf(logger.writer, " %s\u2713%s Service struct: %s\n", B_GREEN, RESET, ctx.Config.ServiceName)
	fmt.Fprintf(logger.writer, " %s\u2713%s Wire name: %s\n", B_GREEN, RESET, ctx.Config.WireName)
	fmt.Fprintf(logger.writer, " %s\u2713%s Service pattern: %s\n", B_GREEN, RESET, ctx.Config.ServicePattern.Name)
	fmt.Fprintf(logger.writer, " %s\u2713%s Auto-registration: Enabled\n", B_GREEN, RESET)
	fmt.Fprintf(logger.writer, " %s\u2713%s Swagger annotations: Generated\n", B_GREEN, RESET)

	if ctx.Config.GenerateModel {
		fmt.Fprintf(logger.writer, " %s\u2713%s Database model: Generated\n", B_GREEN, RESET)
	}

	if ctx.Config.GenerateTests {
		fmt.Fprintf(logger.writer, " %s\u2713%s Test file: Generated\n", B_GREEN, RESET)
	}

	fmt.Fprintln(logger.writer, "")
	fmt.Fprintln(logger.writer, " "+P_CYAN+"Next steps:"+RESET)
	fmt.Fprintln(logger.writer, "   1. Add service to config.yaml:")
	fmt.Fprintf(logger.writer, "      services:\n        %s: true\n", strings.ToLower(ctx.Config.ServiceName)+"_service")
	fmt.Fprintln(logger.writer, "")
	fmt.Fprintln(logger.writer, "   2. Implement business logic in handler methods")
	fmt.Fprintln(logger.writer, "   3. Regenerate Swagger docs: go run scripts/swagger/swagger.go")
	fmt.Fprintln(logger.writer, "   4. Test the service endpoints")
	fmt.Fprintln(logger.writer, "")
	fmt.Fprintln(logger.writer, GRAY+"======================================================================"+RESET)
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

// ─── Terminal Safety Guard ──────────────────────────────────────────────────────

// ttyGuard saves terminal state before entering TUI mode and restores it on exit.
// It is a last-resort safety net for abnormal exits (panic, uncatchable signal).
// It does NOT interfere with bubbletea's own terminal management.
type ttyGuard struct {
	fd       int
	oldState *term.State
}

func (g *ttyGuard) Save() error {
	g.fd = int(os.Stdin.Fd())
	oldState, err := term.GetState(g.fd)
	if err != nil {
		return fmt.Errorf("term.GetState: %w", err)
	}
	g.oldState = oldState
	return nil
}

func (g *ttyGuard) Restore() {
	if g.oldState == nil {
		return
	}
	_ = term.Restore(g.fd, g.oldState)
	g.oldState = nil
	// Leave alternate screen buffer if stuck in it.
	fmt.Fprint(os.Stderr, "\033[?1049l")
}

// setupTUISignalHandler catches signals during TUI mode and restores the terminal.
// Returns a channel that is closed when TUI mode ends (caller must close it).
func setupTUISignalHandler(guard *ttyGuard) chan struct{} {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		select {
		case sig := <-sigCh:
			guard.Restore()
			fmt.Fprintf(os.Stderr, "\r\nReceived %v\n", sig)
			os.Exit(128 + int(sig.(syscall.Signal)))
		case <-done:
			signal.Stop(sigCh)
		}
	}()
	return done
}

// ─── TUI Types ──────────────────────────────────────────────────────────────────

type stepStatus int

const (
	statusPending stepStatus = iota
	statusRunning
	statusSuccess
	statusError
	statusSkipped
)

type promptType int

const (
	promptNone promptType = iota
	promptYesNo
	promptText
	promptSelect
	promptConfirm
)

type stepInfo struct {
	name        string
	status      stepStatus
	message     string
	prompt      promptType
	promptLabel string
	promptDef   string
	defVal      bool
	action      func(*ServiceContext, *Logger) error
}

type (
	tickMsg          time.Time
	promptStepDoneMsg struct {
		index int
		err   error
		msg   string
	}
	promptTimeoutMsg struct{ index int }
	doneTimeoutMsg   struct{}
)

type ServiceTuiModel struct {
	steps       []stepInfo
	current     int
	spinner     spinner.Model
	ctx         *ServiceContext
	logger      *Logger
	width       int
	height      int
	started     time.Time
	done        bool
	success     bool
	doneAt      time.Time
	completedIn time.Duration

	promptActive  bool
	promptStarted time.Time

	textInput     textinput.Model
	textInputStep int

	selectIdx int

	ready    bool
	quitting bool

	banner string
	log    *logState

	stepPipeR *os.File
	stepPipeW *os.File
}

type logState struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func (s *logState) append(line string) {
	s.mu.Lock()
	s.lines = append(s.lines, line)
	if len(s.lines) > s.max {
		s.lines = s.lines[len(s.lines)-s.max:]
	}
	s.mu.Unlock()
}

func (s *logState) visible(n int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.lines) <= n {
		out := make([]string, len(s.lines))
		copy(out, s.lines)
		return out
	}
	out := make([]string, n)
	copy(out, s.lines[len(s.lines)-n:])
	return out
}

type logCaptureWriter struct {
	log *logState
}

func (w *logCaptureWriter) Write(p []byte) (n int, err error) {
	clean := stripANSI(string(p))
	lines := strings.Split(strings.TrimRight(clean, "\r\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			w.log.append(trimmed)
		}
	}
	return len(p), nil
}

func stripANSI(s string) string {
	var b bytes.Buffer
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				i = j + 1
			} else {
				i = j
			}
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// ─── TUI Styles ─────────────────────────────────────────────────────────────────

var svcBannerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#8daea5"))

var svcSubStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6272A4")).
	Italic(true)

var svcStepNameStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F8F8F2")).
	Width(34)

var svcStepNameBoldStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFB86C")).
	Bold(true).
	Width(34)

var svcIconStyle = lipgloss.NewStyle().
	Width(2)

var svcMsgStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C0C0C0"))

var svcErrorMsgStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FF5555"))

var svcSuccessStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#50FA7B")).
	Bold(true)

var svcPromptStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFB86C")).
	Bold(true)

var svcFooterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6272A4"))

var svcDividerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#44475A"))

var svcLogHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#6272A4"))

var svcLogLineStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C0C0C0"))

var svcSummaryLineStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F8F8F2"))

var svcSummaryHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#50FA7B"))

var svcPatternStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFB86C"))

var svcPatternDescStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C0C0C0"))

func svcDivider(width int) string {
	return svcDividerStyle.Render(strings.Repeat("\u2500", width))
}

func readSvcBanner(projectDir string) string {
	data, err := os.ReadFile(filepath.Join(projectDir, "pkg", "assets", "banner.txt"))
	if err != nil {
		return "  stackyrd"
	}
	return string(data)
}

// ─── TUI Model ──────────────────────────────────────────────────────────────────

func NewServiceTuiModel(ctx *ServiceContext, logger *Logger) ServiceTuiModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))

	ti := textinput.New()
	ti.Placeholder = ""
	ti.Prompt = "\u25b6 "
	ti.CharLimit = 64
	ti.Width = 40

	stepDefs := []struct {
		name   string
		action func(*ServiceContext, *Logger) error
		prompt promptType
		label  string
		defVal string
		def    bool
	}{
		{"Find Project Root", (*ServiceContext).ensureProjectRoot, promptNone, "", "", false},
		{"Service Name", nil, promptText, "Enter service name (e.g., Orders, Inventory)", "", false},
		{"Wire Name", nil, promptText, "Enter wire name", "", false},
		{"File Name", nil, promptText, "Enter file name", "", false},
		{"Service Pattern", nil, promptSelect, "Select service pattern", "", false},
		{"Generate Tests", nil, promptYesNo, "Generate test file?", "N", false},
		{"Generate Model", nil, promptYesNo, "Generate database model (GORM)?", "N", false},
		{"Custom Routes", nil, promptYesNo, "Add custom routes?", "N", false},
		{"Display Configuration", nil, promptNone, "", "", false},
		{"Confirm Generation", nil, promptConfirm, "Proceed with generation?", "Y", true},
		{"Generate Service File", (*ServiceContext).generateService, promptNone, "", "", false},
		{"Generate Test File", (*ServiceContext).generateTestFile, promptNone, "", "", false},
		{"Display Summary", nil, promptNone, "", "", false},
	}

	steps := make([]stepInfo, len(stepDefs))
	for i, sd := range stepDefs {
		steps[i] = stepInfo{
			name:        sd.name,
			status:      statusPending,
			prompt:      sd.prompt,
			promptLabel: sd.label,
			promptDef:   sd.defVal,
			defVal:      sd.def,
			action:      sd.action,
		}
	}

	return ServiceTuiModel{
		steps:    steps,
		ctx:      ctx,
		logger:   logger,
		spinner:  s,
		textInput: ti,
		started:  time.Now(),
		banner:   readSvcBanner(ctx.ProjectDir),
		log: &logState{
			lines: make([]string, 0, 100),
			max:   100,
		},
	}
}

func (m ServiceTuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickSvcCmd(),
		func() tea.Msg {
			return tea.WindowSizeMsg{Width: 100, Height: 30}
		},
	)
}

func tickSvcCmd() tea.Cmd {
	return tea.Every(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m ServiceTuiModel) runStepCmd(index int) tea.Cmd {
	step := m.steps[index]
	return func() tea.Msg {
		if m.stepPipeW != nil {
			oldOut := os.Stdout
			oldErr := os.Stderr
			os.Stdout = m.stepPipeW
			os.Stderr = m.stepPipeW
			err := step.action(m.ctx, m.logger)
			os.Stdout = oldOut
			os.Stderr = oldErr
			msg := ""
			if err == nil {
				msg = "Done"
			} else {
				msg = err.Error()
			}
			time.Sleep(20 * time.Millisecond)
			return promptStepDoneMsg{index: index, err: err, msg: msg}
		}
		err := step.action(m.ctx, m.logger)
		msg := ""
		if err == nil {
			msg = "Done"
		} else {
			msg = err.Error()
		}
		return promptStepDoneMsg{index: index, err: err, msg: msg}
	}
}

func (m ServiceTuiModel) startPromptTimeout(index int) tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return promptTimeoutMsg{index: index}
	})
}

func (m *ServiceTuiModel) advanceToNext() tea.Cmd {
	m.current++
	if m.current >= len(m.steps) {
		return m.setDone(true)
	}
	return m.triggerCurrentStep()
}

func (m *ServiceTuiModel) setDone(success bool) tea.Cmd {
	m.done = true
	m.success = success
	m.doneAt = time.Now()
	m.completedIn = time.Since(m.started).Round(time.Millisecond)
	return tea.Tick(15*time.Second, func(t time.Time) tea.Msg { return doneTimeoutMsg{} })
}

func (m *ServiceTuiModel) triggerCurrentStep() tea.Cmd {
	step := &m.steps[m.current]

	if step.status == statusSkipped {
		return m.advanceToNext()
	}

	if step.prompt != promptNone && step.status == statusPending {
		step.status = statusRunning
		m.promptActive = true
		m.promptStarted = time.Now()

		switch step.prompt {
		case promptText:
			m.textInput.Reset()
			m.textInput.Placeholder = step.promptDef
			m.textInput.Focus()
			m.textInputStep = m.current
			return textinput.Blink
		case promptYesNo:
			return nil
		case promptSelect:
			return nil
		case promptConfirm:
			m.promptStarted = time.Now()
			return m.startPromptTimeout(m.current)
		}
	}

	if step.action == nil {
		step.status = statusSuccess
		step.message = "ok"
		return m.advanceToNext()
	}

	step.status = statusRunning
	return m.runStepCmd(m.current)
}

func (m ServiceTuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.done {
			m.quitting = true
			return m, tea.Quit
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.promptActive {
				m.promptActive = false
				s := &m.steps[m.current]
				_ = s
				// Treat as cancel
				m.quitting = true
				return m, tea.Quit
			}
			m.quitting = true
			return m, tea.Quit
		}

		if m.promptActive {
			s := &m.steps[m.current]

			switch s.prompt {
			case promptText:
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				if msg.String() == "enter" {
					val := strings.TrimSpace(m.textInput.Value())
					if val == "" && m.textInput.Placeholder != "" {
						val = m.textInput.Placeholder
					}
					if val != "" || m.textInput.Placeholder == "" {
						m.textInput.Blur()
						m.promptActive = false

						switch m.current {
						case 1: // Service Name
							if val == "" {
								return m, nil
							}
							val = strings.ToUpper(val[:1]) + val[1:]
							exists, err := m.ctx.checkServiceExists(val)
							if err != nil {
								s.status = statusError
								s.message = err.Error()
								return m, m.setDone(false)
							}
							if exists {
								m.textInput.Reset()
								m.textInput.Placeholder = val + " (already exists)"
								m.textInput.Focus()
								m.promptActive = true
								return m, textinput.Blink
							}
							m.ctx.Config.ServiceName = val
							m.ctx.Config.WireName = strings.ToLower(val) + "-service"
							m.ctx.Config.FileName = strings.ToLower(val) + "_service.go"
							s.status = statusSuccess
							s.message = val
							return m, m.advanceToNext()

						case 2: // Wire Name
							if val == "" {
								val = m.ctx.Config.WireName
							}
							m.ctx.Config.WireName = val
							s.status = statusSuccess
							s.message = val
							return m, m.advanceToNext()

						case 3: // File Name
							if val == "" {
								val = m.ctx.Config.FileName
							}
							if !strings.HasSuffix(val, ".go") {
								val += ".go"
							}
							m.ctx.Config.FileName = val
							s.status = statusSuccess
							s.message = val
							return m, m.advanceToNext()

						}
					}
				}
				return m, cmd

			case promptYesNo:
				switch msg.String() {
				case "y", "Y":
					m.promptActive = false
					s.status = statusSuccess
					switch m.current {
					case 5:
						m.ctx.Config.GenerateTests = true
						s.message = "yes"
					case 6:
						m.ctx.Config.GenerateModel = true
						s.message = "yes"
					case 7:
						m.ctx.Config.CustomRoutes = nil
						s.message = "yes"
					}
					return m, m.advanceToNext()
				case "n", "N", "enter":
					m.promptActive = false
					s.status = statusSuccess
					switch m.current {
					case 5:
						m.ctx.Config.GenerateTests = false
						s.message = "no"
					case 6:
						m.ctx.Config.GenerateModel = false
						s.message = "no"
					case 7:
						s.message = "no"
					}
					return m, m.advanceToNext()
				}

			case promptSelect:
				num := -1
				switch msg.String() {
				case "1":
					num = 0
				case "2":
					num = 1
				case "3":
					num = 2
				case "4":
					num = 3
				case "5":
					num = 4
				case "6":
					num = 5
				case "enter":
					num = 0
				}
				if num >= 0 && num < len(ServicePatterns) {
					m.promptActive = false
					m.selectIdx = num
					m.ctx.Config.ServicePattern = ServicePatterns[num]
					s.status = statusSuccess
					s.message = fmt.Sprintf("%d: %s", num+1, ServicePatterns[num].Name)
					return m, m.advanceToNext()
				}

		case promptConfirm:
			switch msg.String() {
			case "y", "Y", "enter":
				conflicts, _ := m.ctx.checkMethodDuplication(m.logger)
				if len(conflicts) > 0 {
					m.promptActive = false
					s.status = statusError
					s.message = "Method conflicts detected"
					return m, m.setDone(false)
				}
				m.promptActive = false
				s.status = statusSuccess
				s.message = "confirmed"
				return m, m.advanceToNext()
				case "n", "N":
					m.promptActive = false
					s.status = statusSuccess
					s.message = "cancelled"
					return m, m.setDone(true)
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.ready = true
			return m, m.triggerCurrentStep()
		}

	case tickMsg:
		if m.done {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, tea.Batch(cmd, tickSvcCmd())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case promptStepDoneMsg:
		if msg.index < len(m.steps) {
			s := &m.steps[msg.index]
			if msg.err != nil {
				s.status = statusError
				s.message = msg.msg
				return m, m.setDone(false)
			}
			s.status = statusSuccess
			s.message = msg.msg
			return m, m.advanceToNext()
		}

	case promptTimeoutMsg:
		if m.promptActive && msg.index == m.current {
			m.promptActive = false
			s := &m.steps[msg.index]
			if s.prompt == promptConfirm {
				s.status = statusSuccess
				s.message = "proceeding (timeout)"
				return m, m.advanceToNext()
			}
		}

	case doneTimeoutMsg:
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m ServiceTuiModel) View() string {
	if !m.ready {
		return ""
	}

	var b strings.Builder

	if m.banner != "" {
		lines := strings.Split(strings.TrimRight(m.banner, "\n"), "\n")
		for _, l := range lines {
			trimmed := strings.TrimRight(l, " ")
			if trimmed != "" {
				b.WriteString(svcBannerStyle.Render("  " + trimmed))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(svcBannerStyle.Render("  stackyrd Service Generator"))
	b.WriteString("\n")
	b.WriteString(svcSubStyle.Render("  by diameter-tscd"))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(svcDivider(min(m.width, 80)))
	b.WriteString("\n")

	for i, s := range m.steps {
		var icon, statusText, label string
		label = s.name

		switch s.status {
		case statusPending:
			icon = svcIconStyle.Render(" ")
			statusText = svcMsgStyle.Render("waiting")
		case statusRunning:
			if s.prompt != promptNone && m.promptActive {
				icon = svcIconStyle.Render("?")

				switch s.prompt {
				case promptText:
					inputVal := m.textInput.View()
					promptLine := s.promptLabel
					if m.current == 1 {
						promptLine = fmt.Sprintf("%s: %s", s.promptLabel, inputVal)
					} else if m.current == 2 {
						defVal := m.ctx.Config.WireName
						promptLine = fmt.Sprintf("%s (default: %s): %s", s.promptLabel, defVal, inputVal)
					} else if m.current == 3 {
						defVal := m.ctx.Config.FileName
						promptLine = fmt.Sprintf("%s (default: %s): %s", s.promptLabel, defVal, inputVal)
					}
					statusText = svcPromptStyle.Render(promptLine)

				case promptYesNo:
					statusText = svcPromptStyle.Render(s.promptLabel + " (y/N)")

				case promptSelect:
					var patternLines strings.Builder
					patternLines.WriteString("Select pattern (1-6): ")
					for pi, pat := range ServicePatterns {
						delim := " "
						if pi > 0 {
							delim = ", "
						}
						if pi == m.selectIdx {
							patternLines.WriteString(fmt.Sprintf("%s[%d] %s", delim, pi+1, pat.Name))
						} else {
							patternLines.WriteString(fmt.Sprintf("%s[%d] %s", delim, pi+1, pat.Name))
						}
					}
					statusText = svcPromptStyle.Render(patternLines.String())

				case promptConfirm:
					elapsed := time.Since(m.promptStarted)
					remaining := 10*time.Second - elapsed
					if remaining < 0 {
						remaining = 0
					}
					secs := int(remaining.Seconds())
					statusText = svcPromptStyle.Render(fmt.Sprintf("%s (Y/n) [%ds]", s.promptLabel, secs))
				}
			} else {
				icon = svcIconStyle.Render(m.spinner.View())
				statusText = svcMsgStyle.Render("running...")
			}
		case statusSuccess:
			icon = svcIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Render("*"))
			if s.message == "Done" || s.message == "" {
				statusText = svcSuccessStyle.Render("ok")
			} else if s.message == "yes" {
				statusText = svcSuccessStyle.Render("enabled")
			} else if s.message == "no" || s.message == "confirmed" || s.message == "none" {
				statusText = svcMsgStyle.Render(s.message)
			} else if strings.HasPrefix(s.message, "no") || s.message == "cancelled" || s.message == "skipped" {
				statusText = svcMsgStyle.Render(s.message)
			} else {
				statusText = svcMsgStyle.Render(s.message)
			}
		case statusError:
			icon = svcIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Render("!"))
			statusText = svcErrorMsgStyle.Render(s.message)
		case statusSkipped:
			icon = svcIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Render("-"))
			statusText = svcMsgStyle.Render(s.message)
		}

		nameStyle := svcStepNameStyle
		if i == m.current && s.status == statusRunning {
			nameStyle = svcStepNameBoldStyle
		}

		line := fmt.Sprintf("  %s %s %s",
			icon,
			nameStyle.Render(label),
			statusText,
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(svcDivider(min(m.width, 80)))
	b.WriteString("\n")

	availLogLines := m.height - 28
	if len(m.banner) > 0 {
		bannerLineCount := strings.Count(m.banner, "\n")
		availLogLines -= bannerLineCount - 1
	}
	if availLogLines < 3 {
		availLogLines = 3
	}

	visibleLogs := m.log.visible(availLogLines)

	if len(visibleLogs) > 0 || !m.done {
		maxWidth := min(m.width, 80)
		if maxWidth < 30 {
			maxWidth = 30
		}

		b.WriteString(svcLogHeaderStyle.Render("  \u25aa Build Log"))
		b.WriteString("\n")
		b.WriteString(svcDividerStyle.Render(strings.Repeat("\u2500", maxWidth-4)))
		b.WriteString("\n")

		for _, line := range visibleLogs {
			display := line
			if len(display) > maxWidth-8 {
				display = display[:maxWidth-8]
			}
			b.WriteString(svcLogLineStyle.Render("  " + display))
			b.WriteString("\n")
		}

		remainingLines := availLogLines - len(visibleLogs)
		for i := 0; i < remainingLines; i++ {
			b.WriteString("\n")
		}
	}

	if m.done {
		if m.success {
			b.WriteString(svcSummaryHeaderStyle.Render(fmt.Sprintf("  \u2713 Generation complete in %s\n", m.completedIn)))
			b.WriteString("\n")

			routeCount := len(m.ctx.Config.CustomRoutes)
			routeInfo := fmt.Sprintf("%d standard", len(m.ctx.getPatternMethods()))
			if routeCount > 0 {
				routeInfo = fmt.Sprintf("%d standard + %d custom", len(m.ctx.getPatternMethods()), routeCount)
			}

			summaryLines := []struct{ label, value string }{
				{"Service", m.ctx.Config.ServiceName},
				{"Wire Name", m.ctx.Config.WireName},
				{"File", filepath.Join(m.ctx.ServicesDir, m.ctx.Config.FileName)},
				{"Pattern", m.ctx.Config.ServicePattern.Name},
				{"Tests", yesNo(m.ctx.Config.GenerateTests)},
				{"Model", yesNo(m.ctx.Config.GenerateModel)},
				{"Routes", routeInfo},
			}

			for _, sl := range summaryLines {
				b.WriteString(svcSummaryLineStyle.Render(
					fmt.Sprintf("     %12s  %s", sl.label+":", sl.value)))
				b.WriteString("\n")
			}

			b.WriteString("\n")
			b.WriteString(svcSummaryLineStyle.Render("     Next steps:"))
			b.WriteString("\n")
			nextSteps := []string{
				fmt.Sprintf("Add to config.yaml: %s: true", strings.ToLower(m.ctx.Config.ServiceName)+"_service"),
				"Implement business logic in handler methods",
				"Regenerate Swagger: go run scripts/swagger/swagger.go",
				"Test the service endpoints",
			}
			for i, step := range nextSteps {
				b.WriteString(svcSummaryLineStyle.Render(
					fmt.Sprintf("     %12s  %d. %s", "", i+1, step)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true).Render("  Generation failed"))
			b.WriteString("\n")
			b.WriteString(svcErrorMsgStyle.Render("  Check the errors above"))
		}
		sinceDone := time.Since(m.doneAt)
		remaining := 15*time.Second - sinceDone
		secs := int(remaining.Seconds())
		if secs < 0 {
			secs = 0
		}
		closing := ""
		if m.success {
			closing = fmt.Sprintf("  Auto-closing in %ds", secs)
		} else {
			closing = "  Press any key to exit"
		}
		b.WriteString("\n")
		b.WriteString(svcFooterStyle.Render(closing))
	} else if m.promptActive && m.steps[m.current].prompt == promptSelect {
		b.WriteString(svcFooterStyle.Render("  1-6 to select  |  q / ctrl+c to quit"))
	} else if m.promptActive && m.steps[m.current].prompt == promptText {
		b.WriteString(svcFooterStyle.Render("  Type and press Enter  |  ctrl+c to quit"))
	} else if m.promptActive && m.steps[m.current].prompt == promptYesNo {
		b.WriteString(svcFooterStyle.Render("  y / n  |  q / ctrl+c to quit"))
	} else if m.promptActive && m.steps[m.current].prompt == promptConfirm {
		b.WriteString(svcFooterStyle.Render("  Y / n  |  q / ctrl+c to quit"))
	} else {
		b.WriteString(svcFooterStyle.Render("  Generating...  |  ctrl+c to quit"))
	}

	b.WriteString("\n")
	container := lipgloss.NewStyle().Padding(1, 2)
	return container.Render(b.String())
}

func RunServiceTUI(ctx *ServiceContext, logger *Logger) (retCtx *ServiceContext, retErr error) {
	// ── terminal safety guard ──
	var guard ttyGuard
	if err := guard.Save(); err != nil {
		logger.Warn("Failed to save terminal state: %v", err)
	}
	defer guard.Restore()
	sigDone := setupTUISignalHandler(&guard)
	defer close(sigDone)

	// ── panic recovery ──
	defer func() {
		if r := recover(); r != nil {
			guard.Restore()
			// Re-panic so the stack trace prints.
			panic(r)
		}
	}()

	m := NewServiceTuiModel(ctx, logger)

	logR, logW, err := os.Pipe()
	if err == nil {
		m.stepPipeR = logR
		m.stepPipeW = logW
		go func() {
			_, _ = io.Copy(&logCaptureWriter{log: m.log}, logR)
		}()
	}

	logger.writer = &logCaptureWriter{log: m.log}

	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if m.stepPipeW != nil {
		m.stepPipeW.Close()
	}
	if m.stepPipeR != nil {
		m.stepPipeR.Close()
	}
	if err != nil {
		return ctx, err
	}
	fm, ok := final.(ServiceTuiModel)
	if !ok {
		return ctx, fmt.Errorf("unexpected model type")
	}
	if fm.success {
		return fm.ctx, nil
	}
	for _, s := range fm.steps {
		if s.status == statusError {
			return fm.ctx, fmt.Errorf("%s: %s", s.name, s.message)
		}
	}
	return fm.ctx, fmt.Errorf("generation failed")
}

// ─── CLI Runner ─────────────────────────────────────────────────────────────────

func runCLIService(ctx *ServiceContext, logger *Logger) {
	ClearScreen()
	printBanner()

	steps := []struct {
		name string
		fn   func(*Logger) error
	}{
		{"Finding project root", ctx.ensureProjectRoot},
		{"Prompting for service name", ctx.promptServiceName},
		{"Prompting for wire name", ctx.promptWireName},
		{"Prompting for file name", ctx.promptFileName},
		{"Selecting service pattern", ctx.promptServicePattern},
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

	if !ctx.Config.DryRun {
		fmt.Println("")
		fmt.Println(GRAY+"======================================================================"+RESET)
		fmt.Println(" "+B_PURPLE+"SUCCESS!"+RESET+" "+P_GREEN+"Service generated:"+RESET+" "+UNDERLINE+B_WHITE+ctx.Config.FileName+RESET)
		fmt.Println(GRAY+"======================================================================"+RESET)
	}
}

// ─── Main ───────────────────────────────────────────────────────────────────────

func isTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func main() {
	var (
		verbose = flag.Bool("verbose", false, "Enable verbose logging")
		dryRun  = flag.Bool("dry-run", false, "Only analyze, don't generate")
		noTUI   = flag.Bool("no-tui", false, "Disable TUI, use plain CLI output")
	)
	flag.Parse()

	logger := NewLogger(*verbose)

	projectDir, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get current directory: %v", err)
		os.Exit(1)
	}

	ctx := &ServiceContext{
		Config: ServiceConfig{
			Verbose: *verbose,
			DryRun:  *dryRun,
		},
		ProjectDir: projectDir,
	}

	if *noTUI || !isTerminal() {
		_, cancel := context.WithCancel(context.Background())
		setupSignalHandler(cancel)
		runCLIService(ctx, logger)
	} else {
		_, err := RunServiceTUI(ctx, logger)
		ClearScreen()
		if err != nil {
			fmt.Printf("Generation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\u2713%s Service generated: %s%s%s\n",
			B_GREEN, RESET, B_WHITE, ctx.Config.FileName, RESET)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
