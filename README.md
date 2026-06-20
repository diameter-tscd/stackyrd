
<div align="center">
  <img src=".github/assets/stackyrd-banner.png" alt="stackyrd-nano" style="width: 100%; max-width: 700px;"/>
</div>
<div align="center">
  <img src="https://www.shieldcn.dev/github/license/diameter-tscd/stackyrd-nano.svg?variant=secondary&size=xs" alt="License"/>
  <img src="https://www.shieldcn.dev/badge/Go-00ADD8.svg?logo=go&logoColor=fff&variant=branded&size=xs" alt="Go Version"/>
  <img src="https://www.shieldcn.dev/github/ci/diameter-tscd/stackyrd-nano.svg?variant=secondary&size=xs" alt="Build Status"/>
  <img src="https://www.shieldcn.dev/github/diameter-tscd/stackyrd-nano/ci.svg?variant=secondary&size=xs&logo=lu%3AShield&label=Security+scan&name=security.yml" alt="Security Status"/>
  <img src="https://www.shieldcn.dev/github/release/diameter-tscd/stackyrd-nano.svg?size=xs&variant=secondary" alt="Release"/>
  <img src="https://www.shieldcn.dev/badge/Agent--friendly-AGENTS.md-D97757.svg?variant=secondary&size=xs" alt="Agents Friendly"/>
</div>
<br>

**stackyrd-nano** is a lightweight, modular service framework for Go built on [Gin](https://github.com/gin-gonic/gin). It provides auto-discovered services, middleware, and infrastructure components — so you can focus on business logic while the framework handles wiring and lifecycle.

### Core Architecture

| Layer | What it does |
|-------|-------------|
| **Services** | Business logic modules auto-registered via `init()`, toggled via config |
| **Middleware** | Pluggable HTTP middleware chain (JWT, CORS, rate-limit, audit, security headers) |
| **Infrastructure** | Managed clients for PostgreSQL with async init and health checks |
| **TUI / Console** | Interactive bubbletea dashboard or console fallback |

### What you can build

- **Microservices** with standardized routing, config, and observability out of the box
- **REST APIs** with cursor-based pagination, request validation, structured responses
- **Real-time features** with built-in WebSocket hub
- **Fault-tolerant services** with circuit breakers, retry, and timeout patterns

## Quick Start

### Installation & Run

```bash
# Clone the repository
git clone https://github.com/diameter-tscd/stackyrd-nano.git
cd stackyrd-nano

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
- **[Contributing Guide](CONTRIBUTING.md)** — Development workflow and guidelines

## License

Distributed under the Apache License Version 2.0. See `LICENSE` for full information.
