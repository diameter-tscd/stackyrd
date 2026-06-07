# Batch Processing

Generic batch processing with worker pools, accumulation flushing, and paginated reading.

## Overview

The `pkg/batch/` package provides three patterns for processing data in batches:

| Pattern | Struct | Use Case |
|---------|--------|----------|
| Processor | `BatchProcessor[T]` | Split items into batches, process with worker pool |
| Writer | `BatchWriter[T]` | Accumulate items, auto-flush on batch size |
| Reader | `BatchReader[T]` | Paginated reading of large datasets |

## BatchProcessor

Processes a slice of items in parallel batches.

```go
import "github.com/diameter-tscd/stackyrd/pkg/batch"

handler := func(ctx context.Context, items []Item) error {
    // Process batch (e.g., bulk insert to DB)
    return db.BulkInsert(items)
}

processor := batch.NewBatchProcessor(
    batch.DefaultBatchConfig(),
    handler,
)

result, err := processor.Process(ctx, items)
// result.Successful, result.Failed, result.Errors, result.Duration
```

### Custom Configuration

```go
processor := batch.NewBatchProcessor(batch.BatchConfig{
    BatchSize:     50,              // items per batch
    Workers:       8,               // concurrent workers
    Timeout:       60 * time.Second,
    RetryAttempts: 2,
    RetryDelay:    100 * time.Millisecond,
}, handler)
```

### Error Handling

```go
result, err := processor.Process(ctx, items)
if err != nil {
    log.Printf("batch processing failed: %v", err)
}
log.Printf("successful: %d, failed: %d", result.Successful, result.Failed)

for _, batchErr := range result.Errors {
    log.Printf("item error: %v", batchErr)
}
```

## BatchWriter

Accumulates items and flushes them when the batch size is reached.

```go
writer := batch.NewBatchWriter(
    batch.DefaultBatchConfig(),
    func(ctx context.Context, items []Item) error {
        return db.BulkInsert(items)
    },
)

for _, item := range incomingStream {
    writer.Add(ctx, item) // auto-flushes at BatchSize
}

// Flush remaining items
writer.Flush(ctx)
```

### Use Cases

- **Log aggregation**: buffer log entries, flush to storage
- **Event streaming**: batch Kafka events before processing
- **Database writes**: bulk insert accumulated records
- **Metrics reporting**: batch publish metrics

## BatchReader

Reads all items from a paginated source.

```go
reader := batch.NewBatchReader(
    batch.DefaultBatchConfig(),
    func(ctx context.Context, offset, limit int) ([]Item, error) {
        return db.Query("SELECT * FROM items OFFSET ? LIMIT ?", offset, limit)
    },
)

allItems, err := reader.ReadAll(ctx) // reads all pages

// Or read one batch at a time
batch, err := reader.ReadBatch(ctx, 0)
```

### Use Cases

- **Data migration**: read all items from source, process in batches
- **Export jobs**: paginate through large datasets
- **ETL pipelines**: extract-transform-load with page-by-page reading

## BatchResult

```go
type BatchResult struct {
    Successful int             // items processed successfully
    Failed     int             // items that failed
    Errors     []error         // per-item errors
    Duration   time.Duration   // total processing time
}
```

## Best Practices

- Match `BatchSize` to the downstream capacity (e.g., DB max insert params)
- Set `Workers` to `runtime.NumCPU()` or your I/O concurrency limit
- Use `BatchWriter` for streaming/tail data, `BatchProcessor` for finite slices
- Always handle `BatchResult.Errors` — partial failures are common
- Monitor `batch_duration_seconds` metric for performance regressions
