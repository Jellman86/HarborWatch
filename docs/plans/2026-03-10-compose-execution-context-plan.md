# Compose Execution Context Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Centralize compose command context so GitOps deploys and compose-managed updates use the same config files, working directory, and env-file resolution for both `pull` and `up`.

**Architecture:** Add a shared backend helper that builds a compose execution context from explicit inputs. Migrate GitOps deploy execution and compose-managed update execution to use that helper, while extending update requests to carry discovered project `.env` and HarborWatch-managed override env files.

**Tech Stack:** Go, Docker Compose CLI, HarborWatch backend unit tests

---

### Task 1: Add failing shared-helper and update-path tests

**Files:**
- Modify: `backend/internal/gitops/executor_test.go`
- Modify: `backend/internal/updates/executor_test.go`
- Create: `backend/internal/composeexec/context_test.go`

**Step 1: Write the failing tests**

- Add a helper test for compose prefix/env-file ordering.
- Add an update executor test asserting both compose `pull` and `up` receive ordered `--env-file` arguments.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/composeexec ./internal/updates ./internal/gitops -run 'Test(ComposeExecutionContext|RunTrackedDeployPullOnDeployUsesManagedEnvFileForPullAndUp|Compose(Pull|Up)ArgsIncludeEnvFiles)' -count=1`

Expected: failures showing missing shared helper and missing env-file propagation.

**Step 3: Commit**

```bash
git add backend/internal/gitops/executor_test.go backend/internal/updates/executor_test.go backend/internal/composeexec/context_test.go
git commit -m "test(compose): cover shared compose execution context"
```

### Task 2: Implement shared compose execution context

**Files:**
- Create: `backend/internal/composeexec/context.go`
- Test: `backend/internal/composeexec/context_test.go`

**Step 1: Write minimal implementation**

- Add a context struct with compose files, workdir, env files.
- Add helpers to build the common compose prefix and derive `pull` / `up` args.

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/composeexec -count=1`

Expected: PASS

**Step 3: Commit**

```bash
git add backend/internal/composeexec/context.go backend/internal/composeexec/context_test.go
git commit -m "feat(compose): add shared execution context"
```

### Task 3: Migrate GitOps deploy execution

**Files:**
- Modify: `backend/internal/gitops/executor.go`
- Test: `backend/internal/gitops/executor_test.go`

**Step 1: Refactor GitOps deploy execution**

- Keep existing env-file discovery/writing behavior.
- Replace ad-hoc command assembly with the shared context helper.
- Ensure both `pull` and `up` use identical compose prefix arguments.

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/gitops -count=1`

Expected: PASS

**Step 3: Commit**

```bash
git add backend/internal/gitops/executor.go backend/internal/gitops/executor_test.go
git commit -m "refactor(gitops): use shared compose execution context"
```

### Task 4: Extend compose-managed update execution

**Files:**
- Modify: `backend/internal/updates/service.go`
- Modify: `backend/internal/updates/executor.go`
- Modify: `backend/internal/updates/executor_test.go`
- Search: compose update request builders in `backend/internal/httpapi/`

**Step 1: Thread compose env files through update requests**

- Add ordered `ComposeEnvFiles` metadata to update requests.
- Populate it from the discovered compose project `.env` and HarborWatch-managed override file when available.
- Use the shared helper for compose `pull` and `up`.

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/updates -count=1`

Expected: PASS

**Step 3: Commit**

```bash
git add backend/internal/updates/service.go backend/internal/updates/executor.go backend/internal/updates/executor_test.go backend/internal/httpapi/
git commit -m "fix(updates): share compose env context across pull and up"
```

### Task 5: Final verification

**Files:**
- Modify if needed based on failures

**Step 1: Run final verification**

Run: `go test ./internal/composeexec ./internal/gitops ./internal/updates -count=1`

Expected: PASS

**Step 2: Commit final cleanups**

```bash
git add backend/internal/composeexec backend/internal/gitops backend/internal/updates docs/plans/2026-03-10-compose-execution-context-*.md
git commit -m "fix(compose): unify execution context across gitops and updates"
```
