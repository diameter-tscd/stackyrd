package infrastructure

import (
	"context"
	"stackyrd/config"
	"stackyrd/pkg/logger"
	"sync"
	"sync/atomic"
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
	wg       sync.WaitGroup
	ready    atomic.Bool
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
	im.wg.Add(len(components))
	for name, component := range components {
		name := name
		component := component
		go func(compName string, comp InfrastructureComponent) {
			defer im.wg.Done()
			startTime := time.Now()
			// Update status to initialized
			im.updateStatus(compName, &InfraInitStatus{
				Name:        compName,
				Initialized: true,
				StartTime:   startTime,
				Duration:    time.Since(startTime),
				Progress:    1.0,
			})

			// Perform health check with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			done := make(chan map[string]interface{}, 1)
			go func() {
				done <- comp.GetStatus()
			}()

			select {
			case status := <-done:
				if connected, ok := status["connected"].(bool); ok && connected {
					logger.Debug(compName + " health check passed")
				} else {
					logger.Warn(compName + " health check failed or not applicable")
				}
			case <-ctx.Done():
				logger.Warn(compName + " health check timed out")
			}
		}(name, component)
	}

	// Mark as ready once all async health checks complete
	go func() {
		im.wg.Wait()
		im.ready.Store(true)
		close(im.doneChan)
	}()

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

// IsReady returns true once all components have completed their health checks
func (im *InfraInitManager) IsReady() bool {
	return im.ready.Load()
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
