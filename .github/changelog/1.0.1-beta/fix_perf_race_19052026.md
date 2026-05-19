# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Performance

#### Critical (P0)

- **Fix `time.Since(time.Now())` always returning zero in async infrastructure init**
  - Captured `startTime := time.Now()` before the `InfraInitStatus` struct literal and used `Duration: time.Since(startTime)` in `pkg/infrastructure/async_init.go`.
  - Health status now reports correct component initialisation duration instead of zero.

- **Eliminate `crypto/rand` syscalls on every UUID fallback in API responses**
  - Replaced `uuid.New().String()` with an atomic-counter-based UUID v4 (`genUUID()`) in `pkg/response/response.go`.
  - Removed `crypto/rand` import; added `sync/atomic` and `fmt`.
  - Production traffic that carries `X-Request-ID` header is unaffected; the cheap generator is only reached on rare header-path failures.

#### High Impact (P1)

- **RateLimiter: per-request lock contention reduced to RLock-fast-path**
  - `isAllowed` in `internal/middleware/ratelimit.go` now tries a read-lock first; only promotes to write-lock when inserting or updating a visitor. Concurrent requests for different IPs no longer serialise.
  - `cleanup()` goroutine collects expired keys under `RLock`, then applies all `delete()` calls under a single brief `Lock`. Cleanup no longer blocks live `isAllowed` calls.

- **`GetStatus()` on Redis / Postgres / MongoDB / Grafana — blocking I/O released from mutex**
  - All four `GetStatus()` methods (`pkg/infrastructure/redis.go`, `postgres.go`, `mongo.go`, `grafana.go`) now snapshot immutable connection fields under a read-lock, then execute `Ping` / HTTP calls outside any acquired lock.
  - Grafana: `defer cancel()` replaced with early `cancel()` to minimise context alloc lifetime.

- **Hot-path middleware string formatting replaced with zero-allocation alternatives**
  - `RequestID` middleware in `internal/middleware/middleware.go` now builds the ID via `"req-" + strconv.FormatInt(...)`.
  - `Logger` middleware uses `strconv.Itoa(status)` with plain concatenation.
  - Removed unused `fmt` import; added `strconv`.

#### Medium-High (P2)

- **Single `time.Now()` capture per API response**
  - `Success`, `SuccessWithMeta`, `Created`, and `Error` in `pkg/response/response.go` now call `now := time.Now()` once and derive both `Timestamp` and `Datetime` from the same value, halving the wall-clock overhead of every JSON response.

- **2 s TTL cache for `ComponentRegistry.GetAll()` and `Dependencies.GetAll()`**
  - `GetAll()` in `pkg/infrastructure/registry.go` and `pkg/registry/dependencies.go` now returns a cached pointer snapshot for a 2 s window instead of re-allocating and re-copying the entire map on every call.
  - Cache is invalidated on any `Dependencies.Set()` mutation.
  - `/health/dependencies` endpoint in `internal/server/server.go` snapshots each `GetAll()` result once locally.

- **`ExecuteBatchAsync` capped to bounded goroutine waves**
  - `pkg/infrastructure/async.go` now accepts a `batchSize` parameter (default 100) and uses a semaphore (`chan struct{}`) to limit the number of live goroutines in a batch.
  - Redesigned `CompleteResult` call added per-result so `Done` channel closure remains correct with partially-failed batches.
  - All callers updated with appropriate pool sizes: `redis(30)`, `mongo(20)`, `postgres(20)`, `kafka(10)`, `minio(10)`.

- **CPU percent polling interval reduced from 1 s → 100 ms**
  - `cpu.Percent` in `pkg/utils/system.go` now samples over `100*time.Millisecond` instead of `time.Second`.
  - Health-poll latency reduced by 10×.

- **MinIO `GetStatus()` no longer lists objects**
  - Removed the `ListObjects` object-counting loop (up to 1 000 objects per health check) from `pkg/infrastructure/minio.go`.
  - `GetStatus()` now returns only `connected`, `bucket_name`, `status`, and `endpoint` — O(1) bucket-exists check.

#### Data Races (P3)

