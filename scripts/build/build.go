package main

import (
	"archive/tar"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ulikunitz/xz"
)

// Configuration variables
var (
	DIST_DIR   = "dist"
	APP_NAME   = "stackyrd-nano"
	MAIN_PATH  = "./cmd/app"
	CONFIG_YML = "config.yaml"
	BANNER_TXT = "banner.txt"
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

// Archive format constants
const (
	FormatTar     = "tar"
	Format7z      = "7z"
	DefaultFormat = FormatTar
)

// validArchiveFormat checks if the format is supported and returns the normalized value
func validArchiveFormat(f string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case FormatTar, "tar.xz", "txz":
		return FormatTar, true
	case Format7z, "sevenzip", "7-zip":
		return Format7z, true
	default:
		return "", false
	}
}

// Build configuration
type BuildConfig struct {
	UseGarble        bool
	UseGoversioninfo bool
	UseUPX           bool
	Timeout          time.Duration
	Verbose          bool
	ArchiveFormat    string
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

// NewLogger creates a new logger
func NewLogger(verbose bool) *Logger {
	return &Logger{verbose: verbose, writer: os.Stdout}
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
	_ = cmd.Run()
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

// askUserAboutUPX asks user if they want UPX LZMA compression with timeout
func (ctx *BuildContext) askUserAboutUPX(logger *Logger) error {
	fmt.Printf("%sApply UPX LZMA compression to the binary? (y/N, timeout %ds): %s", B_YELLOW, int(ctx.Config.Timeout.Seconds()), RESET)

	inputChan := make(chan string, 1)

	go func() {
		var choice string
		fmt.Scanln(&choice)
		inputChan <- choice
	}()

	select {
	case choice := <-inputChan:
		if strings.ToLower(choice) == "y" || strings.ToLower(choice) == "yes" {
			ctx.Config.UseUPX = true
			logger.Success("UPX compression enabled")
		} else {
			ctx.Config.UseUPX = false
			logger.Info("Skipping UPX compression")
		}
	case <-time.After(ctx.Config.Timeout):
		logger.Info("Timeout reached. Skipping UPX compression")
		ctx.Config.UseUPX = false
	}

	return nil
}

// compressWithUPX compresses the built binary with upx --lzma.
// If upx is not installed, installs it first.
func (ctx *BuildContext) compressWithUPX(logger *Logger) error {
	if !ctx.Config.UseUPX {
		return nil
	}

	outputPath := filepath.Join(ctx.DistPath, APP_NAME)
	if runtime.GOOS == "windows" {
		outputPath += ".exe"
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		logger.Warn("Binary not found at %s, skipping UPX compression", outputPath)
		return nil
	}

	// Check if upx is available, install if not
	if err := exec.Command("upx", "--version").Run(); err != nil {
		logger.Warn("upx not found. Installing...")
		if err := installUPX(logger); err != nil {
			logger.Warn("Failed to install upx: %v. Skipping compression.", err)
			return nil
		}
		logger.Success("upx installed")
	}

	logger.Info("Compressing with upx --lzma...")

	upxArgs := []string{"--lzma", "--best"}
	if runtime.GOOS == "darwin" {
		upxArgs = append(upxArgs, "--force-macos")
	}
	upxArgs = append(upxArgs, outputPath)
	cmd := exec.Command("upx", upxArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Warn("UPX compression failed: %v. Build continues without compression.", err)
		return nil
	}

	// Get compressed size
	info, err := os.Stat(outputPath)
	if err == nil {
		logger.Success("UPX compression complete: %d bytes", info.Size())
	} else {
		logger.Success("UPX compression complete")
	}

	return nil
}

// installUPX installs upx using the system package manager
func installUPX(logger *Logger) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("brew", "install", "upx")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "linux":
		// Try apt, then apk
		if err := exec.Command("apt-get", "--version").Run(); err == nil {
			cmd := exec.Command("apt-get", "update", "-qq")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("apt-get update failed: %w", err)
			}
			cmd = exec.Command("apt-get", "install", "-y", "-qq", "upx")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		if err := exec.Command("apk", "--version").Run(); err == nil {
			cmd := exec.Command("apk", "add", "upx")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		return fmt.Errorf("unsupported package manager on linux (tried apt-get, apk)")
	default:
		return fmt.Errorf("unsupported OS for automatic upx installation: %s", runtime.GOOS)
	}
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
	}

	for _, file := range filesToBackup {
		src := filepath.Join(ctx.DistPath, file)
		dst := filepath.Join(ctx.BackupPath, file)

		if err := moveFile(src, dst); err != nil {
			logger.Warn("Failed to backup %s: %v", file, err)
		}
	}

	logger.Success("Backup created at: %s", ctx.BackupPath)
	return nil
}

