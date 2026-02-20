# Changelog

All notable changes to HarborWatch are documented in this file.

## [0.8.0] - 2026-02-20

### Added
- **Global Progress Notification System:**
  - Implemented a sleek, sticky progress header that aggregates all active background jobs (scans, updates).
  - Added detailed job status messages (e.g., "Scanning vulnerabilities...", "Backing up...") and per-job progress tracking (0-100%).
  - Unified active job discovery via `/api/diagnostics/snapshot`.
- **Visual Health Indicators:**
  - Added real-time health status pulse dots and badges to Fleet Inventory and Container Detail views.
  - Supports `healthy`, `unhealthy`, and `starting` states derived directly from Docker Engine health checks.
- **Enhanced Portainer Integration:**
  - Expanded Portainer API client with support for Environments (Endpoints), Stacks, and Docker Proxy.
  - Implemented **Stack Redeploy with PullImage** support directly from the Stacks view.
  - Added "Manage" deep-links from Stacks to filtered Fleet views.
- **Robustness & Recovery:**
  - Added startup reconciliation for update jobs; stale `running` jobs are now automatically failed on restart to prevent "stuck" UI states.
  - Fixed semaphore deadlock in ClamAV signature updates by making slot acquisition context-aware.

### Changed
- **CPU Metrics Normalization:**
  - Changed CPU usage calculation to be system-wide normalized instead of per-core.
  - Percentages now reflect total system capacity, aligning with host-level monitors like TrueNAS Scale.
- **Images UI Overhaul:**
  - Modernized the Image Repository with a high-density card grid, global stats bar, and live search/sorting.
  - Improved visibility for digest-only artifacts and security posture.
- **Automation Safety:**
  - Added "portainer" to the default automatic ignore list.
  - Fixed a bug where manual lifecycle settings would revert to automatic on save.

### Tests
- `go test ./...` (backend)
- `npm run build` (web)

## [0.7.21] - 2026-02-19

### Added
- **Container Intelligence Readiness Signals:**
  - Added `GET /api/docker/containers/intel-readiness` to return centralized per-container metadata readiness, effective repo/changelog links, provider classification, and actionable issue diagnostics.
  - Container intelligence responses now include readiness fields (`hasRepository`, `hasChangelog`, `releaseIntelReady`, `fullAutomationReady`) plus issue details for operator guidance.
- **Data Lifecycle Retention Controls:**
  - Added configurable retention settings for:
    - `retentionLogsDays`
    - `retentionMetricsDays`
    - `retentionScanResultsDays`
    - `retentionScanJobsDays`
    - `retentionUpdateRunsDays`
    - `retentionComposeAuditDays`
    - `retentionAIUsageDays`
  - Added new maintenance scheduler task `history_retention_prune` to clean aged scan history, update runs/steps, compose audit history, and AI usage events.

### Changed
- **Repository/Changelog Derivation Robustness:**
  - Expanded repository derivation fallback logic to include additional OCI metadata and improved registry heuristics (including LinuxServer image mapping).
  - Fleet cards now use effective intelligence-derived repository links instead of label-only links.
- **Fleet Triage UX:**
  - Added Fleet filter pills for rapid scoping: `All`, `Upgrade Needed`, `Intel Issues`, `High Risk`, and `Ignored`.
  - High-risk filtering now incorporates image intelligence signals (critical/high vulnerabilities and malware detections).
- **Fleet + Container UX for AI Automation Readiness:**
  - Fleet cards now surface an `Intel` warning badge and inline guidance when metadata is insufficient for robust AI release-note automation.
  - Container detail pages now expose explicit intelligence readiness state and issue/action lists in the intelligence tab.
- **Image Repository Clarity for Digest Artifacts:**
  - Added explicit UI explanation for digest-only `sha256:*` artifacts and why they can remain after prune when Docker reference graphs still require them.
- **Retention Runtime Wiring:**
  - `metrics_prune` and `diag_log_prune` now respect configured retention days instead of fixed hardcoded windows.
  - Added environment override support for retention settings:
    - `HW_RETENTION_LOG_DAYS`
    - `HW_RETENTION_METRICS_DAYS`
    - `HW_RETENTION_SCAN_RESULTS_DAYS`
    - `HW_RETENTION_SCAN_JOBS_DAYS`
    - `HW_RETENTION_UPDATE_RUNS_DAYS`
    - `HW_RETENTION_COMPOSE_AUDIT_DAYS`
    - `HW_RETENTION_AI_USAGE_DAYS`
