# HarborWatch

HarborWatch is a professional, local-first container maintenance and security appliance for self-hosted Docker environments. It transforms passive monitoring into an active, asset-centric security strategy.

## Key Features

- **Container Command Center:** Dedicated full-page views for every container with deep insights into metrics, security history, and lifecycle status.
- **Disk Footprint Visibility:** Per-container writable/rootfs disk usage and mount count surfaced directly in the security view.
- **Radical Design System:** 5 fully distinctive UI themes (Tech, Ocean, Midnight, Forest, Sunset) that transform the entire interface architecture, typography, and geometry.
- **Fleet Management:** modern Card and List views for your entire container inventory with real-time "Update Available" detection.
- **Automated Compose Doctor:** Zero-config security auditing. HarborWatch automatically retrieves or reconstructs your `docker-compose.yml` for AI-powered security analysis.
- **Per-Container Lifecycle Policies:** Fine-grained control over updates. Set "Auto", "Manual", or "Locked" policies per asset with custom health check validation.
- **Policy-Aware Upgrade Automation:** Split detection (`container_update_check`) and optional execution (`container_update_apply`) pipelines with per-container policy gates and retry cooldowns.
- **Dual-Engine Security Scanning:** Integrated vulnerability (Trivy) and malware (ClamAV) scanning with shared database persistence.
- **In-Container ClamAV Signature Management:** HarborWatch now manages `freshclam` updates internally, exposes signature status in Settings, and supports scheduled signature refresh automation.
- **High-Resolution Performance Profiling:** Real-time sparklines and detailed historical charts for CPU and Memory utilization.
- **Unified Configuration:** Seamlessly merge `docker-compose` environment variables with persistent database settings.
- **Automated Lifecycle Inputs:** Validation URLs and update form defaults are auto-derived from container metadata, health checks, exposed ports, settings patterns, and repo-aware release context, while remaining fully user-editable.

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
  - `POST /api/scans/malware/container/{id}` (queue container rootfs + mount-point ClamAV scans)

## Robustness Notes (v0.7.3)

- **Container Manage Fix:** Docker inspect parsing now correctly handles `State` object payloads, resolving the `cannot unmarshal ... State of type string` detail-page failure.
- **Metrics Recovery:** Scheduler now auto-normalizes legacy 5-field cron entries to 6-field format (`cron.WithSeconds`), restoring `metrics_collector` execution on upgraded installs.
- **Layout Stability:** Main shell offset/centering uses deterministic sidebar-width CSS variables for correct desktop/mobile behavior without sidebar overlap.
- **AI Providers:** OpenAI, Anthropic (Claude), and Gemini are now supported with provider selection.

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
- `HW_TRIVY_MAX_CONCURRENCY` limits parallel Trivy jobs (default `2`).
- `HW_CLAMAV_MAX_CONCURRENCY` limits parallel ClamAV jobs (default `1`).
- `HW_CLAMAV_SNAPSHOT_MAX_BYTES` caps container snapshot size for ClamAV container scans (default `2147483648`).
- `HW_CLAMAV_UPDATE_TIMEOUT` sets max runtime for manual/scheduled signature updates (default `10m`).
- `HW_CLAMAV_STATUS_TIMEOUT` sets signature status query timeout (default `10s`).
- `HW_CLAMAV_MAX_FILE_MB` caps individual file size scanned by ClamAV (default `32`).
- `HW_CLAMAV_MAX_SCAN_MB` caps aggregate scan size per scan invocation (default `512`).
- `HW_CLAMAV_MAX_FILES` caps files scanned per invocation (default `12000`).
- `HW_CLAMAV_MAX_RECURSION` caps directory recursion depth for ClamAV scans (default `16`).
- `HW_CLAMAV_SCAN_ARCHIVES` toggles archive scanning (`true`/`false`, default `false`).
- `HW_CLAMAV_FRESHCLAM_CHECKS` controls `freshclam --checks` value during updates (default `2`).
- `HW_CLAMAV_DB_PATH` controls ClamAV signature database directory path (default `/var/lib/clamav` in container).
- `HW_SCAN_BIND_ROOT` controls the host root path bind-mounted for scheduled ClamAV mount sweeps in `docker-compose.dev.yml` (default `/mnt/Storage-SSD`). Keep container path identical to host path.

## Validation URL Automation

- If a container lifecycle rule does not define `validateUrl`, HarborWatch derives one automatically from:
  - explicit validation labels (if present),
  - Docker healthcheck URL hints,
  - published/exposed ports,
  - global settings (`validateUrlPattern`, `instanceUrl`).
- `validateUrlPattern` supports placeholders:
  - `{{INSTANCE_URL}}`, `{{HOST}}`, `{{PORT}}`, `{{SCHEME}}`, `{{PATH}}`, `{{CONTAINER_ID}}`, `{{CONTAINER_NAME}}`, `{{IMAGE}}`
- Users can always override the derived URL in the Lifecycle policy UI.

## Upgrade Automation Controls

- `HW_AI_BLOCK_RISK_THRESHOLD` sets the AI risk-score block threshold used during update `release_analysis` (default `80`).
- `HW_AUTO_UPGRADE_MAX_CONCURRENCY` limits auto-apply starts per scheduler run (default `1`).
- `HW_AUTO_UPGRADE_MIN_RETRY_MINUTES` sets auto-apply retry cooldown after failed/rolled-back jobs (default `60`).
- These controls are available in Settings (`AI` and `Automations -> Upgrades`) unless locked by environment overrides.

## AI Provider Configuration

- `AI_PROVIDER`: `openai` | `anthropic` | `gemini` (optional; defaults to first configured provider)
- OpenAI: `OPENAI_API_KEY`, `OPENAI_MODEL`
- Anthropic: `ANTHROPIC_API_KEY`, `ANTHROPIC_MODEL`
- Gemini: `GEMINI_API_KEY` (or `GOOGLE_API_KEY`), `GEMINI_MODEL`

## Technology Stack

- **Backend:** Go 1.26 with `go-chi` router and official Moby Docker SDK.
- **Frontend:** Svelte 5 (Runes) with Tailwind CSS and ApexCharts.
- **Database:** SQLite (Embedded) for persistence of scans, rules, and metrics.
- **AI Core:** OpenAI integration for release note analysis and security auditing.

## Documentation

- `docs/README.md` - docs index and navigation.
- `docs/UPGRADE_AUTOMATION.md` - upgrade pipeline architecture, policies, scheduler tasks, and operational tuning.
- `docs/REVERSE_PROXY.md` - reverse proxy deployment notes.
- `docs/ROADMAP_v2.md` - product roadmap.

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

ClamAV signature updates are also managed inside the HarborWatch container. Persist signature files by bind-mounting `/var/lib/clamav` (compose defaults now include `${HW_CLAMAV_DB_PATH:-./clamav-db}:/var/lib/clamav`).
For scheduled mount scans in dev compose, HarborWatch needs host bind paths visible at the same absolute path inside the container (compose default: `${HW_SCAN_BIND_ROOT:-/mnt/Storage-SSD}:${HW_SCAN_BIND_ROOT:-/mnt/Storage-SSD}:ro`).

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
