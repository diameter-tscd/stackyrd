
<div align="center">
  <img src=".github/assets/stackyrd-banner.png" alt="stackyrd" style="width: 100%; max-width: 700px;"/>
</div>
<div align="center">
  <img src="https://www.shieldcn.dev/github/license/diameter-tscd/stackyrd.svg?variant=secondary&size=xs" alt="License"/>
  <img src="https://shieldcn.dev/badge/Go-00ADD8.svg?logo=go&logoColor=fff&variant=branded&size=xs" alt="Go Version"/>
  <img src="https://www.shieldcn.dev/github/ci/diameter-tscd/stackyrd.svg?variant=secondary&size=xs" alt="Build Status"/>
  <img src="https://shieldcn.dev/github/diameter-tscd/stackyrd/ci.svg?variant=secondary&size=xs&logo=lu%3AShield&label=Security+scan&name=security.yml" alt="Security Status"/>
  <img src="https://www.shieldcn.dev/github/release/diameter-tscd/stackyrd.svg?size=xs&variant=secondary" alt="Release"/>
  <img src="https://www.shieldcn.dev/badge/Agent--friendly-AGENTS.md-D97757.svg?variant=secondary&size=xs" alt="Agents Friendly"/>
</div>
<br>

**stackyrd** is an open-source, modular service framework for Go built on [Gin](https://github.com/gin-gonic/gin). It provides a layered architecture with auto-discovered services, middleware, infrastructure components, and a multi-language plugin system - so you can focus on business logic while the framework handles wiring, observability, and lifecycle.

### Core Architecture

| Layer | What it does |
|-------|-------------|
| **Services** | Business logic modules auto-registered via `init()`, toggled via config |
| **Middleware** | Pluggable HTTP middleware chain (JWT, CORS, rate-limit, audit, security headers) |
| **Infrastructure** | Managed clients for Redis, PostgreSQL, Kafka, MongoDB, MinIO, Grafana — with async init and health checks |
| **Plugins** | TypeScript (sandboxed goja), Lua (gopher-lua VM), Python (gRPC subprocess), or Go plugins callable from any service |
| **TUI / Console** | Interactive bubbletea dashboard or console fallback |

### What you can build

- **Microservices** with standardized routing, config, and observability out of the box
- **Data pipelines** with Kafka, batch processing, and cron scheduling
- **Multi-tenant APIs** with per-tenant Postgres/MongoDB connection management
- **Extensible platforms** where users upload TypeScript/Python scripts that run safely in sandboxed runtimes

## Quick Start

### Installation & Run

```bash
# Clone the repository
git clone https://github.com/diameter-tscd/stackyrd.git
cd stackyrd

# Install dependencies
go mod download

# Run the application
go run cmd/app/main.go

# To build the application
go run scripts/build/build.go

# To download package
go run scripts/pkg/pkg.go

```

## Preview

![Console](.github/assets/console.png)

## Documentation

- **[Full Documentation](docs_wiki/)** — Comprehensive guides and references
- **[Plugin System Guide](PLUGIN_GUIDE.md)** — Creating and managing TypeScript, Lua, Python, and Go plugins
- **[Contributing Guide](CONTRIBUTING.md)** — Development workflow and guidelines

## License

Distributed under the Apache License Version 2.0. See `LICENSE` for full information.
