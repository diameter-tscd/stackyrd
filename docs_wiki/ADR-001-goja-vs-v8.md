# ADR-001: goja JavaScript Runtime vs V8/Node.js

**Status:** Accepted  
**Date:** 2024  
**Decision maker:** Architecture team  
**Tags:** plugin, runtime, javascript

## Context

The plugin system needs to support TypeScript/JavaScript execution for user-defined transformations, validations, and data processing. The options considered were embedding a full JavaScript engine or using the existing Go-based goja runtime.

## Decision

Use [goja](https://github.com/dop251/goja) — a pure-Go JavaScript (ECMAScript 5.1+) runtime.

## Rationale

- **No CGO dependency:** V8 (via v8go) requires CGO and a V8 native binary, complicating cross-compilation and CI.
- **No external process:** Node.js subprocess would add latency, serialization overhead, and process management complexity.
- **Sufficient capability:** goja supports ES5.1+ with some ES6 features (arrow functions, Promises) — adequate for plugin use cases (data transformation, validation, aggregation).
- **Sandboxing:** goja runs in-process with full Go memory safety; we layer timeout and RSS limits on top.
- **Bundle size:** goja is ~5MB; V8 would add ~50MB+.

## Consequences

- Limited ES6+ support — plugins must be transpiled to ES5 (via esbuild, which we do at load time).
- No native Node.js APIs — plugins use our injected globals (`$infra`, `$logger`, `$args`, `$done`).
- Performance is adequate for I/O-bound plugin workloads (<1ms per invocation for typical transformations).
