package plugin

import (
	"context"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"

	"github.com/spf13/afero"
)

type PluginMeta struct {
	Name        string         `yaml:"name"`
	Version     string         `yaml:"version"`
	Description string         `yaml:"description"`
	Author      string         `yaml:"author"`
	DependsOn   []string       `yaml:"depends_on"`
	Entrypoint  string         `yaml:"entrypoint"`
	Limits      ResourceLimits `yaml:"limits"`
}

type ResourceLimits struct {
	MaxMemoryBytes int64 `yaml:"max_memory_bytes"`
	MaxTimeoutMs   int64 `yaml:"max_timeout_ms"`
}

type Context struct {
	ID       string
	Logger   *logger.Logger
	Registry *infrastructure.ComponentRegistry
	Cancel   context.CancelFunc
	Limits   ResourceLimits
}

type Result struct {
	Success bool
	Data    interface{}
	Error   string
}

type Plugin interface {
	Meta() PluginMeta
	Execute(ctx Context, args map[string]interface{}) (*Result, error)
	Validate() error
	Close() error
}

// Runtime is the extension point for adding new plugin execution engines.
// Each Runtime handles a specific entrypoint prefix ("ts:", "wasm:", etc.)
// and knows how to create Plugin instances from metadata + filesystem.
type Runtime interface {
	Prefix() string
	CreatePlugin(meta PluginMeta, fs afero.Fs) (Plugin, error)
}
