# Compose Source-of-Truth Update Automation Design

## Goal
Make HarborWatch treat compose/Portainer stack configuration as the source of truth for automated upgrades, preventing auto-update actions that would diverge from declared compose image references.

## Problem
HarborWatch currently auto-applys updates using the container runtime image reference and digest-based update availability. This is safe for mutable tags, but it can hide or amplify drift relative to the compose source if a container is managed by Compose/Portainer. The user expectation is that image/tag policy is defined in compose files, not inside HarborWatch.

## Design Summary
- Keep **manual updates** unchanged (escape hatch; operator-controlled).
- Enforce **strict compose-source verification for automated upgrades only**.
- For compose-managed containers:
  - Resolve the declared image from the compose source when possible (Portainer stack file + compose service label).
  - Block automated upgrades if the runtime image differs from the declared compose image (drift detected).
  - Block automated upgrades if the chosen target image differs from the declared compose image (no retag divergence).
- If the container is compose-managed but HarborWatch cannot verify the compose source, block auto-update (strict mode) and log a clear skip reason.

## Scope (v1)
- Backend only enforcement in auto-apply pipeline (`automatedUpdateApplyTask` via `buildUpdateRequestForContainer`).
- Portainer stack-backed compose source resolution using `GetStackFile` and compose labels (`com.docker.compose.service`, stack id labels/path).
- Strict skip behavior with explicit error reasons and auto-apply skip accounting.
- Tests for drift detection and source-unavailable blocking.

## Out of Scope (follow-up)
- UI drift badges / warnings in Container page and Dashboard
- Local docker-compose file source parsing from host paths
- Registry tag candidate discovery / advisory lanes
- User-configurable enforcement toggle (hardcoded strict mode for now)

## Comparison Semantics
Use normalized image-reference equivalence (registry aliases and omitted `:latest` handled) instead of raw string equality to avoid false positives such as:
- `nginx` vs `docker.io/library/nginx:latest`
- `docker.io/nginx:1.27` vs `nginx:1.27`

## Failure / Safety Behavior
- Compose source unavailable => **skip auto-update** (safe default)
- Source parse error => **skip auto-update**
- Drift detected => **skip auto-update** and log reason
- Manual updates remain allowed and unchanged

## Testing Strategy
- Unit tests in `backend/internal/httpapi/update_apply_task_test.go` for:
  - Portainer stack source matches runtime => auto starts
  - Portainer stack source differs from runtime => auto skipped
  - Compose-managed source unavailable => auto skipped
- Keep existing auto-apply tests green to confirm non-compose behavior remains unchanged
