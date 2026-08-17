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
