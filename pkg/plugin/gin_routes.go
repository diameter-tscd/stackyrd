package plugin

import (
	"net/http"
	"os"
	"sync"
	"time"

	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/afero"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func wirePluginRoutes(rg *gin.RouterGroup, l *logger.Logger) {
	reg := GetGlobalPluginRegistry()
	for name, p := range reg.GetAll() {
		rr, ok := p.(RouteRegistrarPlugin)
		if !ok {
			continue
		}
		for _, route := range rr.PluginRoutes() {
			if route.StaticDir != "" {
				serveStaticRoute(name, route, rg)
				continue
			}
			if route.Method == RouteWS {
				registerWSRoute(name, p, route, rg, l)
				continue
			}
			registerDynamicRoute(name, p, route, rg, l)
		}
	}
}

func pluginName(p Plugin) string {
	return p.Meta().Name
}

func registerDynamicRoute(pluginName string, p Plugin, def RouteDefinition, rg *gin.RouterGroup, l *logger.Logger) {
	rg.Handle(string(def.Method), def.Path, func(c *gin.Context) {
		body, _ := c.GetRawData()

		params := make(map[string]string)
		for _, p := range c.Params {
			params[p.Key] = p.Value
		}

		args := map[string]interface{}{
			"request": map[string]interface{}{
				"method":  c.Request.Method,
				"path":    c.Request.URL.Path,
				"query":   c.Request.URL.Query(),
				"headers": c.Request.Header,
				"body":    string(body),
				"params":  params,
			},
		}

		meta, _ := GetGlobalPluginRegistry().GetMeta(pluginName)
		ctx := Context{
			ID:       uuid.New().String(),
			Logger:   l,
			Registry: globalInfraRegistry,
			Limits:   meta.Limits,
			State:    GetGlobalPluginRegistry().GetOrCreateState(pluginName),
		}

		reg := GetGlobalPluginRegistry()
		reg.AcquireExecution()
		start := time.Now()
		result, err := p.Execute(ctx, args)
		elapsed := time.Since(start).Seconds() * 1000
		reg.ReleaseExecution()

		if err == nil {
			reg.IncrementExecuteCount(pluginName, elapsed)
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result.Data)
	})
}

type noDirListingFS struct {
	http.FileSystem
}

func (n noDirListingFS) Open(name string) (http.File, error) {
	f, err := n.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	s, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if s.IsDir() {
		_ = f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}

func serveStaticRoute(pluginName string, def RouteDefinition, rg *gin.RouterGroup) {
	fsys, ok := GetGlobalPluginRegistry().GetFilesystem(pluginName)
	if !ok {
		return
	}

	staticFS := afero.NewBasePathFs(fsys, def.StaticDir)

	handler := gin.WrapH(http.FileServer(
		&noDirListingFS{afero.NewHttpFs(staticFS)},
	))

	rg.GET(def.Path, handler)
}

func registerWSRoute(pluginName string, p Plugin, def RouteDefinition, rg *gin.RouterGroup, l *logger.Logger) {
	rg.GET(def.Path, func(c *gin.Context) {
		conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			l.Error("WebSocket upgrade failed", err)
			return
		}
		defer conn.Close()

		meta, _ := GetGlobalPluginRegistry().GetMeta(pluginName)
		wsCtx := Context{
			ID:       uuid.New().String(),
			Logger:   l,
			Registry: globalInfraRegistry,
			Limits:   meta.Limits,
			State:    GetGlobalPluginRegistry().GetOrCreateState(pluginName),
		}

		wsSession := &wsSession{conn: conn, done: make(chan struct{})}
		executeWSPlugin(p, wsCtx, def.Handler, wsSession)
	})
}

type wsSession struct {
	conn *websocket.Conn
	mu   sync.Mutex
	done chan struct{}
}

func (s *wsSession) send(data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteJSON(data)
}

func (s *wsSession) read() ([]byte, error) {
	_, msg, err := s.conn.ReadMessage()
	return msg, err
}
