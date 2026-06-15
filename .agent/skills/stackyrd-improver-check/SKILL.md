---
name: stackyrd-improver-check
description: >
  Analyze the stackyrd Go/Gin framework codebase for performance bottlenecks,
  code issues, enhancement opportunities, and native dependency risks. Use this
  skill whenever a user asks about improving stackyrd — whether they mention
  profiling, optimization, code review, bug hunting, dependency audit, security
  hardening, or architectural improvements. Also trigger when the user asks
  "what's wrong with", "how can we improve", "is this production-ready", "audit
  the codebase", "check for issues", "performance review", or similar diagnostic
  or evaluative questions about the stackyrd project. This skill is the go-to
  for any analysis, review, or improvement-planning task.
---

# stackyrd Improver & Check

Analyze the stackyrd codebase across four dimensions: performance, code issues, enhancements, and native dependency risks. Each analysis produces a structured report with findings ranked by severity.

## Analysis Framework

When asked to analyze or improve stackyrd, run the four checks below in sequence. Each check produces a section in the final report.

### 1. Performance Analysis

Look for these patterns systematically. Use grep, glob, and read to search the codebase.

**Context leak risks** — `context.Background()` used where a timeout or deadline context should be used instead:
- Health check pings without timeouts
- Database disconnect calls
- External API calls (Grafana, MinIO, Kafka)
- Plugin execution contexts
- Startup/shutdown paths
- Search: `context\.Background\(\)` in production code (exclude test files)

**Timer leaks** — `time.After()` in `select` statements without cleanup:
- `time.After()` inside loops or retry logic
- Shutdown wait timers
- Any `case <-time.After(` pattern that doesn't store and stop the timer
- Search: `time\.After\(` across the codebase

**Unbuffered or undersized channels** — channels that can block producers:
- Broadcast hubs with unbuffered channels
- Any channel in a hot path with buffer size 0
- Search: `make(chan` and check buffer sizes

**Goroutine multiplication without bounds** — unbounded goroutine spawning:
- Plugin execution spawning goroutines per call
- WebSocket connection handling
- Webhook event processing without semaphore caps
- Any `go ` call not bounded by a worker pool or semaphore
- Search: `^go ` (preceded by newline) and `go func`

**Missing connection pool tuning** — DB clients without pool configuration:
- Review `pkg/infrastructure/postgres.go`, `mongo.go`, `redis.go`
- Check `SetMaxOpenConns`, `SetMaxIdleConns`, `PoolSize` etc.
- Flag any infrastructure component that opens connections without pool limits

**Pooling gaps** — runtimes created per-execution without reuse:
- goja VM pool (check `vmPool` usage in `pkg/plugin/runtime.go`)
- gopher-lua VM — is it pooled?
- wazero runtime — is it pooled?
- esbuild transpiler — is caching used?
- Search: each runtime type to check pooling pattern

**Rate limiting gaps** — missing request throttles:
- Global rate limiter for all incoming HTTP requests
- Plugin execution management endpoints
- Any public endpoint without rate limiting
- Check: `internal/middleware/ratelimit.go`, `pkg/plugin/gin.go`

**Missing distributed tracing** — no OpenTelemetry or tracing instrumentation:
- No trace propagation across service boundaries
- No span creation in middleware or handlers
- Check: any `otel` or `trace` imports in the codebase

**Missing benchmarks** — performance-critical code without benchmark tests:
- Database operations (Redis, PostgreSQL, MongoDB, Kafka)
- Plugin execution (goja, gopher-lua, wazero)
- WebSocket messaging throughput
- Check: `tests/` directory for benchmark functions
- Search: `Benchmark` in test files

**`time.After()` in select without storing the Timer** — the canonical Go leak:
- `case <-time.After(d):` inside `for` or `select` blocks
- Each call creates a Timer that's only GC'd after firing
- High-frequency paths compound this into significant memory pressure
- Search: `time\.After\(` in `for`/`select` patterns

**`sync.Pool` usage review** — what's pooled and what's not:
- Check existing pools (buffer pools, VM pools)
- Identify hot allocation paths without pooling
- Recommend pooling for hot-path `[]byte`, struct, or channel allocations
- Search: `sync\.Pool` usage

**Missing `context.Context` propagation** — functions accepting context but not threading it through:
- Check DB operation signatures
- Check HTTP handler signatures
- Anywhere a context is created but not passed to downstream calls
- Search: `context\.Background\(\)` and `context\.TODO\(\)` in non-test code

### 2. Code Issue Analysis

**Error swallowing** — discarded return values:
- `_ = ` assignments in defer blocks
- Ignored errors from close operations, flush operations, decode/encode calls
- Search: `_\s*=` patterns in non-test code, especially in `defer` blocks

**Two HTTP routers** — importing a second framework unnecessarily:
- `labstack/echo/v4` is imported only for WebSocket handler
- Check if gin's own WebSocket support could replace it
- Check: `pkg/websocket/handler.go` imports

**Context.Background() in production paths** — specifically flag:
- Health check endpoints (should have timeouts)
- Plugin execution (should respect caller context with deadline)
- Database close/disconnect calls (can hang)
- Search: `context\.Background\(\)` in non-test `.go` files

**Missing graceful error handling** — panics in non-recovery paths:
- Map access without existence check
- Type assertions without ok check
- Slice bounds without length check
- Search for type assertions `\.\(` without `ok` pattern