// archiveBackup creates a compressed archive of the backup using the configured format.
func (ctx *BuildContext) archiveBackup(logger *Logger) error {
	if _, err := os.Stat(ctx.BackupPath); os.IsNotExist(err) {
		logger.Info("No backup created. Skipping archive.")
		return nil
	}

	backupRoot := filepath.Dir(ctx.BackupPath)

	switch ctx.Config.ArchiveFormat {
	case Format7z:
		return ctx.archiveBackup7z(backupRoot, logger)
	default:
		return ctx.archiveBackupTar(backupRoot, logger)
	}
}

func (ctx *BuildContext) archiveBackupTar(backupRoot string, logger *Logger) error {
	logger.Info("Archiving backup with native LZMA2 compression (tar.xz)...")

	archivePath := filepath.Join(backupRoot, ctx.Timestamp+".tar.xz")
	if err := createTarXzArchive(ctx.BackupPath, archivePath); err != nil {
		return fmt.Errorf("failed to create tar.xz archive: %w", err)
	}

	if err := os.RemoveAll(ctx.BackupPath); err != nil {
		logger.Warn("Failed to remove backup directory: %v", err)
	}

	info, err := os.Stat(archivePath)
	if err == nil {
		logger.Success("Backup archived (%s): %s", humanizeSize(info.Size()), archivePath)
	} else {
		logger.Success("Backup archived: %s", archivePath)
	}
	return nil
}

func (ctx *BuildContext) archiveBackup7z(backupRoot string, logger *Logger) error {
	logger.Info("Archiving backup with 7z LZMA2...")

	if err := exec.Command("7z", "-h").Run(); err != nil {
		logger.Warn("7z binary not found. Falling back to native tar.xz compression.")
		ctx.Config.ArchiveFormat = FormatTar
		return ctx.archiveBackupTar(backupRoot, logger)
	}

	archivePath := filepath.Join(backupRoot, ctx.Timestamp+".7z")
	cmd := exec.Command("7z", "a", "-t7z", "-m0=lzma2", "-mx=9", "-bd", "-y",
		archivePath, ctx.BackupPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Warn("7z compression failed: %v. Falling back to native tar.xz.", err)
		ctx.Config.ArchiveFormat = FormatTar
		return ctx.archiveBackupTar(backupRoot, logger)
	}

	if err := os.RemoveAll(ctx.BackupPath); err != nil {
		logger.Warn("Failed to remove backup directory: %v", err)
	}

	info, err := os.Stat(archivePath)
	if err == nil {
		logger.Success("Backup archived (%s): %s", humanizeSize(info.Size()), archivePath)
	} else {
		logger.Success("Backup archived: %s", archivePath)
	}
	return nil
}

// createTarXzArchive creates a tar.xz archive with LZMA2 compression from a directory.
// LZMA2 is the same algorithm used by 7-Zip, providing high compression ratios natively.
func createTarXzArchive(source, target string) error {
	xzFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() { _ = xzFile.Close() }()

	xzWriter, err := xz.NewWriter(xzFile)
	if err != nil {
		return err
	}
	defer func() { _ = xzWriter.Close() }()

	tarWriter := tar.NewWriter(xzWriter)
	defer func() { _ = tarWriter.Close() }()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.ToSlash(filepath.Join(baseDir, strings.TrimPrefix(path, source)))
		}

		if info.IsDir() {
			header.Name += "/"
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		_, err = io.Copy(tarWriter, file)
		return err
	})
}

// humanizeSize converts bytes to a human-readable string
func humanizeSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
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

