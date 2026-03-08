# HarborWatch Stacks UI Polish Plan

## Objective
The current Stacks view is cluttered with repetitive, explanatory text and doesn't clearly prioritize the actionable state of the stack or navigation to its constituent containers. The goal is to transform the per-stack cards into sleek, status-driven dashboards that emphasize health, updates, and quick navigation.

## Current Issues Identified
1. **Information Overload**: Every compose project card repeats long paragraphs explaining "Edit Authority" and "Compose Protection" behavior. This text is unnecessary for daily operations and dilutes important data.
2. **Poor Path Display**: Long file paths (e.g., `/mnt/Storage-SSD/...`) wrap awkwardly, take up premium real estate, and aren't needed at a glance.
3. **Container Navigation**: The "Services" are displayed as small tag-like buttons at the bottom. While they are clickable, their primary function as direct navigation to the Container view isn't intuitively obvious, and their status (running vs stopped) isn't visually distinct.
4. **State Obscurity**: It's hard to tell immediately if a stack has available updates or is fully healthy.

## Proposed Layout Improvements

### 1. Card Header: Identity & Status
- **Left**: Stack Name (prominent and clickable to enter compose view).
- **Right**: A unified "Stack Status" indicator. For example, a crisp "Up to date" green badge or a "X Updates" amber badge to immediately draw attention.

### 2. Compact Metadata Bar
- Replace the large boxes and paragraphs with a single row of compact badges/tags:
  - Source Status (e.g., `Verified / Read-Only`)
  - Edit Authority (e.g., `Compose Read-Only`, `Local .env Editable`)
  - Protection (e.g., `Snapshots: Unchanged`)
- Render the file paths simply as truncated monospace text immediately under the title.

### 3. Services List (Container Navigation)
- Elevate the "Services" block. Instead of small inline tags, render them as a clean, vertical list of distinct, clickable elements inside a lightly tinted container.
- Each service element will clearly show:
  - A status indicator dot (e.g., green for running).
  - Service Name / Container Name.
  - Image Name (truncated).
  - A subtle arrow `→` and an update indicator dot on the right side indicating it's a navigational link to the Container details.

### 4. Clear Actions
- Place primary actions cleanly separated at the bottom of the card:
  - **Open Compose Editor** (Primary full-width button)
- For Portainer stacks: **Redeploy** and **View Containers** will be clearly delineated buttons.

## Implementation Steps
1. **Redesign Metadata**: Rewrite the internal HTML for the compose and portainer cards to use flex-wrap badge rows.
2. **Enhance Services**: Redesign the `{#each p.members}` loop to output the structured navigational list, removing the old button styles.
3. **Clean up Paths**: Apply `truncate` classes to long paths.
4. **Build & Verify**: Validate the new structure via `svelte-check` and standard compilation.