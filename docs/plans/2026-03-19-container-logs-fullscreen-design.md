# Design Document: HarborWatch Full-Screen Container Logs View

**Date:** 2026-03-19  
**Status:** Approved  
**Target Version:** Unreleased

## 1. Overview
This design replaces the Fleet page's container logs modal with a dedicated full-screen logs view for a single container. The goal is to make log inspection feel like a primary workflow, similar to Dozzle, while staying within HarborWatch's existing single-page app routing and current Docker logs API.

## 2. User Experience

### 2.1 Entry Point
* The Logs action in `Containers.svelte` will navigate to a dedicated route instead of opening a modal.
* The new route will receive the selected container ID and optionally retain enough context to return the user to Fleet.

### 2.2 Full-Screen Logs Workspace
* The logs view will occupy the normal content area as a full-page destination.
* A sticky header will show:
  * Back action to Fleet
  * Container name and short ID
  * Container state and health, when available
  * Live status indicator
* A controls row will provide:
  * Pause / Resume live updates
  * Manual Refresh
  * Tail size selector
  * Timestamps toggle

### 2.3 Live Log Behavior
* The page will fetch logs using the existing `/api/docker/{id}/logs` endpoint.
* Live mode will poll on a short interval.
* Pause stops polling but preserves the current log buffer.
* Resume restarts polling without forcing navigation or clearing the buffer.
* Auto-scroll is only applied while live mode is active and the user is already near the bottom of the log pane.

## 3. Technical Design

### 3.1 Routing
* Add a new app route: `container-logs`.
* Register a new page component in `web/src/App.svelte`.
* Reuse the shared `onNavigate` function for transitions from Fleet and back navigation.

### 3.2 Page Component
* Create `web/src/lib/pages/ContainerLogsPage.svelte`.
* Inputs:
  * `id`
  * `containers`
  * `onNavigate`
* Responsibilities:
  * Resolve container metadata from the global fleet list
  * Fetch and render logs
  * Manage polling state
  * Handle pause/resume, refresh, timestamps, and tail selection
  * Render inline loading and error states

### 3.3 Fleet Page Update
* Remove modal state and modal markup from `Containers.svelte`.
* Replace `openLogs(c.id)` with navigation to `container-logs`.

## 4. Error Handling
* If the container no longer exists, show an inline error with a back action.
* If log loading fails, keep the page open and show the failure in the content area.
* If the container cannot be found in the current fleet snapshot, still attempt the logs API request using the route ID.

## 5. Verification Plan
1. Build the frontend to ensure the new route and page compile.
2. Manually verify Fleet -> Logs navigation, back navigation, and pause/resume behavior.
3. Confirm that polling stops when paused and resumes cleanly.
4. Confirm that the modal path has been removed from Fleet.
