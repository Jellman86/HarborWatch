# Upgrade Automation

This document describes HarborWatch's upgrade automation architecture and how policy, scheduling, and UI controls connect.

## Design Goals

- Separate update detection from update execution.
- Keep per-container control explicit.
- Block unsafe runs before mutating workloads.
- Keep all decisions and outcomes observable in System Health and lifecycle history.

## Pipeline Overview

HarborWatch runs upgrade automation in two scheduler stages:

1. `container_update_check`
   - Detects available image updates by comparing local and remote image metadata.
   - Updates `updateAvailable` state used by Fleet and container detail views.

2. `container_update_apply` (optional, disabled by default)
   - Evaluates containers that currently report `updateAvailable=true`.
   - Builds a full update request from container/runtime metadata.
   - Applies policy gates (`auto`, `manual`, `locked`) before starting a job.
   - Starts the standard update engine pipeline only for eligible containers.

Update execution itself is unchanged and still runs:

1. `preflight`
2. `release_analysis` (AI, when configured)
3. `backup`
4. `pull`
5. `recreate`
6. `validate`
7. `ai_health_assessment` (optional)
8. `rollback` (on failure)

## Lifecycle and Policy Model

HarborWatch uses a simplified lifecycle model to manage how updates are applied:

- **Automatic**
  - Follows global automation settings.
  - If the `container_update_apply` schedule is enabled, updates will be applied automatically.
  - Effectively sets `updatePolicy` to `auto` and enables automation inheritance.
- **Manual**
  - Automatic application is disabled for this container.
  - Users must explicitly trigger updates via the UI.
  - Effectively sets `updatePolicy` to `manual` and disables automation inheritance.
  - **Force Update** and **Bypass AI** options are available for manual triggers.

## Global Safety Overrides (Watchtower Mode)

You can globally override safety features in **Settings > Automations > Upgrades**:

- **Global AI Bypass**: Disables AI breaking-change analysis for ALL containers.
- **Watchtower Mode (Skip Health)**: Disables post-update health checks and rollbacks for ALL containers.

These settings are useful for "rolling" release projects (like `nightly` builds) where traditional release notes are unavailable or health checks are unreliable.

## Automation Scoping and Exclusions

Upgrade automation is allowed only when all of the following pass:

- Container is not in global automation ignore list.
- Upgrade domain is enabled for the relevant task.
- Container lifecycle automation inheritance/override allows upgrades.
- Policy gate allows execution (`auto` for scheduled apply).

## Duplicate and Retry Safeguards

The update service prevents duplicate in-flight runs:

- If the latest job for a container is `running`, a new start request is rejected.

The auto-apply scheduler also enforces retry backoff:

- If the latest container job ended in `failed` or `rolled_back`, HarborWatch waits for a cooldown window before retrying.

## Configuration

### Scheduler Tasks

- `container_update_check` (default enabled)
- `container_update_apply` (default disabled)

These are managed in Settings -> Automations -> Upgrades.

### Environment Variables

- `HW_AUTO_UPGRADE_MAX_CONCURRENCY`
  - Max containers started per `container_update_apply` run.
  - Default: `1`
- `HW_AUTO_UPGRADE_MIN_RETRY_MINUTES`
  - Cooldown before retrying auto-apply after failed/rolled-back runs.
  - Default: `60`
- `HW_AI_BLOCK_RISK_THRESHOLD`
  - AI risk score threshold that blocks update execution during `release_analysis`.
  - Default: `80`

### Settings UI Mapping

These runtime values can be configured from Settings unless pinned by environment overrides:

- `aiBlockRiskThreshold` -> Settings -> AI
- `autoUpgradeMaxConcurrency` -> Settings -> Automations -> Upgrades
- `autoUpgradeMinRetryMinutes` -> Settings -> Automations -> Upgrades

## UI Hooks

### Settings -> Automations -> Upgrades

- Shows both scheduler tasks:
  - update detection
  - auto-apply execution
- Supports run-now, enable/disable, and cron schedule edits.

### Container Detail -> Lifecycle

- Choice between **Automatic** and **Manual** modes.
- **Manual Mode** exposes additional safety toggles:
  - **Bypass AI Assessment**: Skip release analysis for this run.
  - **Force Update (Skip Health)**: Skip post-update health validation.
- **Trigger Upgrade**: Starts the update flow in the background while you remain on the page.
- **Breaking Change Signals**: Historical list of past AI alerts for the container.
- Lifecycle log and AI risk outcomes remain visible for auditability.

## Behavior Without AI

If no AI provider is configured:

- `release_analysis` step is marked as skipped.
- Update pipeline continues without AI risk gating.
- Auto-apply remains functional using non-AI safeguards.

## Persistence

Upgrade jobs, steps, and AI analysis summaries are stored in SQLite (`update_runs`, `update_steps`) and survive container restarts/upgrades as long as persistent storage is retained.