- **Automation Settings Feedback:**
  - Domain enable/disable buttons now show active apply-state feedback and disable while updates are in flight, removing ambiguous click behavior.

### Tests
- `go test ./...` (backend)
- `npm --prefix web run build` (frontend)

## [0.7.20] - 2026-02-19

### Fixed
- **Auto-Apply Skip Diagnostics:**
  - `container_update_apply` now logs explicit skip reasons for each skipped candidate (global exclusion, locked/manual policy, cooldown/running gate, request build failures, and start failures).
  - End-of-cycle auto-apply summary now includes a deterministic `skip_reasons=` breakdown so `candidates>0 started=0` outcomes are directly explainable from logs.
- **Per-Container ClamAV Result Visibility:**
  - Container malware summary and detail prefix matching now supports short container IDs as well as canonical full IDs.
  - Malware scan trigger and summary APIs now resolve to canonical container IDs before queue/query, improving consistency across route forms.
  - Container detail security/history fetches now use resolved canonical container IDs after load, preventing stale/missing results when navigating with non-canonical IDs.
- **System Health Log Noise Control:**
  - Scheduler now suppresses routine INFO lifecycle logs for `metrics_collector` (`Executing scheduled task...` / `Scheduled task completed...`) while preserving error visibility.
  - System Health UI adds a default-on `Hide Metrics Noise` filter so historical scheduler spam is hidden immediately without losing other telemetry context.

### Added
- **Fleet Inventory Search + Automation Ignore Indicator:**
  - Added live Fleet search filtering (name, ID, image, state, and labels).
  - Added per-card `Ignored` badge showing whether a container is excluded by automation safety rules (derived from current settings ignore tokens).

### Tests
- `go test ./internal/httpapi ./internal/scanning ./internal/scheduler` (backend)
- `npm --prefix web run build` (frontend)

## [0.7.19] - 2026-02-19

### Fixed
- **Auto-Apply Reliability After Manual Update Checks:**
  - `container_update_apply` now performs an update-status refresh preflight before evaluating candidates, reducing stale `updateAvailable` races when operators run check/apply tasks back-to-back.
  - Added clearer auto-apply cycle diagnostics when no candidates are available versus candidates skipped by policy/cooldown.
- **Scheduled ClamAV Sweep Visibility + Result Attribution:**
  - Scheduled ClamAV sweeps now emit explicit scheduler telemetry (`start`, per-target queue, and run summary counts) into diagnostics logs.
  - Scheduled ClamAV scans now use container-scoped targets (`container:<id>:mount:<path>`) so results appear in container security history views rather than only path-level records.

### Changed
- **Fleet Inventory Card-First UX:**
  - Removed list-mode fleet rendering and standardized on card view for a simpler operator workflow.
  - Reworked card content to prioritize meaningful telemetry at a glance (state, update signal, CPU trend, current CPU, memory, PID count) with direct drill-down via `Manage`.
  - Sparkline rendering now uses full available card width (instead of fixed 100px), fixing truncated trend visuals.

### Tests
- `go test ./...` (backend)
- `npm --prefix web run build` (frontend)

## [0.7.18] - 2026-02-18

### Fixed
- **Cleanup Log Ordering and Completion Semantics:**
  - System log query ordering is now deterministic for same-second events (`timestamp DESC, id DESC`) so prune timelines no longer shuffle lines unpredictably.
  - Image Repository cleanup log ingestion now sorts by `timestamp + id` and deduplicates by log record id where available.
  - Manual prune polling now waits for explicit terminal scheduler messages (`Manual task completed/failed`) before declaring completion.
- **Prune Task Messaging Clarity:**
  - Updated scheduler log wording from `Starting automated Docker system prune...` to `Starting Docker system prune task...` to avoid misleading output during manual runs.

## [0.7.17] - 2026-02-18

### Changed
- **UI Foundation Refinement (Signpost Theme):**
  - Tightened global radius scale to reduce over-rounded surfaces and improve visual precision.
  - Tuned light/dark background treatments for lower visual noise and better content separation.
  - Reduced heavy shadow intensity across the app for a cleaner, more modern data-first look.
