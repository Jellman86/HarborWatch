# HarborWatch UI Polish Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Apply the requested UI/layout/status improvements for Automations, Lifecycle, Stacks, and Settings without backend API changes.

**Architecture:** Modify existing Svelte pages/components in place, reusing current settings/scheduler APIs and state. Add small UI-only state for save error indicators and page/sub-tab selection.

**Tech Stack:** Svelte 5, TypeScript, Vite, Tailwind CSS

---

### Task 1: Documented design + prep validation path

**Files:**
- Create: `docs/plans/2026-02-26-harborwatch-ui-polish-design.md`
- Create: `docs/plans/2026-02-26-harborwatch-ui-polish-plan.md`
- Verify: `web/package.json`

**Step 1: Confirm available verification commands**

Run: `cat web/package.json`
Expected: `build` script exists, no frontend unit-test harness present.

**Step 2: Record TDD constraint**

Note in implementation summary that strict TDD is not practical for this UI-only change because the repo has no frontend test runner/component test setup.

**Step 3: Use compile verification**

Run after code changes: `npm --prefix web run build`
Expected: Svelte app compiles successfully.

### Task 2: Automations UI polish (`Settings.svelte`, `AutomationFlowChart.svelte`)

**Files:**
- Modify: `web/src/lib/pages/Settings.svelte`
- Modify: `web/src/lib/components/AutomationFlowChart.svelte`

**Step 1: Increase flow diagram spacing**
- Raise node row gap and connector spacing so arrows are more visible.

**Step 2: Remove Task Status Overview explainer**
- Delete the static info card under automation flow diagram.

**Step 3: Move Upgrade Default Policy segment**
- Render the default policy block as a full-width card below the primary upgrade runtime controls.

**Step 4: Improve Security automation grouping**
- Present Trivy controls together and ClamAV controls together in security automation context.

### Task 3: Lifecycle section layout + save status icons (`ContainerPage.svelte`)

**Files:**
- Modify: `web/src/lib/pages/ContainerPage.svelte`

**Step 1: Add lifecycle save status error tracking**
- Track save errors keyed to current lifecycle draft signature.

**Step 2: Replace lifecycle dot status with icon state**
- Saved => green tick
- Pending => amber exclamation
- Save/validation error => red x
- Saving => preserve current loading treatment

**Step 3: Re-layout lifecycle page content**
- Keep lifecycle management on left
- Move breaking change signals + lifecycle log into right column stack

### Task 4: Stacks page tabbed views (`Stacks.svelte`)

**Files:**
- Modify: `web/src/lib/pages/Stacks.svelte`

**Step 1: Add internal stacks view tab state**
- `compose` / `portainer`

**Step 2: Add tab UI**
- Preserve page header/refresh controls
- Show counts in tabs if useful

**Step 3: Render tabbed content**
- Only show local compose section OR Portainer stacks section at a time

### Task 5: Settings top-level Backups tab + AI sub-tabs + save status icons (`Settings.svelte`)

**Files:**
- Modify: `web/src/lib/pages/Settings.svelte`

**Step 1: Add settings save error tracking + iconized status badge**
- Same status semantics as lifecycle section

**Step 2: Add top-level `Backups` settings tab**
- Move compose snapshot archive path setting from Automations/Maintenance into new Backups page

**Step 3: Split AI page into sub-tabs**
- `Settings` tab: enable/provider/threshold/provider credentials/history link
- `Costs` tab: AI usage, spend analytics, pricing JSON

**Step 4: Improve settings icon consistency**
- Add icons to top-level settings tabs and touched sub-tab headers/cards

### Task 6: Verification

**Files:**
- Verify: `web/src/lib/pages/Settings.svelte`
- Verify: `web/src/lib/pages/ContainerPage.svelte`
- Verify: `web/src/lib/pages/Stacks.svelte`
- Verify: `web/src/lib/components/AutomationFlowChart.svelte`

**Step 1: Run frontend build**

Run: `npm --prefix web run build`
Expected: successful Vite/Svelte build

**Step 2: Review diff for scope**

Run: `git diff -- web/src/lib/pages/Settings.svelte web/src/lib/pages/ContainerPage.svelte web/src/lib/pages/Stacks.svelte web/src/lib/components/AutomationFlowChart.svelte`
Expected: UI-only changes matching requested improvements

**Step 3: Summarize any residual gaps**
- Note if any requested wording implied broader redesign than implemented.

