# Troubleshooting

## Common Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| Service not registered | `cfg.Services.IsEnabled(name)` false | Enable in `config.yaml` |
| Middleware missing | Not enabled | Enable `middleware.<name>: true` |
| Redis connection failed | `redis.enabled: false` | Set `redis.enabled: true` |
| Port already in use | Another instance running | Change `server.port` |

## Health Checks

- `GET /health` – overall status
- `GET /health/infrastructure` – component status
- `GET /health/dependencies` – registered components
- `GET /health/resources` – memory & goroutines

Run `go vet ./...` and `go test ./...` to validate.

## Tracing an exception

Every error carries `error.details.trace{trace_id,endpoint,method,path,handler,service,kind,code,constraint}` and `correlation_id == trace.trace_id` (`pkg/response/exception.go`).
`kind` is `sql|mongo|redis|kafka|validation|auth|http|panic|internal`; SQL maps `23505→409`, `23503/23502→400`.
Panics are rendered as JSON by `internal/middleware/recovery.go` (stack only when `app.debug: true`).
Grep one ID end to end: `grep <trace_id> logs/stackyrd.log` joins request log ↔ response ↔ DB error.
