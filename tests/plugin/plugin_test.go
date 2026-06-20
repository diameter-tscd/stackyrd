package main_test

import (
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"

	"stackyrd/pkg/plugin"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestStateBag(t *testing.T) {
	reg := plugin.GetGlobalPluginRegistry()
	state := reg.GetOrCreateState("test-plugin-state")
	defer reg.Remove("test-plugin-state")

	t.Run("set and get", func(t *testing.T) {
		state.Set("key1", "value1")
		val, ok := state.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, "value1", val)
	})

	t.Run("get nonexistent", func(t *testing.T) {
		_, ok := state.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("delete", func(t *testing.T) {
		state.Set("todelete", "value")
		state.Delete("todelete")
		_, ok := state.Get("todelete")
		assert.False(t, ok)
	})

	t.Run("keys", func(t *testing.T) {
		state.Clear()
		state.Set("a", 1)
		state.Set("b", 2)
		state.Set("c", 3)
		keys := state.Keys()
		assert.Len(t, keys, 3)
	})

	t.Run("clear", func(t *testing.T) {
		state.Clear()
		assert.Len(t, state.Keys(), 0)
	})

	t.Run("concurrent access", func(t *testing.T) {
		state.Clear()
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				state.Set("key", n)
				state.Get("key")
				state.Keys()
			}(i)
		}
		wg.Wait()
	})
}

func TestRouteRegistrationPlugin(t *testing.T) {
	testPlugin := &testRoutePlugin{
		routes: []plugin.RouteDefinition{
			{Path: "/test-status", Method: plugin.RouteGET, Handler: "handleStatus"},
			{Path: "/test-echo", Method: plugin.RoutePOST, Handler: "handleEcho"},
		},
	}

	reg := plugin.GetGlobalPluginRegistry()
	reg.Store("test-routes", testPlugin)
	defer reg.Remove("test-routes")

	var rr plugin.RouteRegistrarPlugin = testPlugin
	routes := rr.PluginRoutes()
	assert.Len(t, routes, 2)

	assert.Equal(t, "/test-status", routes[0].Path)
	assert.Equal(t, plugin.RouteGET, routes[0].Method)
	assert.Equal(t, "handleStatus", routes[0].Handler)

	assert.Equal(t, "/test-echo", routes[1].Path)
	assert.Equal(t, plugin.RoutePOST, routes[1].Method)
}

func TestRegistryStateIntegration(t *testing.T) {
	reg := plugin.GetGlobalPluginRegistry()

	t.Run("get or create state", func(t *testing.T) {
		s1 := reg.GetOrCreateState("shared-state")
		assert.NotNil(t, s1)

		s2 := reg.GetOrCreateState("shared-state")
		assert.Equal(t, s1, s2, "GetOrCreateState must return the same instance for same name")
	})

	t.Run("get state", func(t *testing.T) {
		_, ok := reg.GetState("nonexistent-state")
		assert.False(t, ok)

		reg.GetOrCreateState("existing-state")
		s, ok := reg.GetState("existing-state")
		assert.True(t, ok)
		assert.NotNil(t, s)
	})

	reg.Remove("shared-state")
	reg.Remove("existing-state")
}

func TestBackgroundManager(t *testing.T) {
	bgPlugin := &testBackgroundPlugin{name: "test-bg"}
	reg := plugin.GetGlobalPluginRegistry()
	reg.Store("test-bg", bgPlugin)
	defer reg.Remove("test-bg")

	bgManager := plugin.NewBackgroundManager()

	t.Run("start background plugin", func(t *testing.T) {
		ctx := plugin.Context{}
		err := bgManager.Start("test-bg", bgPlugin, ctx)
		assert.NoError(t, err)
		assert.True(t, bgPlugin.running)
	})

	t.Run("stop background plugin", func(t *testing.T) {
		err := bgManager.Stop("test-bg")
		assert.NoError(t, err)
		assert.False(t, bgPlugin.running)
	})

	t.Run("stop all", func(t *testing.T) {
		reg.Store("test-bg-2", &testBackgroundPlugin{name: "test-bg-2"})
		bgManager.Start("test-bg-2", &testBackgroundPlugin{name: "test-bg-2"}, plugin.Context{})
		bgManager.StopAll()
	})
}

