package infrastructure

import (
	"cmp"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

const mcpProtocolVersion = "2026-07-28"

var supportedMCPVersions = []string{"2026-07-28", "2025-11-25", "2025-03-26"}

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

type MCPServer struct {
	enabled        bool
	endpoint       string
	token          string
	allowedOrigins []string
	logger         *logger.Logger
	limiter        *rate.Limiter
}

var mcpState struct {
	mu          sync.RWMutex
	initManager *InfraInitManager
	services    []ServiceMeta
}

func (m *MCPServer) Name() string                    { return "MCP" }
func (m *MCPServer) Close() error                    { return nil }
func (m *MCPServer) GetStatus() map[string]any { return map[string]any{"enabled": m.enabled, "endpoint": m.endpoint, "connected": true} }

func (m *MCPServer) RouteHandlers() []RouteHandler {
	return []RouteHandler{{
		Path: m.endpoint,
		Mode: RouterDefault,
		Handler: func(g *echo.Group) {
			g.POST("", m.Handler())
			g.OPTIONS("", m.Handler())
			g.GET("", func(c echo.Context) error {
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
				log.Warn("MCP temporary token generated — set mcp.token in config.yaml for persistence", "token", token, "endpoint", cfg.MCP.Endpoint)
			}
		}
		return &MCPServer{
			enabled:        true,
			endpoint:       cfg.MCP.Endpoint,
			token:          token,
			allowedOrigins: cfg.MCP.AllowedOrigins,
			logger:         log,
			limiter:        rate.NewLimiter(rate.Limit(20), 50),
		}, nil
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
	mcpState.mu.Lock()
	mcpState.initManager = m
	mcpState.mu.Unlock()
}

func SetServices(svcs []ServiceMeta) {
	mcpState.mu.Lock()
	mcpState.services = svcs
	mcpState.mu.Unlock()
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

func (m *MCPServer) Handler() echo.HandlerFunc {
	return func(c echo.Context) error {
		origin := c.Request().Header.Get("Origin")
		if origin != "" {
			if !m.isOriginAllowed(c) {
				return c.JSON(http.StatusForbidden, jsonRPCResp{
					JSONRPC: "2.0",
					Error:   &jsonRPCErr{Code: -32000, Message: "Forbidden: Origin not allowed"},
				})
			}
			c.Response().Header().Set("Access-Control-Allow-Origin", origin)
			c.Response().Header().Set("Vary", "Origin")
			c.Response().Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE, OPTIONS")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, MCP-Protocol-Version, Mcp-Method, Mcp-Name, X-MCP-Token")
			c.Response().Header().Set("Access-Control-Max-Age", "86400")
		}
		if c.Request().Method == http.MethodOptions {
			return c.NoContent(http.StatusNoContent)
		}
		if c.Request().Method != http.MethodPost {
			return c.JSON(http.StatusMethodNotAllowed, jsonRPCResp{
				JSONRPC: "2.0",
				Error:   &jsonRPCErr{Code: -32601, Message: "Method not allowed"},
			})
		}
		if m.limiter != nil && !m.limiter.Allow() {
			return c.JSON(http.StatusTooManyRequests, jsonRPCResp{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &jsonRPCErr{Code: -32003, Message: "Rate limit exceeded"},
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
		var req jsonRPCReq
		if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
			return c.JSON(http.StatusBadRequest, jsonRPCResp{
				JSONRPC: "2.0",
				Error:   &jsonRPCErr{Code: -32700, Message: "Parse error"},
			})
		}
		if req.ID == nil {
			m.route(&req)
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
		if isModernVersion(requestedVersion) {
			if err := validateStreamableHTTPHeaders(c, &req, bodyVersion); err != nil {
				return c.JSON(http.StatusBadRequest, jsonRPCResp{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error:   err,
				})
			}
		}
		resp := m.route(&req)
		if resp.Error != nil && resp.Error.Code == -32601 && isModernVersion(requestedVersion) {
			return c.JSON(http.StatusNotFound, resp)
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

func decodeHeaderValue(v string) string {
	if strings.HasPrefix(v, "=?base64?") && strings.HasSuffix(v, "?=") {
		b64 := v[9 : len(v)-2]
		if decoded, err := base64.StdEncoding.DecodeString(b64); err == nil {
			return string(decoded)
		}
	}
	return v
}

func validateStreamableHTTPHeaders(c echo.Context, req *jsonRPCReq, bodyVersion string) *jsonRPCErr {
	headerVersion := c.Request().Header.Get("MCP-Protocol-Version")
	if headerVersion == "" {
		headerVersion = c.Request().Header.Get("Mcp-Protocol-Version")
	}
	if headerVersion == "" {
		return &jsonRPCErr{Code: -32020, Message: "Header mismatch: missing required MCP-Protocol-Version header"}
	}
	mcpMethod := c.Request().Header.Get("Mcp-Method")
	if mcpMethod == "" {
		mcpMethod = c.Request().Header.Get("MCP-Method")
	}
	if mcpMethod == "" {
		return &jsonRPCErr{Code: -32020, Message: "Header mismatch: missing required Mcp-Method header"}
	}
	if mcpMethod != req.Method {
		return &jsonRPCErr{Code: -32020, Message: fmt.Sprintf("Header mismatch: Mcp-Method header %q does not match body method %q", mcpMethod, req.Method)}
	}
	if req.Method == "tools/call" || req.Method == "resources/read" || req.Method == "prompts/get" {
		expectedName := extractNameOrURI(req.Params, req.Method)
		if expectedName != "" {
			mcpName := c.Request().Header.Get("Mcp-Name")
			if mcpName == "" {
				return &jsonRPCErr{Code: -32020, Message: "Header mismatch: missing required Mcp-Name header"}
			}
			decoded := decodeHeaderValue(mcpName)
			if decoded != expectedName {
				return &jsonRPCErr{Code: -32020, Message: fmt.Sprintf("Header mismatch: Mcp-Name header %q does not match body value %q", decoded, expectedName)}
			}
		}
	}
	return nil
}

func extractNameOrURI(params json.RawMessage, method string) string {
	if len(params) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(params, &m); err != nil {
		return ""
	}
	if method == "resources/read" {
		if v, ok := m["uri"].(string); ok {
			return v
		}
	}
	if v, ok := m["name"].(string); ok {
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
	case "notifications/initialized", "notifications/cancelled":
	default:
		resp.Error = &jsonRPCErr{Code: -32601, Message: "Method not found: " + req.Method}
	}
	return resp
}

func (m *MCPServer) handleInitialize() map[string]any {
	return map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": "stackyrd", "version": "1.0"},
	}
}

func (m *MCPServer) handleDiscover() map[string]any {
	return map[string]any{
		"supportedVersions": supportedMCPVersions,
		"capabilities":      map[string]any{"tools": map[string]any{}},
		"_meta": map[string]any{
			"io.modelcontextprotocol/serverInfo": map[string]any{"name": "stackyrd", "version": "1.0"},
		},
		"instructions": "stackyrd MCP server exposes health, services, infra, and endpoint introspection tools.",
	}
}

func (m *MCPServer) handleToolsList() map[string]any {
	return map[string]any{"tools": m.toolDefs()}
}

func (m *MCPServer) toolDefs() []ToolDef {
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
	mcpState.mu.RLock()
	im := mcpState.initManager
	mcpState.mu.RUnlock()
	if im == nil {
		return `{"status":"unknown","reason":"infra init manager not ready"}`
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
	return marshalJSON(map[string]any{
		"status":     map[bool]string{true: "ready", false: "initializing"}[im.IsReady()],
		"progress":   im.GetInitializationProgress(),
		"components": comps,
	})
}

func (m *MCPServer) toolServices() string {
	mcpState.mu.RLock()
	svcs := mcpState.services
	mcpState.mu.RUnlock()
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
	seen := map[string]bool{}
	var eps []string
	mcpState.mu.RLock()
	svcs := mcpState.services
	mcpState.mu.RUnlock()
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

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}
