# HarborWatch

HarborWatch is a professional, local-first container maintenance and security appliance for self-hosted Docker environments. It transforms passive monitoring into an active, asset-centric security strategy.

## Key Features

- **Container Command Center:** Dedicated full-page views for every container with deep insights into metrics, security history, and lifecycle status.
- **Radical Design System:** 5 fully distinctive UI themes (Tech, Ocean, Midnight, Forest, Sunset) that transform the entire interface architecture, typography, and geometry.
- **Fleet Management:** modern Card and List views for your entire container inventory with real-time "Update Available" detection.
- **Automated Compose Doctor:** Zero-config security auditing. HarborWatch automatically retrieves or reconstructs your `docker-compose.yml` for AI-powered security analysis.
- **Per-Container Lifecycle Policies:** Fine-grained control over updates. Set "Auto", "Manual", or "Locked" policies per asset with custom health check validation.
- **Dual-Engine Security Scanning:** Integrated vulnerability (Trivy) and malware (ClamAV) scanning with shared database persistence.
- **High-Resolution Performance Profiling:** Real-time sparklines and detailed historical charts for CPU and Memory utilization.
- **Unified Configuration:** Seamlessly merge `docker-compose` environment variables with persistent database settings.

## Reliability & Performance Notes (v0.7.1)

- **Safer Startup:** strict initialization path now fails fast on critical DB/bootstrap errors (`NewMuxWithSchedulerE`).
- **Safer Updates:** AI high-risk updates are blocked before execution; rollback selection is deterministic.
- **SSE Stability:** Docker event streaming now uses independent heartbeat + scanner channels, and is no longer constrained by a global timeout middleware.
- **Notification Hygiene:** Discord dispatchers are replaced/removed cleanly on settings changes to prevent duplicate alerts.
- **Lower Metrics Overhead:** Container list sparklines now use batched metrics API calls via `POST /api/metrics/batch`.
- **New API endpoints:**
  - `POST /api/docker/prune` (trigger scheduled prune task)
  - `POST /api/ai/fleet-advice` (fleet-level advisory summary)
  - `POST /api/metrics/batch` (multi-container metrics retrieval)
  - `GET /api/docker/{id}/logs` (container stdout/stderr retrieval with tail/since controls)
  - `GET /api/diagnostics/snapshot` (aggregated diagnostics bundle for autonomous investigation)
  - `GET /api/diagnostics/containers/{id}/logs` (container logs through diagnostics namespace)

## Diagnostics API

- `GET /api/system/logs` now supports filters:
  - `limit` (default `100`, max `2000`)
  - `level` (e.g. `ERROR`)
  - `source` (case-insensitive substring)
  - `since` (unix timestamp in seconds)
- `GET /api/diagnostics/snapshot` supports:
  - `logLimit`, `auditLimit`
  - `includeFleet` (`true`/`false`)
  - `containerId`
  - `containerLogTail`, `containerLogSince`
- `GET /api/docker/{id}/logs` and `GET /api/diagnostics/containers/{id}/logs` support:
  - `tail` (line count)
  - `since` (duration like `1h` or unix timestamp)
  - `timestamps` (`true`/`false`)

## Scanner Timeouts

- `HW_TRIVY_SCAN_TIMEOUT` sets overall vulnerability scan job timeout (default `15m`).
- `HW_TRIVY_INTERNAL_TIMEOUT` sets Trivy CLI timeout argument (default `10m`).
- `HW_CLAMAV_SCAN_TIMEOUT` sets malware scan job timeout (default `15m`).

## Technology Stack

- **Backend:** Go 1.26 with `go-chi` router and official Moby Docker SDK.
- **Frontend:** Svelte 5 (Runes) with Tailwind CSS and ApexCharts.
- **Database:** SQLite (Embedded) for persistence of scans, rules, and metrics.
- **AI Core:** OpenAI integration for release note analysis and security auditing.

## Quick Start

1. Generate shared API types:
```bash
scripts/generate-types.sh
```

2. Start the appliance (Development Mode):
```bash
scripts/dev.sh
```

3. Access the UI at `http://localhost:18080`.

## Architecture Note

HarborWatch is designed as a **single monolithic container**. It serves the REST API, background jobs, and the built Svelte static assets from a single Go binary. No Node.js or complex sidecars are required in production.

## Environment Overrides

HarborWatch prioritizes standard Docker environment variables for configuration. If defined in your `docker-compose.yml`, these values will be locked in the UI:
- `DISCORD_WEBHOOK_URL`
- `OPENAI_API_KEY`
- `PORTAINER_URL` / `PORTAINER_API_KEY`
- `HW_INSTANCE_URL`

## CI/CD

- **PR Validation:** Automatic linting and backend tests.
- **Build & Push:** Automatic image generation to `ghcr.io/jellman86/harborwatch:dev` on every push to the `dev` branch.
- **Releases:** Versioned tags trigger production builds.