// compilePlugins pre-compiles plugin scripts in the builtin directory
func (ctx *BuildContext) compilePlugins(logger *Logger) error {
	logger.Info("Compiling plugin scripts...")

	builtinDir := filepath.Join(ctx.ProjectDir, "pkg", "plugin", "builtin")
	entries, err := os.ReadDir(builtinDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("No builtin plugins directory found, skipping")
			return nil
		}
		return fmt.Errorf("failed to read builtin plugins: %w", err)
	}

	compiled := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		scriptsDir := filepath.Join(builtinDir, entry.Name(), "scripts")
		scriptsEntries, err := os.ReadDir(scriptsDir)
		if err != nil {
			continue
		}
		for _, script := range scriptsEntries {
			if script.IsDir() {
				continue
			}
			name := script.Name()
			scriptPath := filepath.Join(scriptsDir, name)

			if strings.HasSuffix(name, ".py") {
				pycPath := filepath.Join(scriptsDir, strings.TrimSuffix(name, ".py")+".pyc")
				logger.Debug("Compiling %s -> %s", scriptPath, pycPath)
				cmd := exec.Command("python3", "-c",
					fmt.Sprintf("import py_compile; py_compile.compile(%q, cfile=%q, doraise=True)", scriptPath, pycPath))
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					logger.Warn("Failed to compile Python plugin %s: %v", scriptPath, err)
					continue
				}
				compiled++
				logger.Success("Compiled Python plugin: %s", filepath.Base(scriptPath))
			}
		}
	}

	if compiled == 0 {
		logger.Info("No Python plugin scripts to compile")
	} else {
		logger.Success("Compiled %d Python plugin script(s)", compiled)
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
		cmd = exec.Command("garble", "build", "-ldflags=-s -w -buildid=", "-trimpath", "-o", outputPath, MAIN_PATH)
	} else {
		cmd = exec.Command("go", "build", "-ldflags=-s -w -buildid=", "-trimpath", "-o", outputPath, MAIN_PATH)
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
		{CONFIG_YML, filepath.Join(ctx.DistPath, CONFIG_YML)},
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
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

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

// main function
func main() {
	// Parse command line flags
	var (
		timeoutSeconds = flag.Int("timeout", 10, "Timeout for user prompts in seconds")
		verbose        = flag.Bool("verbose", false, "Enable verbose logging")
		useGarble      = flag.Bool("garble", false, "Enable garble obfuscation (skips interactive prompt)")
		useUPX         = flag.Bool("upx", false, "Enable UPX compression (skips interactive prompt)")
		archiveFormat  = flag.String("archive-format", DefaultFormat, "Backup archive format: tar (native LZMA2, default) or 7z (requires 7z binary)")
		noTUI          = flag.Bool("no-tui", false, "Disable TUI, use plain CLI output")
	)
	flag.Parse()

	// Initialize logger
	logger := NewLogger(*verbose)

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

	// Apply flag overrides for non-interactive mode
	if *useGarble {
		ctx.Config.UseGarble = true
	}
	if *useUPX {
		ctx.Config.UseUPX = true
	}

	// Validate archive format with fallback to default
	if normalized, ok := validArchiveFormat(*archiveFormat); ok {
		ctx.Config.ArchiveFormat = normalized
	} else {
		if !*noTUI {
			fmt.Printf("Unknown archive format '%s'. Falling back to '%s'. Supported: tar, 7z\n", *archiveFormat, DefaultFormat)
		} else {
			logger.Warn("Unknown archive format '%s'. Falling back to '%s'. Supported: tar, 7z", *archiveFormat, DefaultFormat)
		}
		ctx.Config.ArchiveFormat = DefaultFormat
	}

	if *noTUI || !isTerminal() {
		runCLIBuild(ctx, logger)
	} else {
		runTUIBuild(ctx, logger)
	}
}

// isTerminal returns true if stdout is a terminal
func isTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// runTUIBuild runs the build with the bubbletea TUI
func runTUIBuild(ctx *BuildContext, logger *Logger) {
	_, err := RunBuildTUI(ctx, logger)
	if err != nil {
		fmt.Printf("\nBuild failed: %v\n", err)
		os.Exit(1)
	}
}

// runCLIBuild runs the build with plain CLI output
func runCLIBuild(ctx *BuildContext, logger *Logger) {
	ClearScreen()
	printBanner()

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
		{"Compiling plugins", ctx.compilePlugins},
		{"Building application", ctx.buildApplication},
		{"Asking user about UPX compression", ctx.askUserAboutUPX},
		{"Compressing with UPX", ctx.compressWithUPX},
		{"Copying assets", ctx.copyAssets},
	}

	for i, step := range steps {
		if step.name == "Asking user about garble" && ctx.Config.UseGarble {
			continue
		}
		if step.name == "Asking user about UPX compression" && ctx.Config.UseUPX {
			continue
		}

		stepNum := fmt.Sprintf("%d/%d", i+1, len(steps))
		fmt.Printf("%s[%s]%s %s%s%s\n", B_PURPLE, stepNum, RESET, P_CYAN, step.name, RESET)

		if err := step.fn(logger); err != nil {
			logger.Error("Step failed: %v", err)
			os.Exit(1)
		}
	}

	printSuccess(ctx.DistPath)
}

