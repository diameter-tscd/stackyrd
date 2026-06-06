package logging

import (
	"encoding/json"
	"hash/fnv"
	"sync"
	"time"
)

// SamplingStrategy represents the sampling strategy
type SamplingStrategy int

const (
	// SampleByRate samples a percentage of logs
	SampleByRate SamplingStrategy = iota
	// SampleByCount samples every N logs
	SampleByCount
	// SampleByTime samples one log per time window
	SampleByTime
)

// LogSampler samples logs based on various strategies
type LogSampler struct {
	strategy SamplingStrategy
	rate     float64
	count    int
	window   time.Duration

	mu           sync.Mutex
	counter      int
	lastSampled  time.Time
	sampledCount int
}

// NewLogSampler creates a new log sampler
func NewLogSampler(strategy SamplingStrategy, rate float64, count int, window time.Duration) *LogSampler {
	return &LogSampler{
		strategy: strategy,
		rate:     rate,
		count:    count,
		window:   window,
	}
}

// ShouldSample determines if a log should be sampled
func (ls *LogSampler) ShouldSample(entry LogEntry) bool {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	switch ls.strategy {
	case SampleByRate:
		return ls.sampleByRate(entry)
	case SampleByCount:
		return ls.sampleByCount()
	case SampleByTime:
		return ls.sampleByTime()
	default:
		return true
	}
}

// sampleByRate samples based on a rate (0.0 to 1.0)
func (ls *LogSampler) sampleByRate(entry LogEntry) bool {
	// Use message hash for deterministic sampling
	h := fnv.New32a()
	h.Write([]byte(entry.Message))
	hash := h.Sum32()

	// Convert hash to 0-1 range
	normalized := float64(hash%10000) / 10000.0

	return normalized < ls.rate
}

// sampleByCount samples every N logs
func (ls *LogSampler) sampleByCount() bool {
	ls.counter++

	if ls.counter >= ls.count {
		ls.counter = 0
		ls.sampledCount++
		return true
	}

	return false
}

// sampleByTime samples one log per time window
func (ls *LogSampler) sampleByTime() bool {
	now := time.Now()

	if now.Sub(ls.lastSampled) >= ls.window {
		ls.lastSampled = now
		ls.sampledCount++
		return true
	}

	return false
}

// Reset resets the sampler state
func (ls *LogSampler) Reset() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.counter = 0
	ls.sampledCount = 0
	ls.lastSampled = time.Time{}
}

// GetStats returns sampler statistics
func (ls *LogSampler) GetStats() map[string]interface{} {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	return map[string]interface{}{
		"strategy":      ls.strategy,
		"rate":          ls.rate,
		"count":         ls.count,
		"window":        ls.window.String(),
		"counter":       ls.counter,
		"sampled_count": ls.sampledCount,
		"last_sampled":  ls.lastSampled,
	}
}

// SamplingLogger wraps a logger with sampling
type SamplingLogger struct {
	logger  *StructuredLogger
	sampler *LogSampler
}

// NewSamplingLogger creates a new sampling logger
func NewSamplingLogger(logger *StructuredLogger, sampler *LogSampler) *SamplingLogger {
	return &SamplingLogger{
		logger:  logger,
		sampler: sampler,
	}
}

// Log logs an entry if it passes sampling
func (sl *SamplingLogger) Log(entry LogEntry) {
	if sl.sampler.ShouldSample(entry) {
		// Write the log entry directly to the underlying logger
		data, err := json.Marshal(entry)
		if err != nil {
			return
		}
		data = append(data, '\n')
		_, _ = sl.logger.writer.Write(data)
	}
}

// Debug logs a debug message with sampling
func (sl *SamplingLogger) Debug(msg string, fields ...map[string]interface{}) {
	if sl.logger.level <= DEBUG {
		entry := sl.logger.createEntry(DEBUG, msg, fields...)
		if sl.sampler.ShouldSample(entry) {
			sl.logger.log(DEBUG, msg, fields...)
		}
	}
}

// Info logs an info message with sampling
func (sl *SamplingLogger) Info(msg string, fields ...map[string]interface{}) {
	if sl.logger.level <= INFO {
		entry := sl.logger.createEntry(INFO, msg, fields...)
		if sl.sampler.ShouldSample(entry) {
			sl.logger.log(INFO, msg, fields...)
		}
	}
}

