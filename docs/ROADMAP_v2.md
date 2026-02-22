# HarborWatch Roadmap (v2, Refined)

Last updated: 2026-02-21

This roadmap is intentionally forward-looking. Historical delivery details belong in `CHANGELOG.md`.

## Product Direction

HarborWatch is evolving into an autonomous security and lifecycle controller for containerized environments:

- AI provides analysis and recommendations.
- Deterministic policy and automation enforce outcomes.
- Safety, rollback, and observability are mandatory for all automated actions.

## Guiding Principles

1. AI advises, policy enforces.
2. Safe-by-default behavior over convenience.
3. Explainable automation with auditable decisions.
4. Operational controls must exist at both global and per-container levels.
5. Performance and queue health are first-class quality targets.

## Current Baseline (Already Delivered)

The following are considered baseline capabilities, not roadmap goals:

- AI release-risk gating with block behavior and user-visible AI-blocked signals.
- Automated update workflows with rollback and health-gate pathways.
- Scheduler service with persistent cron config and runtime controls.
- Trivy/ClamAV security scheduling with scope and exclusion controls.
- Per-container lifecycle controls (`auto` / `manual` / `locked`) and automation inheritance.
- Per-container and global safety toggles for bypass-AI and skip-health behaviors.
- Portainer-aware update handling and safety checks.
- Metrics collection, retention controls, and diagnostics snapshots.

## Open Gaps to Prioritize

1. Notification breadth and reliability hardening (Slack/SMTP parity with Discord).
2. Stronger secrets posture for provider/API keys (rotation, provenance, operational guardrails).
3. Larger-fleet scale optimization for scan/update queueing and diagnostics query paths.
4. Clear policy explainability in UI for “why this did/didn’t run.”
5. Runbook-grade operator workflows for blocked upgrades and remediation.

## Milestones

## M1: Policy Explainability & Operator Actions

Goal: Make every automation decision obvious and actionable.

Scope:
- Add reason codes and operator guidance for all skip/block outcomes.
- Normalize event payloads for UI cards, history, and logs.
- Provide one-click next actions for common blocked states.

Acceptance criteria:
- 100% of skip/block outcomes include a stable reason code.
- Container card, run history, and diagnostics logs show identical reason semantics.
- No “silent” no-op scheduler decisions.

## M2: Notification & Incident Routing

Goal: Ensure high-signal events reach operators quickly through preferred channels.

Scope:
- Expand dispatchers to Slack and SMTP parity with Discord.
- Add per-event severity routing and suppression windows.
- Add delivery status and retry telemetry.

Acceptance criteria:
- Critical events delivered with retry policy and visible success/failure status.
- Channel-specific configuration test endpoint in Settings.
- Duplicate alert suppression for repeated identical events within configured window.

## M3: Secrets & Security Hardening

Goal: Raise assurance for key material and privileged operations.

Scope:
- Strengthen app settings secret handling and storage controls.
- Add explicit key-rotation workflow and operator prompts.
- Improve audit trail around settings and integration changes.

Acceptance criteria:
- Key updates are auditable (who/when/what changed).
- Rotation path documented and test-covered.
- No plaintext key leakage in logs, diagnostics, or API echoes.

## M4: Scale & Performance Engineering

Goal: Keep automation reliable on larger hosts and noisier environments.

Scope:
- Improve scheduler task efficiency and queue bounding.
- Further reduce unnecessary scans/inspections.
- Add targeted indexes and query profiling for high-cardinality tables.

Acceptance criteria:
- Sweep tasks avoid queue inflation under cached-image-heavy hosts.
- P95 API latency for core list/detail endpoints remains within target under synthetic load.
- Background task saturation does not block manual operator actions.

## M5: Upgrade Control Plane UX

Goal: Make lifecycle control safer and easier to operate at scale.

Scope:
- Consolidate per-container and global policy interactions in UI copy and behavior.
- Improve blocked-upgrade remediation workflows.
- Add fleet-level views for “manual-only”, “AI-bypassed”, and “skip-health” containers.

Acceptance criteria:
- Operators can answer “what is unmanaged or force-enabled?” in one screen.
- Manual mode behavior is explicit: excluded from scheduler, manual trigger only.
- Policy toggles persist and reflect correctly across refresh/navigation.

## Non-Goals (This Roadmap Cycle)

- Multi-host/cluster orchestration beyond current single-controller focus.
- Full SIEM platform replacement.
- AI-driven autonomous remediation that bypasses deterministic policy gates.

## Engineering Quality Bar

All milestone work should include:

- Regression tests for new policy behavior.
- Backward-safe DB migrations (`ensureColumn`/additive changes, no destructive defaults).
- Changelog updates and operator-facing release notes.
- Verification evidence before completion claims (`go test`, build, and relevant task scripts).

## Success Metrics

Track continuously:

- Blocked-update false-positive rate.
- Rollback success rate for failed automated upgrades.
- Scan/update queue depth over time.
- Mean time to operator action after critical alert.
- P95 latency for container list/detail and diagnostics endpoints.

