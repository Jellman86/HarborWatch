# HarborWatch v2 Roadmap: The Autonomous Security Appliance

This document outlines the technical specification for transforming HarborWatch from a passive monitoring tool into an active, AI-assisted security appliance.

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
