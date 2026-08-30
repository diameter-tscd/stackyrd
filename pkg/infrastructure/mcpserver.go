package infrastructure

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

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

func memoryThresholds() []map[string]any {
	return []map[string]any{
		{"level": "low", "label": "Healthy", "min": 0, "max": 50, "color": "#22c55e", "bg": "rgba(34,197,94,0.15)"},
		{"level": "moderate", "label": "Moderate", "min": 50, "max": 75, "color": "#eab308", "bg": "rgba(234,179,8,0.15)"},
		{"level": "high", "label": "High", "min": 75, "max": 90, "color": "#f97316", "bg": "rgba(249,115,22,0.15)"},
		{"level": "critical", "label": "Critical", "min": 90, "max": 100, "color": "#ef4444", "bg": "rgba(239,68,68,0.15)"},
	}
}

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

func buildMemoryDetails() map[string]any {
	var vm *mem.VirtualMemoryStat
	if v, err := mem.VirtualMemory(); err == nil {
		vm = v
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
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
	thresholds := memoryThresholds()
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
			"self_mib":        utils.GetMemSelf(),
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
	tracked := len(m.ipStates)
	blocked := 0
	now := time.Now()
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
	b, _ := json.Marshal(payload)
	var buf bytes.Buffer
	buf.WriteString("event: message\n")
	buf.WriteString("data: ")
	buf.Write(b)
	buf.WriteString("\n\n")
	_, err := c.Response().Write(buf.Bytes())
	return err
}

func writeSSEBatch(c echo.Context, payloads []jsonRPCResp) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)
	for _, p := range payloads {
		b, _ := json.Marshal(p)
		var buf bytes.Buffer
		buf.WriteString("event: message\n")
		buf.WriteString("data: ")
		buf.Write(b)
		buf.WriteString("\n\n")
		if _, err := c.Response().Write(buf.Bytes()); err != nil {
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
		body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
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
	auth := c.Request().Header.Get("Authorization")
	if auth != "" {
		const prefix = "Bearer "
		if len(auth) > len(prefix) && auth[:len(prefix)] == prefix {
			return auth[len(prefix):] == m.token
		}
	}
	if header := c.Request().Header.Get("X-MCP-Token"); header != "" {
		return header == m.token
	}
	return false
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
	return []map[string]any{
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
	}
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
	default:
		return nil, &jsonRPCErr{Code: -32602, Message: "Resource not found: " + p.URI}
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
		text = m.toolGoroutines()
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
	hostname := ""
	if info, err := utils.GetNetworkInfo(); err == nil {
		hostname = info["hostname"]
	}
	cpuModel := ""
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		cpuModel = info[0].ModelName
	}
	id := m.getIdentity()
	return marshalJSON(map[string]any{
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
	hostname := ""
	if info, err := utils.GetNetworkInfo(); err == nil {
		hostname = info["hostname"]
	}
	cpuModel := ""
	if info, err := cpu.Info(); err == nil && len(info) > 0 {
		cpuModel = info[0].ModelName
	}
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
	return marshalJSON(map[string]any{
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
		"services":   svcs,
		"infra":      infra,
		"infra_disabled": disabledInfra,
		"middleware": mwOut,
		"endpoints":  endpoints,
	})
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
	return marshalJSON(buildMemoryDetails())
}

func parseGoroutineDump(filter string, limit int) map[string]any {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	dump := string(buf[:n])

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
	return marshalJSON(parseGoroutineDump("", 0))
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}
