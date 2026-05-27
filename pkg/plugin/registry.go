package plugin

import (
	"fmt"
	"sync"

	"github.com/spf13/afero"
)

type PluginFactory func(meta PluginMeta, fs afero.Fs) (Plugin, error)

type PluginRegistry struct {
	plugins   map[string]Plugin
	factories map[string]PluginFactory
	metas     map[string]PluginMeta
	fsystems  map[string]afero.Fs
	mu        sync.RWMutex
}

var (
	globalRegistry *PluginRegistry
	registryOnce   sync.Once
)

func GetGlobalPluginRegistry() *PluginRegistry {
	registryOnce.Do(func() {
		globalRegistry = &PluginRegistry{
			plugins:   make(map[string]Plugin),
			factories: make(map[string]PluginFactory),
			metas:     make(map[string]PluginMeta),
			fsystems:  make(map[string]afero.Fs),
		}
	})
	return globalRegistry
}

func RegisterPlugin(name string, factory PluginFactory) {
	GetGlobalPluginRegistry().Register(name, factory)
}

func (r *PluginRegistry) Register(name string, factory PluginFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

func (r *PluginRegistry) SetMeta(name string, meta PluginMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metas[name] = meta
}

func (r *PluginRegistry) SetFilesystem(name string, fs afero.Fs) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fsystems[name] = fs
}

func (r *PluginRegistry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

func (r *PluginRegistry) GetMeta(name string) (PluginMeta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.metas[name]
	return m, ok
}

func (r *PluginRegistry) GetFilesystem(name string) (afero.Fs, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fs, ok := r.fsystems[name]
	return fs, ok
}

func (r *PluginRegistry) GetAll() map[string]Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]Plugin, len(r.plugins))
	for k, v := range r.plugins {
		result[k] = v
	}
	return result
}

func (r *PluginRegistry) GetAllMetas() map[string]PluginMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]PluginMeta, len(r.metas))
	for k, v := range r.metas {
		result[k] = v
	}
	return result
}

func (r *PluginRegistry) Store(name string, p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[name] = p
}

func (r *PluginRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, name)
	delete(r.factories, name)
	delete(r.metas, name)
	delete(r.fsystems, name)
}

func (r *PluginRegistry) HasFactory(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[name]
	return ok
}

func (r *PluginRegistry) LookupFactory(name string, meta PluginMeta, fs afero.Fs) (Plugin, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no factory registered for plugin: %s", name)
	}
	return factory(meta, fs)
}
