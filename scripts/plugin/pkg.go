package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	APP_NAME  = "stackyrd"
	BASE_URL  = "http://localhost:8080/api/v1/plugins"
	USER_AGENT = "stackyrd-plugin-cli/1.0"
	HTTP_TIMEOUT = 30 * time.Second
)

const (
	RESET     = "\033[0m"
	BOLD      = "\033[1m"
	DIM       = "\033[2m"
	UNDERLINE = "\033[4m"
	P_PURPLE  = "\033[38;5;108m"
	B_PURPLE  = "\033[1;38;5;108m"
	P_CYAN    = "\033[38;5;117m"
	B_CYAN    = "\033[1;38;5;117m"
	P_GREEN   = "\033[38;5;108m"
	B_GREEN   = "\033[1;38;5;108m"
	P_YELLOW  = "\033[93m"
	B_YELLOW  = "\033[1;93m"
	P_RED     = "\033[91m"
	B_RED     = "\033[1;91m"
	GRAY      = "\033[38;5;242m"
	WHITE     = "\033[97m"
	B_WHITE   = "\033[1;97m"
)

type Logger struct{ verbose bool }

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
func (l *Logger) Printf(msg string, args ...interface{}) { fmt.Printf(msg, args...) }
func (l *Logger) Println(msg string)                     { fmt.Println(msg) }

func NewLogger(verbose bool) *Logger { return &Logger{verbose: verbose} }

func printBanner() {
	fmt.Println("")
	fmt.Println("   " + P_PURPLE + " /\\ " + RESET)
	fmt.Println("   " + P_PURPLE + "(  )" + RESET + "   " + B_PURPLE + APP_NAME + " Plugin Manager" + RESET + " " + GRAY + "by" + RESET + " " + B_WHITE + "diameter-tscd" + RESET)
	fmt.Println("   " + P_PURPLE + " \\/ " + RESET)
	fmt.Println(GRAY + "----------------------------------------------------------------------" + RESET)
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Logger     *Logger
}

func NewClient(baseURL string, logger *Logger) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: HTTP_TIMEOUT},
		Logger:     logger,
	}
}

func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	url := c.BaseURL + path
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", USER_AGENT)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTPClient.Do(req)
}

func (c *Client) getJSON(path string, target interface{}) error {
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) postJSON(path string, body, target interface{}) error {
	resp, err := c.doRequest("POST", path, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if target != nil {
		return json.NewDecoder(resp.Body).Decode(target)
	}
	return nil
}

func (c *Client) putJSON(path string, body, target interface{}) error {
	resp, err := c.doRequest("PUT", path, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if target != nil {
		return json.NewDecoder(resp.Body).Decode(target)
	}
	return nil
}

type PluginSummary struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	Type         string `json:"type"`
	Status       string `json:"status"`
	ExecuteCount int64  `json:"execute_count"`
	FileSize     int64  `json:"file_size"`
}

type PluginListResponse struct {
	Plugins       []PluginSummary `json:"plugins"`
	Total         int             `json:"total"`
	Loaded        int             `json:"loaded"`
	ActiveExecs   int32           `json:"active_execs"`
	Goroutines    int             `json:"goroutines"`
	MemoryBytes   int64           `json:"memory_bytes"`
	MemoryLimit   int64           `json:"memory_limit"`
	MemoryPercent float64         `json:"memory_percent"`
	UptimeSeconds float64         `json:"uptime_seconds"`
}

type PluginDetail struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	Description       string `json:"description"`
	Author            string `json:"author"`
	Entrypoint        string `json:"entrypoint"`
	Type              string `json:"type"`
	Status            string `json:"status"`
	LoadTimeMs        float64 `json:"load_time_ms"`
	EmbeddedFileSize  int64  `json:"embedded_file_size"`
	ExecuteCount      int64  `json:"execute_count"`
	LastExecutionMs   float64 `json:"last_execution_ms"`
	TotalExecutionMs  float64 `json:"total_execution_ms"`
}

type ExecuteResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type UploadResponse struct {
	Message string `json:"message"`
	Path    string `json:"path"`
}

type DeleteResponse struct {
	Message string `json:"message"`
	Name    string `json:"name"`
}

