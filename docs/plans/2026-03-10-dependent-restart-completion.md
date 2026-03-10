# Dependent Restart Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure update jobs always complete after successful main redeploys even if dependent restart discovery or restart work stalls.

**Architecture:** Bound the best-effort dependent-restart phase with an internal timeout and inject discovery/restart hooks for regression testing. Preserve warning detail in step messages while guaranteeing the outer update pipeline reaches `completed`.

**Tech Stack:** Go, Docker client, SQLite-backed update job store, Go tests

---

### Task 1: Add the failing regression test

**Files:**
- Modify: `backend/internal/updates/service_test.go`

**Step 1: Write the failing test**

Add a Portainer-managed update test that:

- enables `RestartDependentsAfterUpgrade`
- injects a dependent discovery hook that blocks longer than the dependent-restart timeout
- expects the job to reach `completed`
- expects a terminal `restart_dependents` step message mentioning timeout

**Step 2: Run test to verify it fails**

Run: `go test ./backend/internal/updates -run TestPortainerUpdateCompletesWhenDependentRestartTimesOut -count=1`

Expected: FAIL because the job remains stuck in `running`

### Task 2: Implement bounded dependent restart completion

**Files:**
- Modify: `backend/internal/updates/service.go`

**Step 1: Add minimal implementation**

- add service fields for dependent restart timeout and injected discovery/restart functions
- run the dependent-restart phase under a dedicated timeout context
- ensure every path emits a terminal completion message and returns

**Step 2: Run test to verify it passes**

Run: `go test ./backend/internal/updates -run TestPortainerUpdateCompletesWhenDependentRestartTimesOut -count=1`

Expected: PASS

### Task 3: Verify no regression in existing update flows

**Files:**
- Modify: `backend/internal/updates/service_test.go`

**Step 1: Add or keep coverage for normal completion semantics**

Ensure existing update pipeline tests still pass with the new timeout wrapper.

**Step 2: Run targeted suite**

Run: `go test ./backend/internal/updates -count=1`

Expected: PASS

### Task 4: Commit and push

**Files:**
- Modify: `docs/plans/2026-03-10-dependent-restart-completion-design.md`
- Modify: `docs/plans/2026-03-10-dependent-restart-completion.md`
- Modify: `backend/internal/updates/service.go`
- Modify: `backend/internal/updates/service_test.go`

**Step 1: Commit**

```bash
git add docs/plans/2026-03-10-dependent-restart-completion-design.md docs/plans/2026-03-10-dependent-restart-completion.md backend/internal/updates/service.go backend/internal/updates/service_test.go
git commit -m "fix(updates): bound dependent restart completion"
git push origin dev
```
