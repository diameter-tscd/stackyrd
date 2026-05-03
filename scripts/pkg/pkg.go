package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Configuration constants
const (
	APP_NAME           = "stackyrd-pkg-installer"
	INDEX_URL          = "https://raw.githubusercontent.com/diameter-tscd/stackyrd-pkg/master/index"
	BASE_DOWNLOAD_URL  = "https://raw.githubusercontent.com/diameter-tscd/stackyrd-pkg/master"
	INSTALL_ROOT       = "pkg/infrastructure"
	FILE_WHITELIST     = `\.yrd$|\.go$` // Only allow .yrd and .go files for download
	SCRIPT_BINARY_PATH = "scripts/pkg/"
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

// PackageInfo holds the available versions and their files for a single package
type PackageInfo struct {
	Name      string              // e.g. "cloud/aws/ec2"
	Versions  map[string][]string // version -> list of filenames
	FilePaths map[string][]string // version -> list of full file paths (relative to repo root, without ./)
}

// InstallConfig holds runtime options
type InstallConfig struct {
	Timeout time.Duration
	Verbose bool
}

// InstallContext holds state during execution
type InstallContext struct {
	Config      InstallConfig
	ProjectDir  string
	InstallRoot string
	YrdConvExec string
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
func (ctx *InstallContext) ensureProjectRoot(logger *Logger) error {
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
		ctx.InstallRoot = filepath.Join(projectRoot, INSTALL_ROOT)

		logger.Success("Now in project root")
	} else {
		logger.Info("Already in project root")
	}

	// Ensure install root directory exists
	if err := os.MkdirAll(ctx.InstallRoot, 0755); err != nil {
		logger.Error("Failed to create install directory: %v", err)
		os.Exit(1)
	}

	return nil
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

func NewLogger(verbose bool) *Logger {
	return &Logger{verbose: verbose}
}

// clearScreen clears the terminal
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

// printBanner shows the program header
func printBanner() {
	fmt.Println("")
	fmt.Println("   " + P_PURPLE + " /\\ " + RESET)
	fmt.Println("   " + P_PURPLE + "(  )" + RESET + "   " + B_PURPLE + APP_NAME + RESET + " " + GRAY + "by" + RESET + " " + B_WHITE + "diameter-tscd" + RESET)
	fmt.Println("   " + P_PURPLE + " \\/ " + RESET)
	fmt.Println(GRAY + "----------------------------------------------------------------------" + RESET)
}

// printSuccess prints completion message
func printSuccess(target string) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SUCCESS!" + RESET + " " + P_GREEN + "Package installed at:" + RESET + " " + UNDERLINE + B_WHITE + target + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)
}

// setupSignalHandler gracefully handles Ctrl+C
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

// fetchIndex downloads and parses the package index
func fetchIndex(logger *Logger) ([]*PackageInfo, error) {
	logger.Info("Fetching index from %s", INDEX_URL)
	resp, err := http.Get(INDEX_URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index fetch returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read index body: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	packagesMap := make(map[string]*PackageInfo)
	versionRegex := regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "./") {
			continue
		}
		// Remove leading ./
		path := line[2:]
		segments := strings.Split(path, "/")

		// Find version segment (should match X.Y.Z)
		versionIdx := -1
		for i, seg := range segments {
			if versionRegex.MatchString(seg) {
				versionIdx = i
				break
			}
		}
		if versionIdx < 0 {
			// Only shown in verbose mode
			logger.Debug("Skipping line with no version: %s", line)
			continue
		}

		pkgPath := strings.Join(segments[:versionIdx], "/")
		version := segments[versionIdx]
		filename := segments[len(segments)-1] // last segment is file name

		pkg, exists := packagesMap[pkgPath]
		if !exists {
			pkg = &PackageInfo{
				Name:      pkgPath,
				Versions:  make(map[string][]string),
				FilePaths: make(map[string][]string),
			}
			packagesMap[pkgPath] = pkg
		}
		// Append file if not already present for that version
		existingFiles := pkg.Versions[version]
		found := false
		for _, f := range existingFiles {
			if f == filename {
				found = true
				break
			}
		}
		if !found {
			pkg.Versions[version] = append(pkg.Versions[version], filename)
			// Store the full path (without ./) for dynamic URL construction
			fullPath := path // path already has the ./ removed
			pkg.FilePaths[version] = append(pkg.FilePaths[version], fullPath)
		}
	}

	// Convert map to slice for consistent ordering
	packageList := make([]*PackageInfo, 0, len(packagesMap))
	for _, pkg := range packagesMap {
		packageList = append(packageList, pkg)
	}
	return packageList, nil
}

