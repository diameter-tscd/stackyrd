package infrastructure

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	stackyrdTesting "stackyrd/pkg/testing"

	"github.com/labstack/echo/v4"
)

func TestMCPHandler_Initialize(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp"}
	c, rec := stackyrdTesting.NewTestContext("POST", "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      int64(1),
		"method":  "initialize",
	})
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, 200)

	var resp json.RawMessage
	var raw jsonRPCErr
	_ = raw
	var got struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      *int64          `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.JSONRPC != "2.0" || got.ID == nil || *got.ID != 1 {
		t.Fatalf("unexpected envelope: %+v", got)
	}
	if err := json.Unmarshal(got.Result, &resp); err != nil {
		t.Fatalf("parse result: %v", err)
	}
}

func TestMCPHandler_ToolsList(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp"}
	c, rec := stackyrdTesting.NewTestContext("POST", "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      int64(2),
		"method":  "tools/list",
	})
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, 200)

	var got jsonRPCResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	tools, ok := got.Result.(map[string]any)["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got: %+v", got.Result)
	}
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}
}

func TestMCPHandler_ToolsCall(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp"}
	c, rec := stackyrdTesting.NewTestContext("POST", "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      int64(3),
		"method":  "tools/call",
		"params":  map[string]any{"name": "stackyrd_services"},
	})
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, 200)

	var got jsonRPCResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	content := got.Result.(map[string]any)["content"].([]any)[0].(map[string]any)
	text, _ := content["text"].(string)
	if text == "" {
		t.Fatalf("expected non-empty text content")
	}
	var svcs []ServiceMeta
	if err := json.Unmarshal([]byte(text), &svcs); err != nil {
		t.Fatalf("services JSON unmarshal: %v", err)
	}
}

func TestMCPHandler_UnknownMethod(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp"}
	c, rec := stackyrdTesting.NewTestContext("POST", "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      int64(4),
		"method":  "bogus",
	})
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, 200)

	var got jsonRPCResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Error == nil || got.Error.Code != -32601 {
		t.Fatalf("expected method-not-found error, got: %+v", got)
	}
}

func TestMCPHandler_Notification202(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp"}
	c, rec := stackyrdTesting.NewTestContext("POST", "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, 202)
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestMCPHandler_ParseError(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp"}
	e := echo.New()
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, http.StatusBadRequest)
}

func TestMCPHandler_TokenRequired(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp", token: "secret-token"}
	c, rec := stackyrdTesting.NewTestContext("POST", "/mcp", map[string]any{
		"jsonrpc": "2.0",
		"id":      int64(1),
		"method":  "initialize",
	})
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, http.StatusUnauthorized)
}

func TestMCPHandler_TokenBearerHeader(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp", token: "secret-token"}
	e := echo.New()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, http.StatusOK)
}

func TestMCPHandler_TokenXMCPHeader(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp", token: "secret-token"}
	e := echo.New()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MCP-Token", "secret-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, http.StatusOK)
}

func TestMCPHandler_TokenWrong(t *testing.T) {
	m := &MCPServer{enabled: true, endpoint: "/mcp", token: "secret-token"}
	e := echo.New()
	req := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := m.Handler()(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	stackyrdTesting.AssertStatus(t, rec, http.StatusUnauthorized)
}
