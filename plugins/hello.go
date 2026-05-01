// plugins/hello.go
package main

import (
	"fmt"

	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
)

// 1. Asset plugin definition

// HelloPlugin embeds the convenience BasePlugin implementation.
// It also holds a Gin router group that will receive registered routes.
type HelloPlugin struct {
	*interfaces.BasePlugin
	router *gin.RouterGroup
	log    *logger.Logger
}

func NewHelloPlugin(log *logger.Logger, r *gin.RouterGroup) *HelloPlugin {
	return &HelloPlugin{
		BasePlugin: interfaces.NewBasePlugin(
			"hello",                     // unique name
			"v0.1.0",                    // version
			"simple hello‑world plugin", // description
		),
		router: r,
		log:    log,
	}
}

// 2. Required interface methods

// OnLoad is executed immediately after the shared object is opened.
//
//	For a simple plugin this often just logs that we're ready.
func (h *HelloPlugin) OnLoad() error {
	if h.log != nil {
		h.log.Info("HelloPlugin loaded")
	}
	return nil
}

// OnUnload is executed when the plugin is unloaded (e.g. during a hot‑reload).
func (h *HelloPlugin) OnUnload() error {
	if h.log != nil {
		h.log.Info("HelloPlugin unloaded")
	}
	return nil
}

// Middleware can return nil if the plugin does not provide any.
func (h *HelloPlugin) Middleware() gin.HandlerFunc {
	// This example plugin does not need middleware.
	return nil
}

// RegisterRoutes registers the plugin's endpoints with the given Gin router
// group. All plugin routes live under /plugin/hello by convention.
func (h *HelloPlugin) RegisterRoutes(router *gin.RouterGroup) {
	// Create a sub‑group so we can keep the paths tidy.
	group := router.Group("hello")
	{
		// Health‑check endpoint
		group.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "ok",
				"name":   h.Name(),
			})
		})

		// Example “/greet” endpoint that showcases route parameters
		group.GET("/greet/:name", func(c *gin.Context) {
			name := c.Param("name")
			c.String(200, fmt.Sprintf("Hello, %s!", name))
		})
	}
}

// 3. Export the plugin instance

// The host application loads plugins looking for a symbol named
// "Plugin".  It must be of type interfaces.Plugin.
// Exported variables are the only thing a Go plugin can expose.
var Plugin interfaces.Plugin = NewHelloPlugin(nil, nil)

func main() {} // Required for package main, but never called
