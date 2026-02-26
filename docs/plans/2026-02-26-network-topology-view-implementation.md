# Network Topology View Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a read-only user-defined Docker network topology view with an interactive graph and details panel, backed by a new Docker topology API endpoint.

**Architecture:** Extend the backend Docker client to return a normalized network topology snapshot, expose it via `GET /api/networks/topology`, then add a new frontend page that renders a deterministic SVG graph (networks -> containers) with a selection side panel and manual refresh.

**Tech Stack:** Go backend (`dockerengine`, `httpapi`), Svelte frontend, SVG rendering (no graph library dependency).

---

### Task 1: Add failing backend tests for topology extraction and filtering

**Files:**
- Create: `backend/internal/dockerengine/networks_test.go`
- Modify: `backend/internal/httpapi/server_test.go` (route smoke, if needed)

**Step 1: Write failing tests**
- User-defined filtering excludes `bridge`, `host`, `none`
- Network endpoint extraction yields deduped containers + edges
- Empty topology returns empty slices

**Step 2: Run tests to verify fail**

Run: `go test ./internal/dockerengine -run 'Test.*NetworkTopology'`

Expected: FAIL (helpers/types not implemented)

**Step 3: Minimal implementation**
- Add topology normalization helpers and types

**Step 4: Run tests to verify pass**

Run: `go test ./internal/dockerengine -run 'Test.*NetworkTopology'`

Expected: PASS

### Task 2: Implement backend Docker topology retrieval + API endpoint

**Files:**
- Modify: `backend/internal/dockerengine/client.go`
- Modify: `backend/internal/httpapi/server.go`
- Create: `backend/internal/httpapi/routes_networks.go`
- Modify: Docker client test doubles in `backend/internal/httpapi/*test.go`

**Step 1: Add Docker client method**
- `GetNetworkTopology(ctx)` to `DockerClient` interface
- Implement in `dockerengine.Client`

**Step 2: Add route**
- `GET /api/networks/topology` (read-only)

**Step 3: Run backend tests**

Run: `go test ./internal/httpapi ./internal/dockerengine`

Expected: PASS

### Task 3: Add frontend page and routing for Networks view

**Files:**
- Create: `web/src/lib/pages/Networks.svelte`
- Modify: `web/src/lib/api-types.ts` (optional manual type)
- Modify: app routing/nav file(s) that register pages

**Step 1: Build snapshot + refresh data flow**
- Fetch `/api/networks/topology`
- Handle loading/empty/error states

**Step 2: Implement graph (SVG)**
- deterministic columns: networks left, containers right
- curved edges
- hover/selection edge labels
- selection highlight/dimming

**Step 3: Implement side panel**
- network details
- container details

**Step 4: Wire route/nav entry**

**Step 5: Build verification**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`

Expected: PASS

### Task 4: Final verification and polish

**Files:**
- No code changes expected

**Step 1: Run backend tests**

Run: `go test ./internal/dockerengine ./internal/httpapi`

Expected: PASS

**Step 2: Run web build**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`

Expected: PASS

**Step 3: UX sanity review**
- Confirm read-only wording is visible
- Confirm default networks are excluded
- Confirm hover/selection label behavior is clean
