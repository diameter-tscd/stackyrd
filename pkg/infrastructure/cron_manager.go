package infrastructure

import (
	"fmt"
	"stackyrd/config"
	"stackyrd/pkg/logger"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Schedule string    `json:"schedule"`
	LastRun  time.Time `json:"last_run"`
	NextRun  time.Time `json:"next_run"`
	EntryID  cron.EntryID
	cmd      func() // original wrapped command, used by RunJobNow
}

type CronManager struct {
	cron    *cron.Cron
	jobs    map[cron.EntryID]*CronJob
	mu      sync.RWMutex
	pool    *WorkerPool // Worker pool for async job execution
	poolMu  sync.Mutex
	poolSet bool
}

// Name returns the display name of the component
func (c *CronManager) Name() string {
	return "Cron Scheduler"
}

func NewCronManager() *CronManager {
	return &CronManager{
		cron: cron.New(cron.WithSeconds()), // Enable seconds field
		jobs: make(map[cron.EntryID]*CronJob),
	}
}

func (c *CronManager) ensurePool() {
	c.poolMu.Lock()
	defer c.poolMu.Unlock()
	if !c.poolSet {
		c.pool = NewWorkerPool(5)
		c.pool.Start()
		c.poolSet = true
	}
}

func (c *CronManager) Start() {
	c.cron.Start()
}

func (c *CronManager) Stop() {
	c.cron.Stop()
}

func (c *CronManager) AddJob(name, schedule string, cmd func()) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Wrap cmd to update LastRun
	wrappedCmd := func() {
		cmd()
	}

	id, err := c.cron.AddFunc(schedule, wrappedCmd)
	if err != nil {
		return 0, err
	}

	c.jobs[id] = &CronJob{
		ID:       int(id),
		Name:     name,
		Schedule: schedule,
		EntryID:  id,
		cmd:      wrappedCmd,
	}

	return int(id), nil
}

func (c *CronManager) GetJobs() []CronJob {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var list []CronJob
	entries := c.cron.Entries()

	for _, entry := range entries {
		if job, ok := c.jobs[entry.ID]; ok {
			j := *job
			j.LastRun = entry.Prev
			j.NextRun = entry.Next
			list = append(list, j)
		}
	}
	return list
}
func (c *CronManager) GetStatus() map[string]interface{} {
	if c == nil {
		return map[string]interface{}{"active": false, "jobs": []interface{}{}}
	}
	return map[string]interface{}{
		"active": true, // Always true if manager exists
		"jobs":   c.GetJobs(),
	}
}

// Async Cron Operations

// AddAsyncJob adds a job that will be executed asynchronously in the worker pool
func (c *CronManager) AddAsyncJob(name, schedule string, cmd func()) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Wrap cmd to execute in worker pool
	wrappedCmd := func() {
		c.SubmitAsyncJob(cmd)
	}

	id, err := c.cron.AddFunc(schedule, wrappedCmd)
	if err != nil {
		return 0, err
	}

	c.jobs[id] = &CronJob{
		ID:       int(id),
		Name:     name,
		Schedule: schedule,
		EntryID:  id,
		cmd:      wrappedCmd,
	}

	return int(id), nil
}

// RunJobNow runs a job immediately (asynchronously)
func (c *CronManager) RunJobNow(jobID int) error {
	c.mu.Lock()
	job, ok := c.jobs[cron.EntryID(jobID)]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("job with ID %d not found", jobID)
	}
	// Take a copy of the closure while we hold the lock
	cmd := job.cmd
	c.mu.Unlock()

	if cmd != nil {
		c.SubmitAsyncJob(cmd)
	}
	return nil
}

// GetJobStatus returns detailed status for a specific job
func (c *CronManager) GetJobStatus(jobID int) (*CronJob, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entryID := cron.EntryID(jobID)
	if job, ok := c.jobs[entryID]; ok {
		entry := c.cron.Entry(entryID)
		j := *job
		j.LastRun = entry.Prev
		j.NextRun = entry.Next
		return &j, nil
	}

	return nil, fmt.Errorf("job with ID %d not found", jobID)
}

// RemoveJob removes a job from the cron schedule
func (c *CronManager) RemoveJob(jobID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entryID := cron.EntryID(jobID)
	if _, ok := c.jobs[entryID]; ok {
		c.cron.Remove(entryID)
		delete(c.jobs, entryID)
		return nil
	}

	return fmt.Errorf("job with ID %d not found", jobID)
}

// UpdateJob updates an existing job's schedule
func (c *CronManager) UpdateJob(jobID int, newSchedule string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entryID := cron.EntryID(jobID)
	if job, ok := c.jobs[entryID]; ok {
		// Remove old job
		c.cron.Remove(entryID)

		// Re-register with the original command
		newID, err := c.cron.AddFunc(newSchedule, job.cmd)
		if err != nil {
			return err
		}

		// Update job info
		job.Schedule = newSchedule
		job.EntryID = newID
		c.jobs[newID] = job
		delete(c.jobs, entryID)

		return nil
	}

	return fmt.Errorf("job with ID %d not found", jobID)
}

// Worker Pool Operations

// SubmitAsyncJob submits a job to the worker pool for async execution
func (c *CronManager) SubmitAsyncJob(job func()) {
	c.ensurePool()
	c.pool.Submit(job)
}

// GetPoolStatus returns the status of the worker pool
func (c *CronManager) GetPoolStatus() map[string]interface{} {
	if c.pool == nil {
		return map[string]interface{}{
			"available": false,
			"workers":   0,
		}
	}

	// Note: WorkerPool doesn't expose internal stats, so we return basic info
	return map[string]interface{}{
		"available": true,
		"workers":   5, // We know this from initialization
	}
}

// Close closes the cron manager and its worker pool
func (c *CronManager) Close() error {
	c.Stop()
	if c.pool != nil {
		c.pool.Close()
	}
	return nil
}

func init() {
	RegisterComponent("cron", func(cfg *config.Config, l *logger.Logger) (InfrastructureComponent, error) {
		if !cfg.Cron.Enabled {
			return nil, nil
		}
		cronManager := NewCronManager()

		// Add configured cron jobs
		for name, schedule := range cfg.Cron.Jobs {
			jobName := name
			jobSchedule := schedule
			_, err := cronManager.AddAsyncJob(jobName, jobSchedule, func() {
				l.Info("Executing Cron Job", "job", jobName)
			})
			if err != nil {
				l.Error("Failed to schedule cron job", err, "job", jobName)
			} else {
				l.Info("Cron job scheduled", "job", jobName, "schedule", jobSchedule)
			}
		}

		cronManager.Start()
		l.Info("Cron jobs initialized with async execution")

		return cronManager, nil
	})
}
