package main

import (
	"archive/zip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Configuration variables
var (
	DIST_DIR   = "dist"
	APP_NAME   = "stackyard"
	MAIN_PATH  = "./cmd/app/main.go"
	CONFIG_YML = "config.yaml"
	BANNER_TXT = "banner.txt"
	DB_FILE    = "monitoring_users.db"
	WEB_DIR    = "web"
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

// Build configuration
type BuildConfig struct {
	UseGarble        bool
	UseGoversioninfo bool
	Timeout          time.Duration
	Verbose          bool
}

// BuildContext holds the build state
type BuildContext struct {
	Config     BuildConfig
	Timestamp  string
	BackupPath string
	DistPath   string
	ProjectDir string
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

// checkPath checks the path folder and ensures we're in the project root
func (ctx *BuildContext) checkPath(logger *Logger) error {
	return ctx.ensureProjectRoot(logger)
}

// clear console screen
func ClearScreen() {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Windows: use cmd /c cls
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		// Linux, macOS, and others: use clear command
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}

// ensureProjectRoot finds the project root and changes to it if needed
func (ctx *BuildContext) ensureProjectRoot(logger *Logger) error {
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
		ctx.DistPath = filepath.Join(projectRoot, DIST_DIR)

		logger.Success("Now in project root")
	} else {
		logger.Info("Already in project root")
	}

	// Ensure dist directory exists
	if err := os.MkdirAll(ctx.DistPath, 0755); err != nil {
		logger.Error("Failed to create dist directory: %v", err)
		os.Exit(1)
	}

	return nil
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

// checkRequiredTools checks if required tools are available
func (ctx *BuildContext) checkRequiredTools(logger *Logger) error {
	logger.Info("Checking required tools...")

	// Check goversioninfo
	if err := exec.Command("goversioninfo", "-h").Run(); err != nil {
		logger.Warn("goversioninfo not found. Skipping version info generation.")
		ctx.Config.UseGoversioninfo = false
	} else {
		logger.Success("goversioninfo found")
		ctx.Config.UseGoversioninfo = true
	}

	// Check garble
	if err := exec.Command("garble", "-h").Run(); err != nil {
		logger.Warn("garble not found. Installing...")
		if err := installGarble(logger); err != nil {
			return fmt.Errorf("failed to install garble: %w", err)
		}
		logger.Success("garble installed")
	} else {
		logger.Success("garble found")
	}

	return nil
}

// installGarble installs garble using go install
func installGarble(logger *Logger) error {
	cmd := exec.Command("go", "install", "mvdan.cc/garble@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// askUserAboutGarble asks user if they want to use garble with timeout
func (ctx *BuildContext) askUserAboutGarble(logger *Logger) error {
	fmt.Printf("%sUse garble build for obfuscation? (y/N, timeout %ds): %s", B_YELLOW, int(ctx.Config.Timeout.Seconds()), RESET)

	// Create a channel to receive user input
	inputChan := make(chan string, 1)

	// Start a goroutine to read input
	go func() {
		var choice string
		fmt.Scanln(&choice)
		inputChan <- choice
	}()

	// Wait for input or timeout
	select {
	case choice := <-inputChan:
		if strings.ToLower(choice) == "y" || strings.ToLower(choice) == "yes" {
			ctx.Config.UseGarble = true
			logger.Success("Using garble build")
		} else {
			ctx.Config.UseGarble = false
			logger.Info("Using regular go build")
		}
	case <-time.After(ctx.Config.Timeout):
		logger.Info("Timeout reached. Using regular go build")
		ctx.Config.UseGarble = false
	}

	return nil
}

// stopRunningProcess stops any running application instances
func (ctx *BuildContext) stopRunningProcess(logger *Logger) error {
	logger.Info("Checking for running process...")

	processes, err := ctx.findRunningProcesses()
	if err != nil {
		return fmt.Errorf("failed to check running processes: %w", err)
	}

	if len(processes) > 0 {
		logger.Warn("App is running. Stopping...")
		for _, pid := range processes {
			if err := ctx.killProcess(pid); err != nil {
				logger.Error("Failed to kill process %d: %v", pid, err)
			}
		}
		time.Sleep(time.Second)
	} else {
		logger.Info("App is not running.")
	}

	return nil
}

// findRunningProcesses finds running processes by name
func (ctx *BuildContext) findRunningProcesses() ([]int, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s.exe", APP_NAME))
	} else {
		cmd = exec.Command("pgrep", "-x", APP_NAME)
	}

	output, err := cmd.Output()
	if err != nil {
		// Process not found is not an error
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return []int{}, nil
		}
		return nil, err
	}

	var pids []int
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "INFO:") || strings.Contains(line, "Image Name") {
			continue
		}

		if runtime.GOOS == "windows" {
			// Parse tasklist output to extract PID
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if pid, err := parsePID(parts[1]); err == nil {
					pids = append(pids, pid)
				}
			}
		} else {
			// Parse pgrep output
			if pid, err := parsePID(line); err == nil {
				pids = append(pids, pid)
			}
		}
	}

	return pids, nil
}

