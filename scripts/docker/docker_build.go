package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Configuration constants
const (
	DEFAULT_APP_NAME   = "stackyrd"
	DEFAULT_IMAGE_NAME = "myapp"
	DEFAULT_TARGET     = "all"
)

// ANSI Colors
const (
	RESET     = "\033[0m"
	BOLD      = "\033[1m"
	DIM       = "\033[2m"
	UNDERLINE = "\033[4m"

	// Pastel Palette (main color: #8daea5)
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

// Docker build configuration
type DockerBuildConfig struct {
	AppName   string
	ImageName string
	Target    string
	Verbose   bool
}

// Docker build context
type DockerBuildContext struct {
	Config     DockerBuildConfig
	ProjectDir string
	Step       int
	TotalSteps int
}

// Docker logger for structured output
type DockerLogger struct {
	verbose bool
}

func (l *DockerLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("%s[INFO]%s %s\n", B_CYAN, RESET, fmt.Sprintf(msg, args...))
}

func (l *DockerLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("%s[WARN]%s %s\n", B_YELLOW, RESET, fmt.Sprintf(msg, args...))
}

func (l *DockerLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("%s[ERROR]%s %s\n", B_RED, RESET, fmt.Sprintf(msg, args...))
}

func (l *DockerLogger) Debug(msg string, args ...interface{}) {
	if l.verbose {
		fmt.Printf("%s[DEBUG]%s %s\n", GRAY, RESET, fmt.Sprintf(msg, args...))
	}
}

func (l *DockerLogger) Success(msg string, args ...interface{}) {
	fmt.Printf("%s[SUCCESS]%s %s\n", B_GREEN, RESET, fmt.Sprintf(msg, args...))
}

func (l *DockerLogger) Step(stepNum, totalSteps int, msg string, args ...interface{}) {
	fmt.Printf("%s[%d/%d]%s %s%s%s\n", B_PURPLE, stepNum, totalSteps, RESET, P_CYAN, fmt.Sprintf(msg, args...), RESET)
}

// NewDockerLogger creates a new logger
func NewDockerLogger(verbose bool) *DockerLogger {
	return &DockerLogger{verbose: verbose}
}

// findProjectRoot searches up the directory tree for go.mod
func findProjectRoot(startDir string) (string, error) {
	current := startDir

	for {
		// Check if go.mod exists in current directory
		goModPath := filepath.Join(current, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return current, nil
		}

		// Move up one directory
		parent := filepath.Dir(current)

		// If we've reached the root directory, stop
		if parent == current {
			break
		}

		current = parent
	}

	return "", fmt.Errorf("go.mod not found in directory tree")
}

// ensureProjectRoot finds the project root and changes to it if needed
func (ctx *DockerBuildContext) ensureProjectRoot(logger *DockerLogger) error {
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	logger.Info("Starting from: %s", currentDir)

	// Find project root by looking for go.mod
	projectRoot, err := findProjectRoot(currentDir)
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	if projectRoot != currentDir {
		logger.Info("Changing to project root: %s", projectRoot)
		if err := os.Chdir(projectRoot); err != nil {
			return fmt.Errorf("failed to change directory to %s: %w", projectRoot, err)
		}

		// Update context with new working directory
		ctx.ProjectDir = projectRoot

		logger.Success("Now in project root")
	} else {
		logger.Info("Already in project root")
	}

	return nil
}

// printDockerBanner prints the Docker build banner
func printDockerBanner() {
	fmt.Println("")
	fmt.Println("   " + P_PURPLE + " /\\ " + RESET)
	fmt.Println("   " + P_PURPLE + "(  )" + RESET + "   " + B_PURPLE + "Docker Builder" + RESET + " " + GRAY + "by" + RESET + " " + B_WHITE + "diameter-tscd" + RESET)
	fmt.Println("   " + P_PURPLE + " \\/ " + RESET)
	fmt.Println(GRAY + "----------------------------------------------------------------------" + RESET)
}

// printDockerSuccess prints the Docker build success message
func printDockerSuccess(logger *DockerLogger, imageName, target string) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SUCCESS!" + RESET + " " + P_GREEN + "Docker images ready:" + RESET)

	// Show only the images that were actually built
	if target == "test" || target == "all" || target == "ultra-test" || target == "ultra-all" {
		fmt.Println("   " + B_WHITE + imageName + ":test" + RESET + "     " + GRAY + "(testing)" + RESET)
	}
	if target == "dev" || target == "all" || target == "ultra-dev" || target == "ultra-all" {
		fmt.Println("   " + B_WHITE + imageName + ":dev" + RESET + "      " + GRAY + "(development)" + RESET)
	}
	if target == "prod" || target == "all" {
		fmt.Println("   " + B_WHITE + imageName + ":latest" + RESET + "  " + GRAY + "(production)" + RESET)
	}
	if target == "prod-slim" {
		fmt.Println("   " + B_WHITE + imageName + ":slim" + RESET + "    " + GRAY + "(slim-production)" + RESET)
	}
	if target == "prod-minimal" {
		fmt.Println("   " + B_WHITE + imageName + ":minimal" + RESET + " " + GRAY + "(minimal-production)" + RESET)
	}
	if target == "ultra-prod" || target == "ultra-all" {
		fmt.Println("   " + B_WHITE + imageName + ":ultra" + RESET + "    " + GRAY + "(ultra-production)" + RESET)
	}

	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println("")

	// Show relevant usage examples based on what was built
	if target == "dev" || target == "all" {
		fmt.Println("  " + GRAY + "# Run development container" + RESET)
		fmt.Println("  " + B_WHITE + "docker run -p 8080:8080 -p 9090:9090 " + imageName + ":dev" + RESET)
		fmt.Println("")
	}

	if target == "prod" || target == "all" {
		fmt.Println("  " + GRAY + "# Run production container" + RESET)
		fmt.Println("  " + B_WHITE + "docker run -p 8080:8080 -p 9090:9090 " + imageName + ":latest" + RESET)
		fmt.Println("")
	}

	if target == "test" || target == "all" {
		fmt.Println("  " + GRAY + "# Run tests" + RESET)
		fmt.Println("  " + B_WHITE + "docker run --rm " + imageName + ":test" + RESET)
	}
}

