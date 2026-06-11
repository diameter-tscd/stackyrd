package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/afero"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type wasmRuntime struct{}

func (r *wasmRuntime) Prefix() string { return "wasm:" }

func (r *wasmRuntime) CreatePlugin(meta PluginMeta, fs afero.Fs) (Plugin, error) {
	return &WasmPlugin{
		name:       meta.Name,
		meta:       meta,
		fs:         fs,
		modulePath: meta.Entrypoint[5:],
	}, nil
}

func init() { RegisterRuntime(&wasmRuntime{}) }

type WasmPlugin struct {
	name       string
	meta       PluginMeta
	fs         afero.Fs
	modulePath string
}

func (p *WasmPlugin) Meta() PluginMeta { return p.meta }

func (p *WasmPlugin) Validate() error {
	if p.name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.modulePath == "" {
		return fmt.Errorf("plugin wasm module path is required")
	}
	_, err := p.fs.Stat(p.modulePath)
	if err != nil {
		return fmt.Errorf("wasm module not found at %s: %w", p.modulePath, err)
	}
	return nil
}

func (p *WasmPlugin) Close() error { return nil }

func (p *WasmPlugin) Execute(ctx Context, args map[string]interface{}) (*Result, error) {
	wasmBytes, err := afero.ReadFile(p.fs, p.modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wasm module %s: %w", p.modulePath, err)
	}

	execCtx := context.Background()
	rt := wazero.NewRuntime(execCtx)
	defer rt.Close(execCtx)

	wasi_snapshot_preview1.MustInstantiate(execCtx, rt)

	mod, err := rt.CompileModule(execCtx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile wasm module: %w", err)
	}

	argsJSON, _ := json.Marshal(args)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	_, err = rt.InstantiateModule(execCtx, mod, wazero.NewModuleConfig().
		WithName(p.name).
		WithArgs("handle").
		WithEnv("PLUGIN_ARGS", string(argsJSON)).
		WithStdout(stdoutBuf).
		WithStderr(stderrBuf).
		WithSysNanosleep().
		WithSysNanotime().
		WithSysWalltime(),
	)
	if err != nil {
		errMsg := err.Error()
		if stderrBuf.Len() > 0 {
			errMsg += ": " + stderrBuf.String()
		}
		return nil, fmt.Errorf("wasm execution failed: %s", errMsg)
	}

	output := bytes.TrimSpace(stdoutBuf.Bytes())
	if len(output) == 0 {
		return &Result{Success: true, Data: nil}, nil
	}

	var result Result
	if err := json.Unmarshal(output, &result); err != nil {
		return &Result{Success: true, Data: string(output)}, nil
	}
	return &result, nil
}

var _ Plugin = (*WasmPlugin)(nil)
