package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/afero"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type externalRuntime struct{}

func (r *externalRuntime) Prefix() string { return "ext:" }

func (r *externalRuntime) CreatePlugin(meta PluginMeta, fs afero.Fs) (Plugin, error) {
	modulePath := meta.Entrypoint[4:]
	return &ExternalPlugin{
		name:       meta.Name,
		meta:       meta,
		fs:         fs,
		modulePath: modulePath,
	}, nil
}

func init() { RegisterRuntime(&externalRuntime{}) }

type ExternalPlugin struct {
	name       string
	meta       PluginMeta
	fs         afero.Fs
	modulePath string

	mu     sync.Mutex
	cmd    *exec.Cmd
	conn   *grpc.ClientConn
	client PluginRuntimeClient
}

func (p *ExternalPlugin) Meta() PluginMeta { return p.meta }

func (p *ExternalPlugin) Validate() error {
	if p.name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if p.modulePath == "" {
		return fmt.Errorf("plugin script path is required")
	}
	_, err := p.fs.Stat(p.modulePath)
	if err != nil {
		return fmt.Errorf("plugin script not found at %s: %w", p.modulePath, err)
	}
	return nil
}

func (p *ExternalPlugin) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		p.client = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
		p.cmd = nil
	}
	return nil
}

func (p *ExternalPlugin) Execute(ctx Context, args map[string]interface{}) (*Result, error) {
	if err := p.ensureRunning(); err != nil {
		return nil, fmt.Errorf("failed to start plugin host: %w", err)
	}

	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	scriptBytes, err := afero.ReadFile(p.fs, p.modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin script %s: %w", p.modulePath, err)
	}

	execCtx := context.Background()
	if ctx.Cancel != nil {
		execCtx, _ = context.WithCancel(execCtx)
	}

	resp, err := p.client.Execute(execCtx, &ExecuteRequest{
		Name:         p.name,
		ArgsJson:     argsJSON,
		ScriptSource: string(scriptBytes),
	})
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("plugin execute error: %w", err)
	}

	result := &Result{Success: resp.Success}
	if resp.Error != "" {
		result.Error = resp.Error
	}
	if len(resp.DataJson) > 0 {
		var data interface{}
		if err := json.Unmarshal(resp.DataJson, &data); err != nil {
			return nil, fmt.Errorf("failed to parse response data: %w", err)
		}
		result.Data = data
	}
	return result, nil
}

func (p *ExternalPlugin) ensureRunning() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		return nil
	}

	var hostScript string
	if v := os.Getenv("PLUGIN_PYTHON_HOST"); v != "" {
		hostScript = v
	} else {
		wd, _ := os.Getwd()
		hostScript = filepath.Join(wd, "pkg", "plugin", "python", "host.py")
	}

	hostPath := "python3"
	if hp, err := exec.LookPath("python3"); err == nil {
		hostPath = hp
	}

	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("plugin-%s-%d.sock", p.name, time.Now().UnixNano()))

	p.cmd = exec.Command(hostPath, hostScript, "--socket", socketPath, "--name", p.name)
	p.cmd.Stderr = os.Stderr

	stdoutPipe, err := p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start python host: %w", err)
	}

	ready := make(chan bool, 1)
	go func() {
		buf := make([]byte, 1024)
		n, _ := stdoutPipe.Read(buf)
		if n > 0 {
			ready <- true
		}
	}()

	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		p.cmd.Process.Kill()
		p.cmd.Wait()
		p.cmd = nil
		return fmt.Errorf("timeout waiting for python host to start")
	}

	conn, err := grpc.Dial(
		"unix:"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
		p.cmd = nil
		return fmt.Errorf("gRPC dial: %w", err)
	}

	p.conn = conn
	p.client = NewPluginRuntimeClient(conn)
	return nil
}

var _ Plugin = (*ExternalPlugin)(nil)
