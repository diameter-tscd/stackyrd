package infrastructure

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sync/singleflight"
	"gopkg.in/yaml.v3"

	"stackyrd/config"
	"stackyrd/internal/middleware"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/utils"

	"github.com/labstack/echo/v4"
)

const mcpProtocolVersion = "2026-07-28"

var supportedMCPVersions = []string{"2026-07-28", "2025-11-25", "2025-03-26"}

type InstanceIdentity struct {
	InstanceID string    `json:"instance_id"`
	PodName    string    `json:"pod_name"`
	PodIP      string    `json:"pod_ip"`
	Namespace  string    `json:"namespace"`
	NodeName   string    `json:"node_name"`
	Hostname   string    `json:"hostname"`
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"started_at"`
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func resolveIdentity(startedAt time.Time) InstanceIdentity {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	pid := os.Getpid()
	podName := envOrDefault("POD_NAME", hostname)
	podIP := envOrDefault("POD_IP", "")
	ns := envOrDefault("POD_NAMESPACE", "default")
	node := envOrDefault("NODE_NAME", "")
	instanceID := strings.TrimSpace(os.Getenv("INSTANCE_ID"))
	if instanceID == "" {
		if podName != hostname && podName != "" {
			instanceID = podName
		} else {
			b := make([]byte, 4)
			_, _ = rand.Read(b)
			instanceID = hex.EncodeToString(b) + "-" + hostname + "-" + strconv.Itoa(pid)
		}
	}
	return InstanceIdentity{
		InstanceID: instanceID,
		PodName:    podName,
		PodIP:      podIP,
		Namespace:  ns,
		NodeName:   node,
		Hostname:   hostname,
		PID:        pid,
		StartedAt:  startedAt,
	}
}

func (m *MCPServer) getIdentity() InstanceIdentity {
	if m != nil && m.identity.InstanceID != "" {
		return m.identity
	}
	mcpSingletonMu.RLock()
	if mcpSingleton != nil && mcpSingleton.identity.InstanceID != "" {
		id := mcpSingleton.identity
		mcpSingletonMu.RUnlock()
		return id
	}
	mcpSingletonMu.RUnlock()
	now := time.Now()
	if m != nil && !m.startTime.IsZero() {
		now = m.startTime
	}
	return resolveIdentity(now)
}

// resolveEffective returns the live MCP instance. The HTTP handler is bound
// to the singleton, but a few tool paths may receive a zero-value receiver;
// fall back to the global singleton in that case instead of duplicating the
// lookup in every tool.
func (m *MCPServer) resolveEffective() *MCPServer {
	if m != nil && !m.startTime.IsZero() {
		return m
	}
	mcpSingletonMu.RLock()
	s := mcpSingleton
	mcpSingletonMu.RUnlock()
	if s != nil {
		return s
	}
	return m
}

var memoryThresholds = []map[string]any{
	{"level": "low", "label": "Healthy", "min": 0, "max": 50, "color": "#22c55e", "bg": "rgba(34,197,94,0.15)"},
	{"level": "moderate", "label": "Moderate", "min": 50, "max": 75, "color": "#eab308", "bg": "rgba(234,179,8,0.15)"},
	{"level": "high", "label": "High", "min": 75, "max": 90, "color": "#f97316", "bg": "rgba(249,115,22,0.15)"},
	{"level": "critical", "label": "Critical", "min": 90, "max": 100, "color": "#ef4444", "bg": "rgba(239,68,68,0.15)"},
}

var (
	resourceDefsCache []map[string]any
	resourceDefsOnce  sync.Once
	sseBufPool        = sync.Pool{New: func() any { return new(bytes.Buffer) }}
	goroutineBufPool  = sync.Pool{New: func() any {
		b := make([]byte, 256<<10)
		return &b
	}}
)

func memoryStatus(pct float64) map[string]any {
	switch {
	case pct >= 90:
		return map[string]any{"level": "critical", "label": "Critical", "color": "#ef4444"}
	case pct >= 75:
		return map[string]any{"level": "high", "label": "High", "color": "#f97316"}
	case pct >= 50:
		return map[string]any{"level": "moderate", "label": "Moderate", "color": "#eab308"}
	default:
		return map[string]any{"level": "low", "label": "Healthy", "color": "#22c55e"}
	}
}

func buildMemoryDetails(ms runtime.MemStats) map[string]any {
	var vm *mem.VirtualMemoryStat
	if v, err := mem.VirtualMemory(); err == nil {
		vm = v
	}
	toMiB := func(b uint64) uint64 { return b / 1024 / 1024 }
	toMiB32 := func(b uint64) float64 { return float64(b) / 1024 / 1024 }
	var sysTotal, sysAvailable, sysUsed, sysFree, sysBuffers, sysCached uint64
	var usedPct float64
	if vm != nil {
		sysTotal = vm.Total
		sysAvailable = vm.Available
		sysUsed = vm.Used
		sysFree = vm.Free
		sysBuffers = vm.Buffers
		sysCached = vm.Cached
		usedPct = vm.UsedPercent
	}
	thresholds := memoryThresholds
	status := memoryStatus(usedPct)
	scale := []map[string]any{
		{"value": 0, "label": "0%"},
		{"value": 25, "label": "25%"},
		{"value": 50, "label": "50%"},
		{"value": 75, "label": "75%"},
		{"value": 90, "label": "90%"},
		{"value": 100, "label": "100%"},
	}
	return map[string]any{
		"system": map[string]any{
			"total_bytes":     sysTotal,
			"total_mib":       toMiB(sysTotal),
			"total_gib":       toMiB32(sysTotal) / 1024,
			"available_bytes": sysAvailable,
			"available_mib":   toMiB(sysAvailable),
			"used_bytes":      sysUsed,
			"used_mib":        toMiB(sysUsed),
			"free_bytes":      sysFree,
			"free_mib":        toMiB(sysFree),
			"buffers_mib":     toMiB(sysBuffers),
			"cached_mib":      toMiB(sysCached),
			"used_percent":    usedPct,
			"available_percent": func() float64 {
				if sysTotal == 0 {
					return 0
				}
				return float64(sysAvailable) / float64(sysTotal) * 100
			}(),
			"free_percent": func() float64 {
				if sysTotal == 0 {
					return 0
				}
				return float64(sysFree) / float64(sysTotal) * 100
			}(),
		},
		"app": map[string]any{
			"alloc_mib":       toMiB(ms.Alloc),
			"alloc_bytes":     ms.Alloc,
			"total_alloc_mib": toMiB(ms.TotalAlloc),
			"sys_mib":         toMiB(ms.Sys),
			"sys_bytes":       ms.Sys,
			"heap_alloc_mib":  toMiB(ms.HeapAlloc),
			"heap_sys_mib":    toMiB(ms.HeapSys),
			"heap_idle_mib":   toMiB(ms.HeapIdle),
			"heap_inuse_mib":  toMiB(ms.HeapInuse),
			"heap_released_mib": toMiB(ms.HeapReleased),
			"heap_objects":    ms.HeapObjects,
			"stack_inuse_mib": toMiB(ms.StackInuse),
			"stack_sys_mib":   toMiB(ms.StackSys),
			"gc_sys_mib":      toMiB(ms.GCSys),
			"gc_cpu_fraction": ms.GCCPUFraction,
			"num_gc":          ms.NumGC,
			"num_goroutine":   runtime.NumGoroutine(),
			"self_mib":        toMiB(ms.Sys),
		},
		"visualization": map[string]any{
			"thresholds": thresholds,
			"scale":      scale,
			"gauge": map[string]any{
				"percent":    usedPct,
				"normalized": usedPct / 100,
				"status":     status,
			},
			"status": status,
		},
		"used_percent": usedPct,
		"status":       status,
	}
}

type ServiceMeta struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	WireName  string   `json:"wire_name"`
	Endpoints []string `json:"endpoints"`
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type ipState struct {
	count        int
	windowStart  time.Time
	blockedUntil time.Time
}

type MCPServer struct {
	enabled              bool
	endpoint             string
	token                string
	allowedOrigins       []string
	logger               *logger.Logger
	rateLimitEnabled     bool
	rateLimitMax         int
	rateLimitWindow      time.Duration
	rateLimitCooldown    time.Duration
	rateLimitExclude     map[string]struct{}
	ipStates             map[string]*ipState
	ipMu                 sync.Mutex
	mu          sync.RWMutex
	initManager *InfraInitManager
	services    []ServiceMeta
	startTime   time.Time
	appName     string
	appVersion  string
	appEnv      string
	serverPort  string
	identity InstanceIdentity

	toolDefsCache []ToolDef

	dashboardCache   string
	dashboardExpiry  time.Time
	resourcesCache   string
	resourcesExpiry  time.Time
	memoryCache      string
	memoryExpiry     time.Time
	cachedHostname   string
	cachedCPUModel   string
	cachedHostExpiry time.Time
	cachedCPUExpiry  time.Time

	configCache  string
	configExpiry time.Time

	sfGroup          singleflight.Group
	goroutineCache   string
	goroutineExpiry  time.Time
	goroutineKey     string

	execEnabled   bool
	execTimeout   time.Duration
	execMaxOutput int
	execAllowed   map[string]struct{}
	buildInfo     map[string]any

	fmEnabled        bool
	fmRoot           string
	fmMaxUpload      int64
	fmThumbMaxBytes  int64
	fmThumbSize      int
}

var (
	mcpSingleton   *MCPServer
	mcpSingletonMu sync.RWMutex
)

func (m *MCPServer) Name() string { return "MCP" }
func (m *MCPServer) Close() error { return nil }
func (m *MCPServer) GetStatus() map[string]any {
	toolNames := make([]string, 0, len(m.toolDefs()))
	for _, t := range m.toolDefs() {
		toolNames = append(toolNames, t.Name)
	}
	st := m.startTime
	if st.IsZero() {
		st = time.Now()
	}
	uptime := time.Since(st).Round(time.Second)
	m.ipMu.Lock()
	now := time.Now()
	for ip, s := range m.ipStates {
		if !s.blockedUntil.IsZero() && now.After(s.blockedUntil) && time.Since(s.windowStart) > m.rateLimitWindow+m.rateLimitCooldown {
			delete(m.ipStates, ip)
			continue
		}
		if s.blockedUntil.IsZero() && time.Since(s.windowStart) > m.rateLimitWindow*2 && s.count == 0 {
			delete(m.ipStates, ip)
		}
	}
	tracked := len(m.ipStates)
	blocked := 0
	for _, s := range m.ipStates {
		if !s.blockedUntil.IsZero() && now.Before(s.blockedUntil) {
			blocked++
		}
	}
	m.ipMu.Unlock()
	excludeList := make([]string, 0, len(m.rateLimitExclude))
	for k := range m.rateLimitExclude {
		excludeList = append(excludeList, k)
	}
	slices.Sort(excludeList)
	allowedList := make([]string, 0, len(m.execAllowed))
	for k := range m.execAllowed {
		allowedList = append(allowedList, k)
	}
	slices.Sort(allowedList)
	id := m.getIdentity()
	return map[string]any{
		"enabled":                    m.enabled,
		"endpoint":                   m.endpoint,
		"connected":                  true,
		"protocol_version":           mcpProtocolVersion,
		"supported_versions":         supportedMCPVersions,
		"tools":                      toolNames,
		"tools_count":                len(toolNames),
		"allowed_origins":            m.allowedOrigins,
		"auth_required":              m.token != "",
		"token_set":                  m.token != "",
		"uptime":                     uptime.String(),
		"uptime_seconds":             int64(uptime.Seconds()),
		"started_at":                 st.Format(time.RFC3339),
		"rate_limit_enabled":         m.rateLimitEnabled,
		"rate_limit_ip":              m.rateLimitMax,
		"rate_limit_time":            int(m.rateLimitWindow.Seconds()),
		"rate_limit_cooldowntime":    int(m.rateLimitCooldown.Seconds()),
		"rate_limit_excludeip":       excludeList,
		"rate_limit_tracked_ips":     tracked,
		"rate_limit_blocked_ips":     blocked,
		"exec_enabled":               m.execEnabled,
		"exec_timeout_seconds":       int(m.execTimeout.Seconds()),
		"exec_max_output":            m.execMaxOutput,
		"exec_allowed_commands":      allowedList,
		"filemanager_enabled":        m.fmEnabled,
		"filemanager_root":           m.fmRoot,
		"filemanager_max_upload":     m.fmMaxUpload,
		"filemanager_thumbnail_max":  m.fmThumbMaxBytes,
		"filemanager_thumbnail_size": m.fmThumbSize,
		"build":                      m.buildInfo,
		"instance_id":                id.InstanceID,
		"instance":                   id,
	}
}

func (m *MCPServer) RouteHandlers() []RouteHandler {
	return []RouteHandler{{
		Path: m.endpoint,
		Mode: RouterDefault,
		Handler: func(g *echo.Group) {
			g.POST("", m.Handler())
			g.OPTIONS("", m.Handler())
			g.GET("", func(c echo.Context) error {
				if isSSE(c) {
					c.Response().Header().Set("Content-Type", "text/event-stream")
					c.Response().Header().Set("Cache-Control", "no-cache")
					c.Response().Header().Set("Connection", "keep-alive")
					c.Response().WriteHeader(http.StatusMethodNotAllowed)
					_, _ = c.Response().Write([]byte("event: error\ndata: {\"error\":\"GET not supported on MCP endpoint\"}\n\n"))
					return nil
				}
				return c.JSON(http.StatusMethodNotAllowed, jsonRPCResp{JSONRPC: "2.0", Error: &jsonRPCErr{Code: -32601, Message: "Method not allowed: GET not supported on MCP endpoint"}})
			})
			g.DELETE("", func(c echo.Context) error {
				return c.JSON(http.StatusMethodNotAllowed, jsonRPCResp{JSONRPC: "2.0", Error: &jsonRPCErr{Code: -32601, Message: "Method not allowed: DELETE not supported on MCP endpoint"}})
			})
		},
	}}
}

func init() {
	RegisterComponent("mcp", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.MCP.Enabled {
			return nil, nil
		}
		token := cfg.MCP.Token
		if token == "" {
			token = generateTempToken()
			if log != nil {
				log.Warn("MCP temporary token generated — set mcp.token in config.yaml for persistence", "endpoint", cfg.MCP.Endpoint)
			}
		}
		window := time.Duration(cfg.MCP.RateLimitTime) * time.Second
		if window == 0 {
			window = time.Duration(60) * time.Second
		}
		cooldown := time.Duration(cfg.MCP.RateLimitCooldownTime) * time.Second
		if cooldown == 0 {
			cooldown = time.Duration(300) * time.Second
		}
		max := cfg.MCP.RateLimitIP
		if max <= 0 {
			max = 100
		}
		exclude := make(map[string]struct{}, len(cfg.MCP.RateLimitExcludeIP))
		for _, ip := range cfg.MCP.RateLimitExcludeIP {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				exclude[ip] = struct{}{}
			}
		}
		if len(exclude) == 0 {
			exclude["127.0.0.1"] = struct{}{}
			exclude["::1"] = struct{}{}
			exclude["localhost"] = struct{}{}
		}
		execTimeout := time.Duration(cfg.MCP.ExecTimeout) * time.Second
		if execTimeout <= 0 {
			execTimeout = 10 * time.Second
		}
		if execTimeout > 60*time.Second {
			execTimeout = 60 * time.Second
		}
		execMax := cfg.MCP.ExecMaxOutput
		if execMax <= 0 {
			execMax = 65536
		}
		execAllowed := make(map[string]struct{}, len(cfg.MCP.ExecAllowedCommands))
		for _, c := range cfg.MCP.ExecAllowedCommands {
			c = strings.TrimSpace(c)
			if c != "" {
				execAllowed[c] = struct{}{}
			}
		}
		fmMax := cfg.MCP.FileManagerMaxUpload
		if fmMax <= 0 {
			fmMax = 209715200
		}
		if fmMax > 500<<20 {
			fmMax = 500 << 20
		}
		fmThumbMax := cfg.MCP.FileManagerThumbnailMaxBytes
		if fmThumbMax <= 0 {
			fmThumbMax = 10 << 20
		}
		fmThumbSize := cfg.MCP.FileManagerThumbnailSize
		if fmThumbSize <= 0 || fmThumbSize > 1024 {
			fmThumbSize = 256
		}
		fmRoot := strings.TrimSpace(cfg.MCP.FileManagerRoot)
		if fmRoot == "" {
			fmRoot = "."
		}
		if abs, err := filepath.Abs(fmRoot); err == nil {
			fmRoot = abs
		}
		_ = os.MkdirAll(fmRoot, 0750)
		now := time.Now()
		srv := &MCPServer{
			enabled:            true,
			endpoint:           cfg.MCP.Endpoint,
			token:              token,
			allowedOrigins:     cfg.MCP.AllowedOrigins,
			logger:             log,
			rateLimitEnabled:   cfg.MCP.RateLimitEnabled,
			rateLimitMax:       max,
			rateLimitWindow:    window,
			rateLimitCooldown:  cooldown,
			rateLimitExclude:   exclude,
			ipStates:           make(map[string]*ipState),
			startTime:          now,
			appName:            cfg.App.Name,
			appVersion:         cfg.App.Version,
			appEnv:             cfg.App.Env,
			serverPort:         cfg.Server.Port,
			identity: resolveIdentity(now),
			execEnabled:   cfg.MCP.ExecEnabled,
			execTimeout:   execTimeout,
			execMaxOutput: execMax,
			execAllowed:   execAllowed,
			buildInfo:     resolveBuildInfo(cfg),
			fmEnabled:       cfg.MCP.FileManagerEnabled,
			fmRoot:          fmRoot,
			fmMaxUpload:     fmMax,
			fmThumbMaxBytes: fmThumbMax,
			fmThumbSize:     fmThumbSize,
		}
		mcpSingletonMu.Lock()
		mcpSingleton = srv
		mcpSingletonMu.Unlock()
		return srv, nil
	})
}

func generateTempToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate MCP token: %v", err))
	}
	return hex.EncodeToString(b)
}

func SetInitManager(m *InfraInitManager) {
	mcpSingletonMu.RLock()
	inst := mcpSingleton
	mcpSingletonMu.RUnlock()
	if inst != nil {
		inst.mu.Lock()
		inst.initManager = m
		inst.mu.Unlock()
		return
	}
	if comp, ok := GetGlobalRegistry().Get("mcp"); ok {
		if srv, ok := comp.(*MCPServer); ok {
			srv.mu.Lock()
			srv.initManager = m
			srv.mu.Unlock()
		}
	}
}

func SetServices(svcs []ServiceMeta) {
	mcpSingletonMu.RLock()
	inst := mcpSingleton
	mcpSingletonMu.RUnlock()
	if inst != nil {
		inst.mu.Lock()
		inst.services = svcs
		inst.mu.Unlock()
		return
	}
	if comp, ok := GetGlobalRegistry().Get("mcp"); ok {
		if srv, ok := comp.(*MCPServer); ok {
			srv.mu.Lock()
			srv.services = svcs
			srv.mu.Unlock()
		}
	}
}

type jsonRPCReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResp struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id"`
	Result  any         `json:"result,omitempty"`
	Error   *jsonRPCErr `json:"error,omitempty"`
}

type jsonRPCErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func isSSE(c echo.Context) bool {
	return strings.Contains(c.Request().Header.Get("Accept"), "text/event-stream")
}

func writeSSE(c echo.Context, payload any) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)
	buf := sseBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer sseBufPool.Put(buf)
	buf.WriteString("event: message\n")
	buf.WriteString("data: ")
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return err
	}
	buf.WriteString("\n")
	_, err := c.Response().Write(buf.Bytes())
	return err
}

func writeSSEBatch(c echo.Context, payloads []jsonRPCResp) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)
	for _, p := range payloads {
		buf := sseBufPool.Get().(*bytes.Buffer)
		buf.Reset()
		buf.WriteString("event: message\n")
		buf.WriteString("data: ")
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(p); err != nil {
			sseBufPool.Put(buf)
			return err
		}
		buf.WriteString("\n")
		_, err := c.Response().Write(buf.Bytes())
		sseBufPool.Put(buf)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *MCPServer) Handler() echo.HandlerFunc {
	return func(c echo.Context) error {
		origin := c.Request().Header.Get("Origin")
		reqHeaders := c.Request().Header.Get("Access-Control-Request-Headers")
		allowHeaders := "Content-Type, Accept, Authorization, MCP-Protocol-Version, X-MCP-Token, X-Requested-With"
		if reqHeaders != "" {
			allowHeaders = reqHeaders
		}
		c.Response().Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE, OPTIONS")
		c.Response().Header().Set("Access-Control-Allow-Headers", allowHeaders)
		c.Response().Header().Set("Access-Control-Max-Age", "86400")
		id := m.getIdentity()
		c.Response().Header().Set("X-MCP-Instance-ID", id.InstanceID)
		c.Response().Header().Set("X-MCP-Pod-Name", id.PodName)
		if origin != "" {
			if !m.isOriginAllowed(c) {
				if isSSE(c) {
					c.Response().Header().Set("Content-Type", "text/event-stream")
					_, _ = c.Response().Write([]byte("event: error\ndata: {\"code\":-32000,\"message\":\"Forbidden: Origin not allowed\"}\n\n"))
					return c.String(http.StatusForbidden, "")
				}
				return c.JSON(http.StatusForbidden, jsonRPCResp{
					JSONRPC: "2.0",
					Error:   &jsonRPCErr{Code: -32000, Message: "Forbidden: Origin not allowed"},
				})
			}
			c.Response().Header().Set("Access-Control-Allow-Origin", origin)
			c.Response().Header().Set("Vary", "Origin")
		} else {
			c.Response().Header().Set("Access-Control-Allow-Origin", "*")
		}
		if c.Request().Method == http.MethodOptions {
			return c.NoContent(http.StatusNoContent)
		}
		if c.Request().Method != http.MethodPost {
			if isSSE(c) {
				c.Response().Header().Set("Content-Type", "text/event-stream")
				c.Response().WriteHeader(http.StatusMethodNotAllowed)
				_, _ = c.Response().Write([]byte("event: error\ndata: {\"code\":-32601,\"message\":\"Method not allowed\"}\n\n"))
				return nil
			}
			return c.JSON(http.StatusMethodNotAllowed, jsonRPCResp{
				JSONRPC: "2.0",
				Error:   &jsonRPCErr{Code: -32601, Message: "Method not allowed"},
			})
		}
		clientIP := c.RealIP()
		if ok, retry := m.allowIP(clientIP); !ok {
			c.Response().Header().Set("Retry-After", fmt.Sprintf("%.0f", retry.Seconds()))
			return c.JSON(http.StatusTooManyRequests, jsonRPCResp{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &jsonRPCErr{Code: -32003, Message: fmt.Sprintf("Rate limit exceeded, retry after %ds", int(retry.Seconds())), Data: map[string]any{"retry_after": int(retry.Seconds()), "cooldown": int(m.rateLimitCooldown.Seconds())}},
			})
		}
		if m.token != "" {
			if !m.authenticate(c) {
				return c.JSON(http.StatusUnauthorized, jsonRPCResp{
					JSONRPC: "2.0",
					Error:   &jsonRPCErr{Code: -32603, Message: "Unauthorized"},
				})
			}
		}
		body, err := io.ReadAll(io.LimitReader(c.Request().Body, 300<<20))
		if err != nil {
			return c.JSON(http.StatusBadRequest, jsonRPCResp{
				JSONRPC: "2.0",
				Error:   &jsonRPCErr{Code: -32700, Message: "Parse error"},
			})
		}
		trimmed := bytes.TrimSpace(body)
		if len(trimmed) == 0 {
			return c.JSON(http.StatusBadRequest, jsonRPCResp{
				JSONRPC: "2.0",
				Error:   &jsonRPCErr{Code: -32700, Message: "Parse error"},
			})
		}
		isBatch := len(trimmed) > 0 && trimmed[0] == '['
		sse := isSSE(c)
		if isBatch {
			var batch []jsonRPCReq
			if err := json.Unmarshal(trimmed, &batch); err != nil {
				return c.JSON(http.StatusBadRequest, jsonRPCResp{
					JSONRPC: "2.0",
					Error:   &jsonRPCErr{Code: -32700, Message: "Parse error"},
				})
			}
			if len(batch) == 0 {
				return c.JSON(http.StatusBadRequest, jsonRPCResp{
					JSONRPC: "2.0",
					Error:   &jsonRPCErr{Code: -32600, Message: "Invalid Request: empty batch"},
				})
			}
			headerVersion := c.Request().Header.Get("MCP-Protocol-Version")
			if headerVersion == "" {
				headerVersion = c.Request().Header.Get("Mcp-Protocol-Version")
			}
			if headerVersion != "" && !slices.Contains(supportedMCPVersions, headerVersion) {
				return c.JSON(http.StatusBadRequest, jsonRPCResp{
					JSONRPC: "2.0",
					Error: &jsonRPCErr{
						Code:    -32022,
						Message: "Unsupported protocol version",
						Data:    map[string]any{"supported": supportedMCPVersions, "requested": headerVersion},
					},
				})
			}
			selectedVersion := headerVersion
			if selectedVersion == "" {
				selectedVersion = "2025-03-26"
			}
			c.Response().Header().Set("MCP-Protocol-Version", selectedVersion)
			if sse {
				c.Response().Header().Set("MCP-Protocol-Version", selectedVersion)
			}
			responses := make([]jsonRPCResp, 0, len(batch))
			for i := range batch {
				req := &batch[i]
				if req.JSONRPC != "2.0" || req.Method == "" {
					responses = append(responses, jsonRPCResp{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCErr{Code: -32600, Message: "Invalid Request"}})
					continue
				}
				if req.ID == nil {
					_ = m.route(req)
					continue
				}
				resp := m.route(req)
				responses = append(responses, resp)
			}
			if len(responses) == 0 {
				return c.NoContent(http.StatusAccepted)
			}
			if sse {
				return writeSSEBatch(c, responses)
			}
			return c.JSON(http.StatusOK, responses)
		}
		var req jsonRPCReq
		if err := json.Unmarshal(trimmed, &req); err != nil {
			return c.JSON(http.StatusBadRequest, jsonRPCResp{
				JSONRPC: "2.0",
				Error:   &jsonRPCErr{Code: -32700, Message: "Parse error"},
			})
		}
		if req.ID == nil {
			_ = m.route(&req)
			return c.NoContent(http.StatusAccepted)
		}
		headerVersion := c.Request().Header.Get("MCP-Protocol-Version")
		if headerVersion == "" {
			headerVersion = c.Request().Header.Get("Mcp-Protocol-Version")
		}
		bodyVersion := extractProtocolVersion(req.Params)
		requestedVersion := bodyVersion
		if requestedVersion == "" {
			requestedVersion = headerVersion
		}
		if headerVersion != "" && bodyVersion != "" && headerVersion != bodyVersion {
			return c.JSON(http.StatusBadRequest, jsonRPCResp{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &jsonRPCErr{Code: -32020, Message: fmt.Sprintf("Header mismatch: MCP-Protocol-Version header %q does not match body _meta protocolVersion %q", headerVersion, bodyVersion)},
			})
		}
		if requestedVersion == "" {
			requestedVersion = "2025-03-26"
		}
		if !slices.Contains(supportedMCPVersions, requestedVersion) {
			return c.JSON(http.StatusBadRequest, jsonRPCResp{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &jsonRPCErr{
					Code:    -32022,
					Message: "Unsupported protocol version",
					Data:    map[string]any{"supported": supportedMCPVersions, "requested": requestedVersion},
				},
			})
		}
		c.Response().Header().Set("MCP-Protocol-Version", requestedVersion)
		resp := m.route(&req)
		if resp.Error != nil && resp.Error.Code == -32601 && isModernVersion(requestedVersion) {
			if sse {
				c.Response().Header().Set("MCP-Protocol-Version", requestedVersion)
				return writeSSE(c, resp)
			}
			return c.JSON(http.StatusNotFound, resp)
		}
		if sse {
			return writeSSE(c, resp)
		}
		return c.JSON(http.StatusOK, resp)
	}
}

