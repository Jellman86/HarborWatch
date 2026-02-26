# Settings and Lifecycle Consistency Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make lifecycle rules persist reliably, add clear dirty/save state mechanics, and improve automation default-policy information architecture in Settings.

**Architecture:** Fix lifecycle rule persistence in the backend save route by persisting user-entered rules without applying effective/default expansion on save. In the frontend, introduce snapshot-based dirty detection for `ContainerPage` lifecycle controls and the global `Settings` object, then restructure the Upgrade Automation policy UI into a distinct, visually differentiated default-policy section with clearer CTA placement and icon cues.

**Tech Stack:** Go (Chi HTTP API), Svelte 5, TypeScript, Tailwind utility classes, existing HarborWatch toasts/state patterns.

---

### Task 1: Lifecycle Rules Persistence Bugfix (Backend, TDD)

**Files:**
- Modify: `backend/internal/httpapi/server.go`
- Modify/Test: `backend/internal/httpapi/server_test.go`

**Steps:**
1. Keep/add failing regression test proving `validateMode=docker` is preserved for first save (`Exists=false`).
2. Run targeted test and verify failure.
3. Patch rules save route to avoid applying `effectiveContainerRules(...)` before persistence.
4. Normalize only explicit persisted fields needed on save (e.g. `validateMode`).
5. Re-run targeted test and package tests.

### Task 2: Dirty Save Mechanics (Container Lifecycle)

**Files:**
- Modify: `web/src/lib/pages/ContainerPage.svelte`

**Steps:**
1. Add lifecycle form snapshot/signature baseline captured after sync/load.
2. Derive `lifecycleDirty` from current form state vs saved baseline.
3. Disable `Save Selection` when unchanged and show `Pending/Saved/Saving` state.
4. Ensure baseline refreshes after successful save and load.

### Task 3: Dirty Save Mechanics (Global Settings)

**Files:**
- Modify: `web/src/lib/pages/Settings.svelte`

**Steps:**
1. Add settings baseline snapshot/signature (`last loaded`) and derived `settingsDirty`.
2. Normalize/sanitize settings for dirty comparison and save payload consistently.
3. Disable top-level `Save Settings` when unchanged and surface pending/saved status.
4. Preserve existing immediate-save scheduler/task interactions.

### Task 4: Automation Default Policy IA + Visual Scanability

**Files:**
- Modify: `web/src/lib/pages/Settings.svelte`

**Steps:**
1. Split Upgrade Automation UI into clearly distinct `Runtime Controls` vs `Default Policy` cards.
2. Move/associate bulk apply CTA with the default policy card.
3. Add small icons/badges to major sub-sections to reduce text density and improve scanability.
4. Keep existing functionality/field bindings unchanged.

### Task 5: Verification and Review

**Files:**
- Verify only

**Steps:**
1. Run `go test ./internal/httpapi`.
2. Run `npm --prefix /config/workspace/HarborWatch/web run build`.
3. Review diff for save semantics regressions and misleading UI copy.
4. Summarize changes and any residual risks.
