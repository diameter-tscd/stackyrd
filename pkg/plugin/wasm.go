//go:generate wat2wasm builtin/wasm_greeter_plugin/scripts/handler.wat -o builtin/wasm_greeter_plugin/scripts/handler.wasm

package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/afero"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type wasmRuntime struct{}

func (r *wasmRuntime) Prefix() string { return "wasm:" }

func (r *wasmRuntime) CreatePlugin(meta PluginMeta, fs afero.Fs) (Plugin, error) {
	modulePath := meta.Entrypoint[5:]
	return &WASMPlugin{
		name:       meta.Name,
		meta:       meta,
		fs:         fs,
		modulePath: modulePath,
	}, nil
}

func init() { RegisterRuntime(&wasmRuntime{}) }

type WASMPlugin struct {
	name       string
	meta       PluginMeta
	fs         afero.Fs
	modulePath string
}

func (p *WASMPlugin) Meta() PluginMeta { return p.meta }

func (p *WASMPlugin) Validate() error {
	if p.name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.modulePath == "" {
		return fmt.Errorf("wasm module path is required")
	}
	_, err := p.fs.Stat(p.modulePath)
	if err != nil {
		return fmt.Errorf("wasm module not found at %s: %w", p.modulePath, err)
	}
	return nil
}

func (p *WASMPlugin) Close() error { return nil }

func (p *WASMPlugin) Execute(pluginCtx Context, args map[string]interface{}) (*Result, error) {
	wasmBytes, err := afero.ReadFile(p.fs, p.modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wasm module %s: %w", p.modulePath, err)
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	execCtx := context.Background()
	if pluginCtx.Cancel != nil {
		execCtx, _ = context.WithCancel(execCtx)
	}

	engine := wazero.NewRuntime(execCtx)
	defer engine.Close(execCtx)

	wasi_snapshot_preview1.MustInstantiate(execCtx, engine)

	pluginLogger := pluginCtx.Logger
	pluginID := pluginCtx.ID
	registry := pluginCtx.Registry

	_, err = engine.NewHostModuleBuilder("env").
		NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, level int32, msgPtr int32, msgLen int32) {
			mem := m.Memory()
			if mem == nil {
				return
			}
			msg := readWASMMemory(mem, msgPtr, msgLen)
			switch level {
			case 0:
				pluginLogger.Debug(msg, "plugin", pluginID)
			case 1:
				pluginLogger.Info(msg, "plugin", pluginID)
			case 2:
				pluginLogger.Warn(msg, "plugin", pluginID)
			default:
				pluginLogger.Error(msg, nil, "plugin", pluginID)
			}
		}).Export("host_log").
		NewFunctionBuilder().WithFunc(func(_ context.Context, m api.Module, namePtr int32, nameLen int32, outPtr int32, outMax int32) int32 {
			mem := m.Memory()
			if mem == nil {
				return 0
			}
			compName := readWASMMemory(mem, namePtr, nameLen)
			comp, ok := registry.Get(compName)
			if !ok {
				return 0
			}
			status := comp.GetStatus()
			statusJSON, err := json.Marshal(status)
			if err != nil {
				return 0
			}
			return writeWASMMemory(mem, outPtr, statusJSON)
		}).Export("host_infra_get").
		Instantiate(execCtx)

	if err != nil {
		return nil, fmt.Errorf("failed to instantiate host module: %w", err)
	}

	mod, err := engine.Instantiate(execCtx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate wasm module: %w", err)
	}

	execute := mod.ExportedFunction("execute")
	if execute == nil {
		return nil, fmt.Errorf("wasm module %s does not export 'execute' function", p.modulePath)
	}

	var mem api.Memory
	if m := mod.Memory(); m != nil {
		mem = m
	}

	mem.Write(4096, argsJSON)

	results, err := execute.Call(execCtx, 4096, uint64(len(argsJSON)))
	if err != nil {
		return nil, fmt.Errorf("wasm execute error: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("wasm execute returned no results")
	}

	resPtr := int32(results[0])
	resData := readWASMMemoryToEnd(mem, resPtr)

	var result Result
	if err := json.Unmarshal(resData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse wasm result JSON: %w", err)
	}
	return &result, nil
}

func readWASMMemory(mem api.Memory, ptr, length int32) string {
	if length <= 0 {
		return ""
	}
	data, ok := mem.Read(uint32(ptr), uint32(length))
	if !ok {
		return ""
	}
	return string(data)
}

func readWASMMemoryToEnd(mem api.Memory, ptr int32) []byte {
	if mem == nil {
		return nil
	}
	memLen := mem.Size()
	if uint32(ptr) >= memLen {
		return nil
	}
	avail := memLen - uint32(ptr)
	data, ok := mem.Read(uint32(ptr), avail)
	if !ok {
		return nil
	}
	for i, b := range data {
		if b == 0 {
			return data[:i]
		}
	}
	return data
}

func writeWASMMemory(mem api.Memory, ptr int32, data []byte) int32 {
	if mem == nil || len(data) == 0 {
		return 0
	}
	mem.Write(uint32(ptr), data)
	return int32(len(data))
}

var _ Plugin = (*WASMPlugin)(nil)
