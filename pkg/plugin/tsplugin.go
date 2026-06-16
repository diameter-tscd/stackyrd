package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"stackyrd/pkg/logger"

	"github.com/dop251/goja"
	"github.com/spf13/afero"
)

type tsRuntime struct{}

func (r *tsRuntime) Prefix() string { return "ts:" }

func (r *tsRuntime) CreatePlugin(meta PluginMeta, fs afero.Fs) (Plugin, error) {
	base := &TSScriptPlugin{
		name:   meta.Name,
		meta:   meta,
		fs:     fs,
		cache:  NewTSCache(".cache"),
		script: meta.Entrypoint[3:],
	}
	if meta.Background {
		bg := &tsBackgroundPlugin{
			TSScriptPlugin: base,
			routes:         meta.Routes,
			done:           make(chan struct{}),
		}
		return bg, nil
	}
	if len(meta.Routes) > 0 {
		return &scriptRoutePlugin{Plugin: base, routes: meta.Routes}, nil
	}
	return base, nil
}

func init() { RegisterRuntime(&tsRuntime{}) }

type TSScriptPlugin struct {
	name   string
	meta   PluginMeta
	fs     afero.Fs
	cache  *TSCache
	script string
}

func (p *TSScriptPlugin) Meta() PluginMeta {
	return p.meta
}

func (p *TSScriptPlugin) Execute(ctx Context, args map[string]interface{}) (*Result, error) {
	if p.fs == nil {
		return nil, fmt.Errorf("filesystem not available for plugin: %s", p.name)
	}

	runtime := NewScriptRuntime(p, p.fs, p.cache)
	return runtime.Execute(ctx, p.script, args)
}

func (p *TSScriptPlugin) Validate() error {
	if p.name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.script == "" {
		return fmt.Errorf("plugin script path is required")
	}
	return nil
}

func (p *TSScriptPlugin) Close() error {
	return nil
}

type tsBackgroundPlugin struct {
	*TSScriptPlugin
	routes       []RouteDefMeta
	vm           *goja.Runtime
	program      *goja.Program
	cancel       context.CancelFunc
	done         chan struct{}
	mu           sync.Mutex
	running      bool
	eventHandlers map[string][]func(args map[string]interface{})
}

func (p *tsBackgroundPlugin) PluginRoutes() []RouteDefinition {
	result := make([]RouteDefinition, len(p.routes))
	for i, r := range p.routes {
		result[i] = RouteDefinition{
			Path:       r.Path,
			Method:     RouteMethod(r.Method),
			Handler:    r.Handler,
			Public:     r.Public,
			StaticDir:  r.StaticDir,
			StaticIndex: r.StaticIndex,
		}
	}
	return result
}

func (p *tsBackgroundPlugin) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *tsBackgroundPlugin) Start(ctx Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
	}

	source, err := afero.ReadFile(p.fs, p.script)
	if err != nil {
		return fmt.Errorf("failed to read script %s: %w", p.script, err)
	}

	compiledJS, err := p.cache.Compile(p.fs, p.script, source)
	if err != nil {
		return fmt.Errorf("failed to compile %s: %w", p.script, err)
	}

	prog, err := goja.Compile(p.script, string(compiledJS), false)
	if err != nil {
		return fmt.Errorf("failed to parse compiled script %s: %w", p.script, err)
	}
	p.program = prog

	p.vm = goja.New()
	p.eventHandlers = make(map[string][]func(args map[string]interface{}))

	_ = p.vm.Set("$args", map[string]interface{}{})
	_ = p.vm.Set("$limits", map[string]int64{
		"max_timeout_ms":   ctx.Limits.MaxTimeoutMs,
		"max_memory_bytes": ctx.Limits.MaxMemoryBytes,
	})

	loggerObj := p.vm.NewObject()
	_ = loggerObj.Set("info", func(msg string) { ctx.Logger.Info(msg, "plugin", p.name) })
	_ = loggerObj.Set("warn", func(msg string) { ctx.Logger.Warn(msg, "plugin", p.name) })
	_ = loggerObj.Set("error", func(msg string) { ctx.Logger.Error(msg, nil, "plugin", p.name) })
	_ = loggerObj.Set("debug", func(msg string) { ctx.Logger.Debug(msg, "plugin", p.name) })
	_ = p.vm.Set("$logger", loggerObj)

	infraObj := p.vm.NewObject()
	_ = infraObj.Set("get", func(name string) interface{} {
		comp, ok := ctx.Registry.Get(name)
		if !ok {
			return nil
		}
		return comp
	})
	_ = p.vm.Set("$infra", infraObj)

	injectStateGlobals(p.vm, ctx.State)

	doneCh := make(chan *Result, 1)
	p.vm.Set("$done", func(opts map[string]interface{}) {
		success, _ := opts["success"].(bool)
		data := opts["data"]
		errStr, _ := opts["error"].(string)
		result := &Result{Success: success, Data: data, Error: errStr}
		select {
		case doneCh <- result:
		default:
		}
	})

	bgObj := p.vm.NewObject()
	_ = bgObj.Set("setInterval", func(ms int64, fn func()) string {
		return ""
	})
	_ = bgObj.Set("setTimeout", func(ms int64, fn func()) string {
		return ""
	})
	_ = bgObj.Set("clearInterval", func(id string) {})
	_ = bgObj.Set("clearTimeout", func(id string) {})
	_ = bgObj.Set("sleep", func(ms int64) {
		select {
		case <-p.done:
		case <-time.After(time.Duration(ms) * time.Millisecond):
		}
	})
	_ = p.vm.Set("$background", bgObj)

	execCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	go func() {
		defer func() {
			if r := recover(); r != nil {
				ctx.Logger.Error("Background plugin panicked", nil, "name", p.name, "panic", r)
			}
			close(p.done)
		}()

		p.mu.Lock()
		p.running = true
		p.mu.Unlock()

		_, err := p.vm.RunProgram(p.program)
		if err != nil {
			select {
			case <-execCtx.Done():
			default:
				ctx.Logger.Error("Background plugin script error", err, "name", p.name)
			}
		}
	}()

	return nil
}

func (p *tsBackgroundPlugin) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	if p.cancel != nil {
		p.cancel()
	}
	if p.vm != nil {
		p.vm.Interrupt("shutdown")
	}
	p.mu.Unlock()

	<-p.done

	p.mu.Lock()
	p.vm = nil
	p.program = nil
	p.running = false
	p.mu.Unlock()
	return nil
}

var _ RouteRegistrarPlugin = (*tsBackgroundPlugin)(nil)
var _ BackgroundPlugin = (*tsBackgroundPlugin)(nil)

func injectBackgroundGlobalsTS(vm *goja.Runtime, logger *logger.Logger, state StateBag) {
	// Already handled in Start() above
}

func injectStateGlobals(vm *goja.Runtime, state StateBag) {
	stateObj := vm.NewObject()
	_ = stateObj.Set("get", func(key string) interface{} {
		val, _ := state.Get(key)
		return val
	})
	_ = stateObj.Set("set", func(key string, val interface{}) {
		state.Set(key, val)
	})
	_ = stateObj.Set("delete", func(key string) {
		state.Delete(key)
	})
	_ = stateObj.Set("clear", func() {
		state.Clear()
	})
	_ = stateObj.Set("keys", func() []string {
		return state.Keys()
	})
	_ = vm.Set("$state", stateObj)
}

var _ Plugin = (*TSScriptPlugin)(nil)
