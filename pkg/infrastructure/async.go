package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// AsyncResult represents the result of an asynchronous operation
type AsyncResult[T any] struct {
	Value T
	Error error
	Done  chan struct{}
}

// NewAsyncResult creates a new async result
func NewAsyncResult[T any]() *AsyncResult[T] {
	return &AsyncResult[T]{
		Done: make(chan struct{}, 1), // buffered so a single CompleteResult signal never blocks
	}
}

// Complete marks the async operation as complete
func (r *AsyncResult[T]) Complete(value T, err error) {
	r.Value = value
	r.Error = err
	close(r.Done)
}

// Wait blocks until the operation is complete and returns the result
func (r *AsyncResult[T]) Wait() (T, error) {
	<-r.Done
	return r.Value, r.Error
}

// WaitWithTimeout waits for the operation with a timeout.
// Uses time.NewTimer so the underlying timer resource is always reclaimed
// via defer timer.Stop(), preventing a goroutine + FD leak per call.
func (r *AsyncResult[T]) WaitWithTimeout(timeout time.Duration) (T, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-r.Done:
		return r.Value, r.Error
	case <-timer.C:
		var zero T
		return zero, context.DeadlineExceeded
	}
}

// IsDone checks if the operation is complete without blocking
func (r *AsyncResult[T]) IsDone() bool {
	select {
	case <-r.Done:
		return true
	default:
		return false
	}
}

// AsyncOperation represents an operation that can be executed asynchronously
type AsyncOperation[T any] func(ctx context.Context) (T, error)

var asyncSemaphore = make(chan struct{}, 1000) // global cap on concurrent async goroutines

// ExecuteAsync executes an operation asynchronously and returns an AsyncResult
func ExecuteAsync[T any](ctx context.Context, operation AsyncOperation[T]) *AsyncResult[T] {
	result := NewAsyncResult[T]()

	asyncSemaphore <- struct{}{}
	go func() {
		defer func() { <-asyncSemaphore }()
		defer func() {
			if r := recover(); r != nil {
				// Handle panic in async operation
				result.Complete(*new(T), fmt.Errorf("async operation panicked: %v", r))
			}
		}()

		value, err := operation(ctx)
		result.Complete(value, err)
	}()

	return result
}

// BatchAsyncResult represents the result of a batch asynchronous operation
type BatchAsyncResult[T any] struct {
	Results   []AsyncResult[T]
	Done      chan struct{}
	batchSize int
	pending   int32 // number of results outstanding; CompleteResult is the sole completer
}

// NewBatchAsyncResult creates a new batch async result
func NewBatchAsyncResult[T any](count int, batchSize int) *BatchAsyncResult[T] {
	results := make([]AsyncResult[T], count)
	for i := range results {
		results[i] = *NewAsyncResult[T]()
	}

	return &BatchAsyncResult[T]{
		Results:   results,
		Done:      make(chan struct{}),
		batchSize: batchSize,
		pending:   int32(count),
	}
}

// Complete is removed: CompleteResult is the sole completer for BatchAsyncResult.
// Kept for callers that still reference it but no-ops to preserve backward compat.
func (br *BatchAsyncResult[T]) Complete() {}

// CompleteResult marks one individual operation result as done and, when all
// operations in the batch have completed, closes the batch Done channel.
// This is the sole completer for BatchAsyncResult; Close() delegates here.
func (br *BatchAsyncResult[T]) CompleteResult(index int) {
	br.Results[index].Complete(br.Results[index].Value, br.Results[index].Error)
	if atomic.AddInt32(&br.pending, -1) == 0 {
		close(br.Done)
	}
}

// WaitAll waits for all operations in the batch to complete
func (br *BatchAsyncResult[T]) WaitAll() ([]T, []error) {
	<-br.Done

	values := make([]T, len(br.Results))
	errors := make([]error, len(br.Results))

	for i, result := range br.Results {
		values[i], errors[i] = result.Wait()
	}

	return values, errors
}

// ExecuteBatchAsync executes multiple operations asynchronously, capping the
// number of concurrent goroutines at batchSize (waves).  A batchSize of 0 or
// less falls back to the default of 100.
func ExecuteBatchAsync[T any](ctx context.Context, operations []AsyncOperation[T], batchSize ...int) *BatchAsyncResult[T] {
	limit := 100 // safe default
	if len(batchSize) > 0 && batchSize[0] > 0 {
		limit = batchSize[0]
	}

	result := NewBatchAsyncResult[T](len(operations), limit)
	var wg sync.WaitGroup
	wg.Add(len(operations))

	sem := make(chan struct{}, limit)

	for i, operation := range operations {
		i, operation := i, operation // capture
		select {
		case sem <- struct{}{}: // acquire slot (blocks when limit is reached)
		case <-ctx.Done():
			result.Results[i].Error = ctx.Err()
			result.CompleteResult(i)
			wg.Done()
			continue
		}
		go func() {
			defer func() {
				<-sem // release slot
				wg.Done()
			}()
			defer func() {
				if r := recover(); r != nil {
					result.Results[i].Error = fmt.Errorf("batch operation panicked: %v", r)
					result.CompleteResult(i)
				}
			}()

			value, err := operation(ctx)
			result.Results[i].Value = value
			result.Results[i].Error = err
			result.CompleteResult(i)
		}()
	}

	return result
}

// WorkerPool manages a pool of goroutines for executing async operations
type WorkerPool struct {
	workers  int
	jobQueue chan func()
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		workers:  workers,
		jobQueue: make(chan func(), workers*2),
		stopChan: make(chan struct{}),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// Stop stops the worker pool, draining any queued jobs first.
func (wp *WorkerPool) Stop() {
	// Drain buffered jobs before signalling workers to stop so that Submit
	// never races with close (only Stop ever closes stopChan).
	for len(wp.jobQueue) > 0 {
		<-wp.jobQueue
	}
	close(wp.stopChan)
	wp.wg.Wait()
}

// Submit submits a job to the worker pool.  Blocks if the queue is full;
// call SubmitOrDrop for a non-blocking variant.
func (wp *WorkerPool) Submit(job func()) {
	wp.jobQueue <- job
}

// SubmitOrDrop attempts to submit a job without blocking.  Returns false
// if the queue is full and the job was dropped.
func (wp *WorkerPool) SubmitOrDrop(job func()) bool {
	select {
	case wp.jobQueue <- job:
		return true
	default:
		return false
	}
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			_ = r
		}
	}()

	for {
		select {
		case job := <-wp.jobQueue:
			job()
		case <-wp.stopChan:
			return
		}
	}
}

// Close closes the worker pool
func (wp *WorkerPool) Close() {
	wp.Stop()
	close(wp.jobQueue)
}
