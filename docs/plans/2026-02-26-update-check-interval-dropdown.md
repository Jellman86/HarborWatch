# Update Check Interval Dropdown Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the generic scheduler cadence/time controls for the `container_update_check` automation task with a fixed dropdown of supported intervals (10m, 30m, 60m, 5h, 12h, 24h).

**Architecture:** Keep the backend scheduler unchanged (it already accepts 6-field cron expressions). Implement a task-specific UI branch in `Settings.svelte` with explicit interval<->cron mapping and round-trip parsing, while preserving the existing generic schedule editor for all other tasks.

**Tech Stack:** Svelte 5 (`Settings.svelte`), TypeScript, existing `/api/scheduler/update` endpoint

---

### Task 1: Add update-check interval mapping helpers

**Files:**
- Modify: `web/src/lib/pages/Settings.svelte`

**Step 1: Write the failing test**

No frontend test runner is currently configured in `web/package.json`. Use build verification instead and keep logic small/pure.

**Step 2: Run test to verify it fails**

N/A (no test harness configured for the Svelte frontend).

**Step 3: Write minimal implementation**

Add:
- fixed interval option list
- interval -> cron mapping helpers
- cron -> interval parsing helper (with safe fallback)
- task-specific dirty/save logic for `container_update_check`

**Step 4: Run test to verify it passes**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`
Expected: build succeeds

**Step 5: Commit**

User did not request a commit in this task.

### Task 2: Replace the update-check schedule controls with a dropdown

**Files:**
- Modify: `web/src/lib/pages/Settings.svelte`

**Step 1: Write the failing test**

N/A (frontend test harness not configured).

**Step 2: Run test to verify it fails**

N/A.

**Step 3: Write minimal implementation**

Render a task-specific control block when `task.id === "container_update_check"`:
- dropdown with the requested six options only
- explanatory help text
- save button using existing scheduler update API

Keep generic cadence/time UI unchanged for every other task.

**Step 4: Run test to verify it passes**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`
Expected: build succeeds

**Step 5: Commit**

User did not request a commit in this task.