// ─── Bubble Tea TUI ──────────────────────────────────────────────────────────

type stepStatus int

const (
	statusPending stepStatus = iota
	statusRunning
	statusSuccess
	statusError
	statusSkipped
)

type stepInfo struct {
	name       string
	status     stepStatus
	message    string
	isPrompt   bool
	promptText string
	promptDef  bool
	action     func(*BuildContext, *Logger) error
}

type (
	tickMsg     time.Time
	stepDoneMsg struct {
		index int
		err   error
		msg   string
	}
	promptTimeoutMsg struct{ index int }
)

type BuildTuiModel struct {
	steps   []stepInfo
	current int
	spinner spinner.Model
	ctx     *BuildContext
	logger  *Logger
	width   int
	height  int
	started time.Time
	done    bool
	success bool

	promptActive  bool
	promptStarted time.Time

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

var buildBannerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#8daea5"))

var buildSubStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6272A4")).
	Italic(true)

var buildStepNameStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F8F8F2")).
	Width(34)

var buildStepNameBoldStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFB86C")).
	Bold(true).
	Width(34)

var buildIconStyle = lipgloss.NewStyle().
	Width(2)

var buildMsgStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C0C0C0"))

var buildErrorMsgStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FF5555"))

var buildSuccessStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#50FA7B")).
	Bold(true)

var buildPromptStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFB86C")).
	Bold(true)

var buildFooterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6272A4"))

var buildDividerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#44475A"))

func divider(width int) string {
	return buildDividerStyle.Render(strings.Repeat("─", width))
}

func readBanner(projectDir string) string {
	data, err := os.ReadFile(filepath.Join(projectDir, "pkg", "assets", "banner.txt"))
	if err != nil {
		return "  stackyrd-nano"
	}
	return string(data)
}

func NewBuildTuiModel(ctx *BuildContext, logger *Logger) BuildTuiModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))

	prompts := map[string]struct {
		text string
		def  bool
	}{
		"Configure Garble":          {"Use garble build for obfuscation?", false},
		"Configure UPX Compression": {"Apply UPX LZMA compression to the binary?", false},
	}

	skipPrompt := map[string]bool{
		"Configure Garble":          ctx.Config.UseGarble,
		"Configure UPX Compression": ctx.Config.UseUPX,
	}

	stepDefs := []struct {
		name   string
		action func(*BuildContext, *Logger) error
	}{
		{"Check Project Path", (*BuildContext).checkPath},
		{"Check Required Tools", (*BuildContext).checkRequiredTools},
		{"Configure Garble", nil},
		{"Stop Running Process", (*BuildContext).stopRunningProcess},
		{"Create Backup", (*BuildContext).createBackup},
		{"Archive Backup", (*BuildContext).archiveBackup},
		{"Compile Plugins", (*BuildContext).compilePlugins},
		{"Build Application", (*BuildContext).buildApplication},
		{"Configure UPX Compression", nil},
		{"Compress with UPX", (*BuildContext).compressWithUPX},
		{"Copy Assets", (*BuildContext).copyAssets},
	}

	steps := make([]stepInfo, len(stepDefs))
	for i, sd := range stepDefs {
		st := statusPending
		if skipPrompt[sd.name] {
			st = statusSkipped
		}
		info := stepInfo{
			name:   sd.name,
			status: st,
			action: sd.action,
		}
		if p, ok := prompts[sd.name]; ok {
			info.isPrompt = true
			info.promptText = p.text
			info.promptDef = p.def
		}
		if st == statusSkipped {
			info.message = "enabled via flag"
		}
		steps[i] = info
	}

	return BuildTuiModel{
		steps:   steps,
		ctx:     ctx,
		logger:  logger,
		spinner: s,
		started: time.Now(),
		banner:  readBanner(ctx.ProjectDir),
		log: &logState{
			lines: make([]string, 0, 100),
			max:   100,
		},
	}
}

func (m BuildTuiModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
		func() tea.Msg {
			return tea.WindowSizeMsg{Width: 100, Height: 30}
		},
	)
}