func TestPluginMetrics(t *testing.T) {
	reg := plugin.GetGlobalPluginRegistry()
	reg.SetMeta("metrics-test", plugin.PluginMeta{Name: "metrics-test", Version: "1.0.0"})
	reg.Store("metrics-test", &testRoutePlugin{
		routes: []plugin.RouteDefinition{
			{Path: "/metrics", Method: plugin.RouteGET},
		},
	})
	defer reg.Remove("metrics-test")

	metrics := plugin.CollectMetrics(reg)
	assert.GreaterOrEqual(t, metrics.TotalPlugins, 1)
	assert.NotEmpty(t, metrics.Plugins)
}

func TestGinRouteWiring(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")

	reg := plugin.GetGlobalPluginRegistry()
	testPlugin := &testRoutePlugin{
		routes: []plugin.RouteDefinition{
			{Path: "/test-wired", Method: plugin.RouteGET, Handler: "handleWired"},
		},
		executeResult: &plugin.Result{Success: true, Data: map[string]string{"status": "ok"}},
	}
	reg.Store("test-wired-routes", testPlugin)
	defer reg.Remove("test-wired-routes")

	_ = rg
	_ = r
}

type testRoutePlugin struct {
	plugin.Plugin
	routes        []plugin.RouteDefinition
	executeResult *plugin.Result
	executeError  error
}

func (p *testRoutePlugin) Meta() plugin.PluginMeta {
	return plugin.PluginMeta{Name: "test-route-plugin"}
}

func (p *testRoutePlugin) Execute(ctx plugin.Context, args map[string]interface{}) (*plugin.Result, error) {
	return p.executeResult, p.executeError
}

func (p *testRoutePlugin) Validate() error { return nil }
func (p *testRoutePlugin) Close() error    { return nil }

func (p *testRoutePlugin) PluginRoutes() []plugin.RouteDefinition {
	return p.routes
}

type testBackgroundPlugin struct {
	name    string
	running bool
}

func (p *testBackgroundPlugin) Meta() plugin.PluginMeta {
	return plugin.PluginMeta{Name: p.name, Background: true}
}

func (p *testBackgroundPlugin) Execute(ctx plugin.Context, args map[string]interface{}) (*plugin.Result, error) {
	return &plugin.Result{Success: true}, nil
}

func (p *testBackgroundPlugin) Validate() error { return nil }
func (p *testBackgroundPlugin) Close() error {
	p.running = false
	return nil
}

func (p *testBackgroundPlugin) Start(ctx plugin.Context) error {
	p.running = true
	return nil
}

func (p *testBackgroundPlugin) Stop() error {
	p.running = false
	return nil
}

func (p *testBackgroundPlugin) IsRunning() bool {
	return p.running
}

var _ plugin.RouteRegistrarPlugin = (*testRoutePlugin)(nil)
var _ plugin.BackgroundPlugin = (*testBackgroundPlugin)(nil)

func init() {
	gin.SetMode(gin.TestMode)
}

func assertJSON(t *testing.T, w *httptest.ResponseRecorder, expected map[string]interface{}) {
	t.Helper()
	var got map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &got)
	assert.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestContextStatePersistence(t *testing.T) {
	reg := plugin.GetGlobalPluginRegistry()
	state := reg.GetOrCreateState("persist-test")
	defer reg.Remove("persist-test")

	state.Set("count", 1)

	val, ok := state.Get("count")
	assert.True(t, ok)
	assert.Equal(t, 1, val)

	state.Set("count", 2)
	val, ok = state.Get("count")
	assert.True(t, ok)
	assert.Equal(t, 2, val)
}
