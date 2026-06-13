---
name: stackyrd-dev
description: A comprehensive guide to extending the stackyrd Go/Gin framework. Use this whenever the user wants to add new features, API endpoints, HTTP middleware, database or external system integrations, or runtime-extensible plugin logic. This is the primary skill for all code generation and framework extension in stackyrd projects. Also use it when the user asks about the framework's architecture, auto-registration patterns, configuration conventions, boot order, or testing approach. If the user says anything about "adding", "creating", "scaffolding", "implementing", or "extending" in the context of this project, this skill should be loaded.
---

# stackyrd Development Guide

Use this guide to extend stackyrd at any of its four extension points: services, middleware, infrastructure components, and plugins. All four follow the same fundamental pattern: implement the interface, register via `init()`, and add a config toggle.

## Architecture Foundation

Understanding the boot order explains everything about how extensions wire themselves in:

```
main.go → config loading → Infrastructure async init → Dependencies populated →
Plugin init → Middleware auto-discovery → Service auto-discovery → Route registration → Server start
```

Every extension point uses Go's `init()` function for **auto-registration**. This means you never manually import or wire anything — you just create a file in the right directory with an `init()` that calls the package's registration function, and the framework picks it up.

Key convention: all auto-registered components default to **enabled** unless explicitly set to `false` in `config.yaml`.

## Decision Framework

Ask the user what they want to create:

| Goal | Create | Package | Interface | Reference |
|------|--------|---------|-----------|-----------|
| New API endpoints, business logic | Service | `internal/services/modules/` | `interfaces.Service` | `references/service.md` |
| HTTP request filtering, auth, logging | Middleware | `internal/middleware/` | `gin.HandlerFunc` (via factory) | `references/middleware.md` |
| External system integration (DB, queue, storage) | Infrastructure Component | `pkg/infrastructure/` | `InfrastructureComponent` | `references/infrastructure.md` |
| Runtime-extensible logic (TS, Lua, Python, Go) | Plugin | `pkg/plugin/builtin/{name}/` | Varies by runtime | `references/plugin.md` |

## Extension Point Templates

Each reference file contains:
- Interface definitions and requirements
- Complete code templates for the file structure
- Real examples from the existing codebase
- Registration pattern using `init()`
- Config toggle setup
- Testing guidance
- Edge cases and pitfalls

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
