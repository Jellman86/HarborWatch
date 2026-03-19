# Stacks and GitOps Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign HarborWatch's Stacks workspace across compose projects, Portainer stacks, and GitOps repositories, while codifying a reusable design language spec for future UI work.

**Architecture:** Keep backend behavior intact and focus on frontend layout, hierarchy, and state communication. Use the new design language spec to normalize `Stacks.svelte` and `GitOps.svelte`, then verify the result with a production build and manual state checks.

**Tech Stack:** Svelte 5, TypeScript, Tailwind utility classes, HarborWatch SPA router

---

### Task 1: Write the design docs

**Files:**
- Create: `docs/plans/2026-03-19-design-language-spec.md`
- Create: `docs/plans/2026-03-19-stacks-gitops-redesign-design.md`
- Create: `docs/plans/2026-03-19-stacks-gitops-redesign-plan.md`

**Step 1: Write the docs**

- Capture the approved HarborWatch design language rules.
- Capture the approved Stacks/GitOps redesign.
- Save the implementation plan.

**Step 2: Commit**

```bash
git add docs/plans/2026-03-19-design-language-spec.md docs/plans/2026-03-19-stacks-gitops-redesign-design.md docs/plans/2026-03-19-stacks-gitops-redesign-plan.md
git commit -m "docs: add stacks redesign and design language spec"
```

### Task 2: Add a failing verification pass for the redesigned surfaces

**Files:**
- Modify: `web/src/lib/pages/Stacks.svelte`
- Modify: `web/src/lib/pages/GitOps.svelte`

**Step 1: Define the target changes before implementation**

- Identify the exact page shell, card, and state sections to replace.
- Confirm the route and action props are already sufficient so backend changes are unnecessary.

**Step 2: Run frontend build as a baseline**

Run: `npm --prefix web run build`

Expected: PASS before redesign, establishing a clean baseline.

### Task 3: Redesign the Stacks page shell and compose/portainer cards

**Files:**
- Modify: `web/src/lib/pages/Stacks.svelte`

**Step 1: Implement the new shell and section headers**

- Normalize page width and header treatment.
- Replace flatter tab chrome with the new orchestration workspace shell.
- Add stronger section intro and aggregate stat treatments.

**Step 2: Implement compose and portainer card hierarchy**

- Rebuild compose project cards with clearer identity, state, and action zones.
- Rebuild portainer stack cards with stronger operational summaries.
- Add in-card active-state treatment for redeploying stacks.

**Step 3: Run build to verify it passes**

Run: `npm --prefix web run build`

Expected: PASS

**Step 4: Commit**

```bash
git add web/src/lib/pages/Stacks.svelte
git commit -m "feat(ui): redesign stacks workspace shell"
```

### Task 4: Redesign GitOps repositories and deployment cards

**Files:**
- Modify: `web/src/lib/pages/GitOps.svelte`

**Step 1: Rework the repository header and iconography**

- Replace the current page icon with a clearer repository-orchestration symbol.
- Rebuild repository cards to emphasize identity, sync state, branch, commit, auth, and top-level actions.

**Step 2: Rebuild deployment rule presentation**

- Convert deployment rows into nested stack/deployment cards.
- Preserve all existing actions.
- Add stronger queued/running visual treatment at the deployment-card level.

**Step 3: Run build to verify it passes**

Run: `npm --prefix web run build`

Expected: PASS

**Step 4: Commit**

```bash
git add web/src/lib/pages/GitOps.svelte
git commit -m "feat(ui): redesign gitops repository controls"
```

### Task 5: Final verification

**Files:**
- Modify if needed based on failures

**Step 1: Run final verification**

Run: `npm --prefix web run build`

Expected: PASS

**Step 2: Manual smoke checklist**

- Verify Stacks page width is normalized.
- Verify compose, portainer, and repositories share the same visual language.
- Verify redeploying Portainer cards visibly change state.
- Verify queued/running GitOps deployment cards visibly change state.
- Verify sync, add stack, deploy, edit, delete, compose editor navigation, and view containers still work.
