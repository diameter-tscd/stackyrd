package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const (
	APP_NAME           = "stackyrd-pkg-installer"
	INDEX_URL          = "https://raw.githubusercontent.com/diameter-tscd/stackyrd-pkg/master/index"
	BASE_DOWNLOAD_URL  = "https://raw.githubusercontent.com/diameter-tscd/stackyrd-pkg/master"
	INSTALL_ROOT       = "pkg/infrastructure"
	FILE_WHITELIST     = `\.yrd$|\.go$`
	SCRIPT_BINARY_PATH = "scripts/pkg/"
	MANIFEST_FILE      = "package.yml"
	INDEX_CACHE_PATH   = "store/pkg-index.cache"
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

// Archive format constants
const (
	FormatTar     = "tar"
	Format7z      = "7z"
	DefaultFormat = FormatTar
)

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

type PackageInfo struct {
	Name      string
	Versions  map[string][]string
	FilePaths map[string][]string
}

type InstallConfig struct {
	Timeout       time.Duration
	Verbose       bool
	ArchiveFormat string
	NoTUI         bool
}

type InstallContext struct {
	Config      InstallConfig
	ProjectDir  string
	InstallRoot string
	YrdConvExec string
	Timestamp   string
	DistPath    string

	Packages        []*PackageInfo
	SelectedPkg     *PackageInfo
	SelectedVersion string
	Files           []string
	ReadmeURL       string
}

type Manifest struct {
	Meta     ManifestMeta                `yaml:"meta"`
	Packages map[string]InstalledPackage `yaml:"packages"`
}

type ManifestMeta struct {
	LastUpdated string `yaml:"last_updated"`
	IndexURL    string `yaml:"index_url"`
}

type InstalledPackage struct {
	Name        string   `yaml:"name"`
	Version     string   `yaml:"version"`
	InstalledAt string   `yaml:"installed_at"`
	UpdatedAt   string   `yaml:"updated_at"`
	Files       []string `yaml:"files"`
	InstallRoot string   `yaml:"install_root"`
	Source      string   `yaml:"source"`
	ManualPath  string   `yaml:"manual_path,omitempty"`
}

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
func (l *Logger) Printf(msg string, args ...interface{}) { fmt.Fprintf(l.writer, msg, args...) }
func (l *Logger) Println(msg string)                     { fmt.Fprintln(l.writer, msg) }
func NewLogger(verbose bool) *Logger                     { return &Logger{verbose: verbose, writer: os.Stdout} }

func findProjectRoot(startDir string) (string, error) {
	current := startDir
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
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

func (ctx *InstallContext) ensureProjectRoot(logger *Logger) error {
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
		ctx.InstallRoot = filepath.Join(projectRoot, INSTALL_ROOT)
		logger.Success("Now in project root")
	} else {
		logger.Info("Already in project root")
	}
	if err := os.MkdirAll(ctx.InstallRoot, 0755); err != nil {
		logger.Error("Failed to create install directory: %v", err)
		os.Exit(1)
	}
	return nil
}

func ClearScreen() {
	cmd := exec.Command("clear")
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	}
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

func printBanner() {
	fmt.Println("")
	fmt.Println("   " + P_PURPLE + " /\\ " + RESET)
	fmt.Println("   " + P_PURPLE + "(  )" + RESET + "   " + B_PURPLE + APP_NAME + RESET + " " + GRAY + "by" + RESET + " " + B_WHITE + "diameter-tscd" + RESET)
	fmt.Println("   " + P_PURPLE + " \\/ " + RESET)
	fmt.Println(GRAY + "----------------------------------------------------------------------" + RESET)
}

func printSuccess(target string) {
	fmt.Println("")
	fmt.Println(GRAY + "======================================================================" + RESET)
	fmt.Println(" " + B_PURPLE + "SUCCESS!" + RESET + " " + P_GREEN + "Package installed at:" + RESET + " " + UNDERLINE + B_WHITE + target + RESET)
	fmt.Println(GRAY + "======================================================================" + RESET)
}

func isTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

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

type TerminalGuard struct {
	fd       int
	oldState *term.State
}

func NewTerminalGuard() (*TerminalGuard, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("make raw: %w", err)
	}
	return &TerminalGuard{fd: fd, oldState: oldState}, nil
}

func (g *TerminalGuard) Restore() {
	if g.oldState == nil {
		return
	}
	_ = term.Restore(g.fd, g.oldState)
	g.oldState = nil
}

func GuardTerminal() *TerminalGuard {
	g, err := NewTerminalGuard()
	if err != nil {
		fmt.Fprintf(os.Stderr, "raw mode: %v\n", err)
		os.Exit(1)
	}
	return g
}

func confirmPrompt(msg string, logger *Logger) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s%s [y/N]:%s ", B_YELLOW, msg, RESET)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func parseIndexLines(body string, logger *Logger) []*PackageInfo {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	packagesMap := make(map[string]*PackageInfo)
	versionRegex := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "./") {
			continue
		}
		cleanPath := line[2:]
		segments := strings.Split(cleanPath, "/")
		versionIdx := -1
		for i, seg := range segments {
			if versionRegex.MatchString(seg) {
				versionIdx = i
				break
			}
		}
		if versionIdx < 0 {
			logger.Debug("Skipping line with no version: %s", line)
			continue
		}
		pkgPath := strings.Join(segments[:versionIdx], "/")
		version := segments[versionIdx]
		filename := segments[len(segments)-1]
		pkg, exists := packagesMap[pkgPath]
		if !exists {
			pkg = &PackageInfo{
				Name:      pkgPath,
				Versions:  make(map[string][]string),
				FilePaths: make(map[string][]string),
			}
			packagesMap[pkgPath] = pkg
		}
		found := false
		for _, f := range pkg.Versions[version] {
			if f == filename {
				found = true
				break
			}
		}
		if !found {
			pkg.Versions[version] = append(pkg.Versions[version], filename)
			pkg.FilePaths[version] = append(pkg.FilePaths[version], cleanPath)
		}
	}
	list := make([]*PackageInfo, 0, len(packagesMap))
	for _, pkg := range packagesMap {
		list = append(list, pkg)
	}
	return list
}

func fetchIndex(logger *Logger) ([]*PackageInfo, error) {
	logger.Info("Fetching index from %s", INDEX_URL)
	resp, err := http.Get(INDEX_URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download index: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("index fetch returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read index body: %w", err)
	}
	return parseIndexLines(string(body), logger), nil
}

func loadCachedIndex(logger *Logger) ([]*PackageInfo, error) {
	data, err := os.ReadFile(INDEX_CACHE_PATH)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(INDEX_CACHE_PATH); err == nil && time.Since(info.ModTime()) > time.Hour {
		logger.Warn("Index cache is over 1 hour old. Run 'update' to refresh.")
	}
	return parseIndexLines(string(data), logger), nil
}