// validateTarget validates the build target
func validateTarget(target string) error {
	validTargets := []string{
		"all", "test", "dev", "prod", "prod-slim", "prod-minimal",
		"ultra-prod", "ultra-all", "ultra-dev", "ultra-test",
	}

	for _, valid := range validTargets {
		if target == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid target: %s. Valid targets: %s", target, strings.Join(validTargets, ", "))
}

// calculateTotalSteps calculates the total number of steps based on target
func calculateTotalSteps(target string) int {
	switch target {
	case "all", "ultra-all":
		return 4
	case "test", "ultra-test":
		return 2
	case "dev", "ultra-dev", "prod", "ultra-prod":
		return 1
	default:
		return 1
	}
}

// checkDockerfile checks if Dockerfile exists
func (ctx *DockerBuildContext) checkDockerfile(logger *DockerLogger) error {
	dockerfilePath := filepath.Join(ctx.ProjectDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		logger.Error("Dockerfile not found in current directory")
		return err
	}
	return nil
}

// checkDocker checks if Docker is available
func (ctx *DockerBuildContext) checkDocker(logger *DockerLogger) error {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Os}}")
	if err := cmd.Run(); err != nil {
		logger.Error("Docker is not installed or not in PATH")
		return err
	}
	return nil
}

// buildTestStage builds the test stage
func (ctx *DockerBuildContext) buildTestStage(logger *DockerLogger, imageName string) error {
	ctx.Step++
	logger.Step(ctx.Step, ctx.TotalSteps, "Building test image...")

	cmd := exec.Command("docker", "build", "--target", "test", "-t", imageName+":test", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Test build failed")
		return err
	}

	logger.Success("Test image built: %s", imageName+":test")
	return nil
}

// runTests runs the tests in the test container
func (ctx *DockerBuildContext) runTests(logger *DockerLogger, imageName string) error {
	ctx.Step++
	logger.Step(ctx.Step, ctx.TotalSteps, "Running tests...")

	cmd := exec.Command("docker", "run", "--rm", imageName+":test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Tests failed")
		return err
	}

	logger.Success("Tests passed")
	return nil
}

// buildDevStage builds the development stage
func (ctx *DockerBuildContext) buildDevStage(logger *DockerLogger, imageName string) error {
	ctx.Step++
	logger.Step(ctx.Step, ctx.TotalSteps, "Building development image...")

	cmd := exec.Command("docker", "build", "--target", "dev", "-t", imageName+":dev", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Development build failed")
		return err
	}

	logger.Success("Development image built: %s", imageName+":dev")
	return nil
}