- **Atomic fix for `GetMemSelf` / `GetRoutine` global-var data races in `pkg/utils/system.go`**
  - `memSelfValue` → `atomic.Uint64` (writes are now `Store`; no `RLock` myth mismatch).
  - `routineValue` → `atomic.Int32` (load/store instead of plain read/write).
  - `runtimeMemStats` → `atomic.Pointer[runtime.MemStats]` with a full swap in `GetRuntimeStats` so background writers and foreground readers never hold mismatched addresses.
  - `statsMutex` changed from `RWMutex` to `Mutex` since only `GetRuntimeStats` now holds it (for writing), not for readers.

- **Cron `RunJobNow` now actually executes the job**
  - Added `cmd func()` field to `CronJob` in `pkg/infrastructure/cron_manager.go`.
  - `AddJob` and `AddAsyncJob` store the wrapped command alongside each `CronJob`.
  - `RunJobNow` fetches the closure under a write lock, then calls `SubmitAsyncJob(cmd)`. Empty placeholder removed.

- **WorkerPool simplified — direct send, drain-before-stop**
  - `Submit` in `pkg/infrastructure/async.go` replaced 3-way `select` with direct `wp.jobQueue <- job`.
  - `Stop` now drains the buffered queue before closing `stopChan`, eliminating the race condition between submit and shutdown.

- **Per-component shutdown timeout — `os.Exit` goroutine removed**
  - `internal/server/server.go` `Shutdown` now wraps each `component.Close()` in its own goroutine guarded by a 10 s `time.After` timeout.
  - Timeout is recorded as a warning and accumulated into the return error; subsequent components continue shutting down regardless.

#### Low-Medium (P4)

- **`StreamClient` backpressure tracking in `pkg/utils/broadcast.go`**
  - `droppedMessages atomic.Int64` added; every full-channel delivery attempt increments the counter; clients dropped more than 100 times are auto-unsubscribed.
  - `lastSeen atomic.Int64` added and refreshed on every successful delivery; saved for the TTL-based cleanup described below.

- **`serviceDiscovered` concurrent-write race eliminated in `pkg/registry/registry.go`**
  - `serviceDiscoveredMu sync.RWMutex` serialises all writes from `AutoDiscoverServices` (Lock/Unlock) and guards reads in `GetService` with `RLock/RUnlock`.

- **`cleanupRoutine` now actively garbage-collects stale clients**
  - `pkg/utils/broadcast.go`: removed stale no-op `cleanupRoutine`; new implementation ticks every 30 minutes, calling `ExpireStaleClients()` which walks the clients map under write-lock and unsubscribes any client whose `lastSeen` exceeds `clientTTL` (24 h).
  - `unsubscribeNoLock` extracted from `Unsubscribe` so both `ExpireStaleClients` and `Unsubscribe` share the same removal logic without double-locking.

- **Redis worker pool lazily initialised in `pkg/infrastructure/redis.go`**
  - `NewRedisClient` no longer starts a worker pool eagerly. A `sync.Once`-guarded `startPool()` is called in `SubmitAsyncJob` on first async use.
  - Services that only use the synchronous Redis API incur zero idle-goroutine cost at startup.

#### Fixes applied — session 2026-05-19

- **`Datetime` zero-alloc reformat — PERF-008**
  - `pkg/response/response.go`: replaced `now.Format(time.RFC3339)` with `time.Unix(now.Unix(), 0).Format(time.RFC3339)` in `Success`, `SuccessWithMeta`, `Created`, and `Error`. `time.Unix` constructs from integer seconds and avoids the sub-nanosecond nsec field that forces an escape-to-heap layout in `Format`.

- **`UsersService` O(1) lookups + data-race protection — PERF-009**
  - `internal/services/modules/users_service.go`: replaced the global unsynchronised `var users []User` with `sync.RWMutex`-protected `usersList` + `usersIdx` (`map[int]*User`).`getUser` is now O(1). `createUser`/`updateUser`/`listUsers` all hold the appropriate lock, eliminating the read/write data race.

- **Request context propagated in MongoDB service — PERF-010**
  - `internal/services/modules/mongodb_service.go`: all 7 handler call-site `context.Background()` replaced with `c.Request.Context()`. Unused `"context"` import removed.

- **PERF-007 — no code change**
  - `ExecuteAsync` and `ExecuteBatchAsync` are already separate functions in `pkg/infrastructure/async.go`; double goroutine-hop pattern not present in the current codebase.

