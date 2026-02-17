# Changelog

All notable changes to HarborWatch are documented in this file.

## [0.7.1] - 2026-02-17

### Fixed
- **Startup & Robustness:**
  - Added strict startup initialization path (`NewMuxWithSchedulerE`) and removed fatal constructor behavior from the router bootstrap.
  - Upgraded store/service initialization to fail fast on critical DB init errors instead of silently ignoring them.
  - Added degraded startup fallback router for non-strict initialization paths.
- **Update Engine Safety:**
  - Enforced AI risk-gate behavior so high-risk release analysis blocks the update pipeline before destructive steps.
  - Replaced unsafe container recreation logic with Docker inspect/create/start flow to preserve runtime configuration.
  - Fixed validation retry response-body handling to avoid delayed connection cleanup.
  - Made rollback backup selection deterministic using timestamp-aware backup name parsing.
- **API & Routing Correctness:**
  - Added missing `POST /api/docker/prune` endpoint.
  - Added missing `POST /api/ai/fleet-advice` endpoint.
  - Added backward-compatible rules route alias (`/api/docker/containers/{id}/rules`) while standardizing `/api/docker/{id}/rules`.
  - Added Docker availability guard for `/api/ai/audit-compose/{id}` to prevent nil dereference panic.
  - Fixed audit query logic to remove invalid dependency on a non-existent `containers` SQL table.
- **Notifications:**
  - Added thread-safe dispatcher mutation/iteration.
  - Added dispatcher replacement/removal semantics so Discord webhook updates do not accumulate duplicate dispatchers.
  - Fixed ignored request construction errors in Discord notifier.
- **SSE & Scheduler Reliability:**
  - Removed global request timeout middleware that could terminate long-lived SSE streams.
  - Reworked Docker event streaming loop so heartbeat and event reads are independent.
  - Normalized scheduler cron expressions to 6-field format for `cron.WithSeconds`.
- **UI Correctness:**
  - Fixed container rules save path mismatch.
  - Fixed diagnostics schema mismatch (`numGoroutine`).

### Performance
- Added `POST /api/metrics/batch` to reduce per-container network fan-out for sparkline data.
- Updated container inventory UI to fetch sparkline metrics in batched requests.
- Switched route views in `App.svelte` to dynamic imports to improve frontend code-splitting.

### Tests
- Added API regression coverage for:
  - Docker-unavailable compose audit route behavior.
  - Rules compatibility route behavior.
  - Fleet advice endpoint.
  - Docker prune endpoint trigger behavior.
  - Metrics batch endpoint behavior.
- Added update-engine regression coverage for:
  - AI high-risk gate enforcement.
  - Deterministic backup selection helper.

## [0.7.0] - 2026-02-16

### Added
- **Radical UI Theming Engine:**
  - Implemented 5 structurally unique themes: **Tech Innovation**, **Ocean Depths**, **Midnight Galaxy**, **Forest Canopy**, and **Sunset Boulevard**.
  - Themes now control global geometry (border radius, widths), typography pairs, and advanced effects (starfields, grid backgrounds, glassmorphism blur).
  - Added a persistent **Theme & Interface** switcher in the Settings panel.
- **Architectural Stability:**
  - Implemented a **Unified Database Connection Pool** with WAL (Write-Ahead Logging) mode and 5-second busy timeouts.
  - Resolved intermittent deadlocks and "Querying History..." hangs by sharing a single connection across all services.
  - Stabilized Docker event streaming with a 10-second heartbeat and enhanced SSE compliance for reverse proxies.
- **Monitoring & Lifecycle:**
  - Refined the "Safe-by-Default" policy: Monitoring tasks (metrics, security) are now **On** by default, while destructive tasks (pruning) remain **Off** by default.
  - Added backend debug logging for container inspection to improve diagnostics.

### Fixed
- **Routing & API:**
  - Resolved "Container not found" errors by correcting API route precedence in the backend router.
  - Corrected broken API paths for **Container Details** and **Audit Job** retrieval in the frontend.
- **Synchronization Logic:**
  - Fixed a critical deadlock where settings synchronization would hang if triggered during a database write.
- **Frontend Refinement:**
  - Resolved various Svelte 5 prop-mismatch warnings and TypeScript build errors.
  - Fixed accessibility warnings in the Theme Switcher component.

## [0.6.1] - 2026-02-16

### Added
- **UI Stability & Performance:**
  - Resolved main-thread hangs by enforcing clean ApexCharts re-renders using Svelte `{#key}` blocks.
  - Implemented 1-year immutable `Cache-Control` headers for static assets in the Go backend.
- **Global Toast System:**
  - Added a non-blocking notification system for success/error feedback, replacing browser `alert()`.
  - Integrated toasts into the Settings and Security workflows.
- **Full Aesthetic Standardization:**
  - Refactored **Audit**, **Diagnostics**, **Stacks**, and **Images** pages to match the "Industrial Utilitarian" theme.
  - Added staggered reveal animations to all list-based views for a high-end feel.
- **SEO & Accessibility:**
  - Added meta descriptions and a valid `robots.txt`.
  - Audited and improved touch targets and ARIA labels for mobile responsiveness.

## [0.6.0] - 2026-02-16

### Added
- Milestone 0 bootstrap:
  - Go backend with `/health`
  - Svelte 5 frontend served by Go static hosting
  - OpenAPI contract and generated Go/TypeScript API models
  - Project scripts (`dev.sh`, `lint.sh`, `test.sh`, `generate-types.sh`)
- Milestone 1 Docker inventory:
  - `GET /api/docker/containers`
  - `GET /api/docker/images`
  - `GET /api/docker/events` (SSE)
- Milestone 2 vulnerability scanning:
  - Scanner abstraction with Trivy adapter
  - SQLite persistence for scan results
  - `POST /api/scans/run`, `GET /api/scans/jobs/{id}`, `GET /api/scans/summary`
- Milestone 3 release note intelligence:
  - GitHub Releases API ingestion
  - Heuristic release risk scoring with highlighted excerpts
  - `GET /api/releases/summary?repo=owner/name`
- Milestone 4 safe update pipeline:
  - Async update state machine (`preflight -> backup -> pull -> recreate -> validate -> success/rollback`)
  - Robust container replacement logic (stop-rename-start)
  - Automated rollback to last backup on failure
  - SQLite persistence for update runs and step logs
  - Live progress streaming via SSE
  - `POST /api/updates/run`, `GET /api/updates/jobs/{id}`, `GET /api/updates/events/{id}`
- Milestone 5 ClamAV filesystem scans:
  - Scanner abstraction for malware detection
  - ClamAV adapter for recursive filesystem scanning
  - Persistent malware scan job history and result summaries
  - `POST /api/scans/malware/run`, `GET /api/scans/malware/summary`
- AI Intelligence Core (v2 Milestone 6):
  - Integrated OpenAI provider for semantic analysis of release notes
- Automation & Scheduler (v2 Milestone 7):
  - Implemented core `scheduler` service using `robfig/cron/v3`
  - Added automated `docker system prune` task for weekly system maintenance
- Self-Monitoring & Diagnostics:
  - Implemented `diag` service for persistent internal application logging
- Ecosystem Integrations:
  - Implemented core Notification Dispatcher with multi-platform support
  - Added Discord Webhook integration for critical system alerts

### Changed
- Compose default host port changed to `18080`.
- `.gitignore` now excludes `agents/`.
