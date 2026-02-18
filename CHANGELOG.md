# Changelog

All notable changes to HarborWatch are documented in this file.

## [0.7.7] - 2026-02-18

### Added
- **Container Intelligence Overrides:**
  - Added persistent container metadata override storage (`container_intel_overrides`) for manual `repositoryUrl` and `changelogUrl` inputs.
  - Added `GET /api/docker/intel/overrides` for fleet-wide override inventory.
  - Added `GET|POST /api/docker/{id}/intel` (plus `/api/docker/containers/{id}/intel` alias) for effective intelligence view and override management.
- **Lifecycle History API:**
  - Added `GET /api/updates/container/{id}` to retrieve recent per-container update/lifecycle jobs.
- **Settings -> Containers Tab:**
  - Added a dedicated container intelligence UI for selecting containers and managing repo/changelog overrides with derived/effective previews.

### Changed
- **Update Intelligence Enrichment:**
  - Update runs now pass both effective repository and changelog URLs into AI context generation and release-intelligence derivation.
  - Release repository detection now falls back across repo/changelog references to improve GitHub release context resolution.
- **Container Lifecycle UX Simplification:**
  - Reworked lifecycle tab controls to a two-mode model: `Use Global Automation` or `Manual`.
  - Lifecycle tab now focuses on lifecycle logs and breaking-change signals while retaining explicit trigger-upgrade controls.

### Tests
- `go test ./...` (backend)
- `npm run build` (web)

## [0.7.6] - 2026-02-18

### Added
- **Automation Exclusion Controls:**
  - Added auto-discovered container exclusion controls with per-container toggles in Settings -> Automations.
  - Added an advanced token editor below toggles for custom exclusion patterns while preserving HarborWatch self-protection.

### Changed
- **Automation Flow UX:**
  - Updated wrapped flow connector routing so row transitions exit the final node from the bottom face for clearer visual continuity.
- **Automation Safety Exclusions Layout:**
  - Scoped `Ignored Malware Mount Paths` to the Security automation tab only.
  - Clarified exclusion copy to reflect global container-scoped automation behavior.
- **Chart Rendering Engine Hardening:**
  - Replaced Apex-based metric, sparkline, and disk usage charts with native SVG/bar renderers to remove NaN-prone runtime layout behavior.
  - Removed frontend runtime dependencies on `apexcharts` and `svelte-apexcharts`.
  - Added chart-series guardrails (CPU value bounding and point downsampling) to keep container detail rendering responsive even for high-frequency telemetry histories.
  - Normalized API-delivered CPU percentages to a bounded 0-100 scale for consistent chart semantics.

### Fixed
- **Container Detail Navigation Stability:**
  - Eliminated repeated chart `NaN` SVG/transform console errors observed when opening containers (including `frigate`) and cycling tabs (`insights/security/lifecycle/configuration`).
  - Resolved associated UI slowdown/hang risk during tab traversal.
- **Automation Flow Diagram Connector Semantics:**
  - Refined row-wrap connector routing to exit row-end nodes from the bottom face while entering subsequent rows via alternating side faces (row 2 right, row 3 left, etc.) for clearer directionality.

### Tests
- Playwright verification executed via the documented container-to-container workflow (`agents/PLAYWRIGHT_TESTING_GUIDE.md`) against `frigate` and `yawamf-frontend` tab navigation flows with zero console/page/network errors.

## [0.7.5] - 2026-02-17

### Added
- **In-Container ClamAV Signature Lifecycle:**
  - Added `GET /api/scans/malware/signatures/status` for ClamAV engine/database metadata.
  - Added `POST /api/scans/malware/signatures/update` to run `freshclam` updates directly from HarborWatch.
  - Added a dedicated ClamAV signatures panel in Settings -> System with status refresh, manual update action, and scheduler toggle.
  - Added scheduler task `clamav_signature_update` (default cadence `Daily 02:30`) and exposed it through automation schedule controls.
- **Automation Safety Exclusions:**
  - Added global settings for ignored automation containers and ignored malware mount paths.
  - HarborWatch self-protection is now enforced by default via persistent auto-population of `harborwatch` in ignored containers.

### Changed
- **Runtime Architecture (Compose):**
  - Removed external `clamav` sidecar service from `docker-compose.yml`, `docker-compose.dev.yml`, and `docker-compose.prod.yml`.
  - HarborWatch now manages signatures internally and persists ClamAV DB files through a direct bind mount: `${HW_CLAMAV_DB_PATH:-./clamav-db}:/var/lib/clamav`.
- **ClamAV Scan Guardrails:**
  - Added bounded default scan limits (`--max-filesize`, `--max-scansize`, `--max-files`, `--max-recursion`) and disabled archive scanning by default to reduce runaway CPU/IO on large media trees.

### Fixed
- **Settings UX Feedback:**
  - Added explicit toast feedback for automation domain enable/disable actions, including no-op messaging when already in the requested state.

## [0.7.4] - 2026-02-17