type ManagerStatus struct {
	TotalPlugins     int             `json:"total_plugins"`
	LoadedPlugins    int             `json:"loaded_plugins"`
	TotalExecutions  int64           `json:"total_executions"`
	ActiveExecutions int32           `json:"active_executions"`
	GoroutineCount   int             `json:"goroutine_count"`
	MemoryUsageBytes int64           `json:"memory_usage_bytes"`
	MemoryLimitBytes int64           `json:"memory_limit_bytes"`
	MemoryPercent    float64         `json:"memory_percent"`
	UptimeSeconds    float64         `json:"uptime_seconds"`
	Plugins          []PluginSummary `json:"plugins"`
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func cmdList(logger *Logger, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	baseURL := fs.String("url", BASE_URL, "Base URL of the plugin API")
	verbose := fs.Bool("verbose", false, "Verbose logging")
	_ = fs.Parse(args)
	logger.verbose = *verbose

	client := NewClient(*baseURL, logger)
	var resp PluginListResponse
	if err := client.getJSON("", &resp); err != nil {
		logger.Error("Failed to list plugins: %v", err)
		os.Exit(1)
	}

	logger.Println("")
	logger.Printf("%sPlugins (%d total, %d loaded):%s\n", B_PURPLE, resp.Total, resp.Loaded, RESET)
	logger.Println("")

	pluginTypeColor := func(t string) string {
		switch t {
		case "typescript":
			return P_CYAN
		case "lua":
			return P_GREEN
		case "external":
			return P_YELLOW
		case "go":
			return P_PURPLE
		default:
			return WHITE
		}
	}

	for _, p := range resp.Plugins {
		color := pluginTypeColor(p.Type)
		typeLabel := p.Type
		if typeLabel == "external" {
			typeLabel = "python"
		}

		statusIcon := "✓"
		statusColor := P_GREEN
		if p.Status != "loaded" {
			statusIcon = "✗"
			statusColor = P_RED
		}

		logger.Printf("  %s%-6s%s %s%s%s %s%-20s%s %s%-8s%s %s%3d exec%s %s%s%s",
			color, "["+typeLabel+"]", RESET,
			B_WHITE, p.Name, RESET,
			GRAY, truncate(p.Description, 42), RESET,
			BOLD, p.Version, RESET,
			P_CYAN, p.ExecuteCount, RESET,
			statusColor, statusIcon, RESET,
		)
	}

	logger.Println("")
	logger.Printf("  %sManager:%s %d active execs | %d goroutines | %s memory | %s / %s (%.0f%%) | uptime %.0fs\n",
		GRAY, RESET,
		resp.ActiveExecs, resp.Goroutines,
		formatBytes(resp.MemoryBytes),
		formatBytes(resp.MemoryBytes), formatBytes(resp.MemoryLimit), resp.MemoryPercent,
		resp.UptimeSeconds,
	)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func cmdInfo(logger *Logger, args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	baseURL := fs.String("url", BASE_URL, "Base URL of the plugin API")
	verbose := fs.Bool("verbose", false, "Verbose logging")
	_ = fs.Parse(args)
	logger.verbose = *verbose

	positional := fs.Args()
	if len(positional) == 0 {
		logger.Error("Usage: info <plugin-name>")
		os.Exit(1)
	}
	name := positional[0]

	client := NewClient(*baseURL, logger)
	var detail PluginDetail
	if err := client.getJSON("/"+name, &detail); err != nil {
		logger.Error("Failed to get plugin info: %v", err)
		os.Exit(1)
	}

	typeColor := func(t string) string {
		switch t {
		case "typescript":
			return P_CYAN
		case "lua":
			return P_GREEN
		case "external":
			return P_YELLOW
		case "go":
			return P_PURPLE
		default:
			return WHITE
		}
	}

	statusIcon := "✓"
	statusColor := P_GREEN
	if detail.Status != "loaded" {
		statusIcon = "✗"
		statusColor = P_RED
	}

	logger.Println("")
	logger.Printf("  %sPlugin:%s  %s%s%s\n", BOLD, RESET, B_WHITE, detail.Name, RESET)
	logger.Printf("  %sStatus:%s  %s%s %s%s%s\n", BOLD, RESET, statusColor, statusIcon, RESET, statusColor, detail.Status)
	logger.Printf("  %sType:%s    %s%s%s (%s)\n", BOLD, RESET, typeColor(detail.Type), detail.Type, RESET, detail.Entrypoint)
	logger.Printf("  %sVersion:%s %s\n", BOLD, RESET, detail.Version)
	logger.Printf("  %sAuthor:%s  %s\n", BOLD, RESET, detail.Author)
	logger.Printf("  %sDesc:%s    %s\n", BOLD, RESET, detail.Description)
	logger.Printf("  %sStats:%s   %d executions | last %.0fms | total %.0fms | load %.2fms\n",
		BOLD, RESET, detail.ExecuteCount, detail.LastExecutionMs, detail.TotalExecutionMs, detail.LoadTimeMs)
	logger.Printf("  %sSize:%s    %s\n", BOLD, RESET, formatBytes(detail.EmbeddedFileSize))
	logger.Println("")
}

func cmdExec(logger *Logger, args []string) {
	fs := flag.NewFlagSet("exec", flag.ExitOnError)
	baseURL := fs.String("url", BASE_URL, "Base URL of the plugin API")
	verbose := fs.Bool("verbose", false, "Verbose logging")
	mode := fs.String("mode", "", "Execution mode (plugin-specific)")
	raw := fs.Bool("raw", false, "Print raw JSON response")
	_ = fs.Parse(args)
	logger.verbose = *verbose

	positional := fs.Args()
	if len(positional) == 0 {
		logger.Error("Usage: exec [flags] <plugin-name> [key=val ...]")
		os.Exit(1)
	}

	name := positional[0]
	execArgs := make(map[string]interface{})
	if *mode != "" {
		execArgs["mode"] = *mode
	}
	for _, arg := range positional[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			execArgs[parts[0]] = parts[1]
		}
	}

	client := NewClient(*baseURL, logger)
	var result ExecuteResponse
	body := map[string]interface{}{"args": execArgs}
	if err := client.postJSON("/"+name+"/execute", body, &result); err != nil {
		logger.Error("Execution failed: %v", err)
		os.Exit(1)
	}

	if *raw {
		pretty, _ := json.MarshalIndent(result, "  ", "  ")
		fmt.Println(string(pretty))
		return
	}

	if result.Success {
		logger.Success("Plugin '%s' executed successfully", name)
		pretty, _ := json.MarshalIndent(result.Data, "  ", "  ")
		fmt.Println("")
		fmt.Println(string(pretty))
		fmt.Println("")
	} else {
		logger.Error("Plugin '%s' execution failed: %s", name, result.Error)
		os.Exit(1)
	}
}

func cmdUpload(logger *Logger, args []string) {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)
	baseURL := fs.String("url", BASE_URL, "Base URL of the plugin API")
	verbose := fs.Bool("verbose", false, "Verbose logging")
	file := fs.String("file", "", "Path to the script file to upload")
	content := fs.String("content", "", "Inline script content (alternative to -file)")
	scriptPath := fs.String("script-path", "", "Remote script path (e.g. scripts/handler.ts)")
	fs.Parse(args)
	logger.verbose = *verbose

	positional := fs.Args()
	if len(positional) == 0 {
		logger.Error("Usage: upload [flags] <plugin-name>")
		os.Exit(1)
	}
	name := positional[0]

	if *scriptPath == "" {
		*scriptPath = "scripts/handler.ts"
	}

	var scriptContent string
	if *content != "" {
		scriptContent = *content
	} else if *file != "" {
		data, err := os.ReadFile(*file)
		if err != nil {
			logger.Error("Failed to read file: %v", err)
			os.Exit(1)
		}
		scriptContent = string(data)
	} else {
		logger.Error("Either -content or -file must be provided")
		os.Exit(1)
	}

	client := NewClient(*baseURL, logger)
	body := map[string]string{"content": scriptContent}
	var result UploadResponse
	if err := client.putJSON("/"+name+"/scripts/"+*scriptPath, body, &result); err != nil {
		logger.Error("Upload failed: %v", err)
		os.Exit(1)
	}

	logger.Success("Script uploaded: %s", result.Path)
}

