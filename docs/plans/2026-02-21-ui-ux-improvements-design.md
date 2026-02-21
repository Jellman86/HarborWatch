# Design Document: HarborWatch UI/UX & Fleet Management Improvements

**Date:** 2026-02-21  
**Status:** Approved  
**Target Version:** v0.9.0

## 1. Overview
This design covers a series of UI/UX improvements to HarborWatch, focusing on Fleet Management, Stacks visualization, Container Lifecycle control, and Sidebar organization. The goal is to provide a more robust, truthful, and useful management experience.

## 2. Component Designs

### 2.1 Fleet Management (Containers)
*   **Sorting:** Implement client-side sorting in `Containers.svelte`.
    *   **Attributes:** Name (default), State (Running first), RAM usage, Storage usage.
*   **Ignore Logic:**
    *   "Hide Ignored" toggle in the filter bar, enabled by default.
    *   Visibility Indicator: A small badge near the "Visible" count showing "+N Ignored" when hidden, allowing users to quickly see that containers are being filtered out.
*   **Consequence Management:** Ensure that even when ignored containers are hidden, they are still reachable via direct URL or search (search should override the "Hide Ignored" filter for explicit matches).

### 2.2 Stacks (Orchestration)
*   **Visual Refactor:** Transition from a standard table to a high-density card grid.
*   **Stack-of-Cards Effect:** 
    *   Each stack card will use CSS pseudo-elements (`::before`, `::after`) with slightly offset borders and subtle shadows to create a layered "paper stack" aesthetic.
    *   Card content: Stack Name, Engine Type (Compose/Swarm), Status, Container Count (if available), and Endpoint ID.
    *   Actions: "Manage" (deep link to Fleet with search filter) and "Redeploy".

### 2.3 Container Details: Lifecycle Tab
*   **Mode Selection:** A clear toggle or radio group between **Automatic** (follows global/policy settings) and **Manual** (user-triggered updates only).
*   **Manual Update Controls:**
    *   Add a "Bypass AI Breaking Change Assessment" toggle. When ON, the update pipeline skips the `release_analysis` step or ignores its failure.
*   **Historical Breaking Changes:**
    *   A scrollable list of historical signals detected by the AI for this specific container, extracted from previous `UpdateJobStatus` records.

### 2.4 Container Details: Insights & Intelligence
*   **Insights Tab:**
    *   **Execution History:** Move the "Execution History" (Action History) table from the bottom of the page to the Insights tab to reduce clutter.
    *   **Disk Metrics:** Simplify `DiskUsagePanel.svelte`. Instead of "vs Host", focus on "Container Footprint" (RootFS + Writable) and a clear list of "Active Mounts". Use plain language for what these metrics represent (e.g., "Space taken by container files" vs "Persistent data volumes").
*   **Intelligence Tab:**
    *   **Optimal Setup Guidance:** Add a "Configuration Guide" section that explains *how* to improve AI accuracy (e.g., adding `org.opencontainers.image.source` labels or specific HarborWatch override labels). Link to relevant documentation or example `docker-compose.yml` snippets.

### 2.5 Sidebar
*   **Organization:** Move the "About" navigation item to the bottom of the sidebar list, just above the Theme/Collapse controls.

## 3. Technical Implementation
*   **Frontend:** Svelte 5 (Runes).
*   **Styling:** Vanilla CSS + Tailwind utility classes.
*   **Data Flow:** No backend changes required for sorting (client-side). Lifecycle mode changes will persist to the existing `/api/docker/:id/rules` endpoint.

## 4. Verification Plan
1.  **Manual UI Walkthrough:** Verify sorting correctness and "Hide Ignored" toggle behavior.
2.  **Stack Card Layout:** Check responsiveness and "stack" effect across mobile/desktop.
3.  **Lifecycle Logic:** Confirm that "Manual" mode correctly blocks auto-updates and that the "Bypass AI" toggle is respected by the update trigger.
4.  **Playwright Audit:** Run a regression sweep to ensure no existing functionality is broken.
