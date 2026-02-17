# HarborWatch v2 Roadmap: The Autonomous Security Appliance

This document outlines the technical specification for transforming HarborWatch from a passive monitoring tool into an active, AI-assisted security appliance.

## Implementation Status (2026-02-17)

- Milestones 6-8 core paths are implemented in production code.
- Hardening follow-up completed for:
  - AI update gate enforcement
  - deterministic rollback behavior
  - scheduler cron normalization
  - SSE stability improvements
  - dispatcher deduplication/replacement semantics
  - batched metrics endpoint for inventory sparklines
  - expanded diagnostics API (snapshot bundle + container log retrieval)
  - API-level 4xx/5xx telemetry logging into internal diagnostics logs
- Remaining roadmap items are primarily deeper optimization and feature expansion tracks.

## Core Philosophy: "AI Advises, Policy Enforces"
To ensure robustness, AI will never directly execute privileged commands. Instead, it acts as an **Intelligence Oracle**, flagging risks and setting "Blockers" that the deterministic Update Engine obeys.

---

## Milestone 6: AI Intelligence Core
**Goal:** Replace heuristics with semantic understanding using a user-provided LLM API Key (OpenAI/Anthropic/Gemini).

### 6.1. AI Provider Abstraction
*   **Architecture:** Create `internal/ai` package with a standard interface (`Ask`, `Analyze`).
*   **Security:** API Keys stored encrypted in SQLite or provided via ENV.
*   **Feature:** `AnalyzeReleaseNotes(notes string) -> (RiskLevel, BreakingChanges[])`
    *   **Logic:** Feed release notes to LLM. Ask specifically for "breaking changes," "config format changes," and "security patches."
    *   **Outcome:** If `RiskLevel > High`, set a `Lock` on the container update.

### 6.2. Compose Configuration Analyst
*   **Feature:** "Compose Doctor"
*   **Input:** User pastes `docker-compose.yml` or points to a file.
*   **Analysis:** AI scans for:
    *   Privileged mode usage.
    *   Missing resource limits (CPU/RAM).
    *   Insecure volume mounts.
    *   Network isolation issues.
*   **Output:** Actionable remediation list in the UI.

---

## Milestone 7: Automation & Maintenance Scheduler
**Goal:** "Set and Forget" operations with robust concurrency control.

### 7.1. The Scheduler Service
*   **Architecture:** A persistent `CronService` in Go (not just `time.Ticker`).
*   **Persistence:** Schedule configs stored in SQLite (e.g., `0 3 * * *` for scans).
*   **Throttling:** Ensure scans don't run simultaneously to prevent I/O saturation.

### 7.2. Automated Maintenance Tasks
*   **System Prune:** `docker system prune` scheduler (dangling images/volumes).
    *   *Safety:* Configurable "Keep last X images" policy.
*   **Security Sweeps:**
    *   Scheduled Trivy scans for all running containers.
    *   Scheduled ClamAV scans for host mounts.

### 7.3. The "Smart" Auto-Update Loop
*   **Workflow:**
    1.  **Check:** Scheduler triggers update check.
    2.  **Intel:** Fetch Release Notes -> Send to **AI Core**.
    3.  **Gatekeep:**
        *   If **Safe**: Proceed to Safe Update Pipeline (Milestone 4).
        *   If **Risky**: Pause update, notify user, create "Approval Request" in UI.
    4.  **Result:** Success, Rollback, or "Pending Approval."

---

## Milestone 8: Ecosystem Integrations
**Goal:** Play nicely with existing homelab tools.

### 8.1. Notification Dispatcher
*   **Channels:** Discord (Webhooks), Slack, Email (SMTP), Gotify.
*   **Triggers:**
    *   "Critical Vulnerability Found"
    *   "Update Blocked by AI (Breaking Change Detected)"
    *   "Malware Detected on Host"

