package infrastructure_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"stackyrd/pkg/infrastructure"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandler_MethodNotAllowed(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHandler_OptionsCors(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestHandler_InvalidJSON(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	body := strings.NewReader("not json")
	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "-32700")
}

func TestHandler_EmptyBody(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	body := strings.NewReader("")
	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ToolsList(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "stackyrd_health")
}

func TestHandler_Initialize(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "protocolVersion")
}

func TestHandler_Ping(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_UnknownMethod(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"totally/unknown"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "-32601")
}

func TestHandler_BatchRequest(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","id":2,"method":"tools/list"}]`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "stackyrd_health")
}

func TestHandler_NotificationNoContent(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestHandler_ToolsList_ContainsServiceCall(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "stackyrd_service_call")
}

func TestHandler_ResourcesList_ContainsSvc(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "stackyrd://svc")
}

func TestHandler_ServiceCall_UnknownService(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"stackyrd_service_call","arguments":{"service":"no-such-svc"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "service not found")
	assert.Contains(t, rec.Body.String(), `"isError":true`)
}

func TestHandler_ServiceCall_TraversalRejected(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"stackyrd_service_call","arguments":{"service":"x","path":"../../etc/passwd"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"isError":true`)
}

func TestHandler_ServiceCall_BadMethod(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"stackyrd_service_call","arguments":{"service":"x","method":"TRACE"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "method not allowed")
}

func dbCallBody(t *testing.T, args string) string {
	t.Helper()
	return `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"stackyrd_db","arguments":{` + args + `}}}`
}

func postMCP(t *testing.T, body string) string {
	t.Helper()
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestHandler_ToolsList_ContainsDB(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "stackyrd_db")
}

func TestHandler_ResourcesList_ContainsDB(t *testing.T) {
	m := &infrastructure.MCPServer{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	assert.NoError(t, m.Handler()(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "stackyrd://db")
}

func TestHandler_DBStatus_Unregistered(t *testing.T) {
	res := postMCP(t, dbCallBody(t, `"action":"status"`))
	assert.Contains(t, res, `"isError":false`)
	assert.Contains(t, res, `writes_enabled`)
	assert.Contains(t, res, `postgres`)
	assert.Contains(t, res, `mongo`)
	assert.Contains(t, res, `redis`)
}

func TestHandler_DBUnknownAction(t *testing.T) {
	res := postMCP(t, dbCallBody(t, `"action":"frobnicate"`))
	assert.Contains(t, res, `"isError":true`)
	assert.Contains(t, res, "unknown db action")
}

func TestHandler_DBPGQuery_RejectsWrite(t *testing.T) {
	res := postMCP(t, dbCallBody(t, `"action":"pg_query","sql":"DROP TABLE users"`))
	assert.Contains(t, res, `"isError":true`)
	assert.Contains(t, res, "read-only")
}

func TestHandler_DBPGQuery_NoConnection(t *testing.T) {
	res := postMCP(t, dbCallBody(t, `"action":"pg_query","sql":"SELECT 1"`))
	assert.Contains(t, res, `"isError":true`)
	assert.Contains(t, res, "postgres")
}

func TestHandler_DBPGExec_Gated(t *testing.T) {
	res := postMCP(t, dbCallBody(t, `"action":"pg_exec","sql":"DELETE FROM users WHERE id = 1"`))
	assert.Contains(t, res, `"isError":true`)
	assert.Contains(t, res, "mcp.db_enabled")
}

func TestHandler_DBMongoWrite_Gated(t *testing.T) {
	res := postMCP(t, dbCallBody(t, `"action":"mongo_write","op":"delete_many","collection":"users","filter":"{}"`))
	assert.Contains(t, res, `"isError":true`)
	assert.Contains(t, res, "mcp.db_enabled")
}

func TestHandler_DBRedisWrite_Gated(t *testing.T) {
	res := postMCP(t, dbCallBody(t, `"action":"redis_write","op":"delete","key":"k"`))
	assert.Contains(t, res, `"isError":true`)
	assert.Contains(t, res, "mcp.db_enabled")
}

func TestHandler_DBMongoFind_BadFilter(t *testing.T) {
	res := postMCP(t, dbCallBody(t, `"action":"mongo_find","collection":"users","filter":"{oops"`))
	assert.Contains(t, res, `"isError":true`)
	assert.Contains(t, res, "invalid filter JSON")
}

func TestMCPServer_Name(t *testing.T) {
	m := &infrastructure.MCPServer{}
	assert.Equal(t, "MCP", m.Name())
}

func TestMCPServer_Close(t *testing.T) {
	m := &infrastructure.MCPServer{}
	assert.NoError(t, m.Close())
}

func TestMCPServer_GetStatus(t *testing.T) {
	m := &infrastructure.MCPServer{}
	status := m.GetStatus()
	assert.NotNil(t, status)
}

func TestMCPServer_RouteHandlers(t *testing.T) {
	m := &infrastructure.MCPServer{}
	handlers := m.RouteHandlers()
	assert.NotNil(t, handlers)
	assert.Greater(t, len(handlers), 0)
}