// parsePID converts string to int, handling various formats
func parsePID(pidStr string) (int, error) {
	// Remove any non-numeric characters except digits
	cleanStr := ""
	for _, char := range pidStr {
		if char >= '0' && char <= '9' {
			cleanStr += string(char)
		}
	}

	if cleanStr == "" {
		return 0, fmt.Errorf("no valid PID found")
	}

	return strconv.Atoi(cleanStr)
}

// killProcess kills a process by PID
func (ctx *BuildContext) killProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Kill()
}

// createBackup creates a timestamped backup of existing files
func (ctx *BuildContext) createBackup(logger *Logger) error {
	logger.Info("Backing up old files...")

	// Create backup directory
	backupRoot := filepath.Join(ctx.DistPath, "backups")
	if err := os.MkdirAll(backupRoot, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	ctx.BackupPath = filepath.Join(backupRoot, ctx.Timestamp)

	if _, err := os.Stat(ctx.DistPath); os.IsNotExist(err) {
		logger.Info("No existing dist directory. Skipping backup.")
		return nil
	}

	// Create backup directory
	if err := os.MkdirAll(ctx.BackupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup path: %w", err)
	}

	// Move files to backup
	filesToBackup := []string{
		APP_NAME,
		APP_NAME + ".exe",
		CONFIG_YML,
		BANNER_TXT,
		DB_FILE,
	}

	for _, file := range filesToBackup {
		src := filepath.Join(ctx.DistPath, file)
		dst := filepath.Join(ctx.BackupPath, file)

		if err := moveFile(src, dst); err != nil {
			logger.Warn("Failed to backup %s: %v", file, err)
		}
	}

	// Move web directory
	webSrc := filepath.Join(ctx.DistPath, WEB_DIR)
	webDst := filepath.Join(ctx.BackupPath, WEB_DIR)
	if err := moveDir(webSrc, webDst); err != nil {
		logger.Warn("Failed to backup web directory: %v", err)
	}

	logger.Success("Backup created at: %s", ctx.BackupPath)
	return nil
}

// archiveBackup creates a ZIP archive of the backup
func (ctx *BuildContext) archiveBackup(logger *Logger) error {
	logger.Info("Archiving backup...")

	if _, err := os.Stat(ctx.BackupPath); os.IsNotExist(err) {
		logger.Info("No backup created. Skipping archive.")
		return nil
	}

	backupRoot := filepath.Dir(ctx.BackupPath)
	archivePath := filepath.Join(backupRoot, ctx.Timestamp+".zip")

	if err := createZipArchive(ctx.BackupPath, archivePath); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	// Remove the uncompressed backup directory
	if err := os.RemoveAll(ctx.BackupPath); err != nil {
		logger.Warn("Failed to remove backup directory: %v", err)
	}

	logger.Success("Backup archived: %s", archivePath)
	return nil
}

// createZipArchive creates a ZIP file from a directory
func createZipArchive(source, target string) error {
	zipFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})

	return nil
}

// moveFile moves a file from src to dst
func moveFile(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}

	return os.Remove(src)
}

// moveDir moves a directory from src to dst
func moveDir(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}

	return copyDir(src, dst)
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}

			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildApplication builds the Go application