// promptUserByName lets the user type a package name (or part of it)
// and selects the exact package.
func promptUserByName(packages []*PackageInfo, logger *Logger) (*PackageInfo, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\n%sEnter package name (or part of it) to search (or 'cancel' to exit):%s ", B_YELLOW, RESET)
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("input error: %w", err)
		}
		input = strings.TrimSpace(input)
		if strings.EqualFold(input, "cancel") {
			fmt.Println("Installation cancelled.")
			os.Exit(0)
		}

		// Filter packages containing the search string (case-insensitive)
		var matches []*PackageInfo
		lowerSearch := strings.ToLower(input)
		for _, pkg := range packages {
			if strings.Contains(strings.ToLower(pkg.Name), lowerSearch) {
				matches = append(matches, pkg)
			}
		}

		if len(matches) == 0 {
			fmt.Printf("%sNo matching packages found.%s\n", P_RED, RESET)
			continue
		}

		if len(matches) == 1 {
			logger.Info("Selected package: %s", matches[0].Name)
			return matches[0], nil
		}

		// Multiple matches: display them
		fmt.Printf("\n%sMatching packages:%s\n", B_PURPLE, RESET)
		for _, pkg := range matches {
			fmt.Printf("  %s%s%s\n", B_WHITE, pkg.Name, RESET)
		}

		// Ask for exact name from the list
		for {
			fmt.Printf("\n%sEnter the exact package name from the list (or 'search' to search again, 'cancel' to exit):%s ", B_YELLOW, RESET)
			choice, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("input error: %w", err)
			}
			choice = strings.TrimSpace(choice)
			if strings.EqualFold(choice, "cancel") {
				fmt.Println("Installation cancelled.")
				os.Exit(0)
			}
			if strings.EqualFold(choice, "search") {
				break // go back to the initial search prompt
			}
			// Find exactly matching package from matches
			for _, pkg := range matches {
				if pkg.Name == choice {
					logger.Info("Selected package: %s", pkg.Name)
					return pkg, nil
				}
			}
			fmt.Printf("%sInvalid package name. Please choose exactly from the list above.%s\n", P_RED, RESET)
		}
	}
}

// promptVersion lets the user select a version for the given package (numbered list)
func promptVersion(pkg *PackageInfo, logger *Logger) (string, error) {
	versions := make([]string, 0, len(pkg.Versions))
	for v := range pkg.Versions {
		versions = append(versions, v)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no versions available for %s", pkg.Name)
	}

	if len(versions) == 1 {
		logger.Info("Only one version available: %s", versions[0])
		return versions[0], nil
	}

	fmt.Printf("\n%sAvailable versions for %s:%s\n", B_PURPLE, pkg.Name, RESET)
	for i, v := range versions {
		fmt.Printf("  %s%d.%s %s%s%s\n", P_CYAN, i+1, RESET, B_WHITE, v, RESET)
	}

	reader := bufio.NewReader(os.Stdin)
	var selVersion string
	for {
		fmt.Printf("\n%sSelect version by number (or 0 to cancel):%s ", B_YELLOW, RESET)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "0" {
			fmt.Println("Installation cancelled.")
			os.Exit(0)
		}
		idx := 0
		_, err := fmt.Sscanf(input, "%d", &idx)
		if err != nil || idx < 1 || idx > len(versions) {
			fmt.Printf("%sInvalid choice. Please enter a number between 1 and %d (or 0 to cancel):%s ", B_RED, len(versions), RESET)
			continue
		}
		selVersion = versions[idx-1]
		break
	}
	logger.Info("Selected version: %s", selVersion)
	return selVersion, nil
}

