# Design Document: Stacks and GitOps Redesign

**Date:** 2026-03-19  
**Status:** Approved  
**Target Version:** Unreleased

## 1. Overview
This redesign unifies the HarborWatch `Stacks` experience across local compose projects, Portainer stacks, and GitOps repositories. The current UI contains useful controls, but they are visually inconsistent, flatter than the rest of the app, and weaker than they should be at communicating object state and available operator actions.

## 2. Goals
- Normalize the Stacks page shell and width to match HarborWatch's stronger management pages.
- Apply a single card-based language across compose, portainer, and repository views.
- Improve object-level communication of active work such as redeploys and deployments.
- Clarify control surfaces so operators can quickly understand what each object is and what actions are available.
- Create a reusable design language reference for future UI additions.

## 3. Page-Level Design
### 3.1 Shared Shell
- `Stacks.svelte` becomes the shared orchestration workspace.
- The page header should have stronger identity, a normalized width, and a consistent action zone.
- Tabs should remain `Compose Projects`, `Portainer Stacks`, and `GitOps Repositories`, but share one visual grammar.

### 3.2 Section Grammar
Each tab should use:
- A section intro block with title, subtitle, and aggregate count/stat chips.
- A card grid for primary objects.
- Dedicated empty, loading, and error shells that match the design language spec.

## 4. Compose Projects Tab
- Local compose projects remain card-based, but the cards should gain stronger hierarchy and clearer state grouping.
- The identity zone should prioritize project name and working path.
- The state zone should group source verification, compose authority, env authority, snapshot protection, update count, and member count.
- The controls zone should include direct actions such as `Open Editor` and `Open Fleet` where appropriate.
- Member/service rows should remain subordinate, readable, and clearly navigable.

## 5. Portainer Stacks Tab
- Portainer stacks should remain cards, but stop feeling like placeholders.
- Each stack card should show stack name, endpoint, engine, active/inactive state, and a clearer operational summary.
- The card should expose the main actions consistently, including `View Containers` and `Redeploy` where supported.
- During redeploy, the entire card should shift into an active state treatment with amber emphasis and subtle animated energy so the operator can identify the busy stack immediately.
- The global progress bar remains useful, but it is not enough by itself.

## 6. GitOps Repositories Tab
- `GitOps.svelte` should move from a flatter expandable list to repository command cards.
- Repository cards should have clear identity, sync state, branch, auth method, commit, stack count, and top-level actions.
- Expanding a repository should reveal nested deployment cards rather than loose deployment rows.
- Deployment cards should show compose path, mapped path, deploy state, env source, image refresh policy, last deployed info, and control actions.
- Running or queued deployments should get object-level visual treatment comparable to Portainer redeploy cards.
- The current icon for the GitOps page should be replaced with one that more clearly signals repository-orchestration work.

## 7. Control Surface Audit
### 7.1 Compose
Current controls are mostly sufficient, but the page needs stronger communication of what is editable, safe, and source-controlled.

### 7.2 Portainer
Current controls are functional but under-communicated. Redeploy state is too dependent on global progress and the card body is too generic.

### 7.3 GitOps
Most controls already exist, but they are not presented with enough hierarchy or structure. The redesign should surface existing capability more clearly before inventing new controls.

## 8. Error and Empty States
- Each tab should have explicit error and empty treatments aligned with the design language spec.
- Messages should explain both the issue and the next likely action.
- Empty states should not look like unfinished layout.

## 9. Verification Plan
1. Verify the full Stacks page width and shell are consistent with the rest of the app.
2. Verify all three tabs share the same visual grammar.
3. Verify Portainer redeploy visibly changes the active stack card.
4. Verify queued/running GitOps deployments visibly change the active deployment card.
5. Verify desktop and mobile readability, especially action clustering and chip wrapping.
6. Verify all existing actions still work: compose editor navigation, sync, add stack, deploy now, edit, delete, and redeploy.