func cmdUnload(logger *Logger, args []string) {
	fs := flag.NewFlagSet("unload", flag.ExitOnError)
	baseURL := fs.String("url", BASE_URL, "Base URL of the plugin API")
	verbose := fs.Bool("verbose", false, "Verbose logging")
	fs.Parse(args)
	logger.verbose = *verbose

	positional := fs.Args()
	if len(positional) == 0 {
		logger.Error("Usage: unload <plugin-name>")
		os.Exit(1)
	}
	name := positional[0]

	client := NewClient(*baseURL, logger)
	var result DeleteResponse
	resp, err := client.doRequest("DELETE", "/"+name, nil)
	if err != nil {
		logger.Error("Failed to unload plugin: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Error("API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	_ = json.NewDecoder(resp.Body).Decode(&result)
	logger.Success("Plugin unloaded: %s", result.Name)
}

func cmdStatus(logger *Logger, args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	baseURL := fs.String("url", BASE_URL, "Base URL of the plugin API")
	verbose := fs.Bool("verbose", false, "Verbose logging")
	fs.Parse(args)
	logger.verbose = *verbose

	client := NewClient(*baseURL, logger)
	var status ManagerStatus
	if err := client.getJSON("/manager/status", &status); err != nil {
		logger.Error("Failed to get manager status: %v", err)
		os.Exit(1)
	}

	logger.Println("")
	logger.Printf("%sPlugin Manager Status%s\n", B_PURPLE, RESET)
	logger.Println("")

	logger.Printf("  %sTotal Plugins:%s     %d\n", BOLD, RESET, status.TotalPlugins)
	logger.Printf("  %sLoaded Plugins:%s    %d\n", BOLD, RESET, status.LoadedPlugins)
	logger.Printf("  %sTotal Executions:%s  %d\n", BOLD, RESET, status.TotalExecutions)
	logger.Printf("  %sActive Executions:%s %d\n", BOLD, RESET, status.ActiveExecutions)
	logger.Printf("  %sGoroutines:%s        %d\n", BOLD, RESET, status.GoroutineCount)
	logger.Printf("  %sMemory:%s            %s / %s (%.0f%%)\n", BOLD, RESET, formatBytes(status.MemoryUsageBytes), formatBytes(status.MemoryLimitBytes), status.MemoryPercent)
	logger.Printf("  %sUptime:%s            %.0fs\n", BOLD, RESET, status.UptimeSeconds)

	if len(status.Plugins) > 0 {
		logger.Println("")
		logger.Printf("%sPlugin Details (%d):%s\n", B_CYAN, len(status.Plugins), RESET)
		for _, p := range status.Plugins {
			statusIcon := "✓"
			if p.Status != "loaded" {
				statusIcon = "✗"
			}
			logger.Printf("  %s %s%-20s%s v%-8s %s(%s)%s %d exec",
				statusIcon, B_WHITE, p.Name, RESET, p.Version, GRAY, p.Type, RESET, p.ExecuteCount)
		}
	}
	logger.Println("")
}

func printUsage() {
	fmt.Printf(`%sUsage:%s go run scripts/plugin/pkg.go <command> [flags]

%sCommands:%s
  list      List all loaded plugins with status and manager metrics
  info      Show detailed information for a specific plugin
  exec      Execute a plugin with optional arguments
  upload    Upload or replace a plugin script at runtime
  unload    Unload a plugin from the registry
  status    Show plugin manager health metrics

%sFlags:%s
  -url string        Base URL of the plugin API (default "http://localhost:8080/api/v1/plugins")
  -verbose, -V       Enable verbose logging

%sExamples:%s
  go run scripts/plugin/pkg.go list
  go run scripts/plugin/pkg.go info inspector
  go run scripts/plugin/pkg.go exec inspector -mode ping
  go run scripts/plugin/pkg.go exec aggregator mode=dashboard
  go run scripts/plugin/pkg.go exec lua_demo name=world
  go run scripts/plugin/pkg.go upload inspector -file ./handler.ts
  go run scripts/plugin/pkg.go upload inspector -content "function handler() { ... }"
  go run scripts/plugin/pkg.go unload inspector
  go run scripts/plugin/pkg.go status

Run '%scommand -h%s' for subcommand-specific flags.
`, B_WHITE, RESET, B_WHITE, RESET, B_WHITE, RESET, B_WHITE, RESET, B_CYAN, RESET)
}

func main() {
	flag.Usage = func() {
		printUsage()
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	cmdArgs := os.Args[2:]

	switch cmd {
	case "-h", "--help", "help":
		printUsage()
		return
	}

	verbose := false
	for _, a := range os.Args[1:] {
		if a == "-verbose" || a == "--verbose" || a == "-V" {
			verbose = true
			break
		}
	}
	logger := NewLogger(verbose)
	printBanner()

	switch cmd {
	case "list":
		cmdList(logger, cmdArgs)
	case "info":
		cmdInfo(logger, cmdArgs)
	case "exec":
		cmdExec(logger, cmdArgs)
	case "upload":
		cmdUpload(logger, cmdArgs)
	case "unload":
		cmdUnload(logger, cmdArgs)
	case "status":
		cmdStatus(logger, cmdArgs)
	default:
		logger.Error("Unknown command: %s", cmd)
		fmt.Println("")
		printUsage()
		os.Exit(1)
	}
}