- **Dashboard Visual Density:**
  - Reduced oversized heading/stat card proportions and softened oversized hero geometry.
  - Improved hierarchy between summary cards and AI advisor panel.
- **Fleet View Responsiveness:**
  - Added compact viewport behavior that forces card mode on narrow screens.
  - Disabled list/card toggle interaction in compact mode and surfaced a `Mobile Cards` indicator.
- **Image Repository Mobile UX:**
  - Added dedicated mobile card rendering for image rows to avoid compressed desktop-table presentation on phones.
  - Preserved full desktop table for medium+ breakpoints.
- **System Health Readability Overhaul:**
  - Reworked log control surfaces for better light-mode readability and stronger contrast balance.
  - Added mobile-first log cards (timestamp/level/class/source/message) and kept table mode for desktop.
  - Reduced stat-card bulk and improved filter/control ergonomics.

### Tests
- `npm --prefix web run build` (frontend)

## [0.7.16] - 2026-02-18

### Added
- **About Page:**
  - Added a dedicated modern `About` screen in the main navigation.
  - Included platform overview, operational capabilities, backend/frontend architecture notes, and live service version/status from `/health`.
  - Added icon attribution:
    - `Harbour icons created by Freepik - Flaticon`

### Changed
- **Icon System Refresh (Signpost):**
  - Replaced app icon assets using the provided `agents/signpost.png` source.
  - Generated and wired modern icon outputs for browser/PWA usage:
    - `favicon.ico`
    - `favicon-16x16.png`
    - `favicon-32x32.png`
    - `favicon-48x48.png`
    - `apple-touch-icon.png`
    - `pwa-192x192.png`
    - `pwa-512x512.png`
  - Added reusable UI logo assets:
    - `logo-64.png`
    - `logo-96.png`
    - `logo-128.png`
    - `logo-256.png`
    - `logo-512.png`
- **Theme System Overhaul:**
  - Removed multi-theme selection (`Tech Innovation`, `Forest Canopy`) and replaced with a single robust icon-led theme: `Harbor Signpost`.
  - Updated typography, color tokens, shell styling, and dark-mode gradients to align with the signpost icon palette.
  - Simplified the Appearance UI to mode selection (`light`, `dark`, `system`) plus fixed theme description card.
- **Branding & Navigation UX:**
  - Replaced shield glyph logo usage in sidebar/mobile header with the signpost icon.
  - Added `About` route in application navigation.
- **Settings Automations Layout Refinement:**
  - Moved `Automation Safety Exclusions` below the two-column automations layout and marked it as a separate safety section for clearer scanning.

### Tests
- `npm --prefix web run build` (frontend)

## [0.7.15] - 2026-02-18

### Changed
- **Settings -> Automations Layout Redesign:**
  - Reworked domain controls into a true 2-column composition:
    - left column: automation flow diagram and flow state notes
    - right column: runtime controls and scheduler task toggles/schedules
  - Updated flow connector behavior to a strict vertical path where arrows exit from the bottom of a node and attach to the top of the next node.
- **Automation Task UX Clarity:**
  - Added per-task descriptions explaining what each scheduler task does.
  - Added inline helper text for cadence/time/day selectors so schedule semantics are explicit in the UI.
- **Settings Explanation Pass:**
  - Expanded in-UI descriptions across AI and Integrations settings, including:
    - preferred AI provider behavior (`auto` vs pinned provider)
    - provider API key/model purpose
    - Discord webhook usage
    - Portainer URL/API key usage
  - Added appearance-theme context copy to clarify global theme impact.

### Tests
- `npm --prefix web run build` (frontend)

## [0.7.14] - 2026-02-18

### Added
- **Settings UI Controls for Upgrade/Security Runtime Knobs:**
  - Added Settings -> AI control for `AI Update Block Risk Threshold` (`aiBlockRiskThreshold`).
  - Added Settings -> Automations -> Upgrades controls for:
    - `autoUpgradeMaxConcurrency`
    - `autoUpgradeMinRetryMinutes`
  - Added Settings -> System control for:
    - `clamavSnapshotMaxBytes` with human-readable size preview.