### 8.2. Portainer Integration
*   **Mechanism:** Portainer API Client.
*   **Features:**
    *   **Stack Discovery:** Read Portainer Stacks to understand logical groupings of containers.
    *   **Sync:** When HarborWatch updates a container, optionally trigger a Portainer webhook to keep its state in sync (preventing Portainer from reverting the change).

---

## Implementation Priority

1.  **Refactor:** Create `internal/scheduler` and `internal/ai`.
2.  **Feature:** Implement AI Release Analysis (High value, low risk).
3.  **Feature:** Build the Scheduler & Prune tasks.
4.  **Feature:** Connect AI Gatekeeper to Update Pipeline.
5.  **Feature:** Notifications.

## Technical Requirements
*   **Database:** Add `schedules`, `ai_analysis_cache`, and `app_settings` tables.
*   **UI:** New "Automation" and "Integrations" pages.
*   **Network:** Outbound access to LLM APIs (OpenAI/etc) required.

---

## Data Acquisition & Fallback Strategy

### 1. How we find Release Intelligence
The system follows a tiered discovery process for each container:
1.  **Label Inspection:** Read `org.opencontainers.image.source` from image metadata.
2.  **Registry API:** Query the registry (GHCR/DockerHub) for linked repository metadata.
3.  **Manual Input (Primary Fallback):** Users can define a `HW_INTEL_URL` label on their containers or set it via the UI.
4.  **Heuristic Guessing:** Match `author/image` patterns against known repository formats.
5.  **Hard Fallback:** If no URL is found, AI features are disabled for that container, and the system defaults to "Manual Review Required" for updates.

### 2. How we handle AI Failures
- **Timeout/API Error:** If the LLM provider is down, HarborWatch defaults to the most restrictive policy: "Pause Update."
- **Ambiguous Notes:** If the AI cannot determine risk, it flags the update as "Needs Human Eye."

### 3. How we Discover Scan Targets
- **Vulnerabilities:** Iterates through `docker ps` and passes the image ID to Trivy.
- **Malware:** Parses `docker inspect` for `Mounts[].Source`. It then runs ClamAV recursively on those host paths.
- **Compose Audits:** Users can specify a `COMPOSE_PATH` directory in Settings. HarborWatch will walk this directory to find `.yml` files for AI auditing.

### 4. Portainer Sync Logic
- **Discovery:** System matches container names/labels against the Portainer `/api/stacks` endpoint.
- **Update Hook:** When HarborWatch updates a container, it will optionally trigger the Portainer Stack Webhook (if defined) to ensure Portainer's internal DB stays in sync with the live container state.

---

## Milestone 9: Performance Profiler & AI Diagnostics
**Goal:** Proactively identify resource exhaustion and optimize container efficiency.

### 9.1. The Metrics Collector
*   **Mechanism:** Stream `docker stats --no-stream` for all active containers every 60 seconds.
*   **Data Points:** CPU %, Memory Usage (current/max), Memory Limit, Network I/O, Block I/O.
*   **Persistence:** Store in a `container_metrics` table with a **retention policy** (e.g., 7 days of raw data, 30 days of hourly averages).

### 9.2. Resource Exhaustion Alerts
*   **Feature:** "Early Warning System."
*   **Logic:** If a container's Memory Usage > 90% of its Limit for 3 consecutive checks, trigger a "Near-OOM" alert.
*   **Diagnosis:** If a container crashes, HarborWatch provides a "Crash Context" report showing its resource trends in the 5 minutes leading up to the failure.

### 9.3. AI Performance Consultant
*   **Feature:** "Analyze My Resources."
*   **Prompting:** Pass a 24-hour metric window to the AI.
*   **Insight:** AI identifies "Memory Leaks" (steady linear growth) vs. "Expected Spikes" and suggests optimized `mem_limit` and `cpu_shares` values for your `docker-compose.yml`.

### 9.4. Visual Profiler UI
*   **Dashboard Integration:** Small "Sparkline" charts next to each container in the Inventory.
*   **Deep Dive View:** A dedicated page with interactive charts (ApexCharts) to overlay metrics from multiple containers to find cross-service bottlenecks.
