# Lifecycle AI Block Force-Once Override Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a one-time "Force Upgrade Once" action in the container Lifecycle tab that appears only when the latest upgrade attempt was blocked by AI release analysis.

**Architecture:** Frontend-only change in `ContainerPage.svelte`. Detect AI-blocked state from `activeUpdateJob` and `updateHistory`, render an inline warning card, and call the existing `/api/updates/run` endpoint with a per-request `bypassAi: true` override without changing the saved container lifecycle rules.

**Tech Stack:** Svelte 5, TypeScript, existing HarborWatch update API

---

### Task 1: Detect AI-blocked update state in Lifecycle UI

**Files:**
- Modify: `web/src/lib/pages/ContainerPage.svelte`

**Step 1: Write the failing test**

No frontend test runner is configured for `web`. Use targeted build verification after implementation.

**Step 2: Run test to verify it fails**

N/A (no configured frontend unit test harness).

**Step 3: Write minimal implementation**

Add helpers to:
- detect `AI blocked update:` from update job error/step messages
- extract the block reason
- identify the latest lifecycle update job (active job preferred if newer)
- expose a conditional blocked-state object for UI rendering

**Step 4: Run test to verify it passes**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`
Expected: build succeeds

**Step 5: Commit**

Not requested.

### Task 2: Add one-time force-upgrade action

**Files:**
- Modify: `web/src/lib/pages/ContainerPage.svelte`

**Step 1: Write the failing test**

N/A (no frontend unit test harness).

**Step 2: Run test to verify it fails**

N/A.

**Step 3: Write minimal implementation**

- Extend manual upgrade trigger to support a per-run `forceBypassAiOnce` option.
- Add confirmation prompt for the force-once action.
- Render a Lifecycle warning banner only when the latest update is AI-blocked.
- Add `Force Upgrade Once` button in the banner; do not persist `bypassAi`.

**Step 4: Run test to verify it passes**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`
Expected: build succeeds

**Step 5: Commit**

Not requested.
