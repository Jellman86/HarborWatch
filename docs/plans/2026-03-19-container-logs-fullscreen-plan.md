# Full-Screen Container Logs View Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the Fleet logs modal with a dedicated full-screen container logs page that supports live polling controls such as pause, resume, refresh, tail size, and timestamps.

**Architecture:** Add a new frontend route and page component that reuse the existing Docker logs API, then update Fleet actions to navigate into the new page instead of opening an overlay. Keep the backend unchanged and scope state management to the page component.

**Tech Stack:** Svelte 5, TypeScript, HarborWatch SPA router, existing Docker logs API

---

### Task 1: Add route and page shell

**Files:**
- Modify: `web/src/App.svelte`
- Create: `web/src/lib/pages/ContainerLogsPage.svelte`

**Step 1: Write the failing behavior target**

- Add the new route branch in `App.svelte`.
- Stub a page component export so the app can navigate to `container-logs`.

**Step 2: Run build to verify the route shell compiles**

Run: `npm --prefix web run build`

Expected: build passes with the new route and page shell.

### Task 2: Implement the dedicated logs page

**Files:**
- Modify: `web/src/lib/pages/ContainerLogsPage.svelte`

**Step 1: Implement page behavior**

- Resolve selected container metadata from the shared fleet list.
- Fetch `/api/docker/{id}/logs` with configurable `tail` and `timestamps`.
- Add page controls for pause, resume, refresh, tail size, and timestamps.
- Auto-scroll only when the viewer is live and near the bottom.

**Step 2: Run build to verify the page compiles**

Run: `npm --prefix web run build`

Expected: build passes.

### Task 3: Replace the Fleet modal flow

**Files:**
- Modify: `web/src/lib/pages/Containers.svelte`

**Step 1: Remove modal state and markup**

- Delete modal-specific state and polling logic from `Containers.svelte`.
- Replace the Fleet logs button handler with `onNavigate("container-logs", { id: c.id })`.

**Step 2: Run build to verify the Fleet page still compiles**

Run: `npm --prefix web run build`

Expected: build passes and unused modal code is removed.

### Task 4: Final verification

**Files:**
- Modify if needed based on failures

**Step 1: Run final verification**

Run: `npm --prefix web run build`

Expected: PASS

**Step 2: Manual smoke checklist**

- Fleet logs button navigates to the full-screen logs page.
- Pause halts polling without clearing the current output.
- Resume restarts polling.
- Back returns to Fleet.
