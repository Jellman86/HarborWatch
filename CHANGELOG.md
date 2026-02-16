# Changelog

All notable changes to HarborWatch are documented in this file.

## [Unreleased]

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
- System-wide quality improvements:
  - Idempotent database migration system for seamless schema updates
  - Frontend refactored to Svelte 5 runes for performance and clarity
  - Hardened update validation with retry loops and safe rollback cleanup
- AI Intelligence Core (v2 Milestone 6):
  - Integrated OpenAI provider for semantic analysis of release notes
  - Automated "Risk Gatekeeper" in the update pipeline to pause updates on high-risk changes
  - New "Compose Doctor" view for AI-powered security auditing of Docker Compose files
  - Refactored AI integration to use industry-standard `go-openai` SDK and `yaml.v3` parser
  - New UI components for AI Analysis reports and status monitoring
  - Persistent AI analysis history in SQLite
- Automation & Scheduler (v2 Milestone 7):
  - Implemented core `scheduler` service using `robfig/cron/v3`
  - Added automated `docker system prune` task for weekly system maintenance
  - New "Automation" UI view for managing background tasks and monitoring execution history
  - Added "Off by Default" operational policy for all background tasks
  - Implemented `TrivySweepTask` for automated full-system vulnerability scans
  - Implemented `ClamAVSweepTask` for automated host-volume malware scans
  - Added toggle and manual-run API endpoints for scheduled tasks
  - Integrated official Moby Docker SDK for robust engine interactions
  - Added SQLite persistence for schedules to ensure tasks survive restarts
- UI Expansion (YA-WAMF Inspired):
  - Transitioned from single-page prototype to multi-view security appliance
  - Integrated Tailwind CSS for production-grade styling
  - Implemented persistent Sidebar navigation with collapse support
  - Full Dark/Light mode support with system preference detection
  - Modular component architecture: `Dashboard`, `Containers`, `Images`, `Security`, `Intelligence`, and `Updates`
  - Added `System Health` (Diagnostics) view with real-time telemetry and internal log streaming
  - Added `Operations Log` (Audit) view with expandable detailed execution history
  - Implemented "Contextual Navigation": trigger scans or updates directly from container inventory with pre-filled state
  - Added `System Settings` view for global application configuration
  - Visual update pipeline stepper with live progress terminal
- Self-Monitoring & Diagnostics:
  - Implemented `diag` service for persistent internal application logging
  - Added real-time telemetry for uptime, memory allocation, and database size
  - Built-in automatic log pruning with 7-day retention
  - New `/api/system/status` and `/api/system/logs` endpoints for self-aware monitoring
- System Refinements & Quality Audit:
  - Implemented "Action Hub" pattern in Container Inventory for light-touch management
  - Added AI Fleet Health Advice to the Dashboard for proactive optimization
  - Improved Metrics Collector robustness with per-container execution timeouts
  - Enhanced visibility of auto-discovered labels and intelligence sources
  - Centralized global connectivity state in the main router
  - Resolved Svelte 5 charting compatibility issues using ApexCharts actions
  - Fully audited UI for Accessibility (A11y), adding labels and ARIA support
  - Fixed runtime `TypeError` by ensuring all API-driven arrays default to empty instead of null
- Label-Driven Auto-Discovery:
  - Containers now support `harborwatch.*` labels for "Light Touch" configuration
  - Automatic detection of update policies and intelligence sources directly from container metadata
  - UI visibility for discovered policies in the container inventory
- Performance Profiler (v2 Milestone 9):
  - Implemented `metrics` service for high-resolution container stats collection
  - Added `container_metrics` SQLite table for time-series data storage
  - Built `Collector` task to snapshot CPU, Memory, and I/O every 60 seconds
  - Integrated ApexCharts for interactive CPU and Memory visualization in the UI
  - Added AI Performance Consultant to diagnose resource leaks and optimize limits
  - Added `/api/metrics/{id}` and `/api/ai/analyze-metrics` endpoints
  - Added automated 7-day retention policy via `metrics_prune` task
- CI/CD and deployment artifacts:
  - GitHub Actions workflows for build/push and PR validation
  - Single-container `Dockerfile`
  - Compose manifests (`docker-compose.yml`, `docker-compose.dev.yml`, `docker-compose.prod.yml`)

### Changed
- Compose default host port changed to `18080` to avoid conflicts with existing services using `8080`.
- `.gitignore` now excludes `agents/`.

### Notes
- Trivy-based scan execution requires the `trivy` binary on `PATH`.
- Update pipeline currently includes a rollback hook with persistence and status handling; container replacement rollback logic is implemented as an extensible hook.