**Config struct issues** — sharing the same mapstructure tag:
- `PostgresMultiConfig` and `PostgresConfig` both tag `mapstructure:"postgres"`
- `MongoMultiConfig` and `MongoConfig` both tag `mapstructure:"mongo"`
- Can cause silent config shadowing
- Check: `config/config.go` for tag collisions

**Incomplete test coverage** — areas without tests:
- No infrastructure component benchmark tests
- No plugin execution load tests
- No WebSocket/Webhook throughput tests
- Check: `tests/` directory structure and test files

**Startup/shutdown ordering issues** — shutdown safety:
- Goroutine spawned per dependency during shutdown (can race)
- No guaranteed ordering
- Context.Background() used instead of WithTimeout
- Check: `server.go` Shutdown() method

**goroutine safety in shared state** — structures modified from multiple goroutines without synchronization:
- Check if maps are accessed without `sync.RWMutex` or `sync.Map`
- Check for slice mutations from goroutines
- Check for shared struct field writes without atomics or mutex
- Search for `map\[` access patterns, unprotected `append`, unprotected field writes

### 3. Enhancement Ideas

Based on the findings from (1) and (2), recommend improvements. Prioritize by impact.

**High impact — implement first:**
- Add timeouts to all `context.Background()` calls in health check paths
- Fix `time.After()` leaks by storing and stopping timers
- Replace Echo WebSocket with Gin-native or simpler gorilla/websocket usage
- Add `otelhttp` instrumentation to Gin middleware
- Benchmark and pool Lua/WASM runtimes

**Medium impact:**
- Fix mapstructure tag collision for multi-connection configs
- Add infrastructure component benchmark tests
- Implement global rate limiter
- Address unbuffered broadcast channel in WebSocket hub
- Add `sync.Pool` for hot-path allocations in response serialization

**Low impact / nice to have:**
- Remove duplicate Echo dependency
- Add graceful startup ordering with dependency graph
- Add OpenTelemetry spans to middleware chain
- Documentation for shutdown guarantees

### 4. Native Dependency Check

Systematically verify every dependency for CGO requirements or platform-specific behavior.

**CGO dependency scan:**
- Search for `import "C"` — direct CGO usage
- Search for `// #cgo` directives — CGO build tags
- Search for `CGO_ENABLED` or `cgo` in build scripts, Makefiles, CI configs
- Check `go.mod` for packages known to potentially need CGO:
  - `mattn/go-sqlite3` — requires CGO (not currently in deps, flag if added)
  - `bytedance/sonic` — uses unsafe assembly, CGO-free but platform-sensitive
  - Any `gocv`, `opencv`, `sqlite`, `oracle` drivers

**Architecture-specific code:**
- Check `cloudwego` packages (base64x, iasm) — x86-64 assembly, may fail on ARM in some configurations
- Search for `//go:build` or `// +build` tags
- Check for `.s` (assembly) files
- Search: `_amd64\.go`, `_arm64\.go`, `_s390x\.go` patterns

**Cross-compilation concerns:**
- Check that `CGO_ENABLED=0` builds succeed (verify build script flags)
- Check that the Dockerfile doesn't install C dependencies
- Check CI workflows for cross-platform build matrix
- Search: `CGO_ENABLED` in shell scripts, Dockerfiles, CI YAML

**C library dependencies at runtime:**
- Check if any linked binary depends on system libraries:
  - `ldd dist/stackyrd` or equivalent on macOS `otool -L`
  - Look for `.so` or `.dylib` references
  - Check for `pkg-config` usage in build scripts

**Platform-specific test matrix:**
- Check `.github/workflows/` for OS/arch coverage
- Identify gaps (e.g., only linux/amd64, missing darwin/windows/arm64)
- Recommend CI expansion if limited

## Report Format

Always produce the analysis as a structured markdown report:

```markdown
# stackyrd Analysis Report

## Summary
<Brief overview of findings, 2-3 sentences>

## Performance Issues
### Critical (fix immediately)
- [ ] Finding 1 with code location, impact, and fix suggestion
- [ ] Finding 2 ...

### Warning (should address)
- [ ] Finding 3 ...

### Info (worth noting)
- [ ] Finding 4 ...

## Code Issues
### Critical
- [ ] ...

### Warning
- [ ] ...

### Info
- [ ] ...

## Enhancement Ideas
### High Impact
- [ ] ...

### Medium Impact
- [ ] ...

## Native Dependency Risk
### Critical
- [ ] ...

### Warning
- [ ] ...

### Info
- [ ] ...

## Methodology
<How the analysis was performed — what was searched, what files were checked>
```

Each finding should include:
- **Location**: file path and line number
- **What**: the specific code pattern found
- **Impact**: why it matters (real-world consequence)
- **Fix**: specific, actionable suggestion with code snippet if applicable

## Checking Methodology

Always verify findings by reading the actual code at the reported locations. Don't make assumptions based on grep results alone — check context, imports, and surrounding logic.

For dependency checks, prefer reading `go.sum` / `go.mod` and checking each package's documentation rather than relying on memory about what requires CGO.

For benchmark/performance claims, check whether the code path is actually hot (called per-request) or cold (called at startup) — not all inefficiencies matter equally.
