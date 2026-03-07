# GitOps Deploy Jobs Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Convert GitOps deploy actions into tracked background jobs that drive the global progress bar and GitOps deployment status UI.

**Architecture:** Reuse the shared in-memory job manager for live progress, add durable deployment runtime fields in SQLite for GitOps page state and restart reconciliation, and refactor the deploy endpoint to enqueue work instead of blocking the request.

**Tech Stack:** Go, Chi, SQLite, Svelte

---

### Task 1: Add durable GitOps deploy status fields

**Files:**
- Modify: `backend/internal/gitops/types.go`
- Modify: `backend/internal/gitops/store.go`
- Modify: `backend/internal/migrations/migrations.go`
- Modify: `backend/internal/migrations/runner.go`
- Test: `backend/internal/migrations/runner_test.go`
- Test: `backend/internal/gitops/store_test.go`

**Step 1: Write the failing migration/store tests**
- Add a migration test that expects new `git_deployments` columns to exist after `Run(...)`.
- Add a store test that reads/writes deploy runtime fields.

**Step 2: Run tests to verify they fail**
Run: `go test ./backend/internal/migrations ./backend/internal/gitops`
Expected: FAIL because the new columns and store behavior do not exist.

**Step 3: Write minimal migration and store changes**
- Add guarded SQLite migration for the new runtime columns.
- Extend `GitDeployment` and store queries/update helpers.

**Step 4: Run tests to verify they pass**
Run: `go test ./backend/internal/migrations ./backend/internal/gitops`
Expected: PASS.

**Step 5: Commit**
```bash
git add backend/internal/gitops/types.go backend/internal/gitops/store.go backend/internal/migrations/migrations.go backend/internal/migrations/runner.go backend/internal/migrations/runner_test.go backend/internal/gitops/store_test.go
git commit -m "feat(gitops): persist deploy runtime state"
```

### Task 2: Add GitOps deploy job type and enqueue semantics

**Files:**
- Modify: `backend/internal/jobs/manager.go`
- Modify: `backend/internal/jobs/manager_test.go`
- Modify: `backend/internal/httpapi/routes_gitops.go`
- Test: `backend/internal/httpapi/routes_gitops_test.go`

**Step 1: Write the failing tests**
- Add a job manager test for duplicate GitOps deploy registration by deployment id target.
- Add a route test asserting `/deploy` returns immediately with `jobId`, `status`, and duplicate behavior.

**Step 2: Run tests to verify they fail**
Run: `go test ./backend/internal/jobs ./backend/internal/httpapi`
Expected: FAIL because GitOps deploy jobs and response shape do not exist.

**Step 3: Write minimal implementation**
- Add `JobTypeGitOpsDeploy`.
- Change `/deploy` route to enqueue instead of blocking.
- Return existing active job on duplicates.

**Step 4: Run tests to verify they pass**
Run: `go test ./backend/internal/jobs ./backend/internal/httpapi`
Expected: PASS.

**Step 5: Commit**
```bash
git add backend/internal/jobs/manager.go backend/internal/jobs/manager_test.go backend/internal/httpapi/routes_gitops.go backend/internal/httpapi/routes_gitops_test.go
git commit -m "feat(gitops): enqueue deploy jobs"
```

### Task 3: Refactor GitOps deploy execution into a tracked worker

**Files:**
- Modify: `backend/internal/gitops/executor.go`
- Modify: `backend/internal/gitops/service.go`
- Test: `backend/internal/gitops/service_test.go`
- Test: `backend/internal/httpapi/routes_gitops_test.go`

**Step 1: Write the failing tests**
- Add a service test for successful tracked deploy updating stage/progress and deployment runtime fields.
- Add a service test for failed deploy storing bounded error summary and leaving `lastDeployedAt` unchanged.

**Step 2: Run tests to verify they fail**
Run: `go test ./backend/internal/gitops ./backend/internal/httpapi`
Expected: FAIL because tracked worker execution does not exist.

**Step 3: Write minimal implementation**
- Extract common deploy execution into a method that accepts job progress callbacks.
- Update deployment runtime state through queued/running/completed/failed transitions.
- Bound captured output before persisting.

**Step 4: Run tests to verify they pass**
Run: `go test ./backend/internal/gitops ./backend/internal/httpapi`
Expected: PASS.

**Step 5: Commit**
```bash
git add backend/internal/gitops/executor.go backend/internal/gitops/service.go backend/internal/gitops/service_test.go backend/internal/httpapi/routes_gitops_test.go
git commit -m "feat(gitops): track deploy worker progress"
```

### Task 4: Reconcile stale deploy state on startup

**Files:**
- Modify: `backend/internal/gitops/store.go`
- Modify: `backend/internal/httpapi/server.go`
- Test: `backend/internal/gitops/store_test.go`
- Test: `backend/internal/httpapi/server_test.go`

**Step 1: Write the failing tests**
- Add a store test for marking `queued`/`running` deploy statuses as interrupted.
- Add a server startup test that verifies reconciliation is invoked.

**Step 2: Run tests to verify they fail**
Run: `go test ./backend/internal/gitops ./backend/internal/httpapi`
Expected: FAIL because restart reconciliation does not exist.

**Step 3: Write minimal implementation**
- Add a store method to mark stale in-flight deploys as failed/interrupted.
- Invoke it during server startup next to the other stale job recovery hooks.

**Step 4: Run tests to verify they pass**
Run: `go test ./backend/internal/gitops ./backend/internal/httpapi`
Expected: PASS.

**Step 5: Commit**
```bash
git add backend/internal/gitops/store.go backend/internal/httpapi/server.go backend/internal/gitops/store_test.go backend/internal/httpapi/server_test.go
git commit -m "fix(gitops): recover stale deploy states"
```

### Task 5: Surface deploy jobs in GitOps UI and global progress

**Files:**
- Modify: `web/src/lib/pages/GitOps.svelte`
- Modify: `web/src/lib/components/GlobalProgress.svelte`
- Modify: `web/src/lib/api-types.ts`
- Test: `web/src/lib/components/GlobalProgress.svelte`
- Test: `web/src/lib/pages/GitOps.svelte`

**Step 1: Write the failing UI tests or targeted assertions**
- Add a test for GitOps deployment state rendering from API data.
- Add a test for global progress labels/tags for GitOps deploy jobs.

**Step 2: Run tests to verify they fail**
Run: `npm --prefix web test -- <targeted tests>`
Expected: FAIL because GitOps deploy job state is not rendered.

**Step 3: Write minimal implementation**
- Extend deployment row rendering with status badge/message/timestamps.
- Change `deployNow` to handle enqueue semantics.
- Teach `GlobalProgress` to label GitOps deploy jobs clearly.

**Step 4: Run tests to verify they pass**
Run: `npm --prefix web test -- <targeted tests>`
Expected: PASS.

**Step 5: Commit**
```bash
git add web/src/lib/pages/GitOps.svelte web/src/lib/components/GlobalProgress.svelte web/src/lib/api-types.ts
git commit -m "feat(web): surface gitops deploy progress"
```

### Task 6: Full verification

**Files:**
- Modify: `CHANGELOG.md` (only if user-facing release notes are needed)

**Step 1: Run backend verification**
Run: `go test ./...`
Expected: PASS.

**Step 2: Run frontend verification**
Run: `npm --prefix web run build`
Expected: PASS.

**Step 3: Manual smoke**
- Trigger `Deploy Now` on a GitOps deployment.
- Confirm immediate API return, visible active job in global progress, and final deployment status on the GitOps page.

**Step 4: Commit**
```bash
git add .
git commit -m "feat(gitops): track deploy jobs in progress UI"
```