func saveCachedIndex(body string) error {
	if err := os.MkdirAll(filepath.Dir(INDEX_CACHE_PATH), 0755); err != nil {
		return err
	}
	return os.WriteFile(INDEX_CACHE_PATH, []byte(body), 0644)
}

func loadManifest() (*Manifest, error) {
	data, err := os.ReadFile(MANIFEST_FILE)
	if err != nil {
		if os.IsNotExist(err) {
			return &Manifest{
				Meta:     ManifestMeta{IndexURL: INDEX_URL},
				Packages: make(map[string]InstalledPackage),
			}, nil
		}
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Packages == nil {
		m.Packages = make(map[string]InstalledPackage)
	}
	if m.Meta.IndexURL == "" {
		m.Meta.IndexURL = INDEX_URL
	}
	return &m, nil
}

func saveManifest(m *Manifest) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	tmpPath := MANIFEST_FILE + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, MANIFEST_FILE)
}

func manifestAddPackage(m *Manifest, ip InstalledPackage) {
	m.Packages[ip.Name] = ip
}
func manifestRemovePackage(m *Manifest, name string) {
	delete(m.Packages, name)
}
func manifestIsInstalled(m *Manifest, name string) bool {
	_, ok := m.Packages[name]
	return ok
}
func manifestGetPackage(m *Manifest, name string) (*InstalledPackage, bool) {
	p, ok := m.Packages[name]
	if !ok {
		return nil, false
	}
	return &p, true
}

func promptUserByName(packages []*PackageInfo, logger *Logger) (*PackageInfo, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\n%sEnter package name (or part of it) to search (or 'cancel' to exit):%s ", B_YELLOW, RESET)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if strings.EqualFold(input, "cancel") {
			fmt.Println("Installation cancelled.")
			os.Exit(0)
		}
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
		fmt.Printf("\n%sMatching packages:%s\n", B_PURPLE, RESET)
		for i, pkg := range matches {
			fmt.Printf("  %s%2d.%s %s%s%s\n", P_CYAN, i+1, RESET, B_WHITE, pkg.Name, RESET)
		}
		for {
			fmt.Printf("\n%sEnter number or exact package name (or 'search' to search again, 'cancel' to exit):%s ", B_YELLOW, RESET)
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)
			if strings.EqualFold(choice, "cancel") {
				fmt.Println("Installation cancelled.")
				os.Exit(0)
			}
			if strings.EqualFold(choice, "search") {
				break
			}
			var n int
			if _, err := fmt.Sscanf(choice, "%d", &n); err == nil && n >= 1 && n <= len(matches) {
				logger.Info("Selected package: %s", matches[n-1].Name)
				return matches[n-1], nil
			}
			for _, pkg := range matches {
				if pkg.Name == choice {
					logger.Info("Selected package: %s", pkg.Name)
					return pkg, nil
				}
			}
			fmt.Printf("%sInvalid choice. Enter a number (1-%d) or exact package name from the list.%s\n", P_RED, len(matches), RESET)
		}
	}
}

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
	for {
		fmt.Printf("\n%sSelect version by number (or 0 to cancel):%s ", B_YELLOW, RESET)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "0" {
			fmt.Println("Installation cancelled.")
			os.Exit(0)
		}
		idx := 0
		if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(versions) {
			fmt.Printf("%sInvalid choice. Please enter a number between 1 and %d (or 0 to cancel):%s ", B_RED, len(versions), RESET)
			continue
		}
		return versions[idx-1], nil
	}
}

func downloadFiles(pkg, version string, files []string, targetDir string, logger *Logger) error {
	whitelistRegex := regexp.MustCompile(FILE_WHITELIST)
	logger.Info("Downloading files to %s", targetDir)
	for _, f := range files {
		if !whitelistRegex.MatchString(f) {
			logger.Debug("Skipping file not in whitelist: %s", f)
			continue
		}
		remotePath := fmt.Sprintf("%s/%s/%s/%s", BASE_DOWNLOAD_URL, pkg, version, f)
		logger.Debug("Downloading %s", remotePath)
		resp, err := http.Get(remotePath)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", f, err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return fmt.Errorf("download of %s returned status %d", f, resp.StatusCode)
		}
		localPath := filepath.Join(targetDir, f)
		outFile, err := os.Create(localPath)
		if err != nil {
			_ = resp.Body.Close()
			return fmt.Errorf("failed to create file %s: %w", localPath, err)
		}
		_, err = io.Copy(outFile, resp.Body)
		_ = resp.Body.Close()
		_ = outFile.Close()
		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", localPath, err)
		}
		logger.Success("Downloaded %s", f)
	}
	return nil
}

var yrdconvURLs = map[string]map[string]string{
	"windows": {"amd64": "https://github.com/diameter-tscd/stackyrd-pkg/releases/download/v1.0.0-yrdconv/yrdconv.exe"},
	"darwin": {
		"amd64": "https://github.com/diameter-tscd/stackyrd-pkg/releases/download/v1.0.0-yrdconv/yrdconv_darwin_amd64",
		"arm64": "https://github.com/diameter-tscd/stackyrd-pkg/releases/download/v1.0.0-yrdconv/yrdconv_darwin_arm64",
	},
	"linux": {"amd64": "https://github.com/diameter-tscd/stackyrd-pkg/releases/download/v1.0.0-yrdconv/yrdconv_linux_amd64"},
}

func ensureYrdconv(ctx *InstallContext, logger *Logger) (string, error) {
	yrdPath := filepath.Join(ctx.ProjectDir, SCRIPT_BINARY_PATH)
	yrdPathWithBinary := filepath.Join(yrdPath, "yrdconv")
	if p, err := exec.LookPath(yrdPathWithBinary); err == nil {
		logger.Debug("yrdconv found in PATH: %s", p)
		return yrdPathWithBinary, nil
	}
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	logger.Info("yrdconv not found in PATH, attempting to download for %s/%s", goos, goarch)
	osMap, ok := yrdconvURLs[goos]
	if !ok {
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}
	url, ok := osMap[goarch]
	if !ok {
		return "", fmt.Errorf("unsupported arch for %s: %s", goos, goarch)
	}
	binaryName := "yrdconv"
	if goos == "windows" {
		binaryName = "yrdconv.exe"
	}
	downloadPath := filepath.Join(yrdPath, binaryName)
	ctx.YrdConvExec = downloadPath
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
		return "", fmt.Errorf("failed to create yrdconv: %w", err)
	}
	defer func() { _ = outFile.Close() }()
	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write yrdconv: %w", err)
	}
	if goos != "windows" {
		_ = os.Chmod(downloadPath, 0755)
	}
	logger.Success("yrdconv downloaded: %s", downloadPath)
	return downloadPath, nil
}

