package plugin

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/logger"

	"github.com/labstack/echo/v4"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	builtinFS           embed.FS
	storeBase           string
	globalLogger        *logger.Logger
	globalInfraRegistry *infrastructure.ComponentRegistry
	pluginStartTime     time.Time
	memoryHardLimit     int64
)

type PluginConfig struct {
	Enabled       bool                  `mapstructure:"enabled"`
	DefaultLimits ResourceLimits        `mapstructure:"default_limits"`
	Overrides     map[string]PluginMeta `mapstructure:"overrides"`
	Allowlist     []string              `mapstructure:"allowlist"`
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

func Init(cfg *config.Config, l *logger.Logger, rg *echo.Group) error {
	pCfg := defaultPluginConfig()
	loadPluginConfig(&pCfg)

	globalLogger = l
	globalInfraRegistry = infrastructure.GetGlobalRegistry()
	pluginStartTime = time.Now()

	if !pCfg.Enabled {
		l.Info("Plugin system disabled")
		return nil
	}

	memoryHardLimit = pCfg.DefaultLimits.MaxMemoryBytes

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
	if viper.IsSet("plugins.allowlist") {
		list := viper.GetStringSlice("plugins.allowlist")
		for _, name := range list {
			name = strings.TrimSpace(name)
			if name != "" {
				pCfg.Allowlist = append(pCfg.Allowlist, name)
			}
		}
	}
}

func isAllowed(name string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	for _, allowed := range allowlist {
		if name == allowed {
			return true
		}
	}
	return false
}

func computeEmbeddedFileSize(baseDir string) int64 {
	var total int64
	_ = fs.WalkDir(builtinFS, baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fs.SkipDir
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func computeAllPluginSizes(builtinDir string, pluginNames []string) map[string]int64 {
	sizes := make(map[string]int64, len(pluginNames))
	for _, name := range pluginNames {
		baseDir := filepath.Join(builtinDir, name)
		var dirTotal int64
		_ = fs.WalkDir(builtinFS, baseDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fs.SkipDir
			}
			if !d.IsDir() {
				info, err := d.Info()
				if err == nil {
					dirTotal += info.Size()
				}
			}
			return nil
		})
		sizes[name] = dirTotal
	}
	return sizes
}
func scanBuiltinPlugins(pCfg PluginConfig, l *logger.Logger) error {
	reg := GetGlobalPluginRegistry()
	builtinDir := "builtin"
	entries, err := fs.ReadDir(builtinFS, builtinDir)
	if err != nil {
		l.Debug("No builtin plugins found", "error", err)
		return nil
	}

	skipped := 0

	var pluginDirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			pluginDirs = append(pluginDirs, entry.Name())
		}
	}
	pluginSizes := computeAllPluginSizes(builtinDir, pluginDirs)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginName := entry.Name()
		manifestPath := filepath.Join(builtinDir, pluginName, "plugin.yaml")

		manifestData, err := fs.ReadFile(builtinFS, manifestPath)
		if err != nil {
			l.Warn("Skipping plugin directory (no plugin.yaml)", "name", pluginName)
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

		if !isAllowed(meta.Name, pCfg.Allowlist) {
			l.Debug("Plugin not in allowlist, skipping", "name", meta.Name)
			skipped++
			continue
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

		fileSize := pluginSizes[pluginName]

		pluginType := entrypointType(meta.Entrypoint)
		stats := &PluginStats{
			Name:             meta.Name,
			Status:           "registered",
			Type:             pluginType,
			Entrypoint:       meta.Entrypoint,
			EmbeddedFileSize: fileSize,
		}
		reg.SetStats(meta.Name, stats)
		reg.SetMeta(meta.Name, meta)
		reg.SetFilesystem(meta.Name, fsys)

		l.Info("Registered builtin plugin",
			"name", meta.Name,
			"version", meta.Version,
			"type", pluginType,
			"file_size", fileSize,
		)
	}

	if len(pCfg.Allowlist) > 0 {
		l.Info("Plugin scanning complete", "registered", len(entries)-skipped-1, "skipped", skipped, "allowlist", strings.Join(pCfg.Allowlist, ","))
	} else {
		l.Info("Plugin scanning complete", "registered", len(entries)-skipped-1)
	}

	return nil
}

func initPluginsFromRegistry(pCfg PluginConfig, l *logger.Logger) {
	reg := GetGlobalPluginRegistry()

	for name := range reg.GetAllMetas() {
		meta, _ := reg.GetMeta(name)
		fsys, _ := reg.GetFilesystem(name)

		start := time.Now()
		p, err := instantiatePlugin(name, meta, fsys)
		loadTime := time.Since(start).Seconds() * 1000

		if err != nil {
			l.Error("Failed to instantiate plugin", err, "name", name)
			if s, ok := reg.GetStats(name); ok {
				s.Status = "error"
				s.LoadTimeMs = loadTime
			}
			continue
		}

		if err := p.Validate(); err != nil {
			l.Error("Plugin validation failed", err, "name", name)
			if s, ok := reg.GetStats(name); ok {
				s.Status = "error"
				s.LoadTimeMs = loadTime
			}
			continue
		}

		reg.Store(name, p)

		if s, ok := reg.GetStats(name); ok {
			s.Status = "loaded"
			s.LoadTimeMs = loadTime
		}

		l.Info("Plugin loaded",
			"name", name,
			"entrypoint", meta.Entrypoint,
			"load_time_ms", fmt.Sprintf("%.2f", loadTime),
		)
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
