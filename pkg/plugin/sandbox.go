package plugin

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/shirou/gopsutil/v3/process"
)

type Sandbox struct {
	MaxMemoryBytes int64
	MaxTimeout     time.Duration
}

func NewSandbox(maxMemory int64, maxTimeout time.Duration) *Sandbox {
	return &Sandbox{
		MaxMemoryBytes: maxMemory,
		MaxTimeout:     maxTimeout,
	}
}

func RunWithTimeout(fn func(), timeout time.Duration, onKill func()) {
	done := make(chan struct{}, 1)

	go func() {
		fn()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		if onKill != nil {
			onKill()
		}
	}
}

func MemMonitor(ctx context.Context, pid int32, maxBytes int64, killFn func()) {
	if maxBytes <= 0 {
		return
	}

	p, err := process.NewProcess(pid)
	if err != nil {
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			memInfo, err := p.MemoryInfo()
			if err != nil {
				continue
			}
			if int64(memInfo.RSS) > maxBytes {
				if killFn != nil {
					killFn()
				}
				return
			}
		}
	}
}

type PluginSandbox struct {
	sandbox *Sandbox
}

func NewPluginSandbox(defaultLimits ResourceLimits) *PluginSandbox {
	return &PluginSandbox{
		sandbox: NewSandbox(defaultLimits.MaxMemoryBytes, time.Duration(defaultLimits.MaxTimeoutMs)*time.Millisecond),
	}
}

func (ps *PluginSandbox) ExecuteWithGuard(ctx context.Context, limits ResourceLimits, fn func()) error {
	timeout := ps.sandbox.MaxTimeout
	if limits.MaxTimeoutMs > 0 && time.Duration(limits.MaxTimeoutMs)*time.Millisecond < timeout {
		timeout = time.Duration(limits.MaxTimeoutMs) * time.Millisecond
	}

	memory := ps.sandbox.MaxMemoryBytes
	if limits.MaxMemoryBytes > 0 && limits.MaxMemoryBytes < memory {
		memory = limits.MaxMemoryBytes
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("plugin panic: %v", r)
			}
		}()

		if memory > 0 {
			go MemMonitor(execCtx, int32(os.Getpid()), memory, func() {
				cancel()
			})
		}

		fnDone := make(chan struct{}, 1)
		go func() {
			fn()
			fnDone <- struct{}{}
		}()

		select {
		case <-fnDone:
			errCh <- nil
		case <-execCtx.Done():
			if execCtx.Err() == context.DeadlineExceeded {
				errCh <- fmt.Errorf("plugin execution timed out after %v", timeout)
			} else {
				errCh <- fmt.Errorf("plugin execution cancelled: %w", execCtx.Err())
			}
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
