# GitOps Pull-On-Deploy Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a per-deployment `pullOnDeploy` toggle so GitOps redeploys can optionally pull fresh images before applying the stack.

**Architecture:** Extend the Git deployment schema and API with `pullOnDeploy`, then update the GitOps executor to run `docker-compose pull` before `up` when enabled, while preserving the tracked deploy-job flow.

**Tech Stack:** Go, SQLite, Svelte

---

### Task 1: Add persistent `pull_on_deploy` state

**Files:**
- Modify: `backend/internal/gitops/types.go`
- Modify: `backend/internal/gitops/store.go`
- Modify: `backend/internal/migrations/migrations.go`
- Modify: `backend/internal/migrations/runner.go`
- Test: `backend/internal/migrations/runner_test.go`
- Test: `backend/internal/gitops/store_test.go`

**Step 1: Write the failing test**
- Add a migration test asserting `git_deployments.pull_on_deploy` exists.
- Add a store test asserting the field persists through create/read/update.

**Step 2: Run test to verify it fails**
Run: `go test ./internal/migrations ./internal/gitops`
Expected: FAIL because the column and store support do not exist.

**Step 3: Write minimal implementation**
- Add the field to `GitDeployment`.
- Add a guarded SQLite migration.
- Extend store queries and create/update persistence.

**Step 4: Run test to verify it passes**
Run: `go test ./internal/migrations ./internal/gitops`
Expected: PASS.

### Task 2: Add executor pull behavior

**Files:**
- Modify: `backend/internal/gitops/executor.go`
- Test: `backend/internal/gitops/executor_test.go`

**Step 1: Write the failing test**
- Add one test proving `pullOnDeploy=false` skips pull.
- Add one test proving `pullOnDeploy=true` runs pull before up.
- Add one test proving pull failure aborts before up.

**Step 2: Run test to verify it fails**
Run: `go test ./internal/gitops`
Expected: FAIL because pull-on-deploy behavior does not exist.

**Step 3: Write minimal implementation**
- Insert a `pull` stage before `up` when enabled.
- Emit a progress update like `Pulling images`.
- Fail fast if pull fails.

**Step 4: Run test to verify it passes**
Run: `go test ./internal/gitops`
Expected: PASS.

### Task 3: Expose the toggle through GitOps routes and UI

**Files:**
- Modify: `backend/internal/httpapi/routes_gitops.go`
- Test: `backend/internal/httpapi/routes_gitops_test.go`
- Modify: `web/src/lib/pages/GitOps.svelte`

**Step 1: Write the failing test**
- Add route tests for create/update/list carrying `pullOnDeploy`.

**Step 2: Run test to verify it fails**
Run: `go test ./internal/httpapi`
Expected: FAIL because the API payload does not include the field.

**Step 3: Write minimal implementation**
- Thread `pullOnDeploy` through request/response handling.
- Add a checkbox/toggle in add/edit forms.
- Show policy on the deployment row.

**Step 4: Run verification**
Run: `go test ./internal/httpapi`
Expected: PASS.
Run: `npm --prefix web run build`
Expected: PASS.

### Task 4: Full verification and push

**Step 1: Run backend verification**
Run: `go test ./...`
Expected: PASS.

**Step 2: Run frontend verification**
Run: `npm --prefix web run build`
Expected: PASS.

**Step 3: Commit and push**
```bash
git add .
git commit -m "feat(gitops): add pull-on-deploy policy"
git push origin dev
```
