package infrastructure

import (
	"context"
	"stackyard/config"
	"stackyard/pkg/logger"
	"sync"
	"time"
)

// InfraInitStatus represents the initialization status of an infrastructure component
type InfraInitStatus struct {
	Name        string        `json:"name"`
	Initialized bool          `json:"initialized"`
	Error       string        `json:"error,omitempty"`
	StartTime   time.Time     `json:"start_time"`
	Duration    time.Duration `json:"duration,omitempty"`
	Progress    float64       `json:"progress"` // 0.0 to 1.0
}

// InfraInitManager manages asynchronous infrastructure initialization
type InfraInitManager struct {
	status   map[string]*InfraInitStatus
	mu       sync.RWMutex
	logger   *logger.Logger
	doneChan chan struct{}
}

// NewInfraInitManager creates a new infrastructure initialization manager
func NewInfraInitManager(logger *logger.Logger) *InfraInitManager {
	return &InfraInitManager{
		status:   make(map[string]*InfraInitStatus),
		logger:   logger,
		doneChan: make(chan struct{}),
	}
}

// StartAsyncInitialization begins asynchronous initialization of all infrastructure components
func (im *InfraInitManager) StartAsyncInitialization(cfg *config.Config, logger *logger.Logger) *ComponentRegistry {
	registry := GetGlobalRegistry()

	// Initialize all registered components
	if err := registry.Initialize(cfg, logger); err != nil {
		logger.Error("Failed to initialize infrastructure components", err)
	}

	// Start async health checks and monitoring (non-blocking)
	components := registry.GetAll()
	for name, component := range components {
		name := name
		component := component
		go func(compName string, comp InfrastructureComponent) {
			// Update status to initialized
			im.updateStatus(compName, &InfraInitStatus{
				Name:        compName,
				Initialized: true,
				StartTime:   time.Now(),
				Duration:    time.Since(time.Now()), // Minimal duration
				Progress:    1.0,
			})

			// Perform health check
			status := comp.GetStatus()
			if connected, ok := status["connected"].(bool); ok && connected {
				logger.Debug(compName + " health check passed")
			} else {
				logger.Warn(compName + " health check failed or not applicable")
			}
		}(name, component)
	}

	// Signal that all synchronous initialization is complete
	close(im.doneChan)

	return registry
}

// updateStatus updates the initialization status of a component
func (im *InfraInitManager) updateStatus(name string, status *InfraInitStatus) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.status[name] = status
}

// updateStatusProgress updates only the progress of a component
func (im *InfraInitManager) updateStatusProgress(name string, progress float64) {
	im.mu.Lock()
	defer im.mu.Unlock()
	if status, exists := im.status[name]; exists {
		status.Progress = progress
	}
}

// GetStatus returns the current initialization status of all components
func (im *InfraInitManager) GetStatus() map[string]*InfraInitStatus {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Create a copy to avoid race conditions
	status := make(map[string]*InfraInitStatus)
	for k, v := range im.status {
		status[k] = v
	}

	return status
}

// IsInitialized checks if a specific component is initialized
func (im *InfraInitManager) IsInitialized(component string) bool {
	im.mu.RLock()
	defer im.mu.RUnlock()

	if status, exists := im.status[component]; exists {
		return status.Initialized
	}
	return false
}

// WaitForInitialization waits for all components to complete initialization
func (im *InfraInitManager) WaitForInitialization(ctx context.Context) error {
	select {
	case <-im.doneChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetInitializationProgress returns overall initialization progress (0.0 to 1.0)
func (im *InfraInitManager) GetInitializationProgress() float64 {
	status := im.GetStatus()
	if len(status) == 0 {
		return 0.0
	}

	totalProgress := 0.0
	for _, s := range status {
		totalProgress += s.Progress
	}

	return totalProgress / float64(len(status))
}
