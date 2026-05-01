package interfaces

import "github.com/gin-gonic/gin"

// PluginLifecycle defines the lifecycle hooks for a plugin
type PluginLifecycle interface {
	// OnLoad is called when the plugin is loaded
	OnLoad() error
	// OnUnload is called when the plugin is unloaded
	OnUnload() error
}

// PluginRouter defines the routing capabilities for a plugin
type PluginRouter interface {
	// RegisterRoutes registers the plugin's routes with the Gin router
	RegisterRoutes(router *gin.RouterGroup)
	// Middleware returns plugin-specific middleware (optional)
	Middleware() gin.HandlerFunc
}

// PluginConfig defines configuration capabilities for a plugin
type PluginConfig interface {
	// Name returns the unique name of the plugin
	Name() string
	// Version returns the plugin version
	Version() string
	// Description returns the plugin description
	Description() string
}

// Plugin is the main interface that all plugins must implement
type Plugin interface {
	PluginConfig
	PluginLifecycle
	PluginRouter
}

// BasePlugin provides a base implementation for simple plugins
type BasePlugin struct {
	name        string
	version     string
	description string
}

func (p *BasePlugin) Name() string        { return p.name }
func (p *BasePlugin) Version() string     { return p.version }
func (p *BasePlugin) Description() string  { return p.description }
func (p *BasePlugin) OnLoad() error        { return nil }
func (p *BasePlugin) OnUnload() error      { return nil }
func (p *BasePlugin) Middleware() gin.HandlerFunc { return nil }
func (p *BasePlugin) RegisterRoutes(router *gin.RouterGroup) {}

// NewBasePlugin creates a new base plugin with the given metadata
func NewBasePlugin(name, version, description string) *BasePlugin {
	return &BasePlugin{
		name:        name,
		version:     version,
		description: description,
	}
}