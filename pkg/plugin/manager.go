package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sync"

	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
)

// Manager handles plugin lifecycle (loading, unloading, listing)
type Manager struct {
	plugins   map[string]interfaces.Plugin
	logger    *logger.Logger
	mu        sync.RWMutex
	pluginDir string
}

// NewManager creates a new plugin manager
func NewManager(pluginDir string, l *logger.Logger) *Manager {
	return &Manager{
		plugins:   make(map[string]interfaces.Plugin),
		logger:    l,
		pluginDir: pluginDir,
	}
}

// LoadPlugin loads a plugin from a .so file
func (m *Manager) LoadPlugin(path string) (interfaces.Plugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Attempting to load plugin", "path", path)

	// Load the shared object
	plug, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin %s: %v", path, err)
	}

	// Look up the "Plugin" symbol
	sym, err := plug.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("failed to find Plugin symbol in %s: %v", path, err)
	}

	m.logger.Info("Found symbol in plugin", "path", path, "type", fmt.Sprintf("%T", sym))

	// Assert the symbol to the Plugin interface
	pl, ok := sym.(interfaces.Plugin)
	if !ok {
		return nil, fmt.Errorf("Plugin symbol in %s does not implement interfaces.Plugin (got %T)", path, sym)
	}

	// Call OnLoad lifecycle hook
	m.logger.Info("Calling OnLoad for plugin", "name", pl.Name())
	if err := pl.OnLoad(); err != nil {
		m.logger.Warn("OnLoad failed", "error", err, "error_type", fmt.Sprintf("%T", err))
		return nil, fmt.Errorf("plugin OnLoad failed for %s: %v", path, err)
	}

	// Store the plugin
	m.plugins[pl.Name()] = pl
	m.logger.Info("Plugin loaded", "name", pl.Name(), "version", pl.Version(), "path", path)

	return pl, nil
}

// LoadAllPlugins loads all .so files from the plugin directory
func (m *Manager) LoadAllPlugins() error {
	if m.pluginDir == "" {
		m.logger.Debug("Plugin directory not configured, skipping plugin loading")
		return nil
	}

	// Check if directory exists
	if _, err := os.Stat(m.pluginDir); os.IsNotExist(err) {
		m.logger.Debug("Plugin directory does not exist", "path", m.pluginDir)
		return nil
	}

	// Find all .so files
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".so" {
			continue
		}

		path := filepath.Join(m.pluginDir, entry.Name())
		if _, err := m.LoadPlugin(path); err != nil {
			m.logger.Warn("Failed to load plugin", "path", path, "error", err)
		}
	}

	return nil
}

// UnloadPlugin unloads a plugin by name
func (m *Manager) UnloadPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pl, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Call OnUnload lifecycle hook
	if err := pl.OnUnload(); err != nil {
		return fmt.Errorf("plugin OnUnload failed: %w", err)
	}

	delete(m.plugins, name)
	m.logger.Info("Plugin unloaded", "name", name)

	return nil
}

// GetPlugin returns a plugin by name
func (m *Manager) GetPlugin(name string) (interfaces.Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pl, ok := m.plugins[name]
	return pl, ok
}

// ListPlugins returns all loaded plugins
func (m *Manager) ListPlugins() []interfaces.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]interfaces.Plugin, 0, len(m.plugins))
	for _, pl := range m.plugins {
		plugins = append(plugins, pl)
	}
	return plugins
}

// RegisterRoutes registers all plugin routes with the router
func (m *Manager) RegisterRoutes(router func(plug interfaces.Plugin)) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, pl := range m.plugins {
		router(pl)
	}
}

// GetAll returns all plugins (for dependency injection)
func (m *Manager) GetAll() map[string]interfaces.Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]interfaces.Plugin, len(m.plugins))
	for k, v := range m.plugins {
		result[k] = v
	}
	return result
}
