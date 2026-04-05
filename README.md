
<div align="center">
  <img src=".assets/stackyrd_logo.PNG" alt="stackyrd" style="width: 50%; max-width: 400px;"/>
</div>
<div align="center">
  <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License"/>
  <img src="https://img.shields.io/badge/go-1.21%2B-00ADD8.svg" alt="Go Version"/>
  <img src="https://img.shields.io/badge/build-passing-brightgreen.svg" alt="Build Status"/>
  <img src="https://img.shields.io/badge/github-diameter--tscd/stackyrd-181717.svg" alt="GitHub Repo"/>
</div>
<br>

stackyrd is a lightweight, production-ready application framework featuring modular architecture, comprehensive monitoring, real-time dashboards, and extensive infrastructure integrations. Built for scalability and ease of deployment.

## Quick Start

### Prerequisites
- Go 1.21+

### Installation & Run

```bash
# Clone the repository
git clone https://github.com/diameter-tscd/stackyrd.git
cd stackyrd

# Install dependencies
go mod download

# Run the application
go run cmd/app/main.go
```

**First Access:**
1. Open `http://localhost:9090` (monitoring dashboard)
2. Login with password: `admin`
3. **Important**: Change the default password immediately!

## Screenshots

### Backend Console
![Backend Console](.assets/Recording_Console.gif)

### Monitoring Dashboard
![Monitoring Dashboard](.assets/Recording_Dashboard.gif)

## Key Features

- **Modular Services**: Enable/disable services via configuration
- **Monitoring Dashboard**: Real-time metrics, logs, and system monitoring
- **Terminal UI**: Interactive boot sequence and live CLI dashboard
- **Infrastructure Support**: Redis, PostgreSQL (multi-tenant), Kafka, MinIO
- **Security**: API encryption, authentication, and access controls
- **Build Tools**: Automated build scripts with backup and archiving

## Documentation

**[Full Documentation](docs_wiki/)** - Comprehensive guides and references

## Project Structure

```
stackyrd/
├── .github/                 # GitHub Actions CI/CD workflows
│   └── workflows/          # Automated testing and deployment
├── cmd/                     # Application entry points
│   └── app/                # Main application executable
├── config/                  # Configuration management
├── docs_wiki/              # Comprehensive project documentation
│   └── blueprint/          # Project architecture analysis
├── internal/                # Private application packages
│   ├── middleware/         # HTTP middleware (auth, security)
│   ├── monitoring/         # Web monitoring dashboard backend
│   ├── server/             # HTTP server setup and routing
│   └── services/           # Modular business services
│       └── modules/        # Individual service implementations
├── pkg/                    # Public reusable packages
│   ├── infrastructure/     # External service integrations
│   ├── logger/             # Structured logging utilities
│   ├── request/            # Request validation and binding
│   ├── response/           # Standardized API responses
│   ├── tui/                # Terminal User Interface components
│   └── utils/              # General utility functions
├── scripts/                # Build and utility scripts
└── web/                    # Web interface assets
    └── monitoring/         # Monitoring dashboard frontend
        └── assets/         # Static web assets
            ├── css/        # Stylesheets
            └── js/         # JavaScript files
```

## License

Apache License Version 2.0: [LICENSE](LICENSE)

---

**Built using Go, Gin, Alpine.js, Tailwind CSS**