func isModernVersion(v string) bool { return v == "2026-07-28" || v == "2025-11-25" }

func extractProtocolVersion(params json.RawMessage) string {
	if len(params) == 0 {
		return ""
	}
	var tmp struct {
		Meta map[string]any `json:"_meta"`
	}
	if err := json.Unmarshal(params, &tmp); err != nil || tmp.Meta == nil {
		return ""
	}
	if v, ok := tmp.Meta["io.modelcontextprotocol/protocolVersion"].(string); ok {
		return v
	}
	return ""
}

func (m *MCPServer) authenticate(c echo.Context) bool {
	if token := extractRequestToken(c); token != "" {
		return token == m.token
	}
	return false
}

func extractRequestToken(c echo.Context) string {
	if auth := c.Request().Header.Get("Authorization"); auth != "" {
		const prefix = "Bearer "
		if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
			return strings.TrimSpace(auth[len(prefix):])
		}
		if strings.TrimSpace(auth) != "" {
			return strings.TrimSpace(auth)
		}
	}
	if header := c.Request().Header.Get("X-MCP-Token"); header != "" {
		return strings.TrimSpace(header)
	}
	if header := c.Request().Header.Get("X-Api-Key"); header != "" {
		return strings.TrimSpace(header)
	}
	q := c.QueryParams()
	for _, key := range []string{"token", "mcp_token", "mcpToken", "access_token", "api_key", "apiKey"} {
		if v := strings.TrimSpace(q.Get(key)); v != "" {
			if strings.HasPrefix(strings.ToLower(v), "bearer ") {
				v = strings.TrimSpace(v[7:])
			}
			return v
		}
	}
	return ""
}

func GetMCPToken() string {
	mcpSingletonMu.RLock()
	if mcpSingleton != nil && mcpSingleton.token != "" {
		t := mcpSingleton.token
		mcpSingletonMu.RUnlock()
		return t
	}
	mcpSingletonMu.RUnlock()
	if cfg, err := config.LoadConfig(); err == nil && cfg.MCP.Token != "" {
		return cfg.MCP.Token
	}
	return ""
}

func getEffectiveMCPToken() string {
	mcpSingletonMu.RLock()
	if mcpSingleton != nil && mcpSingleton.token != "" {
		t := mcpSingleton.token
		mcpSingletonMu.RUnlock()
		return t
	}
	mcpSingletonMu.RUnlock()
	if cfg, err := config.LoadConfig(); err == nil && cfg.MCP.Token != "" {
		return cfg.MCP.Token
	}
	return ""
}

func IsMCPAuthenticated(c echo.Context) bool {
	token := getEffectiveMCPToken()
	if token == "" {
		return false
	}
	if provided := extractRequestToken(c); provided != "" {
		return provided == token
	}
	return false
}

func RequireMCPAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !IsMCPAuthenticated(c) {
			return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Unauthorized: valid MCP token required (Authorization: Bearer <token> or X-MCP-Token or ?token=<token>)"})
		}
		return next(c)
	}
}

func (m *MCPServer) isOriginAllowed(c echo.Context) bool {
	origin := c.Request().Header.Get("Origin")
	if len(m.allowedOrigins) == 0 {
		return true
	}
	for _, o := range m.allowedOrigins {
		if o == "*" || o == origin || isWildcardOrigin(o, origin) {
			return true
		}
	}
	return false
}

func isWildcardOrigin(pattern, origin string) bool {
	idx := strings.Index(pattern, "*.")
	if idx < 0 {
		return false
	}
	suffix := pattern[idx+1:]
	return strings.HasSuffix(origin, suffix)
}

func (m *MCPServer) isRateLimitExcluded(ip string) bool {
	if _, ok := m.rateLimitExclude[ip]; ok {
		return true
	}
	if ip == "127.0.0.1" || ip == "::1" {
		if _, ok := m.rateLimitExclude["localhost"]; ok {
			return true
		}
	}
	hostOnly := ip
	if idx := strings.Index(ip, ":"); idx >= 0 && strings.Count(ip, ":") == 1 {
		hostOnly = ip[:idx]
	}
	if _, ok := m.rateLimitExclude[hostOnly]; ok {
		return true
	}
	return false
}

func (m *MCPServer) allowIP(ip string) (bool, time.Duration) {
	if !m.rateLimitEnabled {
		return true, 0
	}
	if ip == "" {
		ip = "unknown"
	}
	if m.isRateLimitExcluded(ip) {
		return true, 0
	}
	if m.rateLimitWindow <= 0 || m.rateLimitMax <= 0 {
		return true, 0
	}
	now := time.Now()
	m.ipMu.Lock()
	defer m.ipMu.Unlock()
	if m.ipStates == nil {
		m.ipStates = make(map[string]*ipState)
	}
	st, ok := m.ipStates[ip]
	if !ok {
		m.ipStates[ip] = &ipState{windowStart: now, count: 1}
		return true, 0
	}
	if !st.blockedUntil.IsZero() && now.Before(st.blockedUntil) {
		return false, time.Until(st.blockedUntil)
	}
	if !st.blockedUntil.IsZero() && now.After(st.blockedUntil) {
		st.blockedUntil = time.Time{}
		st.windowStart = now
		st.count = 1
		return true, 0
	}
	if now.Sub(st.windowStart) > m.rateLimitWindow {
		st.windowStart = now
		st.count = 1
		return true, 0
	}
	st.count++
	if st.count > m.rateLimitMax {
		st.blockedUntil = now.Add(m.rateLimitCooldown)
		return false, m.rateLimitCooldown
	}
	return true, 0
}

