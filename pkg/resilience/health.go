package resilience

import (
	"context"
	"sync"
	"time"
)

// HealthStatus represents the health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthCheck represents a health check
type HealthCheck struct {
	Name     string
	Check    func(ctx context.Context) error
	Timeout  time.Duration
	Critical bool
}

// HealthResult represents the result of a health check
type HealthResult struct {
	Name      string        `json:"name"`
	Status    HealthStatus  `json:"status"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Critical  bool          `json:"critical"`
}

// HealthReport represents the overall health report
type HealthReport struct {
	Status    HealthStatus             `json:"status"`
	Checks    map[string]*HealthResult `json:"checks"`
	Timestamp time.Time                `json:"timestamp"`
	Duration  time.Duration            `json:"duration"`
}

// HealthChecker manages health checks
type HealthChecker struct {
	checks map[string]*HealthCheck
	mu     sync.RWMutex
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]*HealthCheck),
	}
}

// RegisterCheck registers a health check
func (hc *HealthChecker) RegisterCheck(check *HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if check.Timeout == 0 {
		check.Timeout = 5 * time.Second
	}

	hc.checks[check.Name] = check
}

// RegisterSimpleCheck registers a simple health check
func (hc *HealthChecker) RegisterSimpleCheck(name string, check func() error) {
	hc.RegisterCheck(&HealthCheck{
		Name:    name,
		Check:   func(ctx context.Context) error { return check() },
		Timeout: 5 * time.Second,
	})
}

// RegisterCriticalCheck registers a critical health check
func (hc *HealthChecker) RegisterCriticalCheck(name string, check func(ctx context.Context) error) {
	hc.RegisterCheck(&HealthCheck{
		Name:     name,
		Check:    check,
		Timeout:  5 * time.Second,
		Critical: true,
	})
}

// DeregisterCheck deregisters a health check
func (hc *HealthChecker) DeregisterCheck(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	delete(hc.checks, name)
}

// Check runs all health checks and returns a report
func (hc *HealthChecker) Check(ctx context.Context) *HealthReport {
	start := time.Now()

	hc.mu.RLock()
	checks := make(map[string]*HealthCheck, len(hc.checks))
	for k, v := range hc.checks {
		checks[k] = v
	}
	hc.mu.RUnlock()

	results := make(map[string]*HealthResult)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check *HealthCheck) {
			defer wg.Done()

			result := hc.runCheck(ctx, check)

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(name, check)
	}

	wg.Wait()

	report := &HealthReport{
		Checks:    results,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}

	report.Status = hc.calculateOverallStatus(results)

	return report
}

// CheckSingle runs a single health check
func (hc *HealthChecker) CheckSingle(ctx context.Context, name string) *HealthResult {
	hc.mu.RLock()
	check, exists := hc.checks[name]
	hc.mu.RUnlock()

	if !exists {
		return &HealthResult{
			Name:      name,
			Status:    HealthStatusUnhealthy,
			Error:     "check not found",
			Timestamp: time.Now(),
		}
	}

	return hc.runCheck(ctx, check)
}

// runCheck runs a single health check
func (hc *HealthChecker) runCheck(ctx context.Context, check *HealthCheck) *HealthResult {
	start := time.Now()

	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()

	result := &HealthResult{
		Name:      check.Name,
		Timestamp: start,
		Critical:  check.Critical,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- check.Check(checkCtx)
	}()

	select {
	case err := <-errChan:
		result.Duration = time.Since(start)
		if err != nil {
			result.Status = HealthStatusUnhealthy
			result.Error = err.Error()
		} else {
			result.Status = HealthStatusHealthy
		}
	case <-checkCtx.Done():
		result.Duration = time.Since(start)
		result.Status = HealthStatusUnhealthy
		result.Error = "health check timed out"
	}

	return result
}

// calculateOverallStatus calculates the overall health status
func (hc *HealthChecker) calculateOverallStatus(results map[string]*HealthResult) HealthStatus {
	hasUnhealthy := false
	hasDegraded := false
	hasCriticalUnhealthy := false

	for _, result := range results {
		switch result.Status {
		case HealthStatusUnhealthy:
			hasUnhealthy = true
			if result.Critical {
				hasCriticalUnhealthy = true
			}
		case HealthStatusDegraded:
			hasDegraded = true
		}
	}

	if hasCriticalUnhealthy {
		return HealthStatusUnhealthy
	}

	if hasUnhealthy {
		return HealthStatusDegraded
	}

	if hasDegraded {
		return HealthStatusDegraded
	}

	return HealthStatusHealthy
}

// GetCheckNames returns all registered check names
func (hc *HealthChecker) GetCheckNames() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	names := make([]string, 0, len(hc.checks))
	for name := range hc.checks {
		names = append(names, name)
	}

	return names
}

// GetCheck returns a health check by name
func (hc *HealthChecker) GetCheck(name string) (*HealthCheck, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	check, exists := hc.checks[name]
	return check, exists
}

// IsHealthy returns true if all checks are healthy
func (hc *HealthChecker) IsHealthy(ctx context.Context) bool {
	report := hc.Check(ctx)
	return report.Status == HealthStatusHealthy
}

// IsCriticalHealthy returns true if all critical checks are healthy
func (hc *HealthChecker) IsCriticalHealthy(ctx context.Context) bool {
	report := hc.Check(ctx)

	for _, result := range report.Checks {
		if result.Critical && result.Status != HealthStatusHealthy {
			return false
		}
	}

	return true
}
