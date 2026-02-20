# HarborWatch System Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve Portainer reliability, centralize job management, enhance AI transparency, and refine the UI/UX.

**Architecture:** A new `JobManager` will coordinate all heavy tasks with global concurrency limits and per-container locking. Portainer client timeouts will be increased, and AI conversation logging will be added for transparency. UI improvements will fix sidebar and card view layout issues.

**Tech Stack:** Go (Backend), Svelte 5 (Frontend), SQLite (Database), Playwright (E2E Testing).

---

### Task 1: Increase Portainer Client Timeout

**Files:**
- Modify: `HarborWatch/backend/internal/portainer/client.go`

**Step 1: Modify Client Timeout**
Change `10 * time.Second` to `300 * time.Second` in the `NewClient` function.

**Step 2: Commit**
```bash
git add HarborWatch/backend/internal/portainer/client.go
git commit -m "fix(portainer): increase client timeout to 300s for large image pulls"
```

---

### Task 2: Enhance Container Ignore Logic (Backend)

**Files:**
- Modify: `HarborWatch/backend/internal/httpapi/server.go`

**Step 1: Expand Default Tokens and Update Matching**
Update `isIgnoredContainer` to include more default tokens (`portainer-ce`, `ix-portainer`) and use `strings.Contains` consistently for names and IDs in `containerMatchesToken`.

**Step 2: Commit**
```bash
git add HarborWatch/backend/internal/httpapi/server.go
git commit -m "feat(ignore): expand default ignore list and use substring matching"
```

---

### Task 3: AI Conversation Logging (Backend)

**Files:**
- Modify: `HarborWatch/backend/internal/ai/usage_store.go`
- Modify: `HarborWatch/backend/internal/ai/service.go`

**Step 1: Add `ai_conversations` Table**
Update `UsageSQLiteStore.Init` to create the new table.

**Step 2: Implement logging in `ai.Service`**
Record raw prompt and response after every interaction.

**Step 3: Commit**
```bash
git add HarborWatch/backend/internal/ai/
git commit -m "feat(ai): implement conversation logging to SQLite"
```

---

### Task 4: Diagnostics Paging (Backend)

**Files:**
- Modify: `HarborWatch/backend/internal/diag/logger.go`
- Modify: `HarborWatch/backend/internal/httpapi/server.go`

**Step 1: Update `ListLogs` to support Offset**
Add `offset` parameter to the SQL query.

**Step 2: Update API Endpoint**
Handle `limit` and `offset` query params in `GET /api/system/logs`.

**Step 3: Commit**
```bash
git add HarborWatch/backend/internal/diag/logger.go HarborWatch/backend/internal/httpapi/server.go
git commit -m "feat(diag): add paging support to system logs API"
```

---

### Task 5: Sidebar UI Refinements

**Files:**
- Modify: `HarborWatch/web/src/lib/components/Sidebar.svelte`

**Step 1: Remove Top Collapse Button**
Delete the redundant toggle in the logo area.

**Step 2: Fix Logo Distortion**
Update the logo image class to scale down gracefully when collapsed.

**Step 3: Commit**
```bash
git add HarborWatch/web/src/lib/components/Sidebar.svelte
git commit -m "ui(sidebar): remove redundant toggle and fix logo scaling"
```

---

### Task 6: Fleet Card Layout Fix

**Files:**
- Modify: `HarborWatch/web/src/lib/pages/Containers.svelte`

**Step 1: Adjust Card Header Flex Properties**
Ensure the container name can shrink and pills can wrap to prevent overlap.

**Step 2: Commit**
```bash
git add HarborWatch/web/src/lib/pages/Containers.svelte
git commit -m "ui(fleet): prevent container name overlap in card views"
```

---

### Task 7: Global Job Manager & Concurrency (Phase 1: Backend)

**Files:**
- Create: `HarborWatch/backend/internal/jobs/manager.go`
- Modify: `HarborWatch/backend/internal/settings/store.go`

**Step 1: Implement `JobManager`**
Create a centralized service for safe concurrency and per-container locking.

**Step 2: Commit**
```bash
git add HarborWatch/backend/internal/
git commit -m "feat(jobs): implement centralized JobManager for safe concurrency"
```

---

### Task 8: Global Progress Bar Improvements (Frontend)

**Files:**
- Modify: `HarborWatch/web/src/lib/components/GlobalProgress.svelte`

**Step 1: Use Friendly Names**
Ensure the progress bar displays container names instead of hashes.

**Step 2: Commit**
```bash
git add HarborWatch/web/src/lib/components/GlobalProgress.svelte
git commit -m "ui(progress): display friendly container names in progress bar"
```

---

### Task 9: Verification

**Step 1: Run Backend Tests**
`go test ./backend/internal/...`

**Step 2: Run Frontend Lint**
`npm run lint`

**Step 3: Final UI Audit**
`python3 scripts/full_ui_audit.py`
