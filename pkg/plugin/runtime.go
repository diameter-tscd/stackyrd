package plugin

import (
	"fmt"

	"github.com/dop251/goja"
	"github.com/spf13/afero"
)

type ScriptRuntime struct {
	plugin Plugin
	fs     afero.Fs
	cache  *TSCache
}

func NewScriptRuntime(p Plugin, fsys afero.Fs, cache *TSCache) *ScriptRuntime {
	return &ScriptRuntime{
		plugin: p,
		fs:     fsys,
		cache:  cache,
	}
}

func (r *ScriptRuntime) Execute(ctx Context, scriptPath string, args map[string]interface{}) (*Result, error) {
	source, err := afero.ReadFile(r.fs, scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}

	compiledJS, err := r.cache.Compile(r.fs, scriptPath, source)
	if err != nil {
		return nil, fmt.Errorf("failed to compile %s: %w", scriptPath, err)
	}

	vm := goja.New()

	_ = vm.Set("$args", args)
	_ = vm.Set("$limits", map[string]int64{
		"max_timeout_ms":   ctx.Limits.MaxTimeoutMs,
		"max_memory_bytes": ctx.Limits.MaxMemoryBytes,
	})

	loggerObj := vm.NewObject()
	_ = loggerObj.Set("info", func(msg string) { ctx.Logger.Info(msg, "plugin", ctx.ID) })
	_ = loggerObj.Set("warn", func(msg string) { ctx.Logger.Warn(msg, "plugin", ctx.ID) })
	_ = loggerObj.Set("error", func(msg string) { ctx.Logger.Error(msg, nil, "plugin", ctx.ID) })
	_ = loggerObj.Set("debug", func(msg string) { ctx.Logger.Debug(msg, "plugin", ctx.ID) })
	_ = vm.Set("$logger", loggerObj)

	infraObj := vm.NewObject()
	_ = infraObj.Set("get", func(name string) interface{} {
		comp, ok := ctx.Registry.Get(name)
		if !ok {
			return nil
		}
		return comp
	})
	_ = vm.Set("$infra", infraObj)

	doneCh := make(chan *Result, 1)
	vm.Set("$done", func(opts map[string]interface{}) {
		success, _ := opts["success"].(bool)
		data := opts["data"]
		errStr, _ := opts["error"].(string)
		result := &Result{Success: success, Data: data, Error: errStr}
		select {
		case doneCh <- result:
		default:
		}
	})

	_, err = vm.RunString(string(compiledJS))
	if err != nil {
		return nil, fmt.Errorf("goja execution error: %w", err)
	}

	select {
	case result := <-doneCh:
		return result, nil
	default:
		return &Result{Success: true, Data: nil}, nil
	}
}
