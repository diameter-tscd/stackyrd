package plugin

import (
	"fmt"
	"sync"

	"github.com/dop251/goja"
	"github.com/spf13/afero"
)

// programCache caches pre-compiled goja programs keyed by script path.
// Programs are safe for concurrent use across multiple VMs.
var programCache sync.Map

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

// getOrCompileProgram returns a cached goja.Program for the given script,
// compiling it once and reusing across executions. This avoids re-parsing
// JavaScript on every plugin call.
func (r *ScriptRuntime) getOrCompileProgram(scriptPath string, source []byte) (*goja.Program, error) {
	if prog, ok := programCache.Load(scriptPath); ok {
		return prog.(*goja.Program), nil
	}

	compiledJS, err := r.cache.Compile(r.fs, scriptPath, source)
	if err != nil {
		return nil, fmt.Errorf("failed to compile %s: %w", scriptPath, err)
	}

	prog, err := goja.Compile(scriptPath, string(compiledJS), false)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compiled script %s: %w", scriptPath, err)
	}

	programCache.Store(scriptPath, prog)
	return prog, nil
}

// vmPool is a pool of pre-warmed goja VMs. Creating a new goja.Runtime is
// relatively expensive (~1-5ms), so pooling avoids that per-execution cost.
// Each pooled VM has already been created and will have globals set before
// each execution.
var vmPool = &sync.Pool{
	New: func() interface{} {
		return goja.New()
	},
}

func (r *ScriptRuntime) Execute(ctx Context, scriptPath string, args map[string]interface{}) (*Result, error) {
	source, err := afero.ReadFile(r.fs, scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read script %s: %w", scriptPath, err)
	}

	prog, err := r.getOrCompileProgram(scriptPath, source)
	if err != nil {
		return nil, err
	}

	vm := vmPool.Get().(*goja.Runtime)

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

	_, err = vm.RunProgram(prog)
	if err != nil {
		vmPool.Put(vm)
		return nil, fmt.Errorf("goja execution error: %w", err)
	}

	vmPool.Put(vm)

	select {
	case result := <-doneCh:
		return result, nil
	default:
		return &Result{Success: true, Data: nil}, nil
	}
}