// downloadFiles downloads all yrd files for the chosen package/version directly to the target directory
func downloadFiles(pkg string, version string, files []string, targetDir string, logger *Logger) error {
	// Compile whitelist regex
	whitelistRegex := regexp.MustCompile(FILE_WHITELIST)

	logger.Info("Downloading files to %s", targetDir)
	for _, f := range files {
		// Check file against whitelist
		if !whitelistRegex.MatchString(f) {
			logger.Debug("Skipping file not in whitelist: %s", f)
			continue
		}

		// Construct raw GitHub URL: base + pkg/infrastructure/<pkg>/<version>/<file>
		remotePath := fmt.Sprintf("%s/%s/%s/%s", BASE_DOWNLOAD_URL, pkg, version, f)
		logger.Debug("Downloading %s", remotePath)

		resp, err := http.Get(remotePath)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", f, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("download of %s returned status %d", f, resp.StatusCode)
		}

		localPath := filepath.Join(targetDir, f)
		outFile, err := os.Create(localPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", localPath, err)
		}
		_, err = io.Copy(outFile, resp.Body)
		outFile.Close()
		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", localPath, err)
		}
		logger.Success("Downloaded %s", f)
	}
	return nil
}

// yrdconvURLs maps OS and architecture to the appropriate yrdconv binary URL
var yrdconvURLs = map[string]map[string]string{
	"windows": {
		"amd64": "https://github.com/diameter-tscd/stackyrd-pkg/releases/download/v1.0.0-yrdconv/yrdconv.exe",
	},
	"darwin": {
		"amd64": "https://github.com/diameter-tscd/stackyrd-pkg/releases/download/v1.0.0-yrdconv/yrdconv_darwin_amd64",
		"arm64": "https://github.com/diameter-tscd/stackyrd-pkg/releases/download/v1.0.0-yrdconv/yrdconv_darwin_arm64",
	},
	"linux": {
		"amd64": "https://github.com/diameter-tscd/stackyrd-pkg/releases/download/v1.0.0-yrdconv/yrdconv_linux_amd64",
	},
}

// ensureYrdconv checks if yrdconv binary exists, and downloads it if not found
func ensureYrdconv(ctx *InstallContext, logger *Logger) (string, error) {
	yrdPath := filepath.Join(ctx.ProjectDir, SCRIPT_BINARY_PATH)
	yrdPathWithBinary := filepath.Join(yrdPath, "yrdconv")

	// First, check if yrdconv is already available in PATH
	if path, err := exec.LookPath(yrdPathWithBinary); err == nil {
		logger.Debug("yrdconv found in PATH: %s", path)
		return yrdPathWithBinary, nil
	}

	// Determine OS and architecture
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	logger.Info("yrdconv not found in PATH, attempting to download for %s/%s", goos, goarch)

	// Get the appropriate URL
	osMap, ok := yrdconvURLs[goos]
	if !ok {
		return "", fmt.Errorf("unsupported operating system: %s", goos)
	}
	url, ok := osMap[goarch]
	if !ok {
		return "", fmt.Errorf("unsupported architecture for %s: %s", goos, goarch)
	}

	// Determine the binary name for the current platform
	binaryName := "yrdconv"
	if goos == "windows" {
		binaryName = "yrdconv.exe"
	}

	// Download to a temporary location in the current directory
	// or to a location in PATH (we'll use current directory for simplicity)
	downloadPath := filepath.Join(yrdPath, binaryName)

	// set ctx
	ctx.YrdConvExec = downloadPath

	// Check if already downloaded in current directory
	// if _, err := os.Stat(downloadPath); err == nil {
	// 	// Make sure it's executable on Unix-like systems
	// 	if goos != "windows" {
	// 		if err := os.Chmod(downloadPath, 0755); err != nil {
	// 			return "", fmt.Errorf("failed to make yrdconv executable: %w", err)
	// 		}
	// 	}
	// 	logger.Info("Using previously downloaded yrdconv: %s", downloadPath)
	// 	return "./" + downloadPath, nil
	// }

	// Download the binary
	logger.Info("Downloading yrdconv from %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download yrdconv: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("yrdconv download returned status %d", resp.StatusCode)
	}

	outFile, err := os.Create(downloadPath)
	if err != nil {
		return "", fmt.Errorf("failed to create yrdconv file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write yrdconv binary: %w", err)
	}

	// Make executable on Unix-like systems
	if goos != "windows" {
		if err := os.Chmod(downloadPath, 0755); err != nil {
			return "", fmt.Errorf("failed to make yrdconv executable: %w", err)
		}
	}

	logger.Success("yrdconv downloaded successfully: %s", downloadPath)
	return downloadPath, nil
}

