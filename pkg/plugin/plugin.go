package plugin

import (
	"context"
	"runtime"
	"time"

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

type Runtime interface {
	Prefix() string
	CreatePlugin(meta PluginMeta, fs afero.Fs) (Plugin, error)
}

type PluginStats struct {
	Name              string  `json:"name"`
	Status            string  `json:"status"`
	Type              string  `json:"type"`
	Entrypoint        string  `json:"entrypoint"`
	LoadTimeMs        float64 `json:"load_time_ms"`
	EmbeddedFileSize  int64   `json:"embedded_file_size"`
	ExecuteCount      int64   `json:"execute_count"`
	LastExecuteMs     float64 `json:"last_execution_ms"`
	TotalExecuteMs    float64 `json:"total_execution_ms"`
	MemoryUsageBytes  int64   `json:"memory_usage_bytes"`
}

type PluginManagerMetrics struct {
	TotalPlugins     int           `json:"total_plugins"`
	LoadedPlugins    int           `json:"loaded_plugins"`
	TotalExecutions  int64         `json:"total_executions"`
	ActiveExecutions int32         `json:"active_executions"`
	GoroutineCount   int           `json:"goroutine_count"`
	MemoryUsageBytes uint64        `json:"memory_usage_bytes"`
	MemoryLimitBytes int64         `json:"memory_limit_bytes"`
	MemoryPercent    float64       `json:"memory_percent"`
	UptimeSeconds    float64       `json:"uptime_seconds"`
	Plugins          []PluginStats `json:"plugins"`
}

func entrypointType(entrypoint string) string {
	if len(entrypoint) > 4 && entrypoint[:5] == "wasm:" {
		return "wasm"
	}
	if len(entrypoint) > 3 && entrypoint[:4] == "lua:" {
		return "lua"
	}
	if len(entrypoint) > 2 && entrypoint[:3] == "ts:" {
		return "typescript"
	}
	if len(entrypoint) > 3 && entrypoint[:4] == "ext:" {
		return "external"
	}
	return "go"
}

func CollectMetrics(reg *PluginRegistry) PluginManagerMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metas := reg.GetAllMetas()
	plugins := reg.GetAll()
	allStats := reg.GetAllStats()

	totalExecutions := int64(0)
	loaded := 0
	pluginEntries := make([]PluginStats, 0, len(metas))

	for name, meta := range metas {
		_, isLoaded := plugins[name]
		status := "registered"
		if isLoaded {
			status = "loaded"
			loaded++
		}

		ps := PluginStats{
			Name:       name,
			Status:     status,
			Type:       entrypointType(meta.Entrypoint),
			Entrypoint: meta.Entrypoint,
		}

		if s, ok := allStats[name]; ok {
			ps.LoadTimeMs = s.LoadTimeMs
			ps.EmbeddedFileSize = s.EmbeddedFileSize
			ps.ExecuteCount = s.ExecuteCount
			ps.LastExecuteMs = s.LastExecuteMs
			ps.TotalExecuteMs = s.TotalExecuteMs
			ps.MemoryUsageBytes = s.MemoryUsageBytes
		}

		totalExecutions += ps.ExecuteCount
		pluginEntries = append(pluginEntries, ps)
	}

	uptime := time.Since(pluginStartTime).Seconds()
	usage := memStats.Alloc
	percent := 0.0
	if memoryHardLimit > 0 {
		percent = (float64(usage) / float64(memoryHardLimit)) * 100.0
	}

	return PluginManagerMetrics{
		TotalPlugins:     len(metas),
		LoadedPlugins:    loaded,
		TotalExecutions:  totalExecutions,
		ActiveExecutions: reg.ActiveExecutions(),
		GoroutineCount:   runtime.NumGoroutine(),
		MemoryUsageBytes: usage,
		MemoryLimitBytes: memoryHardLimit,
		MemoryPercent:    percent,
		UptimeSeconds:    uptime,
		Plugins:          pluginEntries,
	}
}
