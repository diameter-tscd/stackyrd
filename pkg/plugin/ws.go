package plugin

import (
	"encoding/json"

	"github.com/dop251/goja"
	"github.com/spf13/afero"
)

type WebSocketPlugin interface {
	RegisterRoutes() []RouteDefinition
}

func executeWSPlugin(p Plugin, ctx Context, handlerName string, session *wsSession) {
	switch plugin := p.(type) {
	case *TSScriptPlugin:
		executeTSWSPlugin(plugin, ctx, handlerName, session)
	case *LuaScriptPlugin:
		executeLuaWSPlugin(plugin, ctx, handlerName, session)
	default:
		for {
			msg, err := session.read()
			if err != nil {
				return
			}
			var data interface{}
			_ = json.Unmarshal(msg, &data)
			args := map[string]interface{}{
				"ws_message": data,
			}
			result, execErr := p.Execute(ctx, args)
			if execErr != nil {
				return
			}
			if result != nil && result.Data != nil {
				_ = session.send(result.Data)
			}
		}
	}
}

func executeTSWSPlugin(p *TSScriptPlugin, ctx Context, handlerName string, session *wsSession) {
	if p.fs == nil {
		ctx.Logger.Error("WS: filesystem not available", nil, "plugin", p.name)
		return
	}

	runtime := NewScriptRuntime(p, p.fs, p.cache)
	source, err := afero.ReadFile(p.fs, p.script)
	if err != nil {
		ctx.Logger.Error("WS: failed to read script", err, "plugin", p.name)
		return
	}

	prog, err := runtime.getOrCompileProgram(p.script, source)
	if err != nil {
		ctx.Logger.Error("WS: failed to compile script", err, "plugin", p.name)
		return
	}

	vm := vmPool.Get().(*goja.Runtime)
	defer vmPool.Put(vm)

	_ = vm.Set("$args", map[string]interface{}{})
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

	injectStateGlobals(vm, ctx.State)

	wsObj := vm.NewObject()
	_ = wsObj.Set("send", func(data map[string]interface{}) {
		_ = session.send(data)
	})
	_ = wsObj.Set("close", func() {
		_ = session.conn.Close()
	})
	_ = vm.Set("$ws", wsObj)

	if _, err := vm.RunProgram(prog); err != nil {
		ctx.Logger.Error("WS: goja execution error", err, "plugin", p.name)
		return
	}

	handlerVal := vm.Get(handlerName)
	handlerFn, ok := goja.AssertFunction(handlerVal)
	if !ok {
		ctx.Logger.Error("WS: handler function not found or not callable", nil, "plugin", p.name, "handler", handlerName)
		return
	}

	msgCh := make(chan []byte, 64)

	go func() {
		for {
			msg, err := session.read()
			if err != nil {
				close(msgCh)
				return
			}
			msgCh <- msg
		}
	}()

	for msg := range msgCh {
		var data map[string]interface{}
		if err := json.Unmarshal(msg, &data); err != nil {
			data = map[string]interface{}{"raw": string(msg)}
		}
		_, callErr := handlerFn(goja.Undefined(), vm.ToValue(data))
		if callErr != nil {
			ctx.Logger.Error("WS: handler error", callErr, "plugin", p.name)
			return
		}
	}
}

func executeLuaWSPlugin(p *LuaScriptPlugin, ctx Context, handlerName string, session *wsSession) {
	ctx.Logger.Warn("WS not supported for Lua plugins yet", "plugin", p.name)
}