func (m *MCPServer) route(req *jsonRPCReq) jsonRPCResp {
	resp := jsonRPCResp{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = m.handleInitialize()
	case "server/discover":
		resp.Result = m.handleDiscover()
	case "tools/list":
		resp.Result = m.handleToolsList()
	case "tools/call":
		resp.Result, resp.Error = m.handleToolsCall(req.Params)
	case "resources/list", "resources/templates/list":
		resp.Result = m.handleResourcesList()
	case "resources/read":
		resp.Result, resp.Error = m.handleResourcesRead(req.Params)
	case "prompts/list":
		resp.Result = map[string]any{"prompts": []any{}}
	case "prompts/get":
		resp.Error = &jsonRPCErr{Code: -32602, Message: "Prompt not found"}
	case "ping":
		resp.Result = map[string]any{}
	case "notifications/initialized", "notifications/cancelled", "notifications/progress":
	default:
		resp.Error = &jsonRPCErr{Code: -32601, Message: "Method not found: " + req.Method}
	}
	return resp
}

func (m *MCPServer) serverVersion() string {
	if v := m.appVersion; v != "" {
		return v
	}
	return "1.0"
}

func (m *MCPServer) handleInitialize() map[string]any {
	id := m.getIdentity()
	return map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{}, "resources": map[string]any{}, "prompts": map[string]any{}},
		"serverInfo":      map[string]any{"name": "stackyrd", "version": m.serverVersion(), "instanceId": id.InstanceID},
		"_meta":           map[string]any{"io.stackyrd/instance": id},
	}
}

func (m *MCPServer) handleDiscover() map[string]any {
	id := m.getIdentity()
	return map[string]any{
		"supportedVersions": supportedMCPVersions,
		"capabilities":      map[string]any{"tools": map[string]any{}, "resources": map[string]any{}, "prompts": map[string]any{}},
		"_meta": map[string]any{
			"io.modelcontextprotocol/serverInfo": map[string]any{"name": "stackyrd", "version": m.serverVersion(), "instanceId": id.InstanceID},
			"io.stackyrd/instance":               id,
		},
		"instructions": "stackyrd MCP server exposes health, services, infra, and endpoint introspection tools.",
	}
}

func (m *MCPServer) handleToolsList() map[string]any {
	return map[string]any{"tools": m.toolDefs()}
}

func (m *MCPServer) handleResourcesList() map[string]any {
	return map[string]any{"resources": m.resourceDefs()}
}

func (m *MCPServer) resourceDefs() []map[string]any {
	resourceDefsOnce.Do(func() {
		resourceDefsCache = []map[string]any{
			{"uri": "stackyrd://dashboard", "name": "Stackyrd Dashboard", "description": "Full TUI sidebar snapshot: app, resources, services, infra, middleware, endpoints and uptime", "mimeType": "application/json"},
			{"uri": "stackyrd://resources", "name": "System Resources", "description": "CPU, RAM, goroutines, host, PID and memory as shown in TUI sidebar Resources section", "mimeType": "application/json"},
			{"uri": "stackyrd://services", "name": "Services", "description": "Service states (running/failed/disabled)", "mimeType": "application/json"},
			{"uri": "stackyrd://infra", "name": "Infrastructure", "description": "Infrastructure components and connection status", "mimeType": "application/json"},
			{"uri": "stackyrd://middleware", "name": "Middleware", "description": "Middleware enabled/disabled states", "mimeType": "application/json"},
			{"uri": "stackyrd://endpoints", "name": "Endpoints", "description": "All registered service endpoints", "mimeType": "application/json"},
			{"uri": "stackyrd://app", "name": "App Info", "description": "App name, version, env, port, uptime and start time", "mimeType": "application/json"},
			{"uri": "stackyrd://identity", "name": "Instance Identity", "description": "Pod identity: instance_id, pod_name, pod_ip, namespace, node, hostname, pid", "mimeType": "application/json"},
			{"uri": "stackyrd://cluster", "name": "Cluster", "description": "Cluster members (phase 1: local instance only; phase 2: Redis-aggregated)", "mimeType": "application/json"},
			{"uri": "stackyrd://memory", "name": "Memory Details", "description": "Detailed memory usage with visualization thresholds and scale for frontend gauges", "mimeType": "application/json"},
			{"uri": "stackyrd://goroutines", "name": "Goroutine Dump", "description": "All goroutines with id, function, state and stack trace for leak detection and visualization", "mimeType": "application/json"},
			{"uri": "stackyrd://config", "name": "Configuration", "description": "Raw config.yaml from embed afero fs (alias config.yaml) with parsed view", "mimeType": "application/json"},
			{"uri": "stackyrd://cron", "name": "Cron Jobs", "description": "Scheduled cron jobs with schedule, last/next run and pool status", "mimeType": "application/json"},
			{"uri": "stackyrd://build", "name": "Build Info", "description": "Build metadata: version, Go version, OS/arch, VCS revision and build time", "mimeType": "application/json"},
			{"uri": "stackyrd://fs", "name": "Filesystem Root", "description": "Filesystem listing at filemanager root (same as filemanager action=list path=.)", "mimeType": "application/json"},
		}
	})
	return resourceDefsCache
}

func (m *MCPServer) handleResourcesRead(params json.RawMessage) (any, *jsonRPCErr) {
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.URI == "" {
		return nil, &jsonRPCErr{Code: -32602, Message: "Missing resource uri"}
	}
	var data string
	switch p.URI {
	case "stackyrd://dashboard":
		data = m.toolDashboard()
	case "stackyrd://resources":
		data = m.toolResources()
	case "stackyrd://services":
		data = m.toolServices()
	case "stackyrd://infra":
		data = m.toolInfra()
	case "stackyrd://middleware":
		data = m.toolMiddleware()
	case "stackyrd://endpoints":
		data = m.toolEndpoints()
	case "stackyrd://app":
		data = m.toolAppInfo()
	case "stackyrd://identity":
		data = m.toolIdentity()
	case "stackyrd://cluster":
		data = m.toolCluster()
	case "stackyrd://memory":
		data = m.toolMemory()
	case "stackyrd://goroutines":
		data = m.toolGoroutines()
	case "stackyrd://config":
		data = m.toolConfig()
	case "stackyrd://cron":
		data = m.toolCron()
	case "stackyrd://build":
		data = m.toolBuild()
	case "stackyrd://fs":
		data = m.toolFileManager(map[string]any{"action": "list", "path": "."})
	default:
		if strings.HasPrefix(p.URI, "stackyrd://fs/") {
			rel := strings.TrimPrefix(p.URI, "stackyrd://fs/")
			if rel == "" {
				rel = "."
			}
			data = m.toolFileManager(map[string]any{"action": "list", "path": rel})
		} else {
			return nil, &jsonRPCErr{Code: -32602, Message: "Resource not found: " + p.URI}
		}
	}
	return map[string]any{
		"contents": []map[string]any{{"uri": p.URI, "mimeType": "application/json", "text": data}},
	}, nil
}

func (m *MCPServer) toolDefs() []ToolDef {
	if m.toolDefsCache == nil {
		m.mu.Lock()
		if m.toolDefsCache == nil {
			m.toolDefsCache = m.buildToolDefs()
		}
		m.mu.Unlock()
	}
	return m.toolDefsCache
}

func (m *MCPServer) buildToolDefs() []ToolDef {
	return []ToolDef{
		{Name: "stackyrd_health", Description: "Get stackyrd infrastructure initialization status and overall progress.", InputSchema: emptySchema()},
		{Name: "stackyrd_services", Description: "List all registered services with their run state, wire name, and endpoints.", InputSchema: emptySchema()},
		{Name: "stackyrd_infra", Description: "List all infrastructure components and their connection status.", InputSchema: emptySchema()},
		{Name: "stackyrd_infra_detail", Description: "Get the full status map of one infrastructure component by name.", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Component name, e.g. redis"},
			},
			"required": []string{"name"},
		}},
		{Name: "stackyrd_endpoints", Description: "List all registered service endpoints.", InputSchema: emptySchema()},
		{Name: "stackyrd_uptime", Description: "Get server uptime, start time and uptime seconds.", InputSchema: emptySchema()},
		{Name: "stackyrd_resources", Description: "Get system resources (CPU, RAM, goroutines, host, PID, memory) as in TUI sidebar Resources.", InputSchema: emptySchema()},
		{Name: "stackyrd_middleware", Description: "List all middleware with enabled/disabled states.", InputSchema: emptySchema()},
		{Name: "stackyrd_dashboard", Description: "Get full dashboard snapshot (app, resources, services, infra, middleware, endpoints, uptime) mirroring TUI sidebar.", InputSchema: emptySchema()},
		{Name: "stackyrd_app", Description: "Get app info (name, version, env, port, uptime).", InputSchema: emptySchema()},
		{Name: "stackyrd_identity", Description: "Get this pod's instance identity (instance_id, pod_name, pod_ip, namespace, node, hostname, pid).", InputSchema: emptySchema()},
		{Name: "stackyrd_cluster", Description: "List cluster members (currently local instance only; future Redis-aggregated).", InputSchema: emptySchema()},
		{Name: "stackyrd_memory", Description: "Get detailed memory usage with visualization thresholds and scale (system, app heap, gauge).", InputSchema: emptySchema()},
		{Name: "stackyrd_goroutines", Description: "Dump all goroutines with id, function, state and full stack trace for leak detection and visualization.", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"filter": map[string]any{"type": "string", "description": "Optional case-insensitive substring filter on function or state (e.g. 'chan', 'net/http')."},
				"limit":  map[string]any{"type": "integer", "description": "Max goroutines to return (default 500, 0 = unlimited)."},
			},
		}},
		{Name: "stackyrd_config", Description: "Read config.yaml from embed afero fs (alias config.yaml) — raw YAML + parsed view.", InputSchema: emptySchema()},
		{Name: "stackyrd_cron", Description: "List all cron jobs with schedule, last/next run and pool status.", InputSchema: emptySchema()},
		{Name: "stackyrd_cron_trigger", Description: "Trigger a cron job immediately by name or id (async).", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Job name (e.g. 'cleanup') — case-insensitive substring match."},
				"id":   map[string]any{"type": "integer", "description": "Job ID (takes precedence over name if set)."},
			},
		}},
		{Name: "stackyrd_build", Description: "Get build info: version, Go version, OS/arch, VCS revision and build time.", InputSchema: emptySchema()},
			{Name: "stackyrd_exec", Description: "Execute a shell command via sh -c and return stdout/stderr/stdin (requires mcp.exec_enabled=true, token auth, allowlist + timeout).", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell expression (e.g. 'ls -la | grep foo', 'cat | grep bar', 'echo hello'). Executed via sh -c."},
				"stdin":   map[string]any{"type": "string", "description": "Optional stdin piped to command (e.g. 'hello\\nworld')."},
				"input":   map[string]any{"type": "string", "description": "Alias for stdin."},
				"args":    map[string]any{"type": "array", "description": "Optional shell args available as $0, $1... inside command (sh -c 'echo $0' sh hello)", "items": map[string]any{"type": "string"}},
				"timeout": map[string]any{"type": "integer", "description": "Timeout seconds (1-60, default from config mcp.exec_timeout)."},
				"workdir": map[string]any{"type": "string", "description": "Working directory (default '.')."},
			},
			"required": []string{"command"},
		}},
		{Name: "stackyrd_filemanager", Description: "Filesystem manager: list/read/write/mkdir/stat/delete/upload/download/thumbnail (max upload 200MB, thumbnail only for small images).", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":   map[string]any{"type": "string", "enum": []string{"list", "read", "write", "mkdir", "stat", "delete", "upload", "download", "thumbnail", "rename"}, "description": "Operation"},
				"path":     map[string]any{"type": "string", "description": "Relative path within filemanager root (e.g. '.', 'uploads/img.png', 'a/b/c.txt')"},
				"dest":     map[string]any{"type": "string", "description": "Destination path for rename/move"},
				"content":  map[string]any{"type": "string", "description": "Text content for write, or base64 for upload/download"},
				"encoding": map[string]any{"type": "string", "enum": []string{"text", "base64"}, "description": "Encoding for content (default text for read/write, base64 for upload/download)"},
				"mkdir_parents": map[string]any{"type": "boolean", "description": "Create parent directories for mkdir/write"},
				"recursive":     map[string]any{"type": "boolean", "description": "Recursive delete"},
				"size":          map[string]any{"type": "integer", "description": "Thumbnail size px (default from config, 32-1024)"},
			},
			"required": []string{"action", "path"},
		}},
		{Name: "stackyrd_system_control", Description: "Safe runtime control: gc (force Go GC) or clear_cache (invalidate MCP dashboard/resources/memory/config/goroutine caches). Protected by existing MCP token auth, no extra config.", InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"gc", "clear_cache"}, "description": "gc = runtime.GC + mem snapshot; clear_cache = drop MCP internal caches"},
			},
			"required": []string{"action"},
		}},
	}
}

func emptySchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (m *MCPServer) handleToolsCall(params json.RawMessage) (map[string]any, *jsonRPCErr) {
	var cp callParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &cp); err != nil {
			return toolResult(`{"error":"invalid tool call arguments"}`, true), nil
		}
	}
	var text string
	var isErr bool
	switch cp.Name {
	case "stackyrd_health":
		text = m.toolHealth()
	case "stackyrd_services":
		text = m.toolServices()
	case "stackyrd_infra":
		text = m.toolInfra()
	case "stackyrd_infra_detail":
		text = m.toolInfraDetail(argString(cp.Arguments, "name"))
	case "stackyrd_endpoints":
		text = m.toolEndpoints()
	case "stackyrd_uptime":
		text = m.toolUptime()
	case "stackyrd_resources":
		text = m.toolResources()
	case "stackyrd_middleware":
		text = m.toolMiddleware()
	case "stackyrd_dashboard":
		text = m.toolDashboard()
	case "stackyrd_app":
		text = m.toolAppInfo()
	case "stackyrd_identity":
		text = m.toolIdentity()
	case "stackyrd_cluster":
		text = m.toolCluster()
	case "stackyrd_memory":
		text = m.toolMemory()
	case "stackyrd_goroutines":
		filter := argString(cp.Arguments, "filter")
		limit := 500
		if v, ok := cp.Arguments["limit"]; ok {
			switch n := v.(type) {
			case float64:
				limit = int(n)
			case int:
				limit = n
			case int64:
				limit = int(n)
			}
			if limit < 0 {
				limit = 0
			}
			if limit > 2000 {
				limit = 2000
			}
		}
		text = m.toolGoroutinesFiltered(filter, limit)
	case "stackyrd_config":
		text = m.toolConfig()
	case "stackyrd_cron":
		text = m.toolCron()
	case "stackyrd_cron_trigger":
		text = m.toolCronTrigger(cp.Arguments)
	case "stackyrd_build":
		text = m.toolBuild()
	case "stackyrd_exec":
		text, isErr = m.toolExec(cp.Arguments)
	case "stackyrd_filemanager":
		text, isErr = m.toolFileManagerResult(cp.Arguments)
	case "stackyrd_system_control":
		text, isErr = m.toolSystemControl(cp.Arguments)
	default:
		text = fmt.Sprintf(`{"error":"unknown tool: %s"}`, cp.Name)
		isErr = true
	}
	return toolResult(text, isErr), nil
}

func toolResult(text string, isErr bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
}

func argString(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func (m *MCPServer) toolHealth() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	im := eff.initManager
	st := eff.startTime
	eff.mu.RUnlock()
	if im == nil {
		return `{"status":"unknown","reason":"infra init manager not ready"}`
	}
	if st.IsZero() {
		st = time.Now()
	}
	status := im.GetStatus()
	type comp struct {
		Name        string  `json:"name"`
		Initialized bool    `json:"initialized"`
		Progress    float64 `json:"progress"`
	}
	comps := make([]comp, 0, len(status))
	for name, st := range status {
		comps = append(comps, comp{Name: name, Initialized: st.Initialized, Progress: st.Progress})
	}
	slices.SortFunc(comps, func(a, b comp) int { return cmp.Compare(a.Name, b.Name) })
	uptime := time.Since(st).Round(time.Second)
	id := eff.getIdentity()
	return marshalJSON(map[string]any{
		"status":         map[bool]string{true: "ready", false: "initializing"}[im.IsReady()],
		"progress":       im.GetInitializationProgress(),
		"components":     comps,
		"uptime":         uptime.String(),
		"uptime_seconds": int64(uptime.Seconds()),
		"started_at":     st.Format(time.RFC3339),
		"instance_id":    id.InstanceID,
		"instance":       id,
	})
}

func (m *MCPServer) toolUptime() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	st := eff.startTime
	eff.mu.RUnlock()
	if st.IsZero() {
		st = time.Now()
	}
	uptime := time.Since(st).Round(time.Second)
	id := eff.getIdentity()
	return marshalJSON(map[string]any{
		"uptime":          uptime.String(),
		"uptime_seconds":  int64(uptime.Seconds()),
		"started_at":      st.Format(time.RFC3339),
		"started_at_unix": st.Unix(),
		"instance_id":     id.InstanceID,
		"instance":        id,
	})
}

func (m *MCPServer) toolAppInfo() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	st := eff.startTime
	name, ver, env, port := eff.appName, eff.appVersion, eff.appEnv, eff.serverPort
	eff.mu.RUnlock()
	if st.IsZero() {
		st = time.Now()
	}
	uptime := time.Since(st).Round(time.Second)
	id := eff.getIdentity()
	return marshalJSON(map[string]any{
		"name":           name,
		"version":        ver,
		"env":            env,
		"port":           port,
		"uptime":         uptime.String(),
		"uptime_seconds": int64(uptime.Seconds()),
		"started_at":     st.Format(time.RFC3339),
		"pid":            os.Getpid(),
		"instance_id":    id.InstanceID,
		"instance":       id,
	})
}

func (m *MCPServer) toolResources() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	if eff.resourcesCache != "" && time.Now().Before(eff.resourcesExpiry) {
		cached := eff.resourcesCache
		eff.mu.RUnlock()
		return cached
	}
	eff.mu.RUnlock()
	v, _, _ := eff.sfGroup.Do("toolResources", func() (any, error) {
		eff.mu.RLock()
		if eff.resourcesCache != "" && time.Now().Before(eff.resourcesExpiry) {
			cached := eff.resourcesCache
			eff.mu.RUnlock()
			return cached, nil
		}
		eff.mu.RUnlock()
		var cpuPct float64
		if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
			cpuPct = p[0]
		}
		var memPct float64
		var memUsed, memTotal uint64
		if v, err := mem.VirtualMemory(); err == nil {
			memPct = v.UsedPercent
			memUsed = v.Used / 1024 / 1024
			memTotal = v.Total / 1024 / 1024
		}
		hostname := eff.getCachedHostname()
		cpuModel := eff.getCachedCPUModel()
		id := eff.getIdentity()
		result := marshalJSON(map[string]any{
			"cpu_percent":  cpuPct,
			"mem_percent":  memPct,
			"mem_used_mib": memUsed,
			"mem_total_mib": memTotal,
			"cores":        runtime.NumCPU(),
			"goroutines":   runtime.NumGoroutine(),
			"app_mem_mib":  utils.GetMemSelf(),
			"hostname":     hostname,
			"cpu_model":    cpuModel,
			"pid":          os.Getpid(),
			"instance_id":  id.InstanceID,
			"instance":     id,
		})
		eff.mu.Lock()
		eff.resourcesCache = result
		eff.resourcesExpiry = time.Now().Add(2 * time.Second)
		eff.mu.Unlock()
		return result, nil
	})
	return v.(string)
}

func (m *MCPServer) toolMiddleware() string {
	reg := middleware.GetGlobalMiddlewareRegistry()
	names := reg.GetNames()
	slices.Sort(names)
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		out = append(out, map[string]any{"name": n, "enabled": reg.IsEnabled(n)})
	}
	return marshalJSON(out)
}

func (m *MCPServer) toolDashboard() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	if eff.dashboardCache != "" && time.Now().Before(eff.dashboardExpiry) {
		cached := eff.dashboardCache
		eff.mu.RUnlock()
		return cached
	}
	eff.mu.RUnlock()
	v, _, _ := eff.sfGroup.Do("toolDashboard", func() (any, error) {
		eff.mu.RLock()
		if eff.dashboardCache != "" && time.Now().Before(eff.dashboardExpiry) {
			cached := eff.dashboardCache
			eff.mu.RUnlock()
			return cached, nil
		}
		st := eff.startTime
		name, ver, env, port := eff.appName, eff.appVersion, eff.appEnv, eff.serverPort
		eff.mu.RUnlock()
		if st.IsZero() {
			st = time.Now()
		}
		uptime := time.Since(st).Round(time.Second)
		var cpuPct float64
		if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
			cpuPct = p[0]
		}
		var memPct float64
		var memUsed, memTotal uint64
		if v, err := mem.VirtualMemory(); err == nil {
			memPct = v.UsedPercent
			memUsed = v.Used / 1024 / 1024
			memTotal = v.Total / 1024 / 1024
		}
		hostname := eff.getCachedHostname()
		cpuModel := eff.getCachedCPUModel()
		reg := GetGlobalRegistry()
		all := reg.GetAll()
		infra := make([]map[string]any, 0, len(all))
		names := make([]string, 0, len(all))
		for n := range all {
			names = append(names, n)
		}
		slices.Sort(names)
		for _, n := range names {
			infra = append(infra, map[string]any{"name": n, "status": all[n].GetStatus()})
		}
		disabledInfra := []map[string]any{}
		for _, n := range reg.RegisteredNames() {
			if _, ok := all[n]; !ok {
				disabledInfra = append(disabledInfra, map[string]any{"name": n, "enabled": false, "status": "disabled"})
			}
		}
		mwReg := middleware.GetGlobalMiddlewareRegistry()
		mwNames := mwReg.GetNames()
		slices.Sort(mwNames)
		mwOut := make([]map[string]any, 0, len(mwNames))
		for _, n := range mwNames {
			mwOut = append(mwOut, map[string]any{"name": n, "enabled": mwReg.IsEnabled(n)})
		}
		eff.mu.RLock()
		svcMetas := eff.services
		eff.mu.RUnlock()
		if svcMetas == nil {
			svcMetas = []ServiceMeta{}
		}
		sortedMetas := make([]ServiceMeta, len(svcMetas))
		copy(sortedMetas, svcMetas)
		slices.SortFunc(sortedMetas, func(a, b ServiceMeta) int { return cmp.Compare(a.Name, b.Name) })
		svcs := make([]map[string]any, 0, len(sortedMetas))
		for _, s := range sortedMetas {
			svcs = append(svcs, map[string]any{"name": s.Name, "state": s.State, "wire_name": s.WireName, "endpoints": s.Endpoints})
		}
		var endpoints []string
		seen := map[string]bool{}
		for _, s := range svcMetas {
			for _, ep := range s.Endpoints {
				if !seen[ep] {
					seen[ep] = true
					endpoints = append(endpoints, ep)
				}
			}
		}
		slices.Sort(endpoints)
		if endpoints == nil {
			endpoints = []string{}
		}
		id := eff.getIdentity()
		payload := map[string]any{
			"app": map[string]any{
				"name":           name,
				"version":        ver,
				"env":            env,
				"port":           port,
				"uptime":         uptime.String(),
				"uptime_seconds": int64(uptime.Seconds()),
				"started_at":     st.Format(time.RFC3339),
				"pid":            os.Getpid(),
			},
			"instance_id": id.InstanceID,
			"instance":    id,
			"resources": map[string]any{
				"cpu_percent":   cpuPct,
				"mem_percent":   memPct,
				"mem_used_mib":  memUsed,
				"mem_total_mib": memTotal,
				"cores":         runtime.NumCPU(),
				"goroutines":    runtime.NumGoroutine(),
				"app_mem_mib":   utils.GetMemSelf(),
				"hostname":      hostname,
				"cpu_model":     cpuModel,
				"pid":           os.Getpid(),
			},
			"services":       svcs,
			"infra":          infra,
			"infra_disabled": disabledInfra,
			"middleware":     mwOut,
			"endpoints":      endpoints,
		}
		result := marshalJSON(payload)
		eff.mu.Lock()
		eff.dashboardCache = result
		eff.dashboardExpiry = time.Now().Add(2 * time.Second)
		eff.mu.Unlock()
		return result, nil
	})
	return v.(string)
}