// buildUltraDevStage builds the ultra development stage
func (ctx *DockerBuildContext) buildUltraDevStage(logger *DockerLogger, imageName string) error {
	ctx.Step++
	logger.Step(ctx.Step, ctx.TotalSteps, "Building ultra development image...")

	cmd := exec.Command("docker", "build", "--target", "ultra-dev", "-t", imageName+":dev", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Ultra development build failed")
		return err
	}

	logger.Success("Ultra development image built: %s", imageName+":dev")
	return nil
}

// buildProdStage builds the production stage
func (ctx *DockerBuildContext) buildProdStage(logger *DockerLogger, imageName string) error {
	ctx.Step++
	logger.Step(ctx.Step, ctx.TotalSteps, "Building production image...")

	cmd := exec.Command("docker", "build", "--target", "prod", "-t", imageName+":latest", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Production build failed")
		return err
	}

	logger.Success("Production image built: %s", imageName+":latest")
	return nil
}

// buildSlimProdStage builds the slim production stage
func (ctx *DockerBuildContext) buildSlimProdStage(logger *DockerLogger, imageName string) error {
	ctx.Step++
	logger.Step(ctx.Step, ctx.TotalSteps, "Building slim production image...")

	cmd := exec.Command("docker", "build", "--target", "prod-slim", "-t", imageName+":slim", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Slim production build failed")
		return err
	}

	logger.Success("Slim production image built: %s", imageName+":slim")
	return nil
}

// buildMinimalProdStage builds the minimal production stage
func (ctx *DockerBuildContext) buildMinimalProdStage(logger *DockerLogger, imageName string) error {
	ctx.Step++
	logger.Step(ctx.Step, ctx.TotalSteps, "Building minimal production image...")

	cmd := exec.Command("docker", "build", "--target", "prod-minimal", "-t", imageName+":minimal", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Minimal production build failed")
		return err
	}

	logger.Success("Minimal production image built: %s", imageName+":minimal")
	return nil
}

// buildUltraProdStage builds the ultra production stage
func (ctx *DockerBuildContext) buildUltraProdStage(logger *DockerLogger, imageName string) error {
	ctx.Step++
	logger.Step(ctx.Step, ctx.TotalSteps, "Building ultra production image...")

	cmd := exec.Command("docker", "build", "--target", "ultra-prod", "-t", imageName+":ultra", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Ultra production build failed")
		return err
	}

	logger.Success("Ultra production image built: %s", imageName+":ultra")
	return nil
}

// cleanupDanglingImages cleans up intermediate images
func (ctx *DockerBuildContext) cleanupDanglingImages(logger *DockerLogger) error {
	logger.Step(ctx.Step, ctx.TotalSteps, "Cleaning up dangling images...")

	cmd := exec.Command("docker", "image", "prune", "-f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Debug("Cleanup skipped: %v", err)
		return nil
	}

	logger.Success("Cleanup completed")
	return nil
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

// promptWithDefault prompts user for input with a default value
func promptWithDefault(prompt, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s%s%s [%s]: ", B_CYAN, prompt, RESET, B_WHITE+defaultValue+RESET)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

// selectTarget displays a menu for target selection and returns the selected target
func selectTarget(logger *DockerLogger) string {
	targets := []struct {
		number int
		name   string
		desc   string
	}{
		{1, "all", "Build all images (test, dev, prod)"},
		{2, "test", "Build and run tests only"},
		{3, "dev", "Build development image"},
		{4, "prod", "Build production image"},
		{5, "prod-slim", "Build slim production image"},
		{6, "prod-minimal", "Build minimal production image"},
		{7, "ultra-prod", "Build ultra production image"},
		{8, "ultra-all", "Build all ultra images"},
		{9, "ultra-dev", "Build ultra development image"},
		{10, "ultra-test", "Build and run ultra tests"},
	}

	fmt.Println("")
	fmt.Println(B_PURPLE + "Select Build Target:" + RESET)
	fmt.Println(GRAY + "----------------------------------------" + RESET)
	for _, t := range targets {
		fmt.Printf("  %s%2d%s) %s%-20s%s %s\n", B_WHITE, t.number, RESET, B_CYAN, t.name, RESET, GRAY+"("+t.desc+")"+RESET)
	}
	fmt.Println(GRAY + "----------------------------------------" + RESET)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%sEnter number (1-10) [%s]: %s", B_YELLOW, "1", RESET)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			input = "1"
		}

		num, err := strconv.Atoi(input)
		if err == nil && num >= 1 && num <= 10 {
			return targets[num-1].name
		}
		logger.Error("Invalid selection. Please enter a number between 1 and 10.")
	}
}

