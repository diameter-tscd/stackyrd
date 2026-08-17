# Architecture

The framework is built around three core concepts:

1. **Services** – business logic, registered via `registry.RegisterService` or `RegisterServiceWithDeps`.
2. **Infrastructure** – db, redis, kafka, etc., installed in `pkg/infrastructure`.
3. **Middleware** – request/response interceptors registered in `internal/middleware`.

All components are auto‑discovered at boot and wired by the `registry` package.

## Boot Order

```
parseFlags → loadConfig → initInfra → registerMiddlewares → registerServices → startServer
```

## Request Flow

```
Client → Middleware Chain → Service Handler → Response
```

Core APIs are kept minimal; consult the individual package docs for details.