func convertAndInstall(ctx *InstallContext, pkg, version string, files []string, targetDir string, logger *Logger) error {
	yrdconvPath, err := ensureYrdconv(ctx, logger)
	if err != nil {
		return fmt.Errorf("failed to ensure yrdconv: %w", err)
	}
	whitelistRegex := regexp.MustCompile(FILE_WHITELIST)
	for _, f := range files {
		if !whitelistRegex.MatchString(f) {
			logger.Debug("Skipping non-yrd file: %s", f)
			continue
		}
		yrdPath := filepath.Join(targetDir, f)
		logger.Info("Converting %s", f)
		cmd := exec.Command(yrdconvPath, fmt.Sprintf("-dir=%s", yrdPath))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = targetDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("convert failed for %s: %w", f, err)
		}
		convertedName := strings.TrimSuffix(f, ".yrd")
		convertedPath := filepath.Join(targetDir, convertedName)
		if _, err := os.Stat(convertedPath); os.IsNotExist(err) {
			entries, _ := os.ReadDir(targetDir)
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".yrd") && !e.IsDir() {
					convertedName = e.Name()
					convertedPath = filepath.Join(targetDir, convertedName)
					break
				}
			}
		}
		targetPath := filepath.Join(targetDir, convertedName)
		if convertedPath != targetPath {
			logger.Info("Renaming %s -> %s", filepath.Base(convertedPath), convertedName)
			if err := os.Rename(convertedPath, targetPath); err != nil {
				return fmt.Errorf("failed to rename: %w", err)
			}
		}
		_ = os.Remove(yrdPath)
	}
	return nil
}