- **Persisted Runtime Settings + Env Locking:**
  - Extended settings persistence and environment override locking for:
    - `HW_AI_BLOCK_RISK_THRESHOLD`
    - `HW_AUTO_UPGRADE_MAX_CONCURRENCY`
    - `HW_AUTO_UPGRADE_MIN_RETRY_MINUTES`
    - `HW_CLAMAV_SNAPSHOT_MAX_BYTES`

### Changed
- **Runtime Wiring:**
  - Update AI risk gate now consumes persisted `aiBlockRiskThreshold` when configured.
  - Auto-apply scheduler now consumes persisted concurrency/retry values per run.
  - Container malware snapshot cap now consumes persisted `clamavSnapshotMaxBytes`.
- **API Contract Types:**
  - Regenerated shared Go/TypeScript API settings types to include new runtime fields.

### Tests
- `go test ./...` (backend)
- `npm --prefix web run build` (frontend)

## [0.7.13] - 2026-02-18

### Added
- **Upgrade Auto-Apply Scheduler Stage:**
  - Added scheduler task `container_update_apply` (default disabled) to execute policy-approved upgrades after detection.
  - Added robust candidate gating for auto-apply:
    - container must currently report `updateAvailable=true`
    - container must pass automation domain/exclusion checks
    - container policy must be `auto`
  - Added cooldown guardrails for retrying failed/rolled-back auto upgrades:
    - `HW_AUTO_UPGRADE_MIN_RETRY_MINUTES` (default `60`)
  - Added per-run start cap for auto-apply:
    - `HW_AUTO_UPGRADE_MAX_CONCURRENCY` (default `1`)
- **Manual Update Policy Enforcement:**
  - `POST /api/updates/run` now enforces `updatePolicy=locked` and returns `423 Locked` when blocked.
  - Added centralized update request builder path for both manual and automated starts to keep derivation/policy behavior consistent.
- **Lifecycle UI Policy Hooks:**
  - Added explicit `Update Policy` selector in Container -> Lifecycle (`auto`, `manual`, `locked`).
  - `Trigger Upgrade` action is now disabled when policy is `locked`.
  - Settings -> Automations -> Upgrades now exposes both:
    - `Container Update Check`
    - `Container Auto-Apply`
- **Docs Bootstrap:**
  - Added docs index: `docs/README.md`.
  - Added upgrade automation guide: `docs/UPGRADE_AUTOMATION.md`.
  - Linked docs from the project `README.md`.

### Changed
- **Update Service Safety:**
  - `StartUpdate` now rejects duplicate in-flight runs for the same container when a `running` job already exists.
- **Automation Task Filtering Semantics:**
  - Container automation checks now support task-specific global enable state evaluation, improving consistency between scheduler toggles and task execution.
- **Container Malware Scan UX:**
  - Container security view now polls dedicated malware summary/details/job endpoints instead of repeatedly reloading the full container detail payload.
  - Reduces visible page refresh/jump behavior during long-running container malware scans.

### Fixed
- **Malware Scan Job Visibility and Recovery:**
  - Added `GET /api/scans/jobs` with `type`, `prefix`, and `limit` filters so queued/running/completed scan state is observable in UI flows.
  - Added `GET /api/scans/malware/container/{id}/summary` for lightweight per-container malware history retrieval.
  - On backend startup, stale scan jobs left in `running` state are now reconciled to `failed` with a restart-interruption reason, preventing indefinite ghost-running jobs.
  - ClamAV container scans now transition through explicit `queued -> running -> completed/failed/cancelled` states.

### Tests
- Added backend coverage for:
  - locked policy block on `POST /api/updates/run`
  - auto-apply task start/skip behavior
  - duplicate running update rejection in update service
  - scan jobs list endpoint behavior
- Validation commands:
  - `go test ./internal/httpapi ./internal/scanning ./internal/updates`
  - `npm --prefix web run build`

## [0.7.12] - 2026-02-18

### Added
- **AI Spend History Visualization:**
  - Added spend history graphs in Settings -> AI:
    - daily spend (USD) bars
    - cumulative spend (USD) line over the selected span (`24h/7d/30d/90d`)
  - Added compact spend summary labels for selected range start/end and cumulative total.

