
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

Stackyrd provides an enterprise-grade service fabric foundation for building robust and observable distributed systems in Go. Our goal is to bridge the gap between rapid development cycles and industrial-strength stability, making complex microservices architectures manageable from day one.

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

```

## Preview

![Console](.github/assets/console.png)

## Key Features

- **Modular Services**: Enable/disable services via configuration
- **Terminal UI**: Interactive boot sequence and live CLI dashboard
- **Infrastructure Support**: Redis, PostgreSQL (multi-tenant), Kafka, MinIO and many more at `stackyrd-pkg`
- **Security**: API encryption, authentication, and access controls
- **Build Tools**: Automated build scripts with backup and archiving with `build.go`

## Documentation

**[Full Documentation](docs_wiki/)** - Comprehensive guides and references

## License

Distributed under the Apache License Version 2.0. See `LICENSE` for full information.