func runGoModTidy(projectDir string, logger *Logger) {
	logger.Info("Running go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = projectDir
	if err := cmd.Run(); err != nil {
		logger.Warn("go mod tidy failed: %v (non-fatal)", err)
	}
}

func trackedFiles(files []string) []string {
	whitelistRegex := regexp.MustCompile(FILE_WHITELIST)
	var result []string
	for _, f := range files {
		if !whitelistRegex.MatchString(f) {
			continue
		}
		name := strings.TrimSuffix(f, ".yrd")
		if name != f {
			name += ".go"
		}
		result = append(result, name)
	}
	return result
}

type installResult struct {
	pkgName   string
	version   string
	files     []string
	readmeURL string
}

func installIndexed(ctx *InstallContext, pkgName, version string, files []string, filePaths map[string][]string, logger *Logger) (*installResult, error) {
	if err := downloadFiles(pkgName, version, files, ctx.InstallRoot, logger); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if _, err := ensureYrdconv(ctx, logger); err != nil {
		logger.Warn("Failed to ensure yrdconv: %v (continuing)", err)
	} else if err := convertAndInstall(ctx, pkgName, version, files, ctx.InstallRoot, logger); err != nil {
		return nil, fmt.Errorf("conversion failed: %w", err)
	}
	installed := trackedFiles(files)
	readmeURL := ""
	if fPaths, ok := filePaths[version]; ok {
		for _, fullPath := range fPaths {
			if strings.HasSuffix(fullPath, "README.md") {
				readmeURL = fmt.Sprintf("https://github.com/diameter-tscd/stackyrd-pkg/blob/master/%s", fullPath)
				break
			}
		}
	}
	return &installResult{pkgName: pkgName, version: version, files: installed, readmeURL: readmeURL}, nil
}

func updateManifest(name, version string, files []string, logger *Logger) {
	manifest, err := loadManifest()
	if err != nil {
		manifest = &Manifest{Meta: ManifestMeta{IndexURL: INDEX_URL}, Packages: make(map[string]InstalledPackage)}
	}
	now := time.Now().Format(time.RFC3339)
	manifestAddPackage(manifest, InstalledPackage{
		Name: name, Version: version, InstalledAt: now, UpdatedAt: now,
		Files: files, InstallRoot: INSTALL_ROOT, Source: "index",
	})
	if err := saveManifest(manifest); err != nil {
		logger.Warn("Failed to save manifest: %v", err)
	}
}

func cmdList(logger *Logger) {
	manifest, err := loadManifest()
	if err != nil {
		logger.Error("Failed to load manifest: %v", err)
		os.Exit(1)
	}
	if len(manifest.Packages) == 0 {
		logger.Println("No packages installed.")
		return
	}
	index, indexErr := loadCachedIndex(logger)
	latestVersion := make(map[string]string)
	if indexErr == nil {
		for _, pkg := range index {
			var versions []string
			for v := range pkg.Versions {
				versions = append(versions, v)
			}
			sort.Strings(versions)
			if len(versions) > 0 {
				latestVersion[pkg.Name] = versions[len(versions)-1]
			}
		}
	}
	names := make([]string, 0, len(manifest.Packages))
	for name := range manifest.Packages {
		names = append(names, name)
	}
	sort.Strings(names)
	logger.Println("")
	logger.Printf("%sInstalled packages (%d):%s\n", B_PURPLE, len(names), RESET)
	for _, name := range names {
		p := manifest.Packages[name]
		installedDate := p.InstalledAt
		if len(installedDate) > 10 {
			installedDate = installedDate[:10]
		}
		status := fmt.Sprintf("%s✓%s up to date", P_GREEN, RESET)
		if latest, ok := latestVersion[name]; ok {
			if latest != p.Version {
				status = fmt.Sprintf("%s↑%s %s available", B_YELLOW, RESET, latest)
			}
		} else if indexErr == nil {
			status = fmt.Sprintf("%s-%s not in index", GRAY, RESET)
		}
		logger.Printf("  %s%-25s%s %-10s %-12s %s\n", B_WHITE, name, RESET, p.Version, installedDate, status)
	}
	logger.Println("")
}

func cmdInfo(args []string, logger *Logger) {
	if len(args) == 0 {
		logger.Error("Usage: info <package-name>")
		os.Exit(1)
	}
	manifest, err := loadManifest()
	if err != nil {
		logger.Error("Failed to load manifest: %v", err)
		os.Exit(1)
	}
	p, ok := manifestGetPackage(manifest, args[0])
	if !ok {
		logger.Error("Package '%s' is not installed", args[0])
		os.Exit(1)
	}
	logger.Println("")
	logger.Printf("  %sPackage:%s  %s%s%s\n", BOLD, RESET, B_WHITE, p.Name, RESET)
	logger.Printf("  %sVersion:%s  %s\n", BOLD, RESET, p.Version)
	logger.Printf("  %sSource:%s   %s\n", BOLD, RESET, p.Source)
	if p.ManualPath != "" {
		logger.Printf("  %sPath:%s    %s\n", BOLD, RESET, p.ManualPath)
	}
	logger.Printf("  %sFiles:%s    %s\n", BOLD, RESET, strings.Join(p.Files, ", "))
	logger.Printf("  %sInstalled:%s %s\n", BOLD, RESET, p.InstalledAt)
	logger.Printf("  %sUpdated:%s   %s\n", BOLD, RESET, p.UpdatedAt)
	logger.Printf("  %sStatus:%s   %s✓ installed%s\n", BOLD, RESET, P_GREEN, RESET)
	logger.Println("")
}

func resolvePackageName(manifest *Manifest, input string, logger *Logger) (*InstalledPackage, error) {
	if p, ok := manifestGetPackage(manifest, input); ok {
		return p, nil
	}
	var matches []*InstalledPackage
	for _, p := range manifest.Packages {
		segments := strings.Split(p.Name, "/")
		last := segments[len(segments)-1]
		if last == input || strings.HasSuffix(p.Name, "/"+input) {
			matches = append(matches, &InstalledPackage{Name: p.Name, Version: p.Version, Files: p.Files, InstallRoot: p.InstallRoot, Source: p.Source, InstalledAt: p.InstalledAt, UpdatedAt: p.UpdatedAt, ManualPath: p.ManualPath})
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no installed package matches '%s'", input)
	}
	if len(matches) == 1 {
		logger.Info("Matched package: %s", matches[0].Name)
		return matches[0], nil
	}
	fmt.Printf("\n%sMultiple packages match '%s':%s\n", B_YELLOW, input, RESET)
	for i, p := range matches {
		fmt.Printf("  %s%d.%s %s%s%s @ %s\n", P_CYAN, i+1, RESET, B_WHITE, p.Name, RESET, p.Version)
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\n%sSelect package by number (or 0 to cancel):%s ", B_YELLOW, RESET)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "0" {
			return nil, fmt.Errorf("cancelled")
		}
		idx := 0
		if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(matches) {
			fmt.Printf("%sInvalid choice. Enter a number between 1 and %d:%s ", P_RED, len(matches), RESET)
			continue
		}
		return matches[idx-1], nil
	}
}

func cmdRemove(args []string, logger *Logger) {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	yes := fs.Bool("yes", false, "Skip confirmation prompt")
	dryRun := fs.Bool("dry-run", false, "Preview without removing")
	_ = fs.Parse(args)
	positional := fs.Args()
	if len(positional) == 0 {
		logger.Error("Usage: remove [flags] <package-name>")
		os.Exit(1)
	}
	manifest, err := loadManifest()
	if err != nil {
		logger.Error("Failed to load manifest: %v", err)
		os.Exit(1)
	}
	p, err := resolvePackageName(manifest, positional[0], logger)
	if err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
	projectDir, err := findProjectRoot(".")
	if err != nil {
		logger.Error("Failed to find project root: %v", err)
		os.Exit(1)
	}
	installRoot := filepath.Join(projectDir, p.InstallRoot)
	if *dryRun {
		logger.Info("[DRY-RUN] Would remove %s@%s", p.Name, p.Version)
		for _, f := range p.Files {
			logger.Info("  - %s", filepath.Join(installRoot, f))
		}
		return
	}
	if !*yes && !confirmPrompt(fmt.Sprintf("Remove %s@%s?", p.Name, p.Version), logger) {
		logger.Info("Cancelled.")
		return
	}
	for _, f := range p.Files {
		fullPath := filepath.Join(installRoot, f)
		absRoot, _ := filepath.Abs(installRoot)
		absPath, _ := filepath.Abs(fullPath)
		if !strings.HasPrefix(absPath, absRoot) {
			logger.Warn("Skipping file outside install root: %s", fullPath)
			continue
		}
		if err := os.Remove(fullPath); err != nil {
			if os.IsNotExist(err) {
				logger.Warn("File already removed: %s", fullPath)
			} else {
				logger.Error("Failed to remove %s: %v", fullPath, err)
			}
		} else {
			logger.Success("Removed %s", fullPath)
		}
	}
	manifestRemovePackage(manifest, p.Name)
	if err := saveManifest(manifest); err != nil {
		logger.Warn("Failed to save manifest: %v", err)
	}
	runGoModTidy(projectDir, logger)
	logger.Success("Package '%s' removed", p.Name)
}

func cmdReinstall(args []string, logger *Logger) {
	fs := flag.NewFlagSet("reinstall", flag.ExitOnError)
	installPkg := fs.String("pkg", "", "Package to reinstall (format: 'name@version')")
	yes := fs.Bool("yes", false, "Skip confirmation prompt")
	dryRun := fs.Bool("dry-run", false, "Preview without reinstalling")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	_ = fs.Parse(args)
	logger.verbose = *verbose

	positional := fs.Args()
	if *installPkg == "" && len(positional) == 0 {
		logger.Error("Usage: reinstall [flags] <package-name>")
		os.Exit(1)
	}
	projectDir, err := findProjectRoot(".")
	if err != nil {
		logger.Error("Failed to find project root: %v", err)
		os.Exit(1)
	}
	ctx := &InstallContext{
		ProjectDir:  projectDir,
		InstallRoot: filepath.Join(projectDir, INSTALL_ROOT),
	}
	packages, err := fetchIndex(logger)
	if err != nil {
		logger.Error("Failed to fetch index: %v", err)
		os.Exit(1)
	}
	var pkgName, version string
	if *installPkg != "" {
		parts := strings.SplitN(*installPkg, "@", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			logger.Error("Invalid format. Use 'name@version'")
			os.Exit(1)
		}
		pkgName, version = parts[0], parts[1]
	} else {
		manifest, err := loadManifest()
		if err != nil {
			logger.Error("Failed to load manifest: %v", err)
			os.Exit(1)
		}
		p, err := resolvePackageName(manifest, positional[0], logger)
		if err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
		pkgName = p.Name
		version = p.Version
	}
	var selectedPkg *PackageInfo
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
	files, ok := selectedPkg.Versions[version]
	if !ok {
		logger.Error("Version '%s' not found for '%s'", version, pkgName)
		os.Exit(1)
	}
	if !*yes && !confirmPrompt(fmt.Sprintf("Reinstall %s@%s?", pkgName, version), logger) {
		logger.Info("Cancelled.")
		return
	}
	if *dryRun {
		logger.Info("[DRY-RUN] Would reinstall %s@%s files: %v", pkgName, version, files)
		return
	}
	result, err := installIndexed(ctx, pkgName, version, files, selectedPkg.FilePaths, logger)
	if err != nil {
		logger.Error("Reinstall failed: %v", err)
		os.Exit(1)
	}
	updateManifest(result.pkgName, result.version, result.files, logger)
	runGoModTidy(ctx.ProjectDir, logger)
	if result.readmeURL != "" {
		logger.Info("Documentation: %s", result.readmeURL)
	}
	printSuccess(ctx.InstallRoot)
}

func cmdUpgrade(args []string, logger *Logger) {
	fs := flag.NewFlagSet("upgrade", flag.ExitOnError)
	yes := fs.Bool("yes", false, "Skip confirmation")
	dryRun := fs.Bool("dry-run", false, "Preview without upgrading")
	_ = fs.Parse(args)
	positional := fs.Args()
	upgradeOne := ""
	if len(positional) > 0 {
		upgradeOne = positional[0]
	}
	manifest, err := loadManifest()
	if err != nil {
		logger.Error("Failed to load manifest: %v", err)
		os.Exit(1)
	}
	if len(manifest.Packages) == 0 {
		logger.Info("No packages installed.")
		return
	}
	index, err := fetchIndex(logger)
	if err != nil {
		logger.Error("Failed to fetch index: %v", err)
		os.Exit(1)
	}
	type latestInfo struct {
		Version   string
		Files     []string
		FilePaths map[string][]string
	}
	latest := make(map[string]latestInfo)
	for _, pkg := range index {
		var versions []string
		for v := range pkg.Versions {
			versions = append(versions, v)
		}
		sort.Strings(versions)
		if len(versions) > 0 {
			lv := versions[len(versions)-1]
			latest[pkg.Name] = latestInfo{Version: lv, Files: pkg.Versions[lv], FilePaths: pkg.FilePaths}
		}
	}
	type target struct {
		Name, Current, Latest string
		Files                 []string
		FilePaths             map[string][]string
	}
	var targets []target
	for name, p := range manifest.Packages {
		if upgradeOne != "" && name != upgradeOne {
			continue
		}
		if p.Source == "manual" {
			logger.Debug("Skipping manual-source: %s", name)
			continue
		}
		li, ok := latest[name]
		if !ok {
			logger.Debug("Skipping %s: not in index", name)
			continue
		}
		if li.Version == p.Version {
			continue
		}
		targets = append(targets, target{Name: name, Current: p.Version, Latest: li.Version, Files: li.Files, FilePaths: li.FilePaths})
	}
	if upgradeOne != "" && len(targets) == 0 {
		if _, ok := manifestGetPackage(manifest, upgradeOne); !ok {
			logger.Error("Package '%s' not installed", upgradeOne)
		} else {
			logger.Info("Package '%s' is already at latest.", upgradeOne)
		}
		os.Exit(1)
	}
	if len(targets) == 0 {
		logger.Info("All packages up to date.")
		return
	}
	projectDir, _ := os.Getwd()
	ctx := &InstallContext{
		ProjectDir: projectDir, InstallRoot: filepath.Join(projectDir, INSTALL_ROOT),
	}
	if err := ctx.ensureProjectRoot(logger); err != nil {
		logger.Error("Failed to ensure project root: %v", err)
		os.Exit(1)
	}
	failed := 0
	for _, t := range targets {
		logger.Println("")
		logger.Info("Upgrading %s: %s → %s", t.Name, t.Current, t.Latest)
		if *dryRun {
			logger.Info("[DRY-RUN] Would upgrade %s: %s → %s", t.Name, t.Current, t.Latest)
			continue
		}
		if !*yes && !confirmPrompt(fmt.Sprintf("Upgrade %s %s → %s?", t.Name, t.Current, t.Latest), logger) {
			logger.Info("Skipping %s.", t.Name)
			continue
		}
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		backups := make(map[string]string)
		for _, f := range t.Files {
			srcPath := filepath.Join(ctx.InstallRoot, f)
			if _, err := os.Stat(srcPath); err == nil {
				bakPath := srcPath + ".bak." + timestamp
				if err := os.Rename(srcPath, bakPath); err != nil {
					logger.Warn("Failed to backup %s: %v", srcPath, err)
				} else {
					backups[srcPath] = bakPath
				}
			}
		}
		if err := downloadFiles(t.Name, t.Latest, t.Files, ctx.InstallRoot, logger); err != nil {
			logger.Error("Download failed for %s: %v", t.Name, err)
			for orig, bak := range backups {
				_ = os.Rename(bak, orig)
			}
			failed++
			continue
		}
		if _, err := ensureYrdconv(ctx, logger); err != nil {
			logger.Warn("yrdconv unavailable: %v", err)
		} else if err := convertAndInstall(ctx, t.Name, t.Latest, t.Files, ctx.InstallRoot, logger); err != nil {
			logger.Error("Conversion failed for %s: %v", t.Name, err)
			for orig, bak := range backups {
				_ = os.Rename(bak, orig)
			}
			failed++
			continue
		}
		for _, bak := range backups {
			_ = os.Remove(bak)
		}
		installed := trackedFiles(t.Files)
		now := time.Now().Format(time.RFC3339)
		manifest, err := loadManifest()
		if err != nil {
			manifest = &Manifest{Meta: ManifestMeta{IndexURL: INDEX_URL}, Packages: make(map[string]InstalledPackage)}
		}
		if p, exists := manifestGetPackage(manifest, t.Name); exists {
			p.Version = t.Latest
			p.UpdatedAt = now
			p.Files = installed
			manifestAddPackage(manifest, *p)
		} else {
			manifestAddPackage(manifest, InstalledPackage{
				Name: t.Name, Version: t.Latest, InstalledAt: now, UpdatedAt: now,
				Files: installed, InstallRoot: INSTALL_ROOT, Source: "index",
			})
		}
		if err := saveManifest(manifest); err != nil {
			logger.Warn("Failed to save manifest: %v", err)
		}
		logger.Success("Upgraded %s: %s → %s", t.Name, t.Current, t.Latest)
	}
	runGoModTidy(projectDir, logger)
	if failed > 0 {
		logger.Warn("%d upgrade(s) failed", failed)
	} else {
		logger.Success("All upgrades complete")
	}
}

func cmdUpdate(logger *Logger) {
	manifest, err := loadManifest()
	if err != nil {
		manifest = &Manifest{Meta: ManifestMeta{IndexURL: INDEX_URL}, Packages: make(map[string]InstalledPackage)}
	}
	logger.Info("Fetching index from %s", INDEX_URL)
	resp, err := http.Get(INDEX_URL)
	if err != nil {
		logger.Error("Failed to fetch index: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Error("Index returned status %d", resp.StatusCode)
		os.Exit(1)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read index: %v", err)
		os.Exit(1)
	}
	if err := saveCachedIndex(string(body)); err != nil {
		logger.Warn("Failed to cache index: %v", err)
	}
	manifest.Meta.LastUpdated = time.Now().Format(time.RFC3339)
	manifest.Meta.IndexURL = INDEX_URL
	if err := saveManifest(manifest); err != nil {
		logger.Warn("Failed to save manifest: %v", err)
	}
	packages := parseIndexLines(string(body), logger)
	logger.Success("Index updated (%d packages)", len(packages))
	updates := 0
	for name, p := range manifest.Packages {
		if p.Source == "manual" {
			continue
		}
		for _, pkg := range packages {
			if pkg.Name == name {
				var versions []string
				for v := range pkg.Versions {
					versions = append(versions, v)
				}
				sort.Strings(versions)
				if len(versions) > 0 && versions[len(versions)-1] != p.Version {
					if updates == 0 {
						logger.Info("Updates available:")
					}
					logger.Info("  %s  %s → %s", name, p.Version, versions[len(versions)-1])
					updates++
				}
				break
			}
		}
	}
	if updates == 0 {
		logger.Info("All installed packages are at their latest versions.")
	}
}

func printUsage() {
	fmt.Printf(`%sUsage:%s go run scripts/pkg/pkg.go <command> [flags]

%sCommands:%s
  install    Install a package (interactive, or via -pkg/-path flags)
  reinstall  Re-download and re-install an existing package
  list       List installed packages with version and upgrade status
  info       Show detailed information for an installed package
  remove     Remove an installed package
  upgrade    Upgrade all (or one) installed packages to latest versions
  update     Refresh the local package index cache

%sFlags:%s
  --no-tui            Force plain CLI output mode
  --timeout seconds   Timeout for user prompts (default 30)
  --verbose           Enable verbose logging
  --pkg name@version  Package to install directly (e.g. 'cloud/aws/ec2@1.0.0')
  -h, --help          Show this help message

Run '%scmd -h%s' for subcommand-specific flags (e.g. 'remove -h').
`, B_WHITE, RESET, B_WHITE, RESET, B_WHITE, RESET, B_CYAN, RESET)
}

// ─── Install Step Definitions ────────────────────────────────────────────────

type stepAction func(*InstallContext, *Logger) error

var installSteps = []struct {
	name   string
	action stepAction
}{
	{"Check Project Path", (*InstallContext).stepCheckPath},
	{"Fetch Package Index", stepFetchIndex},
	{"Select Package", stepSelectPackage},
	{"Select Version", stepSelectVersion},
	{"Download Files", stepDownloadFiles},
	{"Ensure yrdconv", stepEnsureYrdconv},
	{"Convert & Install", stepConvertAndInstall},
	{"Update Manifest", stepUpdateManifest},
	{"Run go mod tidy", stepRunGoModTidy},
}

func (ctx *InstallContext) stepCheckPath(logger *Logger) error {
	return ctx.ensureProjectRoot(logger)
}

func stepFetchIndex(ctx *InstallContext, logger *Logger) error {
	packages, err := fetchIndex(logger)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages available")
	}
	ctx.Packages = packages
	return nil
}

func stepSelectPackage(ctx *InstallContext, logger *Logger) error {
	if ctx.SelectedPkg != nil && len(ctx.SelectedPkg.Versions) > 0 {
		return nil
	}
	if ctx.SelectedPkg != nil && ctx.SelectedPkg.Name != "" {
		for _, pkg := range ctx.Packages {
			if pkg.Name == ctx.SelectedPkg.Name {
				ctx.SelectedPkg = pkg
				return nil
			}
		}
		return fmt.Errorf("package '%s' not found in index", ctx.SelectedPkg.Name)
	}
	var err error
	ctx.SelectedPkg, err = promptUserByName(ctx.Packages, logger)
	return err
}

func stepSelectVersion(ctx *InstallContext, logger *Logger) error {
	if ctx.SelectedVersion != "" && len(ctx.Files) > 0 {
		return nil
	}
	if ctx.SelectedVersion != "" && len(ctx.Files) == 0 {
		files, ok := ctx.SelectedPkg.Versions[ctx.SelectedVersion]
		if !ok {
			return fmt.Errorf("version '%s' not found for '%s'", ctx.SelectedVersion, ctx.SelectedPkg.Name)
		}
		ctx.Files = files
		return nil
	}
	var err error
	ctx.SelectedVersion, err = promptVersion(ctx.SelectedPkg, logger)
	if err != nil {
		return err
	}
	ctx.Files = ctx.SelectedPkg.Versions[ctx.SelectedVersion]
	return nil
}

func stepDownloadFiles(ctx *InstallContext, logger *Logger) error {
	return downloadFiles(ctx.SelectedPkg.Name, ctx.SelectedVersion, ctx.Files, ctx.InstallRoot, logger)
}

func stepEnsureYrdconv(ctx *InstallContext, logger *Logger) error {
	_, err := ensureYrdconv(ctx, logger)
	return err
}

func stepConvertAndInstall(ctx *InstallContext, logger *Logger) error {
	return convertAndInstall(ctx, ctx.SelectedPkg.Name, ctx.SelectedVersion, ctx.Files, ctx.InstallRoot, logger)
}

func stepUpdateManifest(ctx *InstallContext, logger *Logger) error {
	installed := trackedFiles(ctx.Files)
	updateManifest(ctx.SelectedPkg.Name, ctx.SelectedVersion, installed, logger)
	for _, fPath := range ctx.SelectedPkg.FilePaths[ctx.SelectedVersion] {
		if strings.HasSuffix(fPath, "README.md") {
			ctx.ReadmeURL = fmt.Sprintf("https://github.com/diameter-tscd/stackyrd-pkg/blob/master/%s", fPath)
			break
		}
	}
	return nil
}

func stepRunGoModTidy(ctx *InstallContext, logger *Logger) error {
	runGoModTidy(ctx.ProjectDir, logger)
	return nil
}

// ─── CLI Mode ─────────────────────────────────────────────────────────────────

func printInstallSuccess(ctx *InstallContext) {
	ClearScreen()
	printBanner()
	fmt.Printf("\n  %s✓ %s installed successfully%s\n", B_GREEN, ctx.SelectedPkg.Name, RESET)
	fmt.Printf("  %sVersion:%s %s\n", BOLD, RESET, ctx.SelectedVersion)
	fmt.Println("")
	if ctx.ReadmeURL != "" {
		fmt.Printf("  %sSee README to read package documentation:%s\n", B_YELLOW, RESET)
		fmt.Printf("  %s%s%s\n", UNDERLINE+WHITE, ctx.ReadmeURL, RESET)
	} else {
		fmt.Printf("  %sSee README to read package documentation%s\n", B_YELLOW, RESET)
	}
	fmt.Println("")
	fmt.Println(GRAY + "----------------------------------------------------------------------" + RESET)
}

func runCLIInstall(ctx *InstallContext, logger *Logger) {
	for i, step := range installSteps {
		stepNum := fmt.Sprintf("%d/%d", i+1, len(installSteps))
		fmt.Printf("%s[%s]%s %s%s%s\n", B_PURPLE, stepNum, RESET, P_CYAN, step.name, RESET)

		if err := step.action(ctx, logger); err != nil {
			logger.Error("Step failed: %v", err)
			os.Exit(1)
		}
	}

	printInstallSuccess(ctx)
}

// ─── Bubble Tea TUI ───────────────────────────────────────────────────────────

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
	promptDef  string
	skipMsg    string
	action     stepAction
}