### Changed
- **AI Usage API Daily Cost Enrichment:**
  - `GET /api/ai/usage` now includes per-day cost fields when pricing is configured:
    - `daily[].estimatedCostUsd`
    - `daily[].cumulativeCostUsd`
  - Daily spend is now calculated from per-day provider/model token usage with configured pricing, rather than a blended approximation.

### Tests
- `go test ./internal/ai ./internal/httpapi ./internal/settings ./internal/updates ./internal/releases` (backend)
- `npm --prefix web run build` (frontend)

## [0.7.11] - 2026-02-18

### Added
- **Compose Audit History Persistence:**
  - Added persistent `compose_audit_history` storage for container compose audit runs, including:
    - effective compose config snapshot
    - normalized markdown analysis
    - sanitized rendered HTML
    - provider/model metadata
    - timestamped history entries
  - Added history APIs:
    - `GET /api/ai/audit-compose/{id}/history`
    - `GET /api/ai/audit-compose/history/{recordID}`

### Changed
- **Compose Analysis Markdown Normalization:**
  - Added backend markdown normalization and rendering pipeline using established Go libraries:
    - `github.com/yuin/goldmark` (Markdown parsing/rendering)
    - `github.com/microcosm-cc/bluemonday` (HTML sanitization)
  - `audit-compose` responses now include both normalized markdown and sanitized HTML payloads for richer UI rendering.
- **Container Configuration UX:**
  - Reworked Compose Doctor output in Container Detail to render prettified markdown (headings, lists, tables, code blocks, links).
  - Added persisted Compose Audit History panel with selectable historical records so users can retrieve previous configs and analyses directly from the UI.

### Tests
- `go test ./internal/ai ./internal/httpapi ./internal/settings ./internal/updates ./internal/releases` (backend)
- `npm --prefix web run build` (frontend)

## [0.7.10] - 2026-02-18

### Added
- **AI Token Usage Telemetry:**
  - Added persistent AI usage event storage (`ai_usage_events`) capturing provider, model, feature, call count, and input/output/total token usage for each AI request.
  - Added `GET /api/ai/usage?span=24h|7d|30d|90d` to return token usage totals, breakdowns, and daily trend data.
- **Optional Cost Estimation:**
  - Added optional settings field `aiPricingJson` to define per-model token pricing manually.
  - Usage endpoint now calculates estimated USD cost when pricing is configured, while continuing to report raw token usage when pricing is not configured.

### Changed
- **Provider Usage Capture:**
  - OpenAI, Anthropic, and Gemini integrations now extract and record upstream token-usage metadata for:
    - release analysis
    - compose audit
    - metrics analysis
    - health log analysis
- **Settings AI UX:**
  - Added AI usage dashboard in Settings -> AI with selectable time span, token totals, feature/model breakdown, and optional estimated cost display.
  - Added editable `AI Pricing JSON` configuration area with token-only fallback behavior when left empty.

### Tests
- `go test ./internal/ai ./internal/httpapi ./internal/settings ./internal/updates ./internal/releases` (backend)
- `npm --prefix web run build` (frontend)

## [0.7.9] - 2026-02-18

### Added
- **Cross-Registry Intelligence Derivation:**
  - Added repository URL derivation fallback from container image references when OCI labels are missing.
  - Added support for deriving repository links from `ghcr.io`, `registry.gitlab.com`, Docker Hub, and Quay image refs.
- **Non-GitHub Release Source Coverage:**
  - Extended release source parsing to understand GitHub, GitLab, and Gitea/Forgejo-style repository/changelog URLs.
  - Added provider-specific changelog fallback URL derivation (`/releases`, `/-/releases`, etc.) for non-GitHub repos.

### Changed
- **Release Context Enrichment Path:**
  - Update runs now attempt release intelligence for any supported repository/changelog reference (not just GitHub owner/repo shorthand).
  - Release context now degrades gracefully with structured fallback text when provider APIs are unavailable or unsupported.
- **AI Upgrade Safety Gate:**
  - AI release analysis now blocks upgrades when any of the following are true:
    - `action_required = true`
    - one or more explicit `breaking_changes` are returned
    - `risk_score` meets/exceeds threshold
  - Added configurable risk threshold via `HW_AI_BLOCK_RISK_THRESHOLD` (default `80`).
  - Added explicit `release_analysis` skip event when no AI provider is configured, so no-AI deployments remain non-blocking and observable.