func tickCmd() tea.Cmd {
	return tea.Every(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m BuildTuiModel) runStepCmd(index int) tea.Cmd {
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
			return stepDoneMsg{index: index, err: err, msg: msg}
		}
		err := step.action(m.ctx, m.logger)
		msg := ""
		if err == nil {
			msg = "Done"
		} else {
			msg = err.Error()
		}
		return stepDoneMsg{index: index, err: err, msg: msg}
	}
}

func (m BuildTuiModel) startPrompt(index int) tea.Cmd {
	m.steps[index].status = statusRunning
	if m.ctx.Config.Timeout > 0 {
		return tea.Tick(m.ctx.Config.Timeout, func(t time.Time) tea.Msg {
			return promptTimeoutMsg{index: index}
		})
	}
	return nil
}

func (m *BuildTuiModel) advanceToNext() tea.Cmd {
	m.current++
	if m.current >= len(m.steps) {
		m.done = true
		m.success = true
		for _, s := range m.steps {
			if s.status == statusError {
				m.success = false
				break
			}
		}
		return tea.Tick(800*time.Millisecond, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	}
	return m.triggerCurrentStep()
}

func (m *BuildTuiModel) triggerCurrentStep() tea.Cmd {
	step := &m.steps[m.current]

	if step.status == statusSkipped {
		return m.advanceToNext()
	}

	if step.isPrompt && step.status == statusPending {
		step.status = statusRunning
		m.promptActive = true
		m.promptStarted = time.Now()
		return m.startPrompt(m.current)
	}

	step.status = statusRunning
	return m.runStepCmd(m.current)
}

func (m BuildTuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				if s.name == "Configure Garble" {
					m.ctx.Config.UseGarble = false
				} else if s.name == "Configure UPX Compression" {
					m.ctx.Config.UseUPX = false
				}
				s.status = statusSuccess
				s.message = "skipped"
				return m, m.advanceToNext()
			}
			m.quitting = true
			return m, tea.Quit
		}

		if m.promptActive {
			s := &m.steps[m.current]
			switch msg.String() {
			case "y", "Y":
				m.promptActive = false
				if s.name == "Configure Garble" {
					m.ctx.Config.UseGarble = true
				} else if s.name == "Configure UPX Compression" {
					m.ctx.Config.UseUPX = true
				}
				s.status = statusSuccess
				s.message = "yes"
				return m, m.advanceToNext()
			case "n", "N", "enter":
				m.promptActive = false
				if s.name == "Configure Garble" {
					m.ctx.Config.UseGarble = false
				} else if s.name == "Configure UPX Compression" {
					m.ctx.Config.UseUPX = false
				}
				s.status = statusSuccess
				s.message = "no"
				return m, m.advanceToNext()
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
			m.quitting = true
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, tea.Batch(cmd, tickCmd())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case stepDoneMsg:
		if msg.index < len(m.steps) {
			s := &m.steps[msg.index]
			if msg.err != nil {
				s.status = statusError
				s.message = msg.msg
				m.done = true
				m.success = false
				return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return tickMsg(t)
				})
			}
			s.status = statusSuccess
			s.message = msg.msg
			return m, m.advanceToNext()
		}

	case promptTimeoutMsg:
		if m.promptActive && msg.index == m.current {
			m.promptActive = false
			s := &m.steps[msg.index]
			if s.name == "Configure Garble" {
				m.ctx.Config.UseGarble = s.promptDef
			} else if s.name == "Configure UPX Compression" {
				m.ctx.Config.UseUPX = s.promptDef
			}
			s.status = statusSuccess
			s.message = "no (timeout)"
			return m, m.advanceToNext()
		}
	}

	return m, nil
}

var logHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#6272A4"))

var logLineStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C0C0C0"))

