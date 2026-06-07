# ADR-003: Gopher-Lua vs C-Bound Lua 5.x

**Status:** Accepted  
**Date:** 2024  
**Decision maker:** Architecture team  
**Tags:** plugin, runtime, lua

## Context

The plugin system needs to support Lua scripting for lightweight transformations and logic. The options were using CGO-bound Lua 5.1/5.4 or the pure-Go [gopher-lua](https://github.com/yuin/gopher-lua).

## Decision

Use **gopher-lua** — a pure-Go Lua 5.1 VM.

## Rationale

- **No CGO:** Lua 5.x C bindings require CGO and platform-specific compiled libraries, complicating cross-compilation.
- **Sandboxing:** gopher-lua allows per-VM resource limits; we can control execution depth, memory, and available standard library functions.
- **Go integration:** Lua functions can call Go functions directly via `L.SetField`, `L.SetGlobal`, etc.
- **Sufficient capability:** Lua 5.1 covers all required plugin use cases (data transformation, conditional logic, string processing).
- **Bundle size:** gopher-lua is ~2MB vs ~10MB for Lua 5.x with C bindings.

## Consequences

- Lua 5.2+ features (goto, bit32 library, etc.) are not available.
- Standard Lua C libraries (io, os, debug) are restricted in sandbox mode.
- Performance is adequate for transformation workloads (<100µs per invocation for typical scripts).