func (m *MCPServer) toolServices() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	svcs := eff.services
	eff.mu.RUnlock()
	if svcs == nil {
		svcs = []ServiceMeta{}
	}
	return marshalJSON(svcs)
}

func (m *MCPServer) toolInfra() string {
	comps := GetGlobalRegistry().GetAll()
	names := make([]string, 0, len(comps))
	for n := range comps {
		names = append(names, n)
	}
	slices.Sort(names)
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		out = append(out, map[string]any{"name": n, "status": comps[n].GetStatus()})
	}
	return marshalJSON(out)
}

func (m *MCPServer) toolInfraDetail(name string) string {
	if name == "" {
		return `{"error":"param 'name' is required"}`
	}
	comps := GetGlobalRegistry().GetAll()
	comp, ok := comps[name]
	if !ok {
		return fmt.Sprintf(`{"error":"component not found: %s"}`, name)
	}
	b, err := json.Marshal(comp.GetStatus())
	if err != nil {
		return fmt.Sprintf(`{"error":"%v"}`, err)
	}
	return string(b)
}

func (m *MCPServer) toolEndpoints() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	svcs := eff.services
	eff.mu.RUnlock()
	seen := map[string]bool{}
	var eps []string
	for _, svc := range svcs {
		for _, ep := range svc.Endpoints {
			if !seen[ep] {
				seen[ep] = true
				eps = append(eps, ep)
			}
		}
	}
	slices.Sort(eps)
	return marshalJSON(eps)
}

func (m *MCPServer) toolIdentity() string {
	return marshalJSON(m.getIdentity())
}

func (m *MCPServer) toolCluster() string {
	id := m.getIdentity()
	return marshalJSON(map[string]any{
		"members": []InstanceIdentity{id},
		"count":   1,
		"self":    id,
	})
}

func (m *MCPServer) toolMemory() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	if eff.memoryCache != "" && time.Now().Before(eff.memoryExpiry) {
		cached := eff.memoryCache
		eff.mu.RUnlock()
		return cached
	}
	eff.mu.RUnlock()
	v, _, _ := eff.sfGroup.Do("toolMemory", func() (any, error) {
		eff.mu.RLock()
		if eff.memoryCache != "" && time.Now().Before(eff.memoryExpiry) {
			cached := eff.memoryCache
			eff.mu.RUnlock()
			return cached, nil
		}
		eff.mu.RUnlock()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		result := marshalJSON(buildMemoryDetails(ms))
		eff.mu.Lock()
		eff.memoryCache = result
		eff.memoryExpiry = time.Now().Add(2 * time.Second)
		eff.mu.Unlock()
		return result, nil
	})
	return v.(string)
}

func (m *MCPServer) toolConfig() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	if eff.configCache != "" && time.Now().Before(eff.configExpiry) {
		cached := eff.configCache
		eff.mu.RUnlock()
		return cached
	}
	eff.mu.RUnlock()
	data, err := Read("config.yaml")
	if err != nil {
		if data2, err2 := Read("config"); err2 == nil {
			data = data2
			err = nil
		}
	}
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "source": "afero:config.yaml"})
	}
	var parsed any
	if err := yaml.Unmarshal(data, &parsed); err == nil {
		result := marshalJSON(map[string]any{"source": "afero:config.yaml", "raw": string(data), "parsed": parsed})
		eff.mu.Lock()
		eff.configCache = result
		eff.configExpiry = time.Now().Add(5 * time.Second)
		eff.mu.Unlock()
		return result
	}
	result := marshalJSON(map[string]any{"source": "afero:config.yaml", "raw": string(data)})
	eff.mu.Lock()
	eff.configCache = result
	eff.configExpiry = time.Now().Add(5 * time.Second)
	eff.mu.Unlock()
	return result
}

func resolveBuildInfo(cfg *config.Config) map[string]any {
	info := map[string]any{
		"name":       cfg.App.Name,
		"version":    cfg.App.Version,
		"env":        cfg.App.Env,
		"go_version": runtime.Version(),
		"goos":       runtime.GOOS,
		"goarch":     runtime.GOARCH,
		"compiler":   runtime.Compiler,
		"num_cpu":    runtime.NumCPU(),
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		info["module"] = bi.Main.Path
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info["module_version"] = bi.Main.Version
		}
		settings := map[string]string{}
		for _, s := range bi.Settings {
			settings[s.Key] = s.Value
		}
		if v, ok := settings["vcs.revision"]; ok && v != "" {
			info["vcs_revision"] = v
		}
		if v, ok := settings["vcs.time"]; ok && v != "" {
			info["vcs_time"] = v
		}
		if v, ok := settings["vcs.modified"]; ok {
			info["vcs_modified"] = v == "true"
		}
	}
	return info
}

func (m *MCPServer) toolCron() string {
	comp, ok := GetGlobalRegistry().Get("cron")
	if !ok {
		return marshalJSON(map[string]any{"enabled": false, "jobs": []any{}, "count": 0, "reason": "cron not registered"})
	}
	cm, ok := comp.(*CronManager)
	if !ok {
		return marshalJSON(map[string]any{"enabled": false, "error": "invalid cron manager type"})
	}
	jobs := cm.GetJobs()
	pool := cm.GetPoolStatus()
	return marshalJSON(map[string]any{"enabled": true, "count": len(jobs), "jobs": jobs, "pool": pool})
}

func (m *MCPServer) toolCronTrigger(args map[string]any) string {
	comp, ok := GetGlobalRegistry().Get("cron")
	if !ok {
		return `{"error":"cron not registered or disabled"}`
	}
	cm, ok := comp.(*CronManager)
	if !ok {
		return `{"error":"invalid cron manager type"}`
	}
	if v, ok := args["id"]; ok {
		var id int
		switch n := v.(type) {
		case float64:
			id = int(n)
		case int:
			id = n
		case int64:
			id = int(n)
		}
		if id != 0 {
			if err := cm.RunJobNow(id); err != nil {
				return marshalJSON(map[string]any{"error": err.Error(), "id": id})
			}
			return marshalJSON(map[string]any{"triggered": true, "id": id, "mode": "id"})
		}
	}
	name := strings.TrimSpace(argString(args, "name"))
	if name == "" {
		return `{"error":"param 'name' or 'id' is required"}`
	}
	jobs := cm.GetJobs()
	lower := strings.ToLower(name)
	var matched *CronJob
	for i := range jobs {
		if strings.EqualFold(jobs[i].Name, name) {
			matched = &jobs[i]
			break
		}
	}
	if matched == nil {
		for i := range jobs {
			if strings.Contains(strings.ToLower(jobs[i].Name), lower) {
				matched = &jobs[i]
				break
			}
		}
	}
	if matched == nil {
		return marshalJSON(map[string]any{"error": "job not found: " + name, "available": jobs})
	}
	if err := cm.RunJobNow(matched.ID); err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "name": matched.Name, "id": matched.ID})
	}
	return marshalJSON(map[string]any{"triggered": true, "name": matched.Name, "id": matched.ID, "mode": "name"})
}

func (m *MCPServer) toolBuild() string {
	eff := m.resolveEffective()
	eff.mu.RLock()
	bi := eff.buildInfo
	st := eff.startTime
	appName, appVer, appEnv, port := eff.appName, eff.appVersion, eff.appEnv, eff.serverPort
	eff.mu.RUnlock()
	if bi == nil {
		bi = map[string]any{"version": appVer, "go_version": runtime.Version()}
	}
	out := make(map[string]any, len(bi)+5)
	for k, v := range bi {
		out[k] = v
	}
	out["app_name"] = appName
	out["app_version"] = appVer
	out["app_env"] = appEnv
	out["port"] = port
	if !st.IsZero() {
		out["started_at"] = st.Format(time.RFC3339)
		out["uptime"] = time.Since(st).Round(time.Second).String()
		out["uptime_seconds"] = int64(time.Since(st).Seconds())
	}
	out["pid"] = os.Getpid()
	out["instance"] = eff.getIdentity()
	return marshalJSON(out)
}

func (m *MCPServer) toolExec(args map[string]any) (string, bool) {
	eff := m.resolveEffective()
	eff.mu.RLock()
	enabled := eff.execEnabled
	timeout := eff.execTimeout
	maxOut := eff.execMaxOutput
	allowed := eff.execAllowed
	eff.mu.RUnlock()
	if !enabled {
		return `{"error":"exec disabled — set mcp.exec_enabled=true in config.yaml"}`, true
	}
	cmdStr := strings.TrimSpace(argString(args, "command"))
	if cmdStr == "" {
		return `{"error":"param 'command' is required"}`, true
	}
	if len(allowed) > 0 {
		first := cmdStr
		if idx := strings.IndexFunc(first, func(r rune) bool { return r == ' ' || r == ';' || r == '|' || r == '&' }); idx >= 0 {
			first = strings.TrimSpace(first[:idx])
		}
		if idx := strings.LastIndex(first, "/"); idx >= 0 {
			first = first[idx+1:]
		}
		if _, ok := allowed[first]; !ok {
			if _, ok := allowed[cmdStr]; !ok {
				return marshalJSON(map[string]any{"error": "command not allowed: " + first, "allowed": allowed, "hint": "allowed checks first token of shell expression"}), true
			}
		}
	}
	stdinStr := argString(args, "stdin")
	if stdinStr == "" {
		stdinStr = argString(args, "input")
	}
	var shellArgs []string
	if v, ok := args["args"]; ok {
		switch arr := v.(type) {
		case []any:
			for _, e := range arr {
				if s, ok := e.(string); ok {
					shellArgs = append(shellArgs, s)
				}
			}
		case []string:
			shellArgs = arr
		}
	}
	if v, ok := args["timeout"]; ok {
		var t int
		switch n := v.(type) {
		case float64:
			t = int(n)
		case int:
			t = n
		case int64:
			t = int(n)
		}
		if t > 0 && t <= 60 {
			timeout = time.Duration(t) * time.Second
		}
	}
	workdir := strings.TrimSpace(argString(args, "workdir"))
	if workdir == "" {
		workdir = "."
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		allArgs := append([]string{"/C", cmdStr}, shellArgs...)
		cmd = exec.CommandContext(ctx, "cmd", allArgs...)
	} else {
		allArgs := append([]string{"-c", cmdStr}, shellArgs...)
		cmd = exec.CommandContext(ctx, "sh", allArgs...)
	}
	cmd.Dir = workdir
	if stdinStr != "" {
		cmd.Stdin = strings.NewReader(stdinStr)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start).Round(time.Millisecond)
	outStr := stdout.String()
	errStr := stderr.String()
	limited := false
	totalLen := stdout.Len() + stderr.Len()
	if maxOut > 0 && totalLen > maxOut {
		combined := outStr + errStr
		if len(combined) > maxOut {
			combined = combined[:maxOut]
		}
		outStr = combined
		errStr = ""
		limited = true
	}
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = 124
		} else {
			exitCode = -1
		}
	}
	result := map[string]any{
		"command":   cmdStr,
		"args":      shellArgs,
		"stdin":     stdinStr,
		"workdir":   workdir,
		"shell":     true,
		"exit_code": exitCode,
		"stdout":    outStr,
		"stderr":    errStr,
		"duration":  elapsed.String(),
		"duration_ms": elapsed.Milliseconds(),
		"truncated": limited,
		"timeout":   timeout.String(),
	}
	if err != nil && errStr == "" && outStr == "" {
		result["error"] = err.Error()
	}
	if limited {
		result["max_output"] = maxOut
	}
	return marshalJSON(result), exitCode != 0
}

