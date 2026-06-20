package plugin

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	lua "github.com/yuin/gopher-lua"
)

type luaRuntime struct{}

func (r *luaRuntime) Prefix() string { return "lua:" }

func (r *luaRuntime) CreatePlugin(meta PluginMeta, fs afero.Fs) (Plugin, error) {
	scriptPath := meta.Entrypoint[4:]
	p := &LuaScriptPlugin{
		name:   meta.Name,
		meta:   meta,
		fs:     fs,
		script: scriptPath,
	}
	source, err := afero.ReadFile(fs, scriptPath)
	if err == nil {
		p.cachedSource = source
	}
	return p, nil
}

func init() { RegisterRuntime(&luaRuntime{}) }

type LuaScriptPlugin struct {
	name         string
	meta         PluginMeta
	fs           afero.Fs
	script       string
	cachedSource []byte
}

func (p *LuaScriptPlugin) Meta() PluginMeta { return p.meta }

func (p *LuaScriptPlugin) Validate() error {
	if p.name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.script == "" {
		return fmt.Errorf("plugin script path is required")
	}
	return nil
}

func (p *LuaScriptPlugin) Close() error { return nil }

func (p *LuaScriptPlugin) Execute(ctx Context, args map[string]interface{}) (*Result, error) {
	if p.fs == nil {
		return nil, fmt.Errorf("filesystem not available for plugin: %s", p.name)
	}

	var source []byte
	if len(p.cachedSource) > 0 {
		source = p.cachedSource
	} else {
		var err error
		source, err = afero.ReadFile(p.fs, p.script)
		if err != nil {
			return nil, fmt.Errorf("failed to read script %s: %w", p.script, err)
		}
	}

	resultCh := make(chan *Result, 1)

	sandbox := NewPluginSandbox(ctx.Limits)
	execErr := sandbox.ExecuteWithGuard(context.Background(), ctx.Limits, func() {
		L := lua.NewState()
		defer L.Close()

		loadSafeLibs(L)

		argsTable := goToLuaTable(L, args)
		L.SetGlobal("args", argsTable)

		limitsTable := L.NewTable()
		limitsTable.RawSetString("max_timeout_ms", lua.LNumber(ctx.Limits.MaxTimeoutMs))
		limitsTable.RawSetString("max_memory_bytes", lua.LNumber(ctx.Limits.MaxMemoryBytes))
		L.SetGlobal("limits", limitsTable)

		L.SetGlobal("plugin_name", lua.LString(p.name))

		loggerTable := L.NewTable()
		L.SetFuncs(loggerTable, map[string]lua.LGFunction{
			"info": func(L *lua.LState) int {
				ctx.Logger.Info(L.ToString(1), "plugin", ctx.ID)
				return 0
			},
			"warn": func(L *lua.LState) int {
				ctx.Logger.Warn(L.ToString(1), "plugin", ctx.ID)
				return 0
			},
			"error": func(L *lua.LState) int {
				ctx.Logger.Error(L.ToString(1), nil, "plugin", ctx.ID)
				return 0
			},
			"debug": func(L *lua.LState) int {
				ctx.Logger.Debug(L.ToString(1), "plugin", ctx.ID)
				return 0
			},
		})
		L.SetGlobal("logger", loggerTable)

		infraTable := L.NewTable()
		L.SetFuncs(infraTable, map[string]lua.LGFunction{
			"get": func(L *lua.LState) int {
				name := L.ToString(1)
				comp, ok := ctx.Registry.Get(name)
				if !ok {
					L.Push(lua.LNil)
					return 1
				}
				L.Push(goToLuaValue(L, comp))
				return 1
			},
		})
		L.SetGlobal("infra", infraTable)

		L.SetGlobal("done", L.NewFunction(func(L *lua.LState) int {
			t := L.ToTable(1)
			success := true
			var data interface{}
			var errStr string
			if v := t.RawGetString("success"); v != lua.LNil {
				success = lua.LVAsBool(v)
			}
			if v := t.RawGetString("data"); v != lua.LNil {
				data = luaValueToGo(v)
			}
			if v := t.RawGetString("error"); v != lua.LNil {
				errStr = lua.LVAsString(v)
			}
			select {
			case resultCh <- &Result{Success: success, Data: data, Error: errStr}:
			default:
			}
			return 0
		}))

		if err := L.DoString(string(source)); err != nil {
			select {
			case resultCh <- &Result{Success: false, Error: fmt.Sprintf("Lua load error: %s", err)}:
			default:
			}
			return
		}

		handleFn := L.GetGlobal("handle")
		if handleFn.Type() != lua.LTFunction {
			select {
			case resultCh <- &Result{Success: false, Error: "no handle() function defined in Lua script"}:
			default:
			}
			return
		}

		if err := L.CallByParam(lua.P{
			Fn:      handleFn,
			NRet:    0,
			Protect: true,
		}, argsTable); err != nil {
			select {
			case resultCh <- &Result{Success: false, Error: fmt.Sprintf("handle() error: %s", err)}:
			default:
			}
			return
		}

		select {
		case resultCh <- &Result{Success: true, Data: nil}:
		default:
		}
	})

	if execErr != nil {
		return nil, execErr
	}

	select {
	case result := <-resultCh:
		return result, nil
	default:
		return &Result{Success: true, Data: nil}, nil
	}
}

func loadSafeLibs(L *lua.LState) {
	openLib := func(name string, fn lua.LGFunction) {
		L.Push(L.NewFunction(fn))
		L.Push(lua.LString(name))
		L.Call(1, 0)
	}

	openLib(lua.LoadLibName, lua.OpenPackage)

	for _, pair := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		openLib(pair.name, pair.fn)
	}
}

func goToLuaTable(L *lua.LState, val map[string]interface{}) *lua.LTable {
	t := L.NewTable()
	for k, v := range val {
		t.RawSetString(k, goToLuaValue(L, v))
	}
	return t
}

func goToLuaValue(L *lua.LState, val interface{}) lua.LValue {
	switch v := val.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(v)
	case int:
		return lua.LNumber(v)
	case int64:
		return lua.LNumber(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []interface{}:
		t := L.NewTable()
		for i, item := range v {
			t.RawSetInt(i+1, goToLuaValue(L, item))
		}
		return t
	case map[string]interface{}:
		t := L.NewTable()
		for k, item := range v {
			t.RawSetString(k, goToLuaValue(L, item))
		}
		return t
	default:
		return lua.LString(fmt.Sprintf("%v", v))
	}
}

func luaValueToGo(val lua.LValue) interface{} {
	switch v := val.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		if v.Len() > 0 {
			result := make([]interface{}, 0, v.Len())
			v.ForEach(func(_, value lua.LValue) {
				result = append(result, luaValueToGo(value))
			})
			return result
		}
		result := make(map[string]interface{})
		v.ForEach(func(key, value lua.LValue) {
			if k, ok := key.(lua.LString); ok {
				result[string(k)] = luaValueToGo(value)
			}
		})
		return result
	default:
		return val.String()
	}
}

var _ Plugin = (*LuaScriptPlugin)(nil)
