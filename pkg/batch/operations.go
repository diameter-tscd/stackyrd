package batch

import (
	"context"
	"sync"
	"time"
)

// BatchConfig holds batch operation configuration
type BatchConfig struct {
	BatchSize     int
	Workers       int
	Timeout       time.Duration
	RetryAttempts int
	RetryDelay    time.Duration
}

// DefaultBatchConfig returns default batch configuration
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		BatchSize:     100,
		Workers:       4,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    100 * time.Millisecond,
	}
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Successful int
	Failed     int
	Errors     []error
	Duration   time.Duration
}

// BatchProcessor processes items in batches
type BatchProcessor[T any] struct {
	config  BatchConfig
	handler func(ctx context.Context, item T) error
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor[T any](config BatchConfig, handler func(ctx context.Context, item T) error) *BatchProcessor[T] {
	return &BatchProcessor[T]{
		config:  config,
		handler: handler,
	}
}

// Process processes items in batches
func (bp *BatchProcessor[T]) Process(ctx context.Context, items []T) (*BatchResult, error) {
	start := time.Now()
	result := &BatchResult{}

	if len(items) == 0 {
		return result, nil
	}

	// Create batches
	batches := bp.createBatches(items)

	// Process batches with workers
	resultChan := make(chan *BatchResult, len(batches))
	errorChan := make(chan error, len(batches))

	// Create worker pool
	workerCtx, cancel := context.WithTimeout(ctx, bp.config.Timeout)
	defer cancel()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, bp.config.Workers)

	for i, batch := range batches {
		wg.Add(1)
		go func(batchIndex int, batchItems []T) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			batchResult := bp.processBatch(workerCtx, batchItems)
			resultChan <- batchResult
		}(i, batch)
	}

	// Wait for all batches to complete
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Collect results
	for batchResult := range resultChan {
		result.Successful += batchResult.Successful
		result.Failed += batchResult.Failed
		result.Errors = append(result.Errors, batchResult.Errors...)
	}

	result.Duration = time.Since(start)

	return result, nil
}

// createBatches creates batches from items
func (bp *BatchProcessor[T]) createBatches(items []T) [][]T {
	var batches [][]T

	for i := 0; i < len(items); i += bp.config.BatchSize {
		end := i + bp.config.BatchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// processBatch processes a single batch
func (bp *BatchProcessor[T]) processBatch(ctx context.Context, items []T) *BatchResult {
	result := &BatchResult{}

	for _, item := range items {
		err := bp.processItemWithRetry(ctx, item)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err)
		} else {
			result.Successful++
		}
	}

	return result
}

// processItemWithRetry processes a single item with retry
func (bp *BatchProcessor[T]) processItemWithRetry(ctx context.Context, item T) error {
	var lastErr error

	for attempt := 0; attempt <= bp.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(bp.config.RetryDelay):
			}
		}

		err := bp.handler(ctx, item)
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return lastErr
}

// BatchWriter writes items in batches
type BatchWriter[T any] struct {
	config BatchConfig
	writer func(ctx context.Context, items []T) error
	items  []T
	mu     sync.Mutex
}

// NewBatchWriter creates a new batch writer
func NewBatchWriter[T any](config BatchConfig, writer func(ctx context.Context, items []T) error) *BatchWriter[T] {
	return &BatchWriter[T]{
		config: config,
		writer: writer,
	}
}

// Add adds an item to the batch
func (bw *BatchWriter[T]) Add(ctx context.Context, item T) error {
	bw.mu.Lock()
	bw.items = append(bw.items, item)
	shouldFlush := len(bw.items) >= bw.config.BatchSize
	bw.mu.Unlock()

	if shouldFlush {
		return bw.Flush(ctx)
	}

	return nil
}

// Flush flushes the batch
func (bw *BatchWriter[T]) Flush(ctx context.Context) error {
	bw.mu.Lock()
	if len(bw.items) == 0 {
		bw.mu.Unlock()
		return nil
	}

	items := make([]T, len(bw.items))
	copy(items, bw.items)
	bw.items = bw.items[:0]
	bw.mu.Unlock()

	return bw.writer(ctx, items)
}

// BatchReader reads items in batches
type BatchReader[T any] struct {
	config BatchConfig
	reader func(ctx context.Context, offset, limit int) ([]T, error)
}

// NewBatchReader creates a new batch reader
func NewBatchReader[T any](config BatchConfig, reader func(ctx context.Context, offset, limit int) ([]T, error)) *BatchReader[T] {
	return &BatchReader[T]{
		config: config,
		reader: reader,
	}
}

// ReadAll reads all items in batches
func (br *BatchReader[T]) ReadAll(ctx context.Context) ([]T, error) {
	var allItems []T
	offset := 0

	for {
		items, err := br.reader(ctx, offset, br.config.BatchSize)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, items...)

		if len(items) < br.config.BatchSize {
			break
		}

		offset += len(items)
	}

	return allItems, nil
}

// ReadBatch reads a single batch
func (br *BatchReader[T]) ReadBatch(ctx context.Context, offset int) ([]T, error) {
	return br.reader(ctx, offset, br.config.BatchSize)
}
