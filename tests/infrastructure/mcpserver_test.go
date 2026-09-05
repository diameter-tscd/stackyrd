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