type (
	tickMsg        time.Time
	stepDoneMsg    struct {
		index int
		err   error
		msg   string
	}
	promptTimeoutMsg struct{ index int }
	doneTimeoutMsg   struct{}
)

type PkgTuiModel struct {
	steps   []stepInfo
	current int
	spinner spinner.Model
	ctx     *InstallContext
	logger  *Logger
	width   int
	height  int
	started time.Time
	done        bool
	success     bool
	doneAt      time.Time
	completedIn time.Duration

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

var pkgBannerStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#8daea5"))

var pkgSubStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6272A4")).
	Italic(true)

var pkgStepNameStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#F8F8F2")).
	Width(34)

var pkgStepNameBoldStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFB86C")).
	Bold(true).
	Width(34)

var pkgIconStyle = lipgloss.NewStyle().
	Width(2)

var pkgMsgStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C0C0C0"))

var pkgErrorMsgStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FF5555"))

var pkgSuccessStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#50FA7B")).
	Bold(true)

var pkgPromptStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFB86C")).
	Bold(true)

var pkgFooterStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#6272A4"))

var pkgDividerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#44475A"))

var logHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#6272A4"))

var logLineStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#C0C0C0"))

func pkgDivider(width int) string {
	return pkgDividerStyle.Render(strings.Repeat("─", width))
}

func readBanner(projectDir string) string {
	data, err := os.ReadFile(filepath.Join(projectDir, "pkg", "assets", "banner.txt"))
	if err != nil {
		return "  stackyrd"
	}
	return string(data)
}

func NewPkgTuiModel(ctx *InstallContext, logger *Logger) PkgTuiModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF79C6"))

	steps := make([]stepInfo, len(installSteps))
	for i, sd := range installSteps {
		st := statusPending
		skipMsg := ""
		if sd.name == "Select Package" && ctx.SelectedPkg != nil {
			st = statusSkipped
			skipMsg = "already selected"
		}
		if sd.name == "Select Version" && ctx.SelectedVersion != "" {
			st = statusSkipped
			skipMsg = "provided via flag"
		}
		steps[i] = stepInfo{
			name:     sd.name,
			status:   st,
			action:   sd.action,
			skipMsg:  skipMsg,
			isPrompt: sd.name == "Select Package" || sd.name == "Select Version",
		}
		if sd.name == "Select Package" {
			steps[i].promptText = "Enter package name to search"
		}
		if sd.name == "Select Version" {
			steps[i].promptText = "Select version number"
		}
	}

	return PkgTuiModel{
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

func (m PkgTuiModel) Init() tea.Cmd {
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

func (m PkgTuiModel) runStepCmd(index int) tea.Cmd {
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

func (m PkgTuiModel) startPrompt(index int) tea.Cmd {
	m.steps[index].status = statusRunning
	if m.ctx.Config.Timeout > 0 {
		return tea.Tick(m.ctx.Config.Timeout, func(t time.Time) tea.Msg {
			return promptTimeoutMsg{index: index}
		})
	}
	return nil
}

func (m *PkgTuiModel) setDone(success bool) tea.Cmd {
	m.done = true
	m.success = success
	m.doneAt = time.Now()
	m.completedIn = time.Since(m.started).Round(time.Millisecond)
	return tea.Tick(15*time.Second, func(t time.Time) tea.Msg {
		return doneTimeoutMsg{}
	})
}

func (m *PkgTuiModel) advanceToNext() tea.Cmd {
	m.current++
	if m.current >= len(m.steps) {
		success := true
		for _, s := range m.steps {
			if s.status == statusError {
				success = false
				break
			}
		}
		return m.setDone(success)
	}
	return m.triggerCurrentStep()
}

func (m *PkgTuiModel) triggerCurrentStep() tea.Cmd {
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

func (m PkgTuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				s.status = statusSuccess
				s.message = "yes"
				return m, m.advanceToNext()
			case "n", "N", "enter":
				m.promptActive = false
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
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, tea.Batch(cmd, tickCmd())

	case doneTimeoutMsg:
		m.quitting = true
		return m, tea.Quit

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
			s.status = statusSuccess
			s.message = "no (timeout)"
			return m, m.advanceToNext()
		}
	}

	return m, nil
}

func (m PkgTuiModel) View() string {
	if !m.ready {
		return ""
	}

	var b strings.Builder

	if m.banner != "" {
		lines := strings.Split(strings.TrimRight(m.banner, "\n"), "\n")
		for _, l := range lines {
			trimmed := strings.TrimRight(l, " ")
			if trimmed != "" {
				b.WriteString(pkgBannerStyle.Render("  " + trimmed))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(pkgBannerStyle.Render("  stackyrd Package Installer"))
	b.WriteString("\n")
	b.WriteString(pkgSubStyle.Render("  by diameter-tscd"))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(pkgDivider(min(m.width, 80)))
	b.WriteString("\n")

	for i, s := range m.steps {
		var icon, statusText, label string
		label = s.name

		switch s.status {
		case statusPending:
			icon = pkgIconStyle.Render(" ")
			statusText = pkgMsgStyle.Render("waiting")
		case statusRunning:
			if s.isPrompt && m.promptActive {
				icon = pkgIconStyle.Render("?")
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
				statusText = pkgPromptStyle.Render(promptLine)
			} else {
				icon = pkgIconStyle.Render(m.spinner.View())
				statusText = pkgMsgStyle.Render("running...")
			}
		case statusSuccess:
			icon = pkgIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Render("*"))
			if s.message == "Done" || s.message == "" {
				statusText = pkgSuccessStyle.Render("ok")
			} else if s.message == "yes" {
				statusText = pkgSuccessStyle.Render("enabled")
			} else if s.message == "no" || s.message == "no (timeout)" || s.message == "skipped" {
				statusText = pkgMsgStyle.Render(s.message)
			} else {
				statusText = pkgMsgStyle.Render(s.message)
			}
		case statusError:
			icon = pkgIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Render("!"))
			statusText = pkgErrorMsgStyle.Render(s.message)
		case statusSkipped:
			icon = pkgIconStyle.Render(lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4")).Render("-"))
			if s.skipMsg != "" {
				statusText = pkgMsgStyle.Render(s.skipMsg)
			} else {
				statusText = pkgMsgStyle.Render(s.message)
			}
		}

		nameStyle := pkgStepNameStyle
		if i == m.current && s.status == statusRunning {
			nameStyle = pkgStepNameBoldStyle
		}

		line := fmt.Sprintf("  %s %s %s",
			icon,
			nameStyle.Render(label),
			statusText,
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString(pkgDivider(min(m.width, 80)))
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
		b.WriteString(pkgDividerStyle.Render(strings.Repeat("─", maxWidth-4)))
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
		if m.success {
			b.WriteString(pkgSuccessStyle.Render(fmt.Sprintf("  \u2713 Completed in %s\n", m.completedIn)))
			b.WriteString("\n")

			detailLines := []struct {
				label string
				value string
			}{
				{"Package", m.ctx.SelectedPkg.Name},
				{"Version", m.ctx.SelectedVersion},
				{"Output", m.ctx.InstallRoot},
			}
			if m.ctx.ReadmeURL != "" {
				detailLines = append(detailLines, struct {
					label string
					value string
				}{"README", m.ctx.ReadmeURL})
			}

			for _, dl := range detailLines {
				b.WriteString(pkgMsgStyle.Render(fmt.Sprintf("     %12s  %s", dl.label+":", dl.value)))
				b.WriteString("\n")
			}

			b.WriteString("\n")
			b.WriteString(pkgMsgStyle.Render("     Next steps:"))
			b.WriteString("\n")
			nextSteps := []string{
				"Import the package in your project to activate it",
				"Run 'go mod tidy' to ensure dependencies are resolved",
			}
			for i, step := range nextSteps {
				b.WriteString(pkgMsgStyle.Render(fmt.Sprintf("     %12s  %d. %s", "", i+1, step)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Bold(true).Render("  Failed"))
			b.WriteString("\n")
			b.WriteString(pkgErrorMsgStyle.Render("  Check the errors above"))
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
		b.WriteString(pkgFooterStyle.Render(closing))
	} else if m.promptActive {
		b.WriteString(pkgFooterStyle.Render("  y / n  |  q to skip  |  ctrl+c to quit"))
	} else {
		b.WriteString(pkgFooterStyle.Render("  Installing...  |  ctrl+c to quit"))
	}

	b.WriteString("\n")
	container := lipgloss.NewStyle().Padding(1, 2)
	return container.Render(b.String())
}

func RunPkgTUI(ctx *InstallContext, logger *Logger) (*InstallContext, error) {
	m := NewPkgTuiModel(ctx, logger)

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
	fm, ok := final.(PkgTuiModel)
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
	return fm.ctx, fmt.Errorf("install failed")
}

func main() {
	cmd := ""
	cmdArgs := []string{}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install", "reinstall", "list", "info", "remove", "upgrade", "update":
			cmd = os.Args[1]
			cmdArgs = os.Args[2:]
		case "-h", "--help", "help":
			printUsage()
			return
		}
	}

	verbose := false
	for _, a := range os.Args[1:] {
		if a == "-verbose" || a == "--verbose" || a == "-V" || a == "-v" {
			verbose = true
			break
		}
	}

	logger := NewLogger(verbose)

	if cmd != "" && cmd != "install" {
		ClearScreen()
		printBanner()
		switch cmd {
		case "reinstall":
			cmdReinstall(cmdArgs, logger)
			return
		case "list":
			cmdList(logger)
			return
		case "info":
			cmdInfo(cmdArgs, logger)
			return
		case "remove":
			cmdRemove(cmdArgs, logger)
			return
		case "upgrade":
			cmdUpgrade(cmdArgs, logger)
			return
		case "update":
			cmdUpdate(logger)
			return
		}
	}

	timeoutSeconds := flag.Int("timeout", 30, "Timeout for user prompts in seconds")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging")
	installPkg := flag.String("pkg", "", "Package to install directly (format: 'name@version')")
	noTUI := flag.Bool("no-tui", false, "Disable TUI, use plain CLI output")
	flag.Parse()

	logger.verbose = *verboseFlag || verbose

	useTUI := !*noTUI && isTerminal()

	if !useTUI {
		_, cancel := context.WithCancel(context.Background())
		setupSignalHandler(cancel)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get current directory: %v", err)
		os.Exit(1)
	}
	ctx := &InstallContext{
		Config: InstallConfig{
			Timeout: time.Duration(*timeoutSeconds) * time.Second,
			Verbose: logger.verbose,
			NoTUI:   *noTUI,
		},
		ProjectDir:  projectDir,
		InstallRoot: filepath.Join(projectDir, INSTALL_ROOT),
	}

	if *installPkg != "" {
		parts := strings.SplitN(*installPkg, "@", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			logger.Error("Invalid format. Use 'name@version'")
			os.Exit(1)
		}
		ctx.SelectedPkg = &PackageInfo{Name: parts[0]}
		ctx.SelectedVersion = parts[1]
	}

	if useTUI && *installPkg == "" {
		ClearScreen()
		printBanner()
		fmt.Println()
		fmt.Printf("%sInteractive selection mode%s\n\n", B_CYAN, RESET)

		logger.Info("Fetching package index...")
		packages, err := fetchIndex(logger)
		if err != nil {
			logger.Error("Failed to fetch index: %v", err)
			os.Exit(1)
		}
		if len(packages) == 0 {
			logger.Error("No packages available")
			os.Exit(1)
		}
		ctx.Packages = packages

		selectedPkg, err := promptUserByName(packages, logger)
		if err != nil {
			logger.Error("Selection error: %v", err)
			os.Exit(1)
		}
		ctx.SelectedPkg = selectedPkg

		selectedVersion, err := promptVersion(selectedPkg, logger)
		if err != nil {
			logger.Error("Version selection error: %v", err)
			os.Exit(1)
		}
		ctx.SelectedVersion = selectedVersion
		ctx.Files = selectedPkg.Versions[selectedVersion]

		if len(ctx.Files) == 0 {
			logger.Error("No files found for %s version %s", ctx.SelectedPkg.Name, ctx.SelectedVersion)
			os.Exit(1)
		}

		if m, e := loadManifest(); e == nil && manifestIsInstalled(m, ctx.SelectedPkg.Name) {
			printInstallSuccess(ctx)
			os.Exit(0)
		}
	}

	if useTUI {
		_, err := RunPkgTUI(ctx, logger)
		ClearScreen()
		if err != nil {
			fmt.Printf("Install failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\u2713 Completed: %s\n", ctx.SelectedPkg.Name)
	} else {
		ClearScreen()
		printBanner()
		runCLIInstall(ctx, logger)
	}
}