// askVerbose asks if verbose logging should be enabled
func askVerbose(logger *DockerLogger) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%sEnable verbose logging? (y/N) [%s]: %s", B_CYAN, "N", RESET)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// main function
func main() {
	// Clear the terminal screen
	fmt.Print("\033[H\033[2J")

	// Initialize a temporary logger for interactive prompts
	tempLogger := NewDockerLogger(false)

	// Interactive selection
	printDockerBanner()

	// Select target interactively
	target := selectTarget(tempLogger)

	// Configure app name and image name
	appName := promptWithDefault("Enter app name", DEFAULT_APP_NAME)
	imageName := promptWithDefault("Enter image name", DEFAULT_IMAGE_NAME)

	// Ask for verbose mode
	verbose := askVerbose(tempLogger)

	// Initialize logger
	logger := NewDockerLogger(verbose)

	// Get project directory
	projectDir, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get current directory: %v", err)
		os.Exit(1)
	}

	// Create build context
	ctx := &DockerBuildContext{
		Config: DockerBuildConfig{
			AppName:   appName,
			ImageName: imageName,
			Target:    target,
			Verbose:   verbose,
		},
		ProjectDir: projectDir,
		Step:       0,
	}

	// Create context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	setupSignalHandler(cancel)

	// Validate target
	if err := validateTarget(target); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}

	// Set total steps
	ctx.TotalSteps = calculateTotalSteps(target)

	// Execute build steps
	steps := []struct {
		name string
		fn   func(*DockerLogger, string) error
	}{
		// Test stage (always needed for test target or all)
		{"Building test image", func(l *DockerLogger, img string) error {
			if target == "test" || target == "all" || target == "ultra-test" || target == "ultra-all" {
				return ctx.buildTestStage(l, img)
			}
			return nil
		}},

		// Run tests (only for test target or all)
		{"Running tests", func(l *DockerLogger, img string) error {
			if target == "test" || target == "all" || target == "ultra-test" || target == "ultra-all" {
				return ctx.runTests(l, img)
			}
			return nil
		}},

		// Development stage
		{"Building development image", func(l *DockerLogger, img string) error {
			if target == "dev" || target == "all" {
				return ctx.buildDevStage(l, img)
			}
			return nil
		}},

		// Ultra development stage
		{"Building ultra development image", func(l *DockerLogger, img string) error {
			if target == "ultra-dev" || target == "ultra-all" {
				return ctx.buildUltraDevStage(l, img)
			}
			return nil
		}},

		// Production stage
		{"Building production image", func(l *DockerLogger, img string) error {
			if target == "prod" || target == "all" {
				return ctx.buildProdStage(l, img)
			}
			return nil
		}},

		// Slim production stage
		{"Building slim production image", func(l *DockerLogger, img string) error {
			if target == "prod-slim" {
				return ctx.buildSlimProdStage(l, img)
			}
			return nil
		}},

		// Minimal production stage
		{"Building minimal production image", func(l *DockerLogger, img string) error {
			if target == "prod-minimal" {
				return ctx.buildMinimalProdStage(l, img)
			}
			return nil
		}},

		// Ultra production stage (for ultra-all)
		{"Building ultra production image", func(l *DockerLogger, img string) error {
			if target == "ultra-all" {
				return ctx.buildUltraProdStage(l, img)
			}
			return nil
		}},

		// Ultra production stage (ultra slim)
		{"Building ultra-production image", func(l *DockerLogger, img string) error {
			if target == "ultra-prod" {
				return ctx.buildUltraProdStage(l, img)
			}
			return nil
		}},

		// Cleanup
		{"Cleaning up dangling images", func(l *DockerLogger, img string) error {
			return ctx.cleanupDanglingImages(l)
		}},
	}

	// First ensure we are in project root
	if err := ctx.ensureProjectRoot(logger); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}

	// Execute validation steps first
	if err := ctx.checkDockerfile(logger); err != nil {
		os.Exit(1)
	}

	if err := ctx.checkDocker(logger); err != nil {
		os.Exit(1)
	}

	// Execute build steps
	for _, step := range steps {
		if err := step.fn(logger, imageName); err != nil {
			logger.Error("Step failed: %v", err)
			os.Exit(1)
		}
	}

	// Print success message
	printDockerSuccess(logger, imageName, target)
}