func (m BuildTuiModel) View() string {
	if !m.ready {
		return ""
	}

	var b strings.Builder

	if m.banner != "" {
		lines := strings.Split(strings.TrimRight(m.banner, "\n"), "\n")
		for _, l := range lines {
			trimmed := strings.TrimRight(l, " ")
			if trimmed != "" {
				b.WriteString(buildBannerStyle.Render("  " + trimmed))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(buildBannerStyle.Render("  stackyrd-nano Builder"))
	b.WriteString("\n")
	b.WriteString(buildSubStyle.Render("  by diameter-tscd"))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(divider(min(m.width, 80)))
	b.WriteString("\n")

	for i, s := range m.steps {
		var icon, statusText, label string
		label = s.name

		switch s.status {
		case statusPending:
			icon = buildIconStyle.Render(" ")
			statusText = buildMsgStyle.Render("waiting")
		case statusRunning:
			if s.isPrompt && m.promptActive {
				icon = buildIconStyle.Render("?")
				elapsed := time.Since(m.promptStarted)
				remaining := m.ctx.Config.Timeout - elapsed
				if remaining < 0 {
					remaining = 0
				}
				secs := int(remaining.Seconds())
				if secs < 0 {
					secs = 0
				}
				promptLine := fmt.Sprintf("%s (y/N) [%ds]", s.promptText, secs)
				statusText = buildPromptStyle.Render(promptLine)
			} else {
				icon = buildIconStyle.Render(m.spinner.View())
				statusText = buildMsgStyle.Render("running...")
			}
		case statusSuccess:
			icon = buildIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Render("*"))
			if s.message == "Done" || s.message == "" {
				statusText = buildSuccessStyle.Render("ok")
			} else if s.message == "yes" {
				statusText = buildSuccessStyle.Render("enabled")
			} else if s.message == "no" || s.message == "no (timeout)" || s.message == "skipped" {
				statusText = buildMsgStyle.Render(s.message)
			} else {
				statusText = buildMsgStyle.Render(s.message)
			}
		case statusError:
			icon = buildIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Render("!"))
			statusText = buildErrorMsgStyle.Render(s.message)
		case statusSkipped:
			icon = buildIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Render("-"))
			statusText = buildMsgStyle.Render(s.message)
		}

		nameStyle := buildStepNameStyle
		if i == m.current && s.status == statusRunning {
			nameStyle = buildStepNameBoldStyle
		}

		line := fmt.Sprintf("  %s %s %s",
			icon,
			nameStyle.Render(label),
			statusText,
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(divider(min(m.width, 80)))
	b.WriteString("\n")

	maxWidth := min(m.width, 80)
	if maxWidth < 30 {
		maxWidth = 30
	}

	availLogLines := m.height - 26
	if len(m.banner) > 0 {
		bannerLineCount := strings.Count(m.banner, "\n")
		availLogLines -= bannerLineCount - 1
	}
	if availLogLines < 3 {
		availLogLines = 3
	}

	visibleLogs := m.log.visible(availLogLines)

	if len(visibleLogs) > 0 || !m.done {
		b.WriteString(logHeaderStyle.Render("  ▪ Build Log"))
		b.WriteString("\n")
		b.WriteString(buildDividerStyle.Render(strings.Repeat("─", maxWidth-4)))
		b.WriteString("\n")

		for _, line := range visibleLogs {
			display := line
			if len(display) > maxWidth-8 {
				display = display[:maxWidth-8]
			}
			b.WriteString(logLineStyle.Render("  " + display))
			b.WriteString("\n")
		}

		remainingLines := availLogLines - len(visibleLogs)
		for i := 0; i < remainingLines; i++ {
			b.WriteString("\n")
		}
	}

	if m.done {
		elapsed := time.Since(m.started).Round(time.Millisecond)
		if m.success {
			b.WriteString(buildSuccessStyle.Render(fmt.Sprintf("  Build complete in %s", elapsed)))
			b.WriteString("\n")
			b.WriteString(buildMsgStyle.Render(fmt.Sprintf("  Output: %s", m.ctx.DistPath)))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true).Render("  Build failed"))
			b.WriteString("\n")
			b.WriteString(buildErrorMsgStyle.Render("  Check the errors above"))
		}
		b.WriteString("\n\n")
		b.WriteString(buildFooterStyle.Render("  Press any key to exit"))
	} else if m.promptActive {
		b.WriteString(buildFooterStyle.Render("  y / n  |  q to skip  |  ctrl+c to quit"))
	} else {
		b.WriteString(buildFooterStyle.Render("  Building...  |  ctrl+c to quit"))
	}

	b.WriteString("\n")
	container := lipgloss.NewStyle().Padding(1, 2)
	return container.Render(b.String())
}

func RunBuildTUI(ctx *BuildContext, logger *Logger) (*BuildContext, error) {
	m := NewBuildTuiModel(ctx, logger)

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
	fm, ok := final.(BuildTuiModel)
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
	return fm.ctx, fmt.Errorf("build failed")
}