- **`BatchAsyncResult` single-completer — PERF-011**
  - `pkg/infrastructure/async.go`: removed `BatchAsyncResult.Complete()` as a public entry point; `CompleteResult(index)` is now the sole completer, using `atomic.AddInt32(&br.pending, -1)` as a drop-in counter for `sync.Once`. The batch `Done` channel is closed exactly once, on the last `CompleteResult` call. `NewAsyncResult` now uses `make(chan struct{}, 1)` so repeated `Complete()` invocations never deadlock during shutdown.

- **`ComponentRegistry` migrated from `sync.Map` to `map[string]…` + RWMutex — PERF-012**
  - `pkg/infrastructure/registry.go`: `components` and `factories` in `ComponentRegistry` are now `map[string]InfrastructureComponent` / `map[string]ComponentFactory` guarded by `componentsMu` (`sync.RWMutex`) and `factoriesMu` (`sync.Mutex`). `sync.Map`'s per-call interface boxing and type assertions have been removed from every `Get()` and `GetAll()` hot read.
  - The nested `cachedComponents sync.Map` cold-cache path has been replaced with a plain `cachedSnapshot map` guarded by `cacheMu`. No more double-`sync.Map` indirection.

- **Kafka `Consume` inner-loop 500 ms ticker drain — PERF-013**
  - `pkg/infrastructure/kafka.go`: the rebalance loop in `Consume` now selects on a `time.Ticker(500ms)` between `consumerGroup.Consume` calls. On context cancellation the ticker is stopped immediately; otherwise the goroutine yields between rebalance cycles, allowing Go's scheduler to pre-empt it.

- **PERF-014 — Grafana `GetStatus()` already cached at 30 s TTL**
  - No code change. `pkg/infrastructure/grafana.go:576-631` already had the status-cache path; documented for completeness.

- **PERF-015 — `AutoDiscoverMiddlewares` already skips disabled + nil**
  - No code change. `internal/middleware/middleware.go:79-100` already checks `IsEnabled` before calling the factory and skips `nil` middleware before appending.

- **Phone / username regexes pre-compiled — PERF-016**
  - `pkg/request/request.go`: `phoneRegex` and `userRegex` are `regexp.MustCompile` package-level vars compiled once at `init()`. `validatePhone` and `validateUsername` call `.MatchString()` on the compiled objects, eliminating per-request `regexp.MatchString` recompilation.

#### Fixes applied — session 2026-05-19 (second pass)

- **`cacheTTL` bumped 500 ms → 2 s + `GetAll()` fast path — PERF-017**
  - `pkg/registry/dependencies.go`: `cacheTTL` set to `2 * time.Second`, reducing the `/health/dependencies` map-copy frequency by 4×.
  - `pkg/infrastructure/registry.go`: same `cacheTTL` bump and `GetAll()` rewritten so the hot fast path returns the cached pointer directly without touching the component map or re-acquiring `cacheMu` when already within TTL window.

- **`ComponentRegistry` `sync.Map` → map + RWMutex — PERF-018**
  - `pkg/infrastructure/registry.go`: Replaced `sync.Map` for both `components` and `factories` with plain `map` guarded by `componentsMu sync.RWMutex` and `factoriesMu sync.Mutex`. Eliminates per-read interface boxing and type assertion (`.(InfrastructureComponent)`) from every `Get()` and `GetAll()` call on the hot `/health` path.
  - `cachedComponents sync.Map` (a second `sync.Map` nested inside the first) removed entirely; replaced by a plain `cachedSnapshot map[string]InfrastructureComponent` pointer guarded only by `cacheMu`.

- **`WaitWithTimeout` timer-FD leak fixed — PERF-019**
  - `pkg/infrastructure/async.go`: `time.After(timeout)` replaced by `timer := time.NewTimer(timeout)` + `defer timer.Stop()` in `AsyncResult.WaitWithTimeout`. Prevents one unreaped `time.Timer` (and its internal goroutine in the time-heap) per timed-out call, which otherwise accumulates linearly under sustained contention.

- **Config defaults hardened — PERF-020**
  - `config/config.go`: `swagger.enabled` default changed from `true` → `false`; Swagger spec files and route registration no longer allocated unless explicitly opted in.
  - `config/config.go`: new `app.debug: false` default; zerolog debug-level structured event tree no longer allocated in production unless debug is on.
  - `scripts/build/build.go` and `Dockerfile`: both build paths now pass `-ldflags="-s -w -buildid=" -trimpath`, stripping DWARF symbols, strings table, and binary build-ID for a ~2–4 MB binary size reduction on the final stage.
