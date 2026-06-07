# ADR-002: gRPC Subprocess for External Plugins vs stdin/stdout

**Status:** Accepted  
**Date:** 2024  
**Decision maker:** Architecture team  
**Tags:** plugin, runtime, python, external

## Context

The plugin system needs to support languages beyond Go/TypeScript/Lua, particularly Python. The options were communicating via stdin/stdout (JSON lines protocol) or using a gRPC subprocess model.

## Decision

Use **gRPC over Unix domain sockets** for external plugin communication.

## Rationale

- **Structured messages:** Protobuf provides typed schemas, avoiding ad-hoc JSON parsing and mismatched field types.
- **Bidirectional streaming:** gRPC enables future use cases (streaming results, health probes, event feeds).
- **Error handling:** gRPC error codes and status details are richer than JSON error strings.
- **Language support:** gRPC has first-class support for Python, Java, Rust, C#, etc.
- **Performance:** Protobuf serialization is faster than JSON for large payloads; Unix domain sockets avoid TCP overhead.
- **Process lifecycle:** The subprocess model provides clean isolation — crashes don't affect the main process.

## Consequences

- Each external plugin runs as a separate OS process with gRPC server overhead.
- Requires the gRPC Python package as a dependency for Python plugins.
- Startup latency (~100-300ms) for Python subprocess gRPC server initialization.
- We provide a base `host.py` that implements the gRPC server and plugin interface.