func (ctx *BuildContext) buildApplication(logger *Logger) error {
	logger.Info("Building Go binary...")

	// Generate version info if available
	if ctx.Config.UseGoversioninfo {
		if err := exec.Command("goversioninfo", "-platform-specific").Run(); err != nil {
			logger.Warn("Failed to generate version info: %v", err)
		}
	} else {
		logger.Info("Skipping goversioninfo (not available)")
	}

	// Build command
	var cmd *exec.Cmd
	outputPath := filepath.Join(ctx.DistPath, APP_NAME)

	if runtime.GOOS == "windows" {
		outputPath += ".exe"
	}

	if ctx.Config.UseGarble {
		cmd = exec.Command("garble", "build", "-ldflags=-s -w", "-o", outputPath, MAIN_PATH)
	} else {
		cmd = exec.Command("go", "build", "-ldflags=-s -w", "-o", outputPath, MAIN_PATH)
	}

	// Set environment for garble
	if ctx.Config.UseGarble {
		cmd.Env = append(os.Environ(), "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	}

	// Run build
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed with exit code: %w", err)
	}

	logger.Success("Build successful: %s", outputPath)
	return nil
}

// copyAssets copies required assets to the dist directory
func (ctx *BuildContext) copyAssets(logger *Logger) error {
	logger.Info("Copying assets...")

	assets := []struct {
		src string
		dst string
	}{
		{WEB_DIR, filepath.Join(ctx.DistPath, WEB_DIR)},
		{CONFIG_YML, filepath.Join(ctx.DistPath, CONFIG_YML)},
		{BANNER_TXT, filepath.Join(ctx.DistPath, BANNER_TXT)},
		{DB_FILE, filepath.Join(ctx.DistPath, DB_FILE)},
	}

	for _, asset := range assets {
		if _, err := os.Stat(asset.src); os.IsNotExist(err) {
			continue
		}

		if strings.HasSuffix(asset.src, "/") || isDir(asset.src) {
			if err := copyDir(asset.src, asset.dst); err != nil {
				logger.Warn("Failed to copy %s: %v", asset.src, err)
			} else {
				logger.Success("Copying %s", asset.src)
			}
		} else {
			if err := copyFile(asset.src, asset.dst); err != nil {
				logger.Warn("Failed to copy %s: %v", asset.src, err)
			} else {
				logger.Success("Copying %s", asset.src)
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// isDir checks if a path is a directory
func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// printBanner prints the application banner
func printBanner() {
	fmt.Println("")
	fmt.Println("   " + P_PURPLE + " /\\ " + RESET)
	fmt.Println("   " + P_PURPLE + "(  )" + RESET + "   " + B_PURPLE + APP_NAME + " Builder" + RESET + " " + GRAY + "by" + RESET + " " + B_WHITE + "diameter-tscd" + RESET)
	fmt.Println("   " + P_PURPLE + " \\/ " + RESET)
	fmt.Println(GRAY + "----------------------------------------------------------------------" + RESET)
}

// printSuccess prints the success message
func printSuccess(distPath string) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SUCCESS!" + RESET + " " + P_GREEN + "Build ready at:" + RESET + " " + UNDERLINE + B_WHITE + distPath + RESET)
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

// main function
func main() {
	ClearScreen()

	// Parse command line flags
	var (
		timeoutSeconds = flag.Int("timeout", 10, "Timeout for user prompts in seconds")
		verbose        = flag.Bool("verbose", false, "Enable verbose logging")
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

	// Create build context
	ctx := &BuildContext{
		Config: BuildConfig{
			Timeout: time.Duration(*timeoutSeconds) * time.Second,
			Verbose: *verbose,
		},
		Timestamp:  time.Now().Format("20060102_150405"),
		DistPath:   filepath.Join(projectDir, DIST_DIR),
		ProjectDir: projectDir,
	}

	// Create context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	setupSignalHandler(cancel)

	// Execute build steps
	steps := []struct {
		name string
		fn   func(*Logger) error
	}{
		{"Checking Project Path", ctx.checkPath},
		{"Checking required tools", ctx.checkRequiredTools},
		{"Asking user about garble", ctx.askUserAboutGarble},
		{"Stopping running process", ctx.stopRunningProcess},
		{"Creating backup", ctx.createBackup},
		{"Archiving backup", ctx.archiveBackup},
		{"Building application", ctx.buildApplication},
		{"Copying assets", ctx.copyAssets},
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
	printSuccess(ctx.DistPath)
}
