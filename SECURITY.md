# Security Policy

## Scanning Pipeline

CI runs automated security checks on every push and daily via `.github/workflows/security.yml`:

**Gosec** — Go source static analysis  
**Nancy + govulncheck** — Dependency CVE scanning  
**Trivy** — Filesystem & misconfiguration scanning  
**StaticCheck + Go-Critic** — Go linters  

Findings are uploaded to GitHub Code Scanning as SARIF.

## Secrets

**Never commit secrets.** The `config.yaml` in this repo contains development credentials only.

In production, load all secrets via environment variables or a secrets manager.

### At-Risk Fields in `config.yaml`

| Field | Risk |
|---|---|
| `auth.secret` | Full auth bypass |
| `postgres.connections[].password` | Database access |
| `encryption.key` | Data decryption |

### Production Checklist

- `app.env: production`, `debug: false`
- JWT/API-key auth enabled (`middleware.jwt: true`)
- Rate limiting and audit logging **on**
- CORS locked to known origins (no `*`)
- `sslmode: require` or `verify-full` on Postgres
- HSTS headers on (provided by `security` middleware)

## Reporting Vulnerabilities

Do **not** open a public issue.

- Open a **private advisory**: <https://github.com/diameter-tscd/stackyrd-nano/security/advisories/new>
- We aim to acknowledge within **7 business days** and patch within **90 days** for high/critical issues.