// convertAndInstall handles convertion of yrd files using yrdconv binary
func convertAndInstall(ctx *InstallContext, pkg string, version string, files []string, targetDir string, logger *Logger) error {

	// Ensure yrdconv binary is available
	yrdconvPath, err := ensureYrdconv(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to ensure yrdconv is available: %w", err)
	}

	// Compile whitelist regex to only allow .yrd files for conversion
	whitelistRegex := regexp.MustCompile(FILE_WHITELIST)

	for _, f := range files {
		// Verify file is allowed for conversion (only .yrd files permitted)
		if !whitelistRegex.MatchString(f) {
			logger.Debug("Skipping file not in conversion whitelist: %s", f)
			continue
		}

		yrdPath := filepath.Join(targetDir, f)
		logger.Debug("Checking yrdPath: %s", yrdPath)
		logger.Debug("Checking yrdConvPath: %s", yrdconvPath)
		logger.Debug("Checking targetDir: %s", targetDir)

		logger.Info("Converting %s", f)
		cmd := exec.Command(yrdconvPath, fmt.Sprintf("-dir=%s", yrdPath))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = targetDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("convert failed for %s: %w", f, err)
		}

		// After conversion, we expect the output file to be named without .yrd
		convertedName := strings.TrimSuffix(f, ".yrd")
		convertedPath := filepath.Join(targetDir, convertedName)
		if _, err := os.Stat(convertedPath); os.IsNotExist(err) {
			// Fallback: try to find any file in targetDir that is not a .yrd (just created)
			entries, _ := os.ReadDir(targetDir)
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".yrd") && !e.IsDir() {
					convertedName = e.Name()
					convertedPath = filepath.Join(targetDir, convertedName)
					break
				}
			}
		}

		// Optionally, if the converted file is not already named correctly, rename
		targetPath := filepath.Join(targetDir, convertedName)
		if convertedPath != targetPath {
			logger.Info("Renaming %s -> %s", filepath.Base(convertedPath), convertedName)
			if err := os.Rename(convertedPath, targetPath); err != nil {
				return fmt.Errorf("failed to rename converted file: %w", err)
			}
		}

		// Clean up original .yrd
		os.Remove(yrdPath)
	}
	return nil
}

