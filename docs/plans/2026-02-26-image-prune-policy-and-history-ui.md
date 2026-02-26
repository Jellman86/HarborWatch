# Image Prune Policy And Cleanup History UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a scheduled prune policy setting (default Docker behavior), a manual “Delete Unused Images” action in Images view, and an always-visible expandable cleanup activity/history UI backed by persisted diagnostics logs.

**Architecture:** Extend the Docker prune scheduler task to support two image prune modes (dangling-only vs all-unused) and register a hidden manual-only aggressive prune task. Persist the scheduled mode as an app setting, expose it in Settings (Maintenance), and add a dedicated Images button plus a two-column cleanup activity/history section that reads persisted scheduler logs from `/api/system/logs`.

**Tech Stack:** Go backend (scheduler/httpapi/settings), Svelte 5 frontend (`Settings.svelte`, `Images.svelte`), existing diagnostics log store/API

---

### Task 1: Backend prune modes and endpoints

**Files:**
- Modify: `backend/internal/scheduler/tasks.go`
- Modify: `backend/internal/httpapi/server.go`

**Step 1: Write the failing test**

Add/extend a small scheduler unit test for prune mode selection helper (if helper is introduced). If impractical due Docker client coupling, verify via build and targeted runtime checks.

**Step 2: Run test to verify it fails**

Run: `go test ./backend/internal/scheduler -run TestDockerPrune -v`
Expected: fails before implementation (or no test if not added)

**Step 3: Write minimal implementation**

- Support `dangling-only` and `all-unused` image prune modes
- Keep `docker_system_prune` defaulting to Docker standard behavior
- Register a hidden manual task for aggressive unused-image cleanup
- Add `/api/docker/prune-unused` endpoint

**Step 4: Run test to verify it passes**

Run: `go test ./backend/internal/scheduler ./backend/internal/httpapi -run TestDockerPrune -v`
Expected: pass (or no matching tests + package compile success)

**Step 5: Commit**

Not requested.

### Task 2: Settings persistence and Automation checkbox

**Files:**
- Modify: `backend/internal/settings/store.go`
- Modify: `backend/internal/gen/types_gen.go`
- Modify: `web/src/lib/api-types.ts`
- Modify: `web/src/lib/pages/Settings.svelte`

**Step 1: Write the failing test**

Prefer a settings-store roundtrip test if one exists; otherwise verify via build and API/manual behavior.

**Step 2: Run test to verify it fails**

Run: `go test ./backend/internal/settings -v`
Expected: compile/test failure before field wiring (if tests compile path touches the new field)

**Step 3: Write minimal implementation**

- Add boolean setting (default false) for scheduled prune including unused tagged images
- Persist/load via `app_settings`
- Expose checkbox in Maintenance Automation UI

**Step 4: Run test to verify it passes**

Run: `go test ./backend/internal/settings -v`
Expected: pass

**Step 5: Commit**

Not requested.

### Task 3: Images page manual button and persistent cleanup history UI

**Files:**
- Modify: `web/src/lib/pages/Images.svelte`

**Step 1: Write the failing test**

No frontend test runner configured in `web`; use build verification and live API checks.

**Step 2: Run test to verify it fails**

N/A.

**Step 3: Write minimal implementation**

- Add `Delete Unused Images` button (manual aggressive endpoint)
- Keep existing standard cleanup button
- Fix diagnostics log response parsing (`{ logs, total }`)
- Add always-visible, expandable two-column cleanup activity UI:
  - console history
  - discrete cleanup jobs with expandable details
- Preload from persisted scheduler logs on page load

**Step 4: Run test to verify it passes**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`
Expected: build succeeds

**Step 5: Commit**

Not requested.
