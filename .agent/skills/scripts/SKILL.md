---
name: scripts
description: Reference docs for the stackyrd CLI subcommands in scripts/ — build, service, pkg, swagger, docker. Use when working on or invoking a specific yrd subcommand implementation.
---

# Stackyrd Scripts Reference

Deep reference docs for each `scripts/yrd` subcommand implementation in `scripts/internal/<name>/`. For CLI architecture (dispatcher, flags, project-root detection), see the `stackyrd-cli-dev` skill.

## Docs

- `BUILD_SCRIPT.md` — `yrd build` (compile, garble, UPX, backup → `dist/`)
- `SERVICE_SCRIPT.md` — `yrd service` (scaffold service from templates)
- `PKG_SCRIPT.md` — `yrd pkg` (install infra from GitHub index)
- `SWAGGER_SCRIPT.md` — `yrd swagger` (generate OpenAPI docs)
- `DOCKER_SCRIPT.md` — `yrd docker` (multi-stage Docker build, 10 targets)