func main() {
	ClearScreen()

	var (
		timeoutSeconds = flag.Int("timeout", 30, "Timeout for user prompts in seconds (unused for now)")
		verbose        = flag.Bool("verbose", false, "Enable verbose logging")
		installPkg     = flag.String("pkg", "", "Package to install directly (format: 'name@version', e.g., 'cloud/aws/ec2@1.0.0'). Skips interactive prompts.")
	)
	flag.Parse()

	logger := NewLogger(*verbose)
	printBanner()

	// Setup context with cancellation for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	setupSignalHandler(cancel)

	// Get current directory
	projectDir, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get current directory: %v", err)
		os.Exit(1)
	}

	ctx := &InstallContext{
		Config: InstallConfig{
			Timeout: time.Duration(*timeoutSeconds) * time.Second,
			Verbose: *verbose,
		},
		ProjectDir:  projectDir,
		InstallRoot: filepath.Join(projectDir, INSTALL_ROOT),
	}

	// Ensure we're in the project root
	if err := ctx.ensureProjectRoot(logger); err != nil {
		logger.Error("Failed to ensure project root: %v", err)
		os.Exit(1)
	}

	logger.Info("Fetching package index...")
	packages, err := fetchIndex(logger)
	if err != nil {
		logger.Error("Failed to fetch index: %v", err)
		os.Exit(1)
	}
	logger.Success("Fetched %d package(s)", len(packages))

	if len(packages) == 0 {
		logger.Error("No packages available")
		os.Exit(1)
	}

	var selectedPkg *PackageInfo
	var selectedVersion string
	var files []string

	if *installPkg != "" {
		// Non-interactive mode: parse package@version
		parts := strings.SplitN(*installPkg, "@", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			logger.Error("Invalid package format. Use 'name@version' (e.g., 'cloud/aws/ec2@1.0.0')")
			os.Exit(1)
		}
		pkgName := parts[0]
		version := parts[1]

		// Find the package in the index
		for _, pkg := range packages {
			if pkg.Name == pkgName {
				selectedPkg = pkg
				break
			}
		}
		if selectedPkg == nil {
			logger.Error("Package '%s' not found in index", pkgName)
			os.Exit(1)
		}

		// Check if the version exists
		var ok bool
		files, ok = selectedPkg.Versions[version]
		if !ok {
			logger.Error("Version '%s' not found for package '%s'", version, pkgName)
			os.Exit(1)
		}
		selectedVersion = version

		logger.Info("Selected package: %s, version: %s, files: %v", selectedPkg.Name, selectedVersion, files)
	} else {
		// Interactive package selection (by name, not by number)
		var err error
		selectedPkg, err = promptUserByName(packages, logger)
		if err != nil {
			logger.Error("Selection error: %v", err)
			os.Exit(1)
		}

		// Version selection (numbered list)
		selectedVersion, err = promptVersion(selectedPkg, logger)
		if err != nil {
			logger.Error("Version selection error: %v", err)
			os.Exit(1)
		}

		files = selectedPkg.Versions[selectedVersion]
	}

	if len(files) == 0 {
		logger.Error("No files found for %s version %s", selectedPkg.Name, selectedVersion)
		os.Exit(1)
	}

	// Extract path string from filepaths
	var packageFileName string
	for _, v := range selectedPkg.FilePaths[selectedVersion] {
		if strings.Contains(v, ".yrd") {
			packageFileName = path.Base(v)
			packageFileName = strings.ReplaceAll(packageFileName, ".yrd", ".go")
			break
		}
	}

	// Check if package is already installed
	if info, err := os.Stat(ctx.InstallRoot); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(ctx.InstallRoot)
		installed := false
		for _, e := range entries {
			if e.Name() == packageFileName {
				installed = true
				break
			}
		}
		if installed {
			logger.Info("Package %s@%s is already installed at %s", selectedPkg.Name, selectedVersion, ctx.InstallRoot)
			printSuccess(ctx.InstallRoot)
			os.Exit(0)
		}
	}

	// Download files directly to target directory
	if err := downloadFiles(selectedPkg.Name, selectedVersion, files, ctx.InstallRoot, logger); err != nil {
		logger.Error("Download failed: %v", err)
		os.Exit(1)
	}

	// Ensure yrdconv binary is available (download if missing)
	if _, err := ensureYrdconv(ctx, logger); err != nil {
		logger.Error("Failed to ensure yrdconv is available: %v", err)
		os.Exit(1)
	}

	// Convert and install (works directly in target directory)
	if err := convertAndInstall(ctx, selectedPkg.Name, selectedVersion, files, ctx.InstallRoot, logger); err != nil {
		logger.Error("Installation failed: %v", err)
		os.Exit(1)
	}

	// Run go mod tidy to update dependencies
	logger.Info("Running go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = ctx.ProjectDir
	if err := cmd.Run(); err != nil {
		logger.Warn("go mod tidy failed: %v (non-fatal, continuing...)", err)
	}

	// Recommend package documentation - dynamically get README URL from stored file paths
	readmeURL := ""
	if filePaths, ok := selectedPkg.FilePaths[selectedVersion]; ok {
		for _, fullPath := range filePaths {
			if strings.HasSuffix(fullPath, "README.md") {
				// Construct the GitHub blob URL using the full path from index
				readmeURL = fmt.Sprintf("https://github.com/diameter-tscd/stackyrd-pkg/blob/master/%s", fullPath)
				break
			}
		}
	}
	if readmeURL != "" {
		logger.Info("Recommended: Read the package documentation at: %s", readmeURL)
	} else {
		logger.Info("No README.md found for this package version")
	}

	// Final success message
	printSuccess(ctx.InstallRoot)
}
