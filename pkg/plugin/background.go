package plugin

import (
	"context"
	"sync"
)

type PluginLifecycle int8

const (
	LifecycleOnDemand  PluginLifecycle = iota
	LifecyclePersistent
)

type BackgroundPlugin interface {
	Plugin
	Start(ctx Context) error
	Stop() error
	IsRunning() bool
}

type BackgroundManager struct {
	plugins map[string]context.CancelFunc
	mu      sync.Mutex
}

func NewBackgroundManager() *BackgroundManager {
	return &BackgroundManager{
		plugins: make(map[string]context.CancelFunc),
	}
}

func (bm *BackgroundManager) Start(name string, p BackgroundPlugin, ctx Context) error {
	bm.mu.Lock()
	if _, exists := bm.plugins[name]; exists {
		bm.mu.Unlock()
		return nil
	}
	bm.mu.Unlock()

	err := p.Start(ctx)
	if err != nil {
		return err
	}

	bm.mu.Lock()
	bm.plugins[name] = ctx.Cancel
	bm.mu.Unlock()
	return nil
}

func (bm *BackgroundManager) Stop(name string) error {
	bm.mu.Lock()
	cancel, ok := bm.plugins[name]
	if !ok {
		bm.mu.Unlock()
		return nil
	}
	delete(bm.plugins, name)
	bm.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	reg := GetGlobalPluginRegistry()
	if p, ok := reg.Get(name); ok {
		if bg, ok := p.(BackgroundPlugin); ok {
			return bg.Stop()
		}
	}
	return nil
}

func (bm *BackgroundManager) StopAll() {
	bm.mu.Lock()
	names := make([]string, 0, len(bm.plugins))
	for name := range bm.plugins {
		names = append(names, name)
	}
	bm.mu.Unlock()

	for _, name := range names {
		_ = bm.Stop(name)
	}
}
