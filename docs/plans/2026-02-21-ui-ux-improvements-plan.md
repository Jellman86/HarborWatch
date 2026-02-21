# UI/UX & Fleet Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve HarborWatch UI/UX with better Fleet sorting, a new Stacks card view, refined Container Lifecycle/Insights tabs, and sidebar reorganization.

**Architecture:** Client-side sorting and filtering for Fleet; CSS-driven "stack" effect for Stacks cards; Refactored Svelte 5 (Runes) components for Container Details.

**Tech Stack:** Svelte 5, Tailwind CSS, HarborWatch Go API.

---

### Task 1: Sidebar Reorganization

**Files:**
- Modify: `HarborWatch/web/src/lib/components/Sidebar.svelte`

**Step 1: Move 'About' to the bottom of `navItems`**

Modify `navItems` array in `Sidebar.svelte` to move the 'about' object to the end of the array, ensuring it appears last in the navigation list.

**Step 2: Verify Sidebar**
- Run `npm run dev` (if applicable) or check the rendered UI.
- Expected: "About" link is at the bottom of the sidebar.

**Step 3: Commit**
```bash
git add HarborWatch/web/src/lib/components/Sidebar.svelte
git commit -m "ui: move about link to bottom of sidebar"
```

### Task 2: Fleet Management - Hide Ignored Containers

**Files:**
- Modify: `HarborWatch/web/src/lib/pages/Containers.svelte`

**Step 1: Add `showIgnored` state and toggle**
- Add `let showIgnored = $state(false);`
- Add a toggle button/checkbox in the filter bar.
- Update `visibleContainers` derived state to respect `showIgnored`.

**Step 2: Update Visibility Indicator**
- Show `+N Ignored` next to the visible count when `showIgnored` is false and there are ignored containers.

**Step 3: Verify Hiding**
- Mark a container as ignored (e.g., via labels or tokens).
- Verify it disappears from "All" view when toggle is OFF.
- Verify it appears when toggle is ON.

**Step 4: Commit**
```bash
git add HarborWatch/web/src/lib/pages/Containers.svelte
git commit -m "feat: add 'Hide Ignored' toggle to Fleet view"
```

### Task 3: Fleet Management - Sorting

**Files:**
- Modify: `HarborWatch/web/src/lib/pages/Containers.svelte`

**Step 1: Add `sortBy` state and logic**
- Add `let sortBy = $state<'name' | 'state' | 'memory' | 'storage'>('name');`
- Implement a sort dropdown or clickable headers.
- Update `visibleContainers` to sort the filtered list.

**Step 2: Verify Sorting**
- Sort by Memory: verify containers with higher RAM usage are at the top.
- Sort by State: verify Running containers are grouped.

**Step 3: Commit**
```bash
git add HarborWatch/web/src/lib/pages/Containers.svelte
git commit -m "feat: implement client-side sorting for Fleet view"
```

### Task 4: Stacks - Card View Refactor

**Files:**
- Modify: `HarborWatch/web/src/lib/pages/Stacks.svelte`

**Step 1: Replace Table with Card Grid**
- Implement a grid layout (`grid-cols-1 md:grid-cols-2 lg:grid-cols-3`).
- Create a card component (internal or separate) for each stack.

**Step 2: Implement "Stack of Cards" Effect**
- Use Tailwind classes and CSS pseudo-elements to create the layered effect.
```css
.stack-card {
  position: relative;
}
.stack-card::before, .stack-card::after {
  content: '';
  position: absolute;
  top: 4px; left: 4px; right: -4px; bottom: -4px;
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 1rem;
  z-index: -1;
}
.stack-card::after {
  top: 8px; left: 8px; right: -8px; bottom: -8px;
  z-index: -2;
}
```

**Step 3: Verify Layout**
- Check the Stacks page to ensure cards look like a physical stack and contain all relevant info.

**Step 4: Commit**
```bash
git add HarborWatch/web/src/lib/pages/Stacks.svelte
git commit -m "ui: refactor Stacks view to high-density card grid"
```

### Task 5: Container Details - Lifecycle Refactor

**Files:**
- Modify: `HarborWatch/web/src/lib/pages/ContainerPage.svelte`

**Step 1: Refactor Lifecycle Mode Selector**
- Implement "Automatic" vs "Manual" radio/button group.
- Ensure it maps correctly to `detail.rules.inheritAutomation`.

**Step 2: Add "Bypass AI" Toggle**
- Add a toggle in the Manual section.
- Ensure it's passed to the `Updates` page via params or stored in local state for the trigger.

**Step 3: Implement Breaking Changes History**
- Filter `updateHistory` for jobs with `aiAnalysis.breakingChanges`.
- Display them in a dedicated list.

**Step 4: Commit**
```bash
git add HarborWatch/web/src/lib/pages/ContainerPage.svelte
git commit -m "feat: refactor Lifecycle tab with Manual mode and breaking changes history"
```

### Task 6: Container Details - Insights & Intelligence Refactor

**Files:**
- Modify: `HarborWatch/web/src/lib/pages/ContainerPage.svelte`
- Modify: `HarborWatch/web/src/lib/components/DiskUsagePanel.svelte`

**Step 1: Move Execution History to Insights**
- Move the Action History table into the `insights` tab block.

**Step 2: Simplify DiskUsagePanel**
- Refactor UI to show "Container Files" (RootFS) and "Writable Layer" more prominently.
- List active mounts clearly.

**Step 3: Add Intelligence Guidance**
- Add a "Pro-tip" or "Optimal Setup" section to the Intelligence tab.

**Step 4: Commit**
```bash
git add HarborWatch/web/src/lib/pages/ContainerPage.svelte HarborWatch/web/src/lib/components/DiskUsagePanel.svelte
git commit -m "ui: refine Insights and Intelligence tabs"
```

### Task 7: Documentation and Changelog

**Files:**
- Modify: `HarborWatch/CHANGELOG.md`
- Modify: `HarborWatch/README.md` (if applicable)

**Step 1: Update CHANGELOG.md**
- Add entries for v0.9.0 including Fleet sorting, Stacks card view, and Lifecycle refactor.

**Step 2: Commit**
```bash
git add HarborWatch/CHANGELOG.md
git commit -m "docs: update changelog for v0.9.0"
```
