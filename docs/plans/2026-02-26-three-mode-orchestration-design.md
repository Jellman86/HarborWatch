# Three-Mode Orchestration Design

**Date:** 2026-02-26

## Goal
Establish HarborWatch as explicitly aware of three container operation modes (`plain_docker`, `docker_compose`, `portainer_stack`) across backend APIs, UI, and update automation pipelines.

## Problem
HarborWatch currently distinguishes Portainer-managed containers for safety, but non-Portainer Docker Compose containers are only partially handled:
- Compose config can often be detected and previewed
- Manual updates fall back to single-container local recreate
- Automated auto-apply intentionally skips compose-managed containers without verifiable Portainer source authority

This creates a mismatch between what HarborWatch can detect and what it can automate.

## Design Principles
- Separate **management mode** from **source authority** and **automation capability**
- Keep automation triggers **per-container** for now (user-approved)
- Add **project-aware UI** for local Compose on the Stacks page
- Preserve strict safety gating for automated changes
- Build a foundation that can evolve to project-level Compose automation later

## Canonical Concepts

### 1. Orchestration Mode
Canonical mode enum for every container:
- `plain_docker`
- `docker_compose`
- `portainer_stack`

### 2. Compose Source Authority (for compose-managed containers)
Separate from mode. Describes whether HarborWatch can safely act on compose source files:
- `unverified` - source file cannot be located/read reliably
- `verified_readonly` - source file readable but not writable
- `verified_writable` - source file readable and writable
- `drifted` - runtime/source divergence detected (future phase)

### 3. Automation Capability (UI-facing)
Derived explanation for lifecycle UI and stacks UI:
- manual supported / unsupported
- auto-apply supported / blocked + reason
- executor path (`local`, `compose`, `portainer`)

## Detection Rules (Backend)

### Portainer Stack
Classify as `portainer_stack` when either:
- `io.portainer.stack_id` label exists, or
- compose label paths indicate Portainer-managed path (`/data/compose/<id>/...`)

### Local Docker Compose
Classify as `docker_compose` when:
- `com.docker.compose.project` and `com.docker.compose.service` labels exist, and
- container is not Portainer-managed by the rules above

### Plain Docker
Fallback when neither Compose nor Portainer rules apply.

## Backend API Changes (Phased)

### Phase 1 (this implementation slice)
- Add reusable mode detection helpers in `httpapi`
- Extend container intel response with mode metadata and compose labels (project/service)
- Add local Compose project discovery endpoint for the Stacks page (read-only metadata first)
- Expose source readability/writability status for local compose projects

### Phase 2
- Add source authority/capability fields to update request builder outputs and lifecycle API surfaces
- Tailor lifecycle messaging and button states by mode/capability

### Phase 3
- Add `executeCompose` pipeline for local Compose service updates
- Enable manual Compose updates through compose-aware execution path
- Enable auto-apply for local Compose only when source is verified+writable

## UI Changes

### Container Lifecycle (mode-aware)
- Show operation mode badge: Docker / Compose / Portainer
- Show capability explanation near upgrade actions
- Use compose-specific wording when mode is `docker_compose`

### Stacks Page (project-aware)
- Keep Portainer stack cards
- Add Local Compose Projects section (or tabs) with:
  - project name
  - config file path(s)
  - working dir
  - member services/containers
  - source authority status
  - automation capability summary (phase 2+ richer)

## Local Compose Automation Constraint (Approved)
HarborWatch will require a configured host mount / compose root mapping for reliable local Compose automation in future phases. Label-only best-effort automation is not the default safety model.

## Risks / Notes
- Compose labels vary by runtime/version; detection should be tolerant and conservative
- Readable/writable host paths depend on HarborWatch container mounts; UI must explain when paths are not accessible from the appliance
- Project-level UI and per-container automation can coexist without implying project-level atomic rollouts (for now)