### Tests
- `go test ./internal/httpapi ./internal/releases ./internal/updates` (backend)

## [0.7.8] - 2026-02-18

### Added
- **Image Repository Intelligence View:**
  - Added `GET /api/docker/images/intelligence` to enrich image inventory with:
    - in-use/outdated/prune-candidate lifecycle status
    - latest Trivy vulnerability totals/severity highlights (when available)
    - malware detection indicators (when available)
  - Added Image Repository security and lifecycle tags so high-risk/outdated/prune-target images are visible at a glance.
- **Image Cleanup Feedback Log:**
  - Added a collapsed cleanup log panel in Image Repository that auto-expands when cleanup is triggered and streams scheduler feedback from system logs.
- **Scan Job Stop Controls:**
  - Added `POST /api/scans/jobs/{id}/cancel` to allow user-initiated cancellation of running Trivy/ClamAV scan jobs.
  - Added stop controls for active scans in Security Suite and Container Security views, with explicit `cancelled` terminal state handling.

### Changed
- **Container Intelligence UX Placement:**
  - Moved Repository URL and Changelog URL override controls from global Settings into a new per-container `intelligence` tab in Container Detail.
  - Added effective/derived metadata display directly in the container context where upgrade decisions are made.
- **Image Reference Normalization (Web UI):**
  - Added a centralized frontend image-reference parser utility and switched Fleet/Images views to use it for consistent repo/tag/digest rendering.
- **Audit Trail Consolidation:**
  - Retired the standalone Audit page in the web UI.
  - System Health now acts as the canonical operational timeline with reusable presets (including an `Audit Trail` preset) plus source/level/search filters.
  - Legacy `audit` navigation now resolves to System Health in audit preset mode.
- **Settings Simplification:**
  - Removed the global Settings `Containers` intelligence tab now that overrides are managed at container level.
- **Scheduler Diagnostics Visibility:**
  - Scheduler start/run/completion/error events now write into diagnostics logs (`/api/system/logs`) instead of stdout-only paths.
  - Docker prune task now logs start/result/error details into diagnostics so System Health and cleanup feedback panels show concrete outcomes.
- **ClamAV Sweep Path Guardrails:**
  - Scheduled malware sweeps now verify host mount source accessibility before queueing scans.
  - Inaccessible mount paths are skipped with diagnostics warnings instead of creating repeated failed scan jobs.

### Fixed
- **System Health Log Searchability:**
  - Added backend `search`/`q` query filtering support for `GET /api/system/logs`.
  - Added System Health log search controls in Diagnostics UI for quick filtering by message/source/level content.
- **System Health Preset Feedback & Classification:**
  - Added explicit active-preset UI feedback, filtered row counts, and refresh status messaging so preset button presses are visibly acknowledged.
  - Added derived log class badges (`Security`, `Automation`, `Updates`, `Audit`, `Error`, `General`) to make event categorization clear at a glance.
  - Fixed preset state synchronization so route params no longer reset manual preset selections back to `All Logs`.
  - Updated empty-state messaging to explain when preset/filter criteria exclude all rows.
- **Image Security Scan Correlation:**
  - Fixed image intelligence matching across tagless, tagged, and digest-style references so scans recorded under `repo` are correctly surfaced for `repo:latest` images (and vice versa).
  - Resolved false `Not scanned` states and incorrect prune-candidate flags caused by strict tag matching.
  - Added default-registry alias normalization (`docker.io` / `index.docker.io` / `registry-1.docker.io`) so running images are no longer incorrectly flagged `Prune Next Run` when container/image refs use different Docker Hub prefix styles.
- **Image Intelligence First-Render Hydration:**
  - Fixed Image Repository startup behavior so intelligence rows are fetched on initial page load even when base image inventory is already preloaded.
  - Resolves missing security/lifecycle tags on first open that previously appeared only after manual refresh.
- **Fleet Inventory Tag Visibility:**
  - Fixed Fleet Inventory image display to prevent tags from being visually lost due truncation by rendering repository and qualifier (`:tag` or digest) separately.
- **Security UI Job Locking:**
  - Fixed Security Suite action locking so completed/cancelled jobs no longer block starting subsequent scans.

### Tests
- `go test ./...` (backend)
- `npm run build` (web)

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
