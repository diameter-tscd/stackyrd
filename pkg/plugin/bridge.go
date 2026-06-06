package plugin

import (
	"time"

	"stackyrd/pkg/logger"
)

type PluginSummary struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Entrypoint  string `json:"entrypoint"`
	Status      string `json:"status"`
}

type PluginBridge struct {
	registry *PluginRegistry
	logger   *logger.Logger
}

func NewPluginBridge(registry *PluginRegistry, l *logger.Logger) *PluginBridge {
	return &PluginBridge{registry: registry, logger: l}
}

func (b *PluginBridge) Name() string {
	return "plugins"
}

func (b *PluginBridge) Close() error {
	return nil
}

func (b *PluginBridge) GetStatus() map[string]interface{} {
	metrics := CollectMetrics(b.registry)
	metas := b.registry.GetAllMetas()
	plugins := b.registry.GetAll()

	loaded := 0
	entries := make([]PluginSummary, 0, len(metas))
	for name, meta := range metas {
		_, isLoaded := plugins[name]
		status := "registered"
		if isLoaded {
			status = "loaded"
			loaded++
		}
		entries = append(entries, PluginSummary{
			Name:        name,
			Version:     meta.Version,
			Description: meta.Description,
			Entrypoint:  meta.Entrypoint,
			Status:      status,
		})
	}

	return map[string]interface{}{
		"total":          len(metas),
		"loaded":         loaded,
		"registered":     len(metas) - loaded,
		"plugins":        entries,
		"active_execs":   metrics.ActiveExecutions,
		"goroutines":     metrics.GoroutineCount,
		"memory_bytes":   metrics.MemoryUsageBytes,
		"memory_limit":   metrics.MemoryLimitBytes,
		"memory_percent": metrics.MemoryPercent,
	}
}

// ── Public API for services & infra components ──────────────────────────

func (b *PluginBridge) HasPlugin(name string) bool {
	_, ok := b.registry.Get(name)
	return ok
}

func (b *PluginBridge) GetMeta(name string) (PluginMeta, bool) {
	return b.registry.GetMeta(name)
}

func (b *PluginBridge) Execute(name string, args map[string]interface{}) (*Result, error) {
	p, ok := b.registry.Get(name)
	if !ok {
		return nil, nil
	}

	meta, _ := b.registry.GetMeta(name)
	ctx := Context{
		Logger:   globalLogger,
		Registry: globalInfraRegistry,
		Limits:   meta.Limits,
	}

	b.registry.AcquireExecution()
	start := time.Now()
	result, err := p.Execute(ctx, args)
	elapsed := time.Since(start).Seconds() * 1000
	b.registry.ReleaseExecution()

	if err == nil {
		b.registry.IncrementExecuteCount(name, elapsed)
	}

	return result, err
}

func (b *PluginBridge) ListPlugins() []PluginSummary {
	metas := b.registry.GetAllMetas()
	plugins := b.registry.GetAll()

	result := make([]PluginSummary, 0, len(metas))
	for name, meta := range metas {
		_, isLoaded := plugins[name]
		status := "registered"
		if isLoaded {
			status = "loaded"
		}
		result = append(result, PluginSummary{
			Name:        name,
			Version:     meta.Version,
			Description: meta.Description,
			Entrypoint:  meta.Entrypoint,
			Status:      status,
		})
	}
	return result
}

func GetGlobalPluginBridge() *PluginBridge {
	if globalLogger == nil {
		return nil
	}
	return NewPluginBridge(GetGlobalPluginRegistry(), globalLogger)
}