func (m *MCPServer) fmResolve(rel string) (string, error) {
	eff := m.resolveEffective()
	eff.mu.RLock()
	root := eff.fmRoot
	eff.mu.RUnlock()
	if root == "" {
		root = "."
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	rel = filepath.Clean(rel)
	if filepath.IsAbs(rel) {
		rel = strings.TrimPrefix(rel, "/")
	}
	joined := filepath.Join(root, rel)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	if absJoined != root && !strings.HasPrefix(absJoined, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes filemanager root: %s", rel)
	}
	return absJoined, nil
}

func (m *MCPServer) toolFileManagerResult(args map[string]any) (string, bool) {
	eff := m.resolveEffective()
	eff.mu.RLock()
	enabled := eff.fmEnabled
	eff.mu.RUnlock()
	if !enabled {
		return `{"error":"filemanager disabled — set mcp.filemanager_enabled=true"}`, true
	}
	s := m.toolFileManager(args)
	isErr := strings.Contains(s, `"error"`) && (strings.Contains(s, `"code"`) || true)
	if strings.Contains(s, `"error":`) {
		isErr = true
		if strings.Contains(s, `"entries"`) || strings.Contains(s, `"content"`) || strings.Contains(s, `"thumbnail"`) {
			isErr = false
			if strings.Contains(s, `"error":`) && !strings.Contains(s, `"entries"`) {
				isErr = strings.Contains(s, `"error":`) && !strings.Contains(s, `"path":`)
			}
		}
	}
	if strings.Contains(s, `"error":`) {
		isErr = true
	} else {
		isErr = false
	}
	return s, isErr
}

func (m *MCPServer) toolFileManager(args map[string]any) string {
	action := strings.ToLower(strings.TrimSpace(argString(args, "action")))
	if action == "" {
		action = strings.ToLower(strings.TrimSpace(argString(args, "op")))
	}
	rel := strings.TrimSpace(argString(args, "path"))
	if rel == "" {
		rel = "."
	}
	eff := m.resolveEffective()
	eff.mu.RLock()
	maxUpload := eff.fmMaxUpload
	thumbMax := eff.fmThumbMaxBytes
	thumbSize := eff.fmThumbSize
	eff.mu.RUnlock()
	if maxUpload <= 0 {
		maxUpload = 209715200
	}
	if thumbMax <= 0 {
		thumbMax = 10 << 20
	}
	if thumbSize <= 0 {
		thumbSize = 256
	}
	if v, ok := args["size"]; ok {
		switch n := v.(type) {
		case float64:
			if int(n) >= 32 && int(n) <= 1024 {
				thumbSize = int(n)
			}
		case int:
			if n >= 32 && n <= 1024 {
				thumbSize = n
			}
		}
	}
	switch action {
	case "list", "ls", "dir":
		return m.fmList(rel)
	case "read", "cat":
		enc := strings.ToLower(strings.TrimSpace(argString(args, "encoding")))
		return m.fmRead(rel, enc)
	case "write", "create", "save":
		content := argString(args, "content")
		enc := strings.ToLower(strings.TrimSpace(argString(args, "encoding")))
		mkdirParents := false
		if v, ok := args["mkdir_parents"]; ok {
			if b, ok := v.(bool); ok {
				mkdirParents = b
			}
		}
		return m.fmWrite(rel, content, enc, mkdirParents, maxUpload)
	case "mkdir", "mkdirs":
		parents := true
		if v, ok := args["mkdir_parents"]; ok {
			if b, ok := v.(bool); ok {
				parents = b
			}
		}
		return m.fmMkdir(rel, parents)
	case "stat", "attributes", "info":
		return m.fmStat(rel)
	case "delete", "rm", "remove":
		recursive := false
		if v, ok := args["recursive"]; ok {
			if b, ok := v.(bool); ok {
				recursive = b
			}
		}
		return m.fmDelete(rel, recursive)
	case "upload":
		content := argString(args, "content")
		return m.fmUpload(rel, content, maxUpload)
	case "download":
		return m.fmDownload(rel, maxUpload)
	case "thumbnail", "thumb":
		return m.fmThumbnail(rel, thumbMax, thumbSize)
	case "rename", "move", "mv":
		dest := strings.TrimSpace(argString(args, "dest"))
		if dest == "" {
			dest = strings.TrimSpace(argString(args, "destination"))
		}
		if dest == "" {
			return `{"error":"param 'dest' is required for rename"}` 
		}
		return m.fmRename(rel, dest)
	default:
		return marshalJSON(map[string]any{"error": "unknown filemanager action: " + action, "allowed": []string{"list", "read", "write", "mkdir", "stat", "delete", "upload", "download", "thumbnail", "rename"}})
	}
}

func (m *MCPServer) fmList(rel string) string {
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	info, err := os.Stat(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	if !info.IsDir() {
		return m.fmStat(rel)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		fi, _ := e.Info()
		item := map[string]any{
			"name":     e.Name(),
			"path":     filepath.Join(rel, e.Name()),
			"is_dir":   e.IsDir(),
			"mode":     "",
			"size":     int64(0),
			"mod_time": "",
		}
		if fi != nil {
			item["size"] = fi.Size()
			item["mode"] = fi.Mode().String()
			item["mod_time"] = fi.ModTime().Format(time.RFC3339)
			item["is_dir"] = fi.IsDir()
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != "" {
			item["ext"] = ext
		}
		if !e.IsDir() && fi != nil && fi.Size() < 10<<20 {
			if _, err := utils.DetectImageFormat(abs + string(os.PathSeparator) + e.Name()); err == nil {
				item["is_image"] = true
			}
		}
		out = append(out, item)
	}
	root, _ := m.fmResolve(".")
	return marshalJSON(map[string]any{"path": rel, "abs": abs, "root": root, "count": len(out), "entries": out})
}

func (m *MCPServer) fmRead(rel, enc string) string {
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	if fi.IsDir() {
		return m.fmList(rel)
	}
	if fi.Size() > 10<<20 && enc != "base64" {
		return marshalJSON(map[string]any{"error": "file too large for text read (use base64/download, max 10MB for text)", "path": rel, "size": fi.Size()})
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	if enc == "base64" {
		return marshalJSON(map[string]any{"path": rel, "size": fi.Size(), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data), "mime": detectMime(abs, data)})
	}
	if !isText(data) {
		return marshalJSON(map[string]any{"path": rel, "size": fi.Size(), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data), "mime": detectMime(abs, data), "note": "binary detected, returned as base64"})
	}
	return marshalJSON(map[string]any{"path": rel, "size": fi.Size(), "encoding": "text", "content": string(data), "mime": detectMime(abs, data)})
}

func (m *MCPServer) fmWrite(rel, content, enc string, mkdirParents bool, maxUpload int64) string {
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	var data []byte
	if enc == "base64" {
		if content == "" {
			data = []byte{}
		} else {
			d, err := base64.StdEncoding.DecodeString(content)
			if err != nil {
				if d2, err2 := base64.URLEncoding.DecodeString(content); err2 == nil {
					d = d2
				} else {
					return marshalJSON(map[string]any{"error": "invalid base64: " + err.Error(), "path": rel})
				}
			}
			data = d
		}
	} else {
		data = []byte(content)
	}
	if int64(len(data)) > maxUpload {
		return marshalJSON(map[string]any{"error": fmt.Sprintf("content exceeds max upload %d bytes", maxUpload), "path": rel, "size": len(data)})
	}
	dir := filepath.Dir(abs)
	if mkdirParents {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
		}
	} else {
		if _, err := os.Stat(dir); err != nil {
			return marshalJSON(map[string]any{"error": "parent dir does not exist (use mkdir_parents=true)", "path": rel, "dir": dir})
		}
	}
	if err := os.WriteFile(abs, data, 0640); err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	return marshalJSON(map[string]any{"path": rel, "abs": abs, "size": len(data), "written": true})
}

func (m *MCPServer) fmMkdir(rel string, parents bool) string {
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	var mkErr error
	if parents {
		mkErr = os.MkdirAll(abs, 0750)
	} else {
		mkErr = os.Mkdir(abs, 0750)
	}
	if mkErr != nil {
		return marshalJSON(map[string]any{"error": mkErr.Error(), "path": rel})
	}
	return marshalJSON(map[string]any{"path": rel, "abs": abs, "created": true})
}

func (m *MCPServer) fmStat(rel string) string {
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	out := map[string]any{
		"path":     rel,
		"abs":      abs,
		"name":     fi.Name(),
		"is_dir":   fi.IsDir(),
		"size":     fi.Size(),
		"mode":     fi.Mode().String(),
		"perm":     fmt.Sprintf("%04o", fi.Mode().Perm()),
		"mod_time": fi.ModTime().Format(time.RFC3339),
	}
	if !fi.IsDir() {
		ext := strings.ToLower(filepath.Ext(abs))
		out["ext"] = ext
		out["mime"] = mimeByExt(ext)
		if data, err := os.ReadFile(abs); err == nil && len(data) > 0 {
			out["mime_detected"] = detectMime(abs, data)
			if w, h, err := imageDimensionsFromBytes(data); err == nil {
				out["width"] = w
				out["height"] = h
				out["is_image"] = true
			}
		}
		if fi.Size() < 10<<20 {
			if _, err := utils.DetectImageFormat(abs); err == nil {
				out["is_image"] = true
			}
		}
	}
	return marshalJSON(out)
}

func (m *MCPServer) fmDelete(rel string, recursive bool) string {
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	eff := m.resolveEffective()
	eff.mu.RLock()
	root := eff.fmRoot
	eff.mu.RUnlock()
	if abs == root {
		return marshalJSON(map[string]any{"error": "refusing to delete filemanager root", "path": rel})
	}
	if fi.IsDir() && !recursive {
		entries, _ := os.ReadDir(abs)
		if len(entries) > 0 {
			return marshalJSON(map[string]any{"error": "directory not empty (use recursive=true)", "path": rel, "count": len(entries)})
		}
	}
	var delErr error
	if fi.IsDir() && recursive {
		delErr = os.RemoveAll(abs)
	} else {
		delErr = os.Remove(abs)
	}
	if delErr != nil {
		return marshalJSON(map[string]any{"error": delErr.Error(), "path": rel})
	}
	return marshalJSON(map[string]any{"path": rel, "deleted": true})
}

func (m *MCPServer) fmUpload(rel, b64 string, maxUpload int64) string {
	if b64 == "" {
		return marshalJSON(map[string]any{"error": "param 'content' is required (base64)", "path": rel})
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		if d2, err2 := base64.URLEncoding.DecodeString(b64); err2 == nil {
			data = d2
			err = nil
		} else {
			return marshalJSON(map[string]any{"error": "invalid base64: " + err.Error(), "path": rel})
		}
	}
	if int64(len(data)) > maxUpload {
		return marshalJSON(map[string]any{"error": fmt.Sprintf("decoded size %d exceeds max upload %d (200MB)", len(data), maxUpload), "path": rel})
	}
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	if err := os.WriteFile(abs, data, 0640); err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	return marshalJSON(map[string]any{"path": rel, "abs": abs, "size": len(data), "uploaded": true})
}

func (m *MCPServer) fmDownload(rel string, maxUpload int64) string {
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	if fi.IsDir() {
		return marshalJSON(map[string]any{"error": "path is directory, use list", "path": rel})
	}
	if fi.Size() > maxUpload {
		return marshalJSON(map[string]any{"error": fmt.Sprintf("file size %d exceeds max upload %d", fi.Size(), maxUpload), "path": rel})
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	return marshalJSON(map[string]any{"path": rel, "size": fi.Size(), "mime": detectMime(abs, data), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data)})
}

func (m *MCPServer) fmThumbnail(rel string, maxBytes int64, size int) string {
	abs, err := m.fmResolve(rel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	if fi.IsDir() {
		return marshalJSON(map[string]any{"error": "path is directory", "path": rel})
	}
	if fi.Size() > maxBytes {
		return marshalJSON(map[string]any{"error": fmt.Sprintf("file too large for thumbnail (%d > %d max)", fi.Size(), maxBytes), "path": rel, "size": fi.Size()})
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": rel})
	}
	if len(data) == 0 {
		return marshalJSON(map[string]any{"error": "empty file", "path": rel})
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return marshalJSON(map[string]any{"error": "not an image or unsupported format: " + err.Error(), "path": rel})
	}
	if int64(cfg.Width)*int64(cfg.Height) > 50_000_000 {
		return marshalJSON(map[string]any{"error": fmt.Sprintf("image dimensions exceed limit %dx%d", cfg.Width, cfg.Height), "path": rel})
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return marshalJSON(map[string]any{"error": "failed to decode image: " + err.Error(), "path": rel})
	}
	thumb := utils.Thumbnail(img, uint(size))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 80}); err != nil {
		if err := png.Encode(&buf, thumb); err != nil {
			return marshalJSON(map[string]any{"error": "failed to encode thumbnail: " + err.Error(), "path": rel})
		}
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return marshalJSON(map[string]any{"path": rel, "width": cfg.Width, "height": cfg.Height, "thumb_size": size, "thumb_bytes": buf.Len(), "mime": "image/jpeg", "encoding": "base64", "thumbnail": b64})
}

func (m *MCPServer) fmRename(srcRel, dstRel string) string {
	srcAbs, err := m.fmResolve(srcRel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": srcRel})
	}
	dstAbs, err := m.fmResolve(dstRel)
	if err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": dstRel})
	}
	if _, err := os.Stat(srcAbs); err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": srcRel})
	}
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0750); err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": dstRel})
	}
	if err := os.Rename(srcAbs, dstAbs); err != nil {
		return marshalJSON(map[string]any{"error": err.Error(), "path": srcRel, "dest": dstRel})
	}
	return marshalJSON(map[string]any{"path": srcRel, "dest": dstRel, "renamed": true})
}

