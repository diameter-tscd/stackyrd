package plugin

import (
	"fmt"

	"github.com/spf13/afero"
)

type tsRuntime struct{}

func (r *tsRuntime) Prefix() string { return "ts:" }

func (r *tsRuntime) CreatePlugin(meta PluginMeta, fs afero.Fs) (Plugin, error) {
	return &TSScriptPlugin{
		name:   meta.Name,
		meta:   meta,
		fs:     fs,
		cache:  NewTSCache(".cache"),
		script: meta.Entrypoint[3:],
	}, nil
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
