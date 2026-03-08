# HarborWatch GitOps UI Polish Plan

## Objective
The GitOps Repositories section (within the Stacks view) currently relies on the old "cards-within-cards" design language. The objective is to bring it in line with the recently polished Settings and Stacks views by flattening the interface, removing nested borders, and creating a cleaner visual hierarchy.

## Current Issues Identified
1. **Nested Metric Cards**: The repository details ("Last Sync", "Commit", "Auth", "Active Stacks") are wrapped in heavy, bordered, tinted cards (`p-4 rounded-2xl bg-slate-50 border`). This adds unnecessary visual clutter inside the main repository card.
2. **Heavy Inner Elements**: The inner "Deployment Rules" list uses thick bordered cards for each compose path, compounding the nested feeling.
3. **Inconsistent Radii**: The main card uses `rounded-[2rem]` which slightly mismatches the `rounded-3xl` used in the rest of the Stacks page.

## Proposed Improvements

### 1. Flatten the Repository Metadata
- Completely remove the background, borders, and heavy padding from the 4-column metadata grid ("Last Sync", "Commit", etc.).
- Transform it into a clean, simple layout. The labels and values will float directly on the white card background, relying on typography (`text-[9px] font-black uppercase text-slate-400` for labels, `font-bold text-slate-800` for values) rather than boxes for separation.

### 2. Streamline the Deployment Rules
- For the expanded deployment list, remove the inner `bg-slate-50` and border from each deployment item.
- Separate deployment items using a simple `border-b border-slate-100 dark:border-slate-800/50 last:border-0` and minimal padding, identical to the rows in the Settings page.

### 3. Harmonize Card Styling
- Update the main outer repository container from `rounded-[2rem]` to `rounded-3xl` to match the local Compose and Portainer stack cards perfectly.
- Soften the border styling slightly to match the updated global standard (`border-slate-200/60`).

## Implementation Steps
1. Refactor the `GitOps.svelte` component to apply these changes.
2. Verify the structural integrity and dark mode contrast of the refactored design.
3. Commit and push the updates.