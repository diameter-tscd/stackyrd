package plugin

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/afero"
)

// cacheTTL bounds how stale a read-only snapshot of the registry may be. The
// Prometheus/admin endpoints copy the full maps on every call; caching them
// for a short window avoids that allocation during bursts of health polls.
const cacheTTL = 2 * time.Second

type PluginFactory func(meta PluginMeta, fs afero.Fs) (Plugin, error)

type PluginRegistry struct {
	plugins          map[string]Plugin
	factories        map[string]PluginFactory
	metas            map[string]PluginMeta
	fsystems         map[string]afero.Fs
	stats            map[string]*PluginStats
	activeExecutions atomic.Int32
	mu               sync.RWMutex

	// statsMu holds a per-plugin lock guarding the mutable counters inside
	// PluginStats (ExecuteCount, LastExecuteMs, TotalExecuteMs). This keeps
	// concurrent IncrementExecuteCount calls off the global registry lock and
	// lets GetAllStats deep-copy without racing. PluginStats itself stays a
	// plain value type so it can be safely copied/appended/serialised.
	statsMu sync.Map // map[string]*sync.Mutex

	// cacheMu guards the read-only snapshots below. cacheExpiry is set to the
	// past by any mutating call (Store/Remove/SetMeta/SetStats/Increment*)
	// so the next read rebuilds the snapshot.
	cacheMu       sync.Mutex
	cacheExpiry   time.Time
	cachedPlugins map[string]Plugin
	cachedMetas   map[string]PluginMeta
	cachedStats   map[string]*PluginStats
}

// statsLock returns the per-plugin mutex for the named stats entry, creating
// it on first use.
func (r *PluginRegistry) statsLock(name string) *sync.Mutex {
	if v, ok := r.statsMu.Load(name); ok {
		return v.(*sync.Mutex)
	}
	m := &sync.Mutex{}
	actual, _ := r.statsMu.LoadOrStore(name, m)
	return actual.(*sync.Mutex)
}

// invalidateCache forces the next read of any cached snapshot to rebuild.
func (r *PluginRegistry) invalidateCache() {
	r.cacheMu.Lock()
	r.cacheExpiry = time.Time{}
	r.cacheMu.Unlock()
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
			stats:     make(map[string]*PluginStats),
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
	r.invalidateCache()
}

func (r *PluginRegistry) SetFilesystem(name string, fs afero.Fs) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fsystems[name] = fs
	r.invalidateCache()
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
	r.cacheMu.Lock()
	if time.Now().Before(r.cacheExpiry) && r.cachedPlugins != nil {
		result := r.cachedPlugins
		r.cacheMu.Unlock()
		return result
	}
	r.cacheMu.Unlock()

	r.mu.RLock()
	result := make(map[string]Plugin, len(r.plugins))
	for k, v := range r.plugins {
		result[k] = v
	}
	r.mu.RUnlock()

	r.cacheMu.Lock()
	r.cachedPlugins = result
	r.cacheExpiry = time.Now().Add(cacheTTL)
	r.cacheMu.Unlock()
	return result
}

func (r *PluginRegistry) GetAllMetas() map[string]PluginMeta {
	r.cacheMu.Lock()
	if time.Now().Before(r.cacheExpiry) && r.cachedMetas != nil {
		result := r.cachedMetas
		r.cacheMu.Unlock()
		return result
	}
	r.cacheMu.Unlock()

	r.mu.RLock()
	result := make(map[string]PluginMeta, len(r.metas))
	for k, v := range r.metas {
		result[k] = v
	}
	r.mu.RUnlock()

	r.cacheMu.Lock()
	r.cachedMetas = result
	r.cacheExpiry = time.Now().Add(cacheTTL)
	r.cacheMu.Unlock()
	return result
}

func (r *PluginRegistry) Store(name string, p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[name] = p
	r.invalidateCache()
}

func (r *PluginRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, name)
	delete(r.factories, name)
	delete(r.metas, name)
	delete(r.fsystems, name)
	delete(r.stats, name)
	r.invalidateCache()
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

// ── Stats tracking ──────────────────────────────────────────────────

func (r *PluginRegistry) SetStats(name string, s *PluginStats) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats[name] = s
	r.invalidateCache()
}

func (r *PluginRegistry) GetStats(name string) (*PluginStats, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.stats[name]
	return s, ok
}

func (r *PluginRegistry) GetAllStats() map[string]*PluginStats {
	r.cacheMu.Lock()
	if time.Now().Before(r.cacheExpiry) && r.cachedStats != nil {
		result := r.cachedStats
		r.cacheMu.Unlock()
		return result
	}
	r.cacheMu.Unlock()

	r.mu.RLock()
	result := make(map[string]*PluginStats, len(r.stats))
	for k, v := range r.stats {
		// Deep-copy the stats entry under its own lock so the cached snapshot
		// is never mutated concurrently by IncrementExecuteCount.
		r.statsLock(k).Lock()
		cp := *v
		r.statsLock(k).Unlock()
		result[k] = &cp
	}
	r.mu.RUnlock()

	r.cacheMu.Lock()
	r.cachedStats = result
	r.cacheExpiry = time.Now().Add(cacheTTL)
	r.cacheMu.Unlock()
	return result
}

func (r *PluginRegistry) IncrementExecuteCount(name string, durationMs float64) {
	// Fast path: grab the stats pointer under RLock then mutate under per-entry
	// lock. This avoids serialising all goroutines on the global write lock.
	r.mu.RLock()
	s, ok := r.stats[name]
	r.mu.RUnlock()
	if ok {
		r.statsLock(name).Lock()
		s.ExecuteCount++
		s.LastExecuteMs = durationMs
		s.TotalExecuteMs += durationMs
		r.statsLock(name).Unlock()
		r.invalidateCache()
	}
}

// ActiveExecutions returns the number of plugin executions currently in flight.
func (r *PluginRegistry) ActiveExecutions() int32 {
	return r.activeExecutions.Load()
}

// AcquireExecution increments the active execution counter.
func (r *PluginRegistry) AcquireExecution() {
	r.activeExecutions.Add(1)
}

// ReleaseExecution decrements the active execution counter.
func (r *PluginRegistry) ReleaseExecution() {
	r.activeExecutions.Add(-1)
}
