# Changelog

All notable changes to HarborWatch are documented in this file.

## [Unreleased]

### Added
- **Container-Driven Architecture (v0.6.0):**
  - Shifted from "Action-First" to **"Asset-First"** navigation.
  - New **"Container Command Center"**: A dedicated full-page view for every container asset.
  - Contextual Tabs: **Insights** (Metrics), **Security** (Scans), **Lifecycle** (Updates/Rules), and **Configuration** (Doctor).
  - Automated **"Compose Doctor"**: Automatically retrieves or reconstructs `docker-compose.yml` via Portainer API, host mounts, or container metadata.
- **Per-Container Intelligence:**
  - Dedicated SQLite rule store for container-specific configuration.
  - Customizable **Update Policies**: "Auto" (automatic), "Manual" (notify), or "Locked" (ignore).
  - Configurable health check validation URLs and auto-rollback toggles per asset.
  - Integrated execution history for every specific container.
- **Universal Configuration System:**
  - Unified tabbed **Settings View** for Notifications, API Keys, and System parameters.
  - Smart configuration merging: Environment variables (Compose) now take priority over database settings.
  - UI indicators for "Locked (ENV)" fields to prevent configuration confusion.
- **Modernized Backend Infrastructure:**
  - Migrated to **`go-chi/chi`** router for advanced routing and middleware support.
  - Unified Docker interactions using the official **Moby SDK**.
  - Enhanced API robustness with defensive defaulting (preventing null arrays in JSON).
  - Implemented detailed technical failure logging to the **System Health** diagnostics.
- **UI/UX Refinement ("Tech Innovation" Aesthetic):**
  - Professional branding with **Montserrat** and **IBM Plex Sans** typography.
  - Modern **Card View** toggle for the Fleet inventory.
  - **High-end Animations**: Staggered reveals, smooth transitions, and pulse indicators for updates.
  - Integrated **Live Sparklines** in the container inventory for real-time CPU monitoring.
  - Industrial-style glassmorphism and subtle grain overlay for a "Security Appliance" feel.
- **Docker Image Repository Enhancements:**
  - Professional list-based view for the image repository.
  - New **"Cleanup Repository"** button to trigger automated pruning of unused artifacts.

## [0.5.0] - 2026-02-15

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
