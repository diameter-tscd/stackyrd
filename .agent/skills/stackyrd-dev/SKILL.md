---
name: stackyrd-dev
description: A comprehensive guide to extending the stackyrd Go/Gin framework. Use this whenever the user wants to add new features, API endpoints, HTTP middleware, database or external system integrations, or runtime-extensible plugin logic. This is the primary skill for all code generation and framework extension in stackyrd projects. Also use it when the user asks about the framework's architecture, auto-registration patterns, configuration conventions, boot order, or testing approach. If the user says anything about "adding", "creating", "scaffolding", "implementing", or "extending" in the context of this project, this skill should be loaded.
---

# stackyrd Development Guide

**Before generating any code, first load and apply the [ponytail](../ponytail/SKILL.md) skill.** The ponytail ladder governs all code generation here: YAGNI first, stdlib before custom, existing deps before new ones, one line over fifty, shortest diff wins.

Use this guide to extend stackyrd at its four extension points. But before reaching for any template, apply the ladder:
1. Does this need to exist at all? If speculative, say so and stop.
2. Can stdlib or an existing dependency cover it?
3. Is the simplest thing (one file, no new interface) enough?
4. Only then: use the minimum viable pattern below.

## Architecture Foundation

The boot order:

```
main.go → config loading → Infrastructure async init → Dependencies populated →
Plugin init → Middleware auto-discovery → Service auto-discovery → Route registration → Server start
```

Every extension point uses Go's `init()` for auto-registration. Create a file in the right directory with an `init()` that calls the package's registration function. All components default to **enabled** unless `false` in `config.yaml`.

## Decision Framework

Ask the user what they want to create, then check ponytail before scaffolding:

| Goal | Create | Package | Interface | Reference |
|------|--------|---------|-----------|-----------|
| New API endpoints, business logic | Service | `internal/services/modules/` | `interfaces.Service` | `references/service.md` |
| HTTP request filtering, auth, logging | Middleware | `internal/middleware/` | `gin.HandlerFunc` (via factory) | `references/middleware.md` |
| External system integration (DB, queue, storage) | Infrastructure Component | `pkg/infrastructure/` | `InfrastructureComponent` | `references/infrastructure.md` |
| Runtime-extensible logic (TS, Lua, Python, Go) | Plugin | `pkg/plugin/builtin/{name}/` | Varies by runtime | `references/plugin.md` |

## Extension Point Templates

Each reference file contains interface defs, code templates, registration pattern, config toggle, and testing guidance.

## Scripts

| Script | Run | Notes |
|--------|-----|-------|
| **Build** | `go run ./scripts/build/` | Compiles app, optional garble/UPX. Package mode only (`./scripts/build/`, not `build.go` alone). |
| **Service Generator** | `go run ./scripts/service/` | Scaffolds new services. Both `./scripts/service/` and `service.go` work. |
| **Swagger** | `go run scripts/swagger/swagger.go` | Regenerates API docs. |

Both build and service scripts use the same bubbletea TUI pattern with ttyGuard fallback (see `TUI_INFO_PATTERN.md` at project root). Common flags: `--no-tui` for CI, `--verbose` for debug.

## General Conventions

| Convention | Rule |
|---|---|
| **Package naming** | Services: `package modules`; Middleware: `package middleware`; Infrastructure: `package infrastructure` |
| **File naming** | Services: `{name}_service.go`; Middleware: `{name}.go`; Infrastructure: `{name}.go` |
| **Test files** | `tests/services/{name}_service_test.go`, `tests/infrastructure/{name}_test.go` |
| **Config naming** | Use underscore_case matching the WireName for config keys |
| **init() registration** | Always call the package's registration function (e.g., `registry.RegisterService`, `RegisterMiddleware`, `RegisterComponent`) |
| **Logger** | Always accept `*logger.Logger` and use structured key-value pairs for all log calls |
| **Response helpers** | Use `pkg/response` helpers: `Success`, `Created`, `BadRequest`, `NotFound`, `Error`, `ValidationError` |
| **Request binding** | Use `pkg/request.Bind()` for validation with field error support |
| **Error handling** | Return structured error responses using `response.Error` with machine-readable error codes |

## Common Abstractions

### The `response` package (`pkg/response`)

All HTTP responses go through standardized helpers. Key functions: `Success(c, data, msg)`, `Created(c, data, msg)`, `BadRequest(c, msg)`, `NotFound(c, msg)`, `Error(c, status, code, msg, details)`, `ValidationError(c, msg, fields)`, `SuccessWithMeta(c, data, meta, msg)`.

### The `request` package (`pkg/request`)

Use `request.Bind(c, &target)` instead of `c.ShouldBindJSON`. It returns typed `*request.ValidationError` with per-field error details that `response.ValidationError` can render.

### The `Dependencies` bag (`pkg/registry/dependencies.go`)

Services receive a `*registry.Dependencies` container populated with all infrastructure components and the PluginBridge. Access them in the service factory: `deps.Get("component_name")` returns `(interface{}, bool)`.
