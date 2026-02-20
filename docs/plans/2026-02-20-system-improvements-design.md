# Design Document: HarborWatch System Improvements (2026-02-20)

## Overview
This document outlines the architectural and UI improvements for HarborWatch, focusing on Portainer reliability, centralized job management, AI transparency, and UI/UX refinements.

## 1. Architecture & Job Management

### 1.1 Global Job Manager
A new internal service, `JobManager`, will coordinate all "heavy" operations.
- **Operations Covered:** Trivy Scans, ClamAV Scans, Container Updates, Stack Redeployments.
- **Queueing:** Jobs will be queued and executed based on available concurrency slots.
- **Safe Concurrency:** 
  - A global `MaxConcurrentJobs` setting (default 1) will control system-wide heavy operations.
  - Strict per-container locking to prevent multiple concurrent operations on the same target.
- **Unified Progress:** All jobs will report status, message, and percentage to a single `JobProgress` API stream.

### 1.2 Portainer Reliability
- **Timeout Increase:** The `portainer.Client` HTTP timeout will be increased from 10 seconds to 300 seconds (5 minutes) for stack updates and redeploys to account for potentially large image pulls.

## 2. Backend Improvements

### 2.1 Enhanced Ignore Logic
- **Tokens:** Expand default ignore list to include `portainer-ce`, `ix-portainer`, `harborwatch`.
- **Matching:** Use case-insensitive substring matching (`strings.Contains`) for container names, image tags, and IDs to ensure variants like `ix-portainer-portainer-1` are correctly caught.
- **Consistency:** Ensure the same logic is applied across both the backend automation triggers and the frontend filters.

### 2.2 AI Conversation Logging
- **Storage:** New SQLite table `ai_conversations` (id, timestamp, provider, model, feature, prompt, response).
- **Service Update:** `ai.Service` will record the raw prompt and response for every interaction.
- **API:** New endpoint `GET /api/ai/conversations` with paging support.

### 2.3 Diagnostics Paging
- **Logs API:** Update `ListLogs` in `logger.go` to support `limit` and `offset` parameters.
- **Data Retention:** Paging will allow users to browse up to the configured `retentionLogsDays` limit without overwhelming the browser.

## 3. Frontend & UI Refinements

### 3.1 Sidebar & Branding
- **Logo Fix:** Adjust CSS in `Sidebar.svelte` to prevent the logo from squashing when collapsed. Transition to a smaller size (e.g., 32px or 48px) instead of forced aspect ratio compression.
- **Collapse Toggle:** Remove the redundant top sidebar collapse button; the bottom toggle is sufficient.

### 3.2 Fleet View (Containers)
- **Header Layout:** Apply `flex-wrap` and `min-w-0` to container name headers in card views to prevent name/pill overlap.
- **Progress Bar:** Ensure the global progress bar pulls the `target` name (friendly container name) from the `JobProgress` object.

### 3.3 Settings
- **New Settings:** Add "Max Concurrent Jobs" control.
- **Conversation Viewer:** A new section in AI Settings to view recent prompts and responses.

## 4. Data Flow
1. **Frontend** triggers a heavy operation (e.g., Redeploy).
2. **Backend JobManager** receives the request, checks concurrency/locks, and queues the job.
3. **JobManager** executes the job, updating the `internal_logs` and `JobProgress` stream.
4. **AI Service** (if involved) logs the prompt/response to `ai_conversations`.
5. **Frontend GlobalProgress** component renders the unified status stream.

## 5. Testing Strategy
- **Unit Tests:** Verify `JobManager` concurrency limits and locking.
- **Integration Tests:** Mock Portainer API with high latency to verify 5-minute timeout.
- **UI Tests:** Playwright tests for sidebar collapse behavior and card view responsiveness.
