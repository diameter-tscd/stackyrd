package plugin

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	builtinFS        embed.FS
	storeBase        string
	globalLogger     *logger.Logger
	globalInfraRegistry   *infrastructure.ComponentRegistry
)

type PluginConfig struct {
	Enabled       bool                  `mapstructure:"enabled"`
	DefaultLimits ResourceLimits        `mapstructure:"default_limits"`
	Overrides     map[string]PluginMeta `mapstructure:"overrides"`
}

func defaultPluginConfig() PluginConfig {
	return PluginConfig{
		Enabled: true,
		DefaultLimits: ResourceLimits{
			MaxTimeoutMs:   30000,
			MaxMemoryBytes: 104857600,
		},
		Overrides: make(map[string]PluginMeta),
	}
}

func Init(cfg *config.Config, l *logger.Logger, rg *gin.RouterGroup) error {
	pCfg := defaultPluginConfig()
	loadPluginConfig(&pCfg)

	globalLogger = l
	globalInfraRegistry = infrastructure.GetGlobalRegistry()

	if !pCfg.Enabled {
		l.Info("Plugin system disabled")
		return nil
	}

	storeBase = filepath.Join("store", "plugins")
	if err := os.MkdirAll(storeBase, 0755); err != nil {
		return fmt.Errorf("failed to create plugin store directory: %w", err)
	}

	if err := scanBuiltinPlugins(pCfg, l); err != nil {
		return fmt.Errorf("failed to scan builtin plugins: %w", err)
	}

	initPluginsFromRegistry(pCfg, l)

	bridge := NewPluginBridge(GetGlobalPluginRegistry(), l)
	globalInfraRegistry.SetComponent("plugins", bridge)
	l.Info("PluginBridge registered in infrastructure ComponentRegistry as 'plugins'")

	RegisterManagementRoutes(rg.Group("/plugins"))

	l.Info("Plugin system initialized")
	return nil
}

func loadPluginConfig(pCfg *PluginConfig) {
	if viper.IsSet("plugins.enabled") {
		pCfg.Enabled = viper.GetBool("plugins.enabled")
	}
	if viper.IsSet("plugins.default_limits.max_timeout_ms") {
		pCfg.DefaultLimits.MaxTimeoutMs = viper.GetInt64("plugins.default_limits.max_timeout_ms")
	}
	if viper.IsSet("plugins.default_limits.max_memory_bytes") {
		pCfg.DefaultLimits.MaxMemoryBytes = viper.GetInt64("plugins.default_limits.max_memory_bytes")
	}
	if viper.IsSet("plugins.overrides") {
		overrides := viper.GetStringMap("plugins.overrides")
		for name := range overrides {
			key := "plugins.overrides." + name
			var meta PluginMeta
			if viper.IsSet(key + ".max_timeout_ms") {
				meta.Limits.MaxTimeoutMs = viper.GetInt64(key + ".max_timeout_ms")
			}
			if viper.IsSet(key + ".max_memory_bytes") {
				meta.Limits.MaxMemoryBytes = viper.GetInt64(key + ".max_memory_bytes")
			}
			pCfg.Overrides[name] = meta
		}
	}
}

func scanBuiltinPlugins(pCfg PluginConfig, l *logger.Logger) error {
	reg := GetGlobalPluginRegistry()
	builtinDir := "builtin"
	entries, err := fs.ReadDir(builtinFS, builtinDir)
	if err != nil {
		l.Debug("No builtin plugins found", "error", err)
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginName := entry.Name()
		manifestPath := filepath.Join(builtinDir, pluginName, "plugin.yaml")

		manifestData, err := fs.ReadFile(builtinFS, manifestPath)
		if err != nil {
			l.Debug("Skipping plugin directory (no plugin.yaml)", "name", pluginName)
			continue
		}

		var meta PluginMeta
		if err := yaml.Unmarshal(manifestData, &meta); err != nil {
			l.Error("Failed to parse plugin.yaml", err, "name", pluginName)
			continue
		}

		if meta.Name == "" {
			meta.Name = pluginName
		}

		if override, ok := pCfg.Overrides[meta.Name]; ok {
			if override.Limits.MaxTimeoutMs > 0 {
				meta.Limits.MaxTimeoutMs = override.Limits.MaxTimeoutMs
			}
			if override.Limits.MaxMemoryBytes > 0 {
				meta.Limits.MaxMemoryBytes = override.Limits.MaxMemoryBytes
			}
		}

		if meta.Limits.MaxTimeoutMs <= 0 {
			meta.Limits.MaxTimeoutMs = pCfg.DefaultLimits.MaxTimeoutMs
		}
		if meta.Limits.MaxMemoryBytes <= 0 {
			meta.Limits.MaxMemoryBytes = pCfg.DefaultLimits.MaxMemoryBytes
		}

		pluginPrefix := filepath.Join(builtinDir, pluginName)
		storeDir := filepath.Join(storeBase, pluginName)
		if err := ensureStoreDir(storeDir); err != nil {
			l.Error("Failed to create plugin store dir", err, "name", pluginName)
			continue
		}

		fsys := buildPluginFS(builtinFS, pluginPrefix, storeDir)

		reg.SetMeta(meta.Name, meta)
		reg.SetFilesystem(meta.Name, fsys)

		l.Info("Registered builtin plugin", "name", meta.Name, "version", meta.Version)
	}

	return nil
}

func initPluginsFromRegistry(pCfg PluginConfig, l *logger.Logger) {
	reg := GetGlobalPluginRegistry()

	for name := range reg.GetAllMetas() {
		meta, _ := reg.GetMeta(name)
		fsys, _ := reg.GetFilesystem(name)

		p, err := instantiatePlugin(name, meta, fsys)
		if err != nil {
			l.Error("Failed to instantiate plugin", err, "name", name)
			continue
		}

		if err := p.Validate(); err != nil {
			l.Error("Plugin validation failed", err, "name", name)
			continue
		}

		reg.Store(name, p)
		l.Info("Plugin loaded", "name", name, "entrypoint", meta.Entrypoint)
	}
}

func instantiatePlugin(name string, meta PluginMeta, fsys afero.Fs) (Plugin, error) {
	if rt, ok := GetRuntimeForEntrypoint(meta.Entrypoint); ok {
		p, err := rt.CreatePlugin(meta, fsys)
		if err != nil {
			return nil, fmt.Errorf("runtime %q failed to create plugin %s: %w", rt.Prefix(), name, err)
		}
		return p, nil
	}

	reg := GetGlobalPluginRegistry()
	if reg.HasFactory(name) {
		p, err := reg.LookupFactory(name, meta, fsys)
		if err != nil {
			return nil, fmt.Errorf("factory error for plugin %s: %w", name, err)
		}
		return p, nil
	}

	return nil, fmt.Errorf("no runtime or factory for plugin %s (entrypoint: %s)", name, meta.Entrypoint)
}

func SetBuiltinFS(fs embed.FS) {
	builtinFS = fs
}

func SetStoreBase(base string) {
	storeBase = base
}

func init() {
	GetGlobalPluginRegistry()
}
