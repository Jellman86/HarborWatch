# HarborWatch

HarborWatch is a local-first container maintenance and security platform for self-hosted Docker environments.

## Milestones 0-4 Status

This repository currently includes:
- Go backend server with `GET /health`
- Svelte 5 frontend with Docker inventory and vulnerability views
- Static web serving through the Go server
- OpenAPI contract at `api/openapi.yaml`
- Generated shared types for Go and TypeScript
- Docker inventory APIs (`/api/docker/containers`, `/api/docker/images`) and live event stream (`/api/docker/events`)
- Vulnerability scan workflow:
  - `POST /api/scans/run` (async scan job)
  - `GET /api/scans/jobs/{id}` (job status)
  - `GET /api/scans/summary` (latest persisted summary in SQLite)
- Release note intelligence:
  - `GET /api/releases/summary?repo=owner/name` (GitHub release heuristic risk + cited excerpts)
- Safe update pipeline:
  - `POST /api/updates/run` (async update run)
  - `GET /api/updates/jobs/{id}` (persisted run + steps)
  - `GET /api/updates/events/{id}` (live SSE step progress)

## Quick Start

1. Generate shared API types:

```bash
scripts/generate-types.sh
```

2. Start backend server:

```bash
scripts/dev.sh
```

3. In a second terminal, start frontend dev server:

```bash
cd web
npm install
npm run dev
```

## Verification

```bash
scripts/test.sh
```

## Notes

- Trivy scanning requires the `trivy` binary to be installed and available on `PATH`.
- Scan results are persisted in SQLite at `/tmp/harborwatch.db` by default (override with `HARBORWATCH_DB_PATH`).
- Docker inventory/update features require Docker socket access (`/var/run/docker.sock`) in containerized deployment.

## CI Image Builds (GitHub)

- Workflow: `.github/workflows/build-and-push.yml`
- PR checks workflow: `.github/workflows/pr-validation.yml`
- Push to `dev`:
  - Runs backend/frontend tests
  - Builds and pushes `ghcr.io/<owner>/harborwatch:dev`
- Push tag `v*` (for example `v0.2.0`):
  - Builds and pushes `ghcr.io/<owner>/harborwatch:v0.2.0`
  - Also updates `ghcr.io/<owner>/harborwatch:latest`
- Pull requests to `main` or `dev`:
  - Run backend tests
  - Build frontend
  - Build Docker image without pushing

## Compose Deployments

- `docker-compose.dev.yml`: runs `ghcr.io/jellman86/harborwatch:dev`
- `docker-compose.prod.yml`: runs `ghcr.io/jellman86/harborwatch:latest`
- `docker-compose.yml`: default latest image compose
- Default host port is `18080` to avoid conflicts with existing services on this environment.
- Override with `HARBORWATCH_PORT`, for example `HARBORWATCH_PORT=19090 docker compose -f docker-compose.dev.yml up -d`.
