# Global Progress Sticky Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the global HarborWatch progress bar detach from normal page flow while scrolling and stick directly beneath the top navigation stack, matching the YA-WAMF/container-page experience.

**Architecture:** Keep `GlobalProgress.svelte` as the single global status surface, but make the existing bar itself sticky so it stays in normal flow before locking beneath the nav stack on scroll. Wire the app shell to give the sticky bar a predictable vertical anchor on mobile and desktop instead of relying on ordinary document flow.

**Tech Stack:** Svelte 5, Vite, Tailwind utility classes, lightweight source-level regression checks via Python.

---

### Task 1: Guard the sticky contract

**Files:**
- Create: `web/scripts/test_global_progress_sticky.py`
- Test: `web/scripts/test_global_progress_sticky.py`

**Step 1: Write the failing test**

Create a source-level regression check that verifies:
- `GlobalProgress.svelte` renders a sticky shell.
- The shell uses a top offset CSS variable for shared layout anchoring.
- `App.svelte` defines the sticky offset variable on the app main container.

**Step 2: Run test to verify it fails**

Run: `python3 web/scripts/test_global_progress_sticky.py`
Expected: FAIL because the current component is rendered in normal flow and the app shell does not provide a shared sticky offset variable.

### Task 2: Implement the sticky layout

**Files:**
- Modify: `web/src/lib/components/GlobalProgress.svelte`
- Modify: `web/src/App.svelte`

**Step 1: Write minimal implementation**

Update the global progress component so:
- The existing progress surface itself becomes sticky while remaining in normal flow.
- The sticky container uses a shared CSS variable for its `top` value.

Update app shell/layout styles so:
- `App.svelte` defines the shared sticky offset variable on the main content area.
- Mobile keeps the bar beneath the mobile header.
- Desktop keeps the bar aligned under the top nav stack/content top edge.

**Step 2: Run test to verify it passes**

Run: `python3 web/scripts/test_global_progress_sticky.py`
Expected: PASS

### Task 3: Verify the UI build

**Files:**
- Test: `web/package.json`

**Step 1: Run web build**

Run: `npm --prefix web run build`
Expected: PASS

**Step 2: Commit**

Run:
```bash
git add docs/plans/2026-04-22-global-progress-sticky.md web/scripts/test_global_progress_sticky.py web/src/lib/components/GlobalProgress.svelte web/src/App.svelte web/src/app.css
git commit -m "fix(web): make global progress bar sticky"
```
