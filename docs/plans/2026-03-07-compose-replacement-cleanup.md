# Compose Replacement Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Automatically remove stale Compose replacement containers before GitOps deploys so redeploys do not fail on leftover name conflicts.

**Architecture:** Add a pre-apply cleanup step in the GitOps executor that uses the Docker API to remove only non-running replacement containers tied to the exact compose file and working directory.

**Tech Stack:** Go, Docker Engine API

---

### Task 1: Add failing tests for stale replacement cleanup

**Files:**
- Modify: `backend/internal/gitops/executor_test.go`

**Step 1: Write the failing test**
- Add tests covering:
  - matching non-running replacement container is removed
  - running replacement container is preserved
  - non-replacement container is preserved
  - removal error is returned

**Step 2: Run test to verify it fails**
Run: `go test ./internal/gitops`
Expected: FAIL because cleanup logic does not yet exist.

### Task 2: Implement targeted cleanup

**Files:**
- Modify: `backend/internal/gitops/executor.go`

**Step 1: Add helper and Docker client seam**
- Add a small Docker client interface for listing/removing containers.
- Add a helper that removes only the targeted stale replacements.

**Step 2: Call cleanup before compose apply**
- Run cleanup after optional pull and before `up -d --remove-orphans`.

**Step 3: Run test to verify it passes**
Run: `go test ./internal/gitops`
Expected: PASS.

### Task 3: Full verification and push

**Step 1: Run backend verification**
Run: `go test ./...`
Expected: PASS.

**Step 2: Commit and push**
```bash
git add .
git commit -m "fix(gitops): clean stale compose replacements"
git push origin dev
```