// Warn logs a warning message with sampling
func (sl *SamplingLogger) Warn(msg string, fields ...map[string]interface{}) {
	if sl.logger.level <= WARN {
		entry := sl.logger.createEntry(WARN, msg, fields...)
		if sl.sampler.ShouldSample(entry) {
			sl.logger.log(WARN, msg, fields...)
		}
	}
}

// Error logs an error message with sampling
func (sl *SamplingLogger) Error(msg string, fields ...map[string]interface{}) {
	if sl.logger.level <= ERROR {
		entry := sl.logger.createEntry(ERROR, msg, fields...)
		if sl.sampler.ShouldSample(entry) {
			sl.logger.log(ERROR, msg, fields...)
		}
	}
}

// Fatal logs a fatal message with sampling
func (sl *SamplingLogger) Fatal(msg string, fields ...map[string]interface{}) {
	if sl.logger.level <= FATAL {
		entry := sl.logger.createEntry(FATAL, msg, fields...)
		if sl.sampler.ShouldSample(entry) {
			sl.logger.log(FATAL, msg, fields...)
		}
	}
}

// createEntry creates a log entry for sampling check
func (sl *StructuredLogger) createEntry(level LogLevel, msg string, fields ...map[string]interface{}) LogEntry {
	entry := LogEntry{
		Timestamp:   time.Now().UTC(),
		Level:       level.String(),
		Message:     msg,
		ServiceName: sl.serviceName,
		Version:     sl.version,
		Environment: sl.environment,
		Fields:      make(map[string]interface{}),
	}

	for k, v := range sl.fields {
		entry.Fields[k] = v
	}

	for _, f := range fields {
		for k, v := range f {
			entry.Fields[k] = v
		}
	}

	return entry
}

// AdaptiveSampler adapts sampling rate based on load
type AdaptiveSampler struct {
	mu               sync.Mutex
	baseRate         float64
	currentRate      float64
	maxRate          float64
	minRate          float64
	window           time.Duration
	logCount         int
	lastAdjustment   time.Time
	adjustmentFactor float64
}

// NewAdaptiveSampler creates a new adaptive sampler
func NewAdaptiveSampler(baseRate, minRate, maxRate float64, window time.Duration) *AdaptiveSampler {
	return &AdaptiveSampler{
		baseRate:         baseRate,
		currentRate:      baseRate,
		maxRate:          maxRate,
		minRate:          minRate,
		window:           window,
		lastAdjustment:   time.Now(),
		adjustmentFactor: 0.1,
	}
}

// ShouldSample determines if a log should be sampled
func (as *AdaptiveSampler) ShouldSample() bool {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.logCount++

	// Adjust rate periodically
	now := time.Now()
	if now.Sub(as.lastAdjustment) >= as.window {
		as.adjustRate()
		as.lastAdjustment = now
	}

	// Sample based on current rate
	h := fnv.New32a()
	h.Write([]byte(time.Now().String()))
	hash := h.Sum32()

	normalized := float64(hash%10000) / 10000.0

	return normalized < as.currentRate
}

// adjustRate adjusts the sampling rate based on log volume
func (as *AdaptiveSampler) adjustRate() {
	// If too many logs, decrease rate
	if as.logCount > 1000 {
		as.currentRate = as.currentRate * (1 - as.adjustmentFactor)
		if as.currentRate < as.minRate {
			as.currentRate = as.minRate
		}
	} else if as.logCount < 100 {
		// If too few logs, increase rate
		as.currentRate = as.currentRate * (1 + as.adjustmentFactor)
		if as.currentRate > as.maxRate {
			as.currentRate = as.maxRate
		}
	}

	as.logCount = 0
}

// GetStats returns adaptive sampler statistics
func (as *AdaptiveSampler) GetStats() map[string]interface{} {
	as.mu.Lock()
	defer as.mu.Unlock()

	return map[string]interface{}{
		"base_rate":       as.baseRate,
		"current_rate":    as.currentRate,
		"max_rate":        as.maxRate,
		"min_rate":        as.minRate,
		"window":          as.window.String(),
		"log_count":       as.logCount,
		"last_adjustment": as.lastAdjustment,
	}
}
