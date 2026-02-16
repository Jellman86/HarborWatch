# Changelog

All notable changes to HarborWatch are documented in this file.

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