func detectMime(path string, data []byte) string {
	if len(data) > 0 {
		m := http.DetectContentType(data)
		if m != "application/octet-stream" {
			return m
		}
	}
	ext := strings.ToLower(filepath.Ext(path))
	if v := mimeByExt(ext); v != "" {
		return v
	}
	return "application/octet-stream"
}

func mimeByExt(ext string) string {
	switch ext {
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".go":
		return "text/x-go"
	case ".md":
		return "text/markdown"
	default:
		return ""
	}
}

func isText(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	sample := data
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	for _, b := range sample {
		if b == 0 {
			return false
		}
	}
	return true
}

func imageDimensionsFromBytes(data []byte) (int, int, error) {
	c, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return c.Width, c.Height, nil
}

func (m *MCPServer) getCachedHostname() string {
	m.mu.RLock()
	if m.cachedHostname != "" && time.Now().Before(m.cachedHostExpiry) {
		h := m.cachedHostname
		m.mu.RUnlock()
		return h
	}
	m.mu.RUnlock()
	hostname := ""
	if info, err := utils.GetNetworkInfo(); err == nil {
		hostname = info["hostname"]
	}
	m.mu.Lock()
	m.cachedHostname = hostname
	m.cachedHostExpiry = time.Now().Add(30 * time.Second)
	m.mu.Unlock()
	return hostname
}

func (m *MCPServer) getCachedCPUModel() string {
	m.mu.RLock()
	if m.cachedCPUModel != "" && time.Now().Before(m.cachedCPUExpiry) {
		c := m.cachedCPUModel
		m.mu.RUnlock()
		return c
	}
	m.mu.RUnlock()
	cpuModel := ""
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		cpuModel = info[0].ModelName
	}
	m.mu.Lock()
	m.cachedCPUModel = cpuModel
	m.cachedCPUExpiry = time.Now().Add(30 * time.Second)
	m.mu.Unlock()
	return cpuModel
}

func parseGoroutineDump(filter string, limit int) map[string]any {
	bufPtr := goroutineBufPool.Get().(*[]byte)
	buf := *bufPtr
	n := runtime.Stack(buf, true)
	var dump string
	if n == len(buf) {
		goroutineBufPool.Put(bufPtr)
		size := len(buf) * 2
		buf = make([]byte, size)
		n = runtime.Stack(buf, true)
		for n == len(buf) {
			size *= 2
			if size > 8<<20 {
				break
			}
			buf = make([]byte, size)
			n = runtime.Stack(buf, true)
		}
		dump = string(buf[:n])
	} else {
		dump = string(buf[:n])
		goroutineBufPool.Put(bufPtr)
	}

	type goroutineInfo struct {
		ID       int    `json:"id"`
		Function string `json:"function"`
		State    string `json:"state"`
		Stack    string `json:"stack"`
	}

	goroutines := []goroutineInfo{}
	lines := strings.Split(dump, "\n")
	var current *goroutineInfo
	var stackLines []string
	flush := func() {
		if current == nil {
			return
		}
		if len(stackLines) > 0 {
			current.Stack = strings.Join(stackLines, "\n")
		}
		goroutines = append(goroutines, *current)
		current = nil
		stackLines = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "goroutine ") {
			flush()
			g := &goroutineInfo{}
			rest := strings.TrimPrefix(line, "goroutine ")
			if idx := strings.Index(rest, " ["); idx >= 0 {
				idStr := strings.TrimSpace(rest[:idx])
				if id, err := strconv.Atoi(idStr); err == nil {
					g.ID = id
				}
				statePart := rest[idx+2:]
				if end := strings.Index(statePart, "]"); end >= 0 {
					g.State = strings.TrimSpace(statePart[:end])
					if comma := strings.Index(g.State, ","); comma >= 0 {
						g.State = strings.TrimSpace(g.State[:comma])
					}
				}
			}
			current = g
		} else if current != nil {
			if strings.HasPrefix(line, "\t") {
				stackLines = append(stackLines, line)
			} else if line != "" && current.Function == "" {
				current.Function = strings.TrimSpace(line)
			}
		}
	}
	flush()

	filtered := []goroutineInfo{}
	f := strings.ToLower(filter)
	for _, g := range goroutines {
		if f != "" && !strings.Contains(strings.ToLower(g.Function), f) && !strings.Contains(strings.ToLower(g.State), f) {
			continue
		}
		filtered = append(filtered, g)
	}

	total := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	states := map[string]int{}
	for _, g := range filtered {
		states[g.State]++
	}

	return map[string]any{
		"count":       total,
		"returned":    len(filtered),
		"truncated":   limit > 0 && total > limit,
		"filter":      filter,
		"states":      states,
		"goroutines":  filtered,
	}
}

func (m *MCPServer) toolGoroutines() string {
	return m.toolGoroutinesFiltered("", 500)
}

func (m *MCPServer) toolGoroutinesFiltered(filter string, limit int) string {
	eff := m.resolveEffective()
	key := filter + "|" + strconv.Itoa(limit)
	eff.mu.RLock()
	if eff.goroutineCache != "" && eff.goroutineKey == key && time.Now().Before(eff.goroutineExpiry) {
		cached := eff.goroutineCache
		eff.mu.RUnlock()
		return cached
	}
	eff.mu.RUnlock()
	v, _, _ := eff.sfGroup.Do("goroutine:"+key, func() (any, error) {
		eff.mu.RLock()
		if eff.goroutineCache != "" && eff.goroutineKey == key && time.Now().Before(eff.goroutineExpiry) {
			cached := eff.goroutineCache
			eff.mu.RUnlock()
			return cached, nil
		}
		eff.mu.RUnlock()
		result := marshalJSON(parseGoroutineDump(filter, limit))
		eff.mu.Lock()
		eff.goroutineCache = result
		eff.goroutineKey = key
		eff.goroutineExpiry = time.Now().Add(2 * time.Second)
		eff.mu.Unlock()
		return result, nil
	})
	return v.(string)
}

func (m *MCPServer) toolSystemControl(args map[string]any) (string, bool) {
	eff := m.resolveEffective()
	action := strings.TrimSpace(argString(args, "action"))
	switch action {
	case "gc":
		var before runtime.MemStats
		runtime.ReadMemStats(&before)
		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		return marshalJSON(map[string]any{
			"action":            "gc",
			"goroutines":        runtime.NumGoroutine(),
			"heap_alloc_before": before.HeapAlloc,
			"heap_alloc_after":  after.HeapAlloc,
			"heap_objects":      after.HeapObjects,
			"num_gc":            after.NumGC,
			"instance_id":       eff.getIdentity().InstanceID,
		}), false
	case "clear_cache":
		eff.mu.Lock()
		eff.dashboardCache = ""
		eff.dashboardExpiry = time.Time{}
		eff.resourcesCache = ""
		eff.resourcesExpiry = time.Time{}
		eff.memoryCache = ""
		eff.memoryExpiry = time.Time{}
		eff.configCache = ""
		eff.configExpiry = time.Time{}
		eff.goroutineCache = ""
		eff.goroutineExpiry = time.Time{}
		eff.goroutineKey = ""
		eff.mu.Unlock()
		return marshalJSON(map[string]any{
			"action":  "clear_cache",
			"cleared": []string{"dashboard", "resources", "memory", "config", "goroutines"},
		}), false
	default:
		return marshalJSON(map[string]any{"error": "unknown action: " + action, "allowed": []string{"gc", "clear_cache"}}), true
	}
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}
