package infrastructure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"stackyrd/config"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
)

const mcpProtocolVersion = "2025-03-26"

// ServiceMeta holds pre-computed service metadata for MCP introspection.
type ServiceMeta struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	WireName  string   `json:"wire_name"`
	Endpoints []string `json:"endpoints"`
}

// ToolDef describes one MCP tool: name, description, JSON-Schema input.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// MCPServer exposes stackyrd internal state to LLM clients over the MCP
// protocol (streamable HTTP). It wraps app-local registries only — no network
// round-trips to stackyrd's own HTTP API. All MCP-specific state lives in
// this file; it is wired in by server.go via SetInitManager/SetServices.
type MCPServer struct {
	enabled  bool
	endpoint string
	token    string
	logger   *logger.Logger
}

// mcpState holds MCP-specific runtime data, owned entirely by this package.
var mcpState struct {
	mu        sync.RWMutex
	initManager *InfraInitManager
	services    []ServiceMeta
}

func (m *MCPServer) Name() string                       { return "MCP" }
func (m *MCPServer) Close() error                       { return nil }
func (m *MCPServer) GetStatus() map[string]interface{}  { return map[string]interface{}{"enabled": m.enabled, "endpoint": m.endpoint, "connected": true} }

// RouteHandlers implements RouteRegistrar so the MCP endpoint is auto-mounted
// alongside all other infrastructure component routes — no MCP-specific block
// needed in server.go Start().
func (m *MCPServer) RouteHandlers() []RouteHandler {
	return []RouteHandler{{
		Path: m.endpoint,
		Mode: RouterDefault,
		Handler: func(g *echo.Group) {
			g.POST("", m.Handler())
		},
	}}
}

func init() {
	RegisterComponent("mcp", func(cfg *config.Config, log *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.MCP.Enabled {
			return nil, nil
		}
		return &MCPServer{enabled: true, endpoint: cfg.MCP.Endpoint, token: cfg.MCP.Token, logger: log}, nil
	})
}

// SetInitManager injects the InfraInitManager so MCP tools can report health.
func SetInitManager(m *InfraInitManager) {
	mcpState.mu.Lock()
	mcpState.initManager = m
	mcpState.mu.Unlock()
}

// SetServices injects pre-computed service metadata so MCP tools can list services.
func SetServices(svcs []ServiceMeta) {
	mcpState.mu.Lock()
	mcpState.services = svcs
	mcpState.mu.Unlock()
}

// JSON-RPC 2.0 envelopes.
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
}

// Handler mounts the MCP endpoint on Echo. Stateless: each POST returns a
// single application/json response; notifications (id absent) get 202.
// When a token is configured, requests must include it via the
// Authorization: Bearer <token> header or the X-MCP-Token header.
func (m *MCPServer) Handler() echo.HandlerFunc {
	return func(c echo.Context) error {
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
		resp := m.route(&req)
		if resp.ID == nil {
			return c.NoContent(http.StatusAccepted)
		}
		return c.JSON(http.StatusOK, resp)
	}
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

func (m *MCPServer) route(req *jsonRPCReq) jsonRPCResp {
	resp := jsonRPCResp{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = m.handleInitialize()
	case "tools/list":
		resp.Result = m.handleToolsList()
	case "tools/call":
		resp.Result, resp.Error = m.handleToolsCall(req.Params)
	case "notifications/initialized", "notifications/cancelled":
		// no response body; caller returns 202.
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
	sort.Slice(comps, func(i, j int) bool { return comps[i].Name < comps[j].Name })
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
	sort.Strings(names)
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
	sort.Strings(eps)
	return marshalJSON(eps)
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}