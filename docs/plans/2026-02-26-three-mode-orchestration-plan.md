# Three-Mode Orchestration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add explicit Docker/Compose/Portainer mode detection and local Compose project visibility in the UI, laying the foundation for mode-specific automation pipelines.

**Architecture:** Introduce a shared backend orchestration-mode classifier and local Compose project discovery helper, expose new metadata via container intel and a `/api/compose/projects` endpoint, and update `Stacks`/`ContainerPage` to render mode-aware UI. Compose execution pipeline changes are intentionally deferred to a later phase.

**Tech Stack:** Go (Chi HTTP API), Svelte 5 frontend, Docker labels, HarborWatch existing settings/intel/update services.

---

### Task 1: Backend mode classification helpers (TDD)

**Files:**
- Create: `backend/internal/httpapi/orchestration_mode.go`
- Create: `backend/internal/httpapi/orchestration_mode_test.go`

**Steps:**
1. Write failing unit tests covering `plain_docker`, local `docker_compose`, and `portainer_stack` label combinations.
2. Run: `go test ./internal/httpapi -run TestDetectContainerOrchestrationMode -v`
3. Implement minimal detection helpers and label parsing utilities.
4. Re-run targeted tests.
5. Commit (optional checkpoint).

### Task 2: Local Compose project discovery + source access status (TDD)

**Files:**
- Update: `backend/internal/httpapi/orchestration_mode.go`
- Update: `backend/internal/httpapi/orchestration_mode_test.go`

**Steps:**
1. Add failing tests for local compose grouping and source status (`unverified`, `verified_readonly`, `verified_writable`) using temp files.
2. Run targeted tests and verify failure.
3. Implement grouping and file access checks.
4. Re-run targeted tests.

### Task 3: Expose local Compose projects API

**Files:**
- Create: `backend/internal/httpapi/routes_compose.go`
- Update: `backend/internal/httpapi/server.go`
- Update: `backend/internal/httpapi/routes_admin.go` (if deps/types need expansion)
- Optional test: `backend/internal/httpapi/routes_compose_test.go`

**Steps:**
1. Add `/api/compose/projects` route returning local compose projects from Docker container list.
2. Register route in admin API router.
3. Return stable/sorted response for deterministic UI.
4. Run targeted `httpapi` tests.

### Task 4: Extend container intel response with mode metadata

**Files:**
- Update: `backend/internal/httpapi/container_intel.go`
- Optional tests: `backend/internal/httpapi/*intel*_test.go`

**Steps:**
1. Add mode + compose project/service + local compose source capability fields.
2. Populate from shared detection helper (avoid duplicated Portainer checks).
3. Run `go test ./internal/httpapi`.

### Task 5: Container page mode-aware lifecycle UI

**Files:**
- Update: `web/src/lib/pages/ContainerPage.svelte`

**Steps:**
1. Extend local `ContainerIntelRecord` type with new backend fields.
2. Add mode badge rendering (`Docker`, `Compose`, `Portainer`).
3. Add lifecycle helper text tailored to mode/source capability.
4. Build frontend: `npm --prefix web run build`.

### Task 6: Stacks page local Compose project section + UI tailoring

**Files:**
- Update: `web/src/lib/pages/Stacks.svelte`

**Steps:**
1. Fetch `/api/compose/projects` alongside Portainer stacks.
2. Render Local Compose Projects section with source status and member services.
3. Preserve existing Portainer stack controls.
4. Build frontend and check runtime shape assumptions.

### Task 7: Verification + documentation updates

**Files:**
- Update: `docs/plans/2026-02-26-three-mode-orchestration-plan.md` (if needed)

**Steps:**
1. Run backend tests: `go test ./internal/httpapi`
2. Run frontend build: `npm --prefix web run build`
3. Summarize what shipped vs deferred (compose execution pipeline still pending).
4. Commit and push when approved.