### Added
- **Container ClamAV Action:**
  - Added `POST /api/scans/malware/container/{id}` to queue malware scans for both container rootfs and mounted paths.
  - Added container detail UI actions for `Run Trivy Scan` and `Scan with ClamAV` with non-blocking progress/result feedback.
  - Added expanded scan result rendering in container security view (vulnerability metadata + per-scope malware threat details).
- **Container Disk Visibility:**
  - Added `diskUsage` payload to container detail responses with writable bytes, rootfs bytes, and mount count.
  - Added container disk usage card in the container security tab.
- **Asset Lifecycle Automation:**
  - Added automatic `validateUrl` derivation from container labels, healthcheck command, ports, and global validation pattern settings.
  - Rules APIs now auto-fill missing validation URLs while still allowing user overrides.
- **Update Pipeline Input Automation:**
  - `POST /api/updates/run` now auto-derives `targetImage` and `validateUrl` when omitted, using container context.
  - Update AI analysis now includes container context and repository intelligence grounded on release notes in the detected current->target version range (with fallback heuristics).
- **Settings Control Plane Toggles:**
  - Added persisted on/off controls for AI (`aiEnabled`), Discord notifications (`discordEnabled`), Portainer integration (`portainerEnabled`), and UI motion (`uiAnimationsEnabled`).
  - Added per-domain automation master controls (enable/disable all tasks in upgrades, maintenance, or security domains).
  - Added system-level metrics collector toggle in Settings.

### Fixed
- **ClamAV Container Scan Build/Runtime Stability:**
  - Fixed Docker `CopyFromContainer` typing mismatch in container malware snapshot logic.
  - Added container existence pre-check for malware-container scan trigger to avoid queueing invalid IDs.
- **Container Lifecycle UX:**
  - Replaced blocking alerts with toast-driven feedback for lifecycle policy save actions.
  - Added inline explanation for Validation URL behavior and automation.
- **Settings Runtime Stability & UX:**
  - Fixed `ReferenceError: PORT is not defined` in Settings by safely rendering the validation URL placeholder token.
  - Reworked automation visuals into deterministic step-flow diagrams (no chart-width `NaN`/`foreignObject` render failures).
  - Hardened Docker events SSE reconnect behavior with backoff and deduplicated reconnect timers to reduce UI console noise.

## [0.7.3] - 2026-02-17

### Fixed
- **Container Detail API:**
  - Fixed Docker inspect decoding in `GET /api/docker/{id}` by handling inspect-style `State` objects instead of list-style `State` strings.
  - Resolves runtime error: `json: cannot unmarshal object into ... containerJSON.State of type string`.
- **Metrics Collection Reliability:**
  - Added scheduler cron-spec normalization for legacy persisted 5-field entries.
  - Auto-upgrades old schedule specs (e.g., `* * * * *`) to 6-field (`0 * * * * *`) to match `cron.WithSeconds`.
  - Added startup seed run for `metrics_collector` so dashboards populate shortly after boot.
- **Layout & Navigation UX:**
  - Reworked app shell/main spacing to use explicit sidebar width offsets, preventing expanded-sidebar overlap and improving centering on desktop/mobile.

### Added
- **AI Provider Support:**
  - Added Anthropic (Claude) provider integration.
  - Added Gemini provider integration.
  - Added provider selection logic (`AI_PROVIDER`) with automatic fallback across configured providers.
  - Added settings fields for provider selection and keys/models across OpenAI, Anthropic, and Gemini.
- **Scheduler Tests:**
  - Added regression tests for cron-spec normalization and legacy schedule upgrade behavior.

## [0.7.2] - 2026-02-17

### Added
- **Diagnostics API Expansion:**
  - Added `GET /api/docker/{id}/logs` with `tail`, `since`, and `timestamps` controls for direct container log retrieval.
  - Added `GET /api/diagnostics/containers/{id}/logs` as a diagnostics namespace endpoint for container logs.
  - Added `GET /api/diagnostics/snapshot` that aggregates:
    - component availability (`docker`, `scanner`, `audit`, `scheduler`, `diag`)
    - runtime system status
    - recent internal logs
    - recent and failed audit jobs
    - scheduler state
    - latest scan summary
    - optional fleet inventory/images and target-container logs
- **API Error Telemetry:**
  - Added HTTP error middleware that records all 4xx/5xx API responses (method, path, status, duration, request-id, client metadata) into diagnostics logs.

### Fixed
- **Container Detail Routing Compatibility:**
  - Added backward-compatible container detail alias route (`/api/docker/containers/{id}`) to fix “Container not found” behavior for older frontend paths.
- **Trivy Scan Stability:**
  - Added configurable scan timeouts (`HW_TRIVY_SCAN_TIMEOUT`, `HW_TRIVY_INTERNAL_TIMEOUT`) and clearer timeout/cancellation errors to reduce opaque `exit -1` failures.
- **System Logs Querying:**
  - Enhanced `GET /api/system/logs` to support `limit`, `level`, `source`, and `since` filters for faster root-cause isolation.

### Tests
- Added API regression coverage for:
  - container log retrieval route behavior
  - diagnostics snapshot response shape
  - filtered diagnostics log query behavior

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
