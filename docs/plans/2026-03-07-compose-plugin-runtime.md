# Compose Plugin Runtime Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix HarborWatch GitOps redeploy failures caused by the legacy `docker-compose` runtime by shipping the Docker Compose plugin in the HarborWatch image.

**Architecture:** Keep HarborWatch's existing runtime selection code and replace the runtime image dependency from `docker-compose` v1 to the Docker Compose plugin.

**Tech Stack:** Dockerfile, Go tests

---

### Task 1: Add a Dockerfile regression test

**Files:**
- Add: `backend/internal/composecli/dockerfile_runtime_test.go`

**Step 1: Write the failing test**
- Assert the HarborWatch Dockerfile no longer installs `docker-compose`.
- Assert the Dockerfile installs a Compose plugin runtime.

**Step 2: Run test to verify it fails**
Run: `go test ./internal/composecli`
Expected: FAIL because the current Dockerfile still installs `docker-compose`.

**Step 3: Write minimal implementation**
- No production code yet. This task exists only to prove the regression test catches the current bad state.

**Step 4: Keep the test red until Task 2**

### Task 2: Patch the runtime image

**Files:**
- Modify: `Dockerfile`

**Step 1: Replace legacy runtime packaging**
- Remove `docker-compose` from the apt install list.
- Install the Docker Compose plugin in Docker's CLI plugin directory.
- Keep `docker.io`, `sqlite3`, and the rest of the runtime packages.

**Step 2: Run the targeted test**
Run: `go test ./internal/composecli`
Expected: PASS.

### Task 3: Full verification and push

**Step 1: Run backend verification**
Run: `go test ./...`
Expected: PASS.

**Step 2: Rebuild the HarborWatch image**
Run: `docker build -t harborwatch-compose-plugin:test .`
Expected: PASS.

**Step 3: Probe runtime in the built image**
Run:
- `docker run --rm harborwatch-compose-plugin:test docker compose version`
- `docker run --rm harborwatch-compose-plugin:test sh -lc 'command -v docker-compose || true'`
Expected:
- plugin version succeeds
- legacy standalone binary is absent or not required

**Step 4: Commit and push**
```bash
git add .
git commit -m "fix(runtime): use Docker Compose plugin"
git push origin dev
```
