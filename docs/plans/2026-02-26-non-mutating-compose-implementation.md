# Non-Mutating Local Compose Snapshots Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Pivot local Compose upgrades to a non-mutating compose-aware flow and add HarborWatch-managed compose/.env snapshot + change detection visibility for non-git users.

**Architecture:** Keep orchestration mode detection and compose project grouping, but remove compose file write/restore behavior from the update executor. Introduce a snapshot service/helper used by local Compose updates and compose project discovery to persist zip archives and expose latest snapshot/change status metadata to the UI.

**Tech Stack:** Go backend (`net/http`, `archive/zip`, existing settings/store/httpapi/updates packages), Svelte frontend.

---

### Task 1: Add failing tests for non-mutating local Compose update execution

**Files:**
- Modify: `backend/internal/updates/service_test.go`
- Modify: `backend/internal/updates/executor_test.go`

**Step 1: Write failing test**
- Add a test asserting local Compose pipeline calls compose-specific pull/up path without compose file mutation requirements.
- Add a test asserting compose rollback does not attempt file restore path in non-mutating mode.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/updates -run 'TestUpdatePipelineComposeModeUsesComposeApply|TestComposeRollback'`

Expected: FAIL because current executor still requires compose YAML patch/backup flow.

**Step 3: Write minimal implementation**
- Refactor executor/service compose path to support non-mutating compose pull/up.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/updates -run 'TestUpdatePipelineComposeModeUsesComposeApply|TestComposeRollback'`

Expected: PASS

### Task 2: Add failing tests for snapshot archive and manifest generation

**Files:**
- Create: `backend/internal/httpapi/compose_snapshots_test.go`
- Modify: `backend/internal/httpapi/orchestration_mode_test.go`

**Step 1: Write failing tests**
- Snapshot creation includes compose files + project `.env` (if present) + `manifest.json`
- Snapshot status reports `none/unchanged/changed`

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/httpapi -run 'TestCreateComposeSnapshot|TestDetectComposeSnapshotStatus'`

Expected: FAIL because snapshot helpers/status fields do not exist.

**Step 3: Write minimal implementation**
- Add snapshot helpers and integrate status into local compose project discovery.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/httpapi -run 'TestCreateComposeSnapshot|TestDetectComposeSnapshotStatus'`

Expected: PASS

### Task 3: Implement backend non-mutating compose execution + snapshot capture

**Files:**
- Modify: `backend/internal/updates/executor.go`
- Modify: `backend/internal/updates/service.go`
- Modify: `backend/internal/httpapi/update_request_builder.go`
- Modify: `backend/internal/httpapi/routes_updates.go` (error messaging if needed)
- Modify: `backend/internal/httpapi/compose_source_authority.go`

**Step 1: Remove compose YAML patch/restore path**
- Replace compose apply logic with compose pull + compose up.
- Preserve compose command file list/working dir support.

**Step 2: Tighten gating**
- Require target image to match declared compose ref for local compose manual + auto updates.
- Return clear precondition error explaining compose source must be changed manually when pinned.

**Step 3: Add snapshot creation hook**
- Create snapshot before compose pull/up.
- Fail compose update if snapshot creation fails.

**Step 4: Run focused tests**

Run: `go test ./internal/updates ./internal/httpapi -run 'Compose|Snapshot'`

Expected: PASS

### Task 4: Add snapshot metadata/change status to local compose API responses

**Files:**
- Modify: `backend/internal/httpapi/orchestration_mode.go`
- Modify: `backend/internal/httpapi/routes_compose.go`
- Modify: `backend/internal/httpapi/container_intel.go`
- Modify: `backend/internal/settings/store.go` (snapshot root setting if needed)
- Modify: `backend/internal/gen/types_gen.go` and `web/src/lib/api-types.ts` (if settings/API types change)

**Step 1: Add snapshot metadata structures**
- Latest snapshot timestamp/path/id
- snapshot status (`none/unchanged/changed`)
- changed file count

**Step 2: Implement snapshot root configuration**
- Add settings field with sensible default under HarborWatch data path.

**Step 3: Populate local compose projects + container intel compose status**

**Step 4: Run tests**

Run: `go test ./internal/httpapi ./internal/settings`

Expected: PASS

### Task 5: Update UI for compose policy + snapshot/change visibility

**Files:**
- Modify: `web/src/lib/pages/ContainerPage.svelte`
- Modify: `web/src/lib/pages/Stacks.svelte`
- Modify: `web/src/lib/pages/Settings.svelte`
- Modify: `web/src/lib/api-types.ts`

**Step 1: Settings**
- Add compose snapshot root path setting field (with helper text).

**Step 2: Container Lifecycle**
- Add explicit non-mutating compose policy helper text.
- Improve blocked message for compose source mismatch.

**Step 3: Stacks (Local Compose Projects)**
- Render snapshot status, changed file count, last snapshot time, and snapshot path hint.

**Step 4: Verify frontend**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`

Expected: PASS

### Task 6: Full verification and review

**Files:**
- No code changes expected

**Step 1: Run backend tests**

Run: `go test ./internal/httpapi ./internal/updates ./internal/settings`

Expected: PASS

**Step 2: Run web build**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`

Expected: PASS

**Step 3: Review for second-/third-order effects**
- Confirm no compose file writes remain in upgrade pipeline
- Confirm snapshot failure blocks compose update
- Confirm pinned compose updates are surfaced but blocked
