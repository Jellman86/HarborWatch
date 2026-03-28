# Compose Env Authority Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Let HarborWatch auto-discover, edit, and create adjacent `.env` files while keeping compose file editability aligned with local, Git-backed, and Portainer source authority.

**Architecture:** Extend compose project metadata so compose and env editability are tracked separately. Update the compose editor backend to load/save adjacent `.env` files independently of compose files, then update the UI so `.env` behaves like a normal editable dotenv file and Git-backed compose remains read-only.

**Tech Stack:** Go backend, Chi HTTP API, Svelte 5 frontend, Docker Compose metadata discovery, dotenv parsing via compose-go.

---

### Task 1: Add source-aware compose/env authority metadata

**Files:**
- Modify: `backend/internal/httpapi/orchestration_mode.go`
- Modify: `backend/internal/httpapi/routes_compose.go`
- Test: `backend/internal/httpapi/orchestration_mode_test.go`

**Step 1: Write the failing test**
- Add tests covering:
  - local compose project => compose editable, env editable
  - Git-backed compose project => compose read-only, env editable/creatable
  - missing working dir => env not editable

**Step 2: Run test to verify it fails**
Run: `go test ./backend/internal/httpapi -run 'TestCompose(Project|Env)Authority'`
Expected: FAIL because metadata does not exist yet.

**Step 3: Write minimal implementation**
- Add env authority fields to `localComposeProject` and `composeProjectEditorResponse`.
- Detect Git-backed compose using existing project/deployment authority.
- Derive adjacent `.env` path and env editability/creatability from working dir writability.

**Step 4: Run test to verify it passes**
Run: `go test ./backend/internal/httpapi -run 'TestCompose(Project|Env)Authority'`
Expected: PASS.

**Step 5: Commit**
```bash
git add backend/internal/httpapi/orchestration_mode.go backend/internal/httpapi/routes_compose.go backend/internal/httpapi/orchestration_mode_test.go
git commit -m "feat: add compose env authority metadata"
```

### Task 2: Make compose editor load adjacent `.env` with create support

**Files:**
- Modify: `backend/internal/composeeditor/service.go`
- Modify: `backend/internal/composeeditor/types.go`
- Test: `backend/internal/composeeditor/service_test.go`

**Step 1: Write the failing test**
- Add tests for:
  - load existing adjacent `.env`
  - return empty editable env model when `.env` is absent but creatable
  - preserve free-text env content

**Step 2: Run test to verify it fails**
Run: `go test ./backend/internal/composeeditor -run 'TestLoadProjectFiles'`
Expected: FAIL because missing `.env` currently returns nil.

**Step 3: Write minimal implementation**
- Extend `ProjectDescriptor` / env state types with env authority flags.
- `LoadProjectFiles` should resolve `workingDir/.env`, return existing file state if present, otherwise an empty file state when creation is allowed.

**Step 4: Run test to verify it passes**
Run: `go test ./backend/internal/composeeditor -run 'TestLoadProjectFiles'`
Expected: PASS.

**Step 5: Commit**
```bash
git add backend/internal/composeeditor/service.go backend/internal/composeeditor/types.go backend/internal/composeeditor/service_test.go
git commit -m "feat: load adjacent compose env files"
```

### Task 3: Allow env-only save without compose writes or compose snapshots

**Files:**
- Modify: `backend/internal/composeeditor/service.go`
- Modify: `backend/internal/httpapi/routes_compose.go`
- Test: `backend/internal/httpapi/routes_compose_editor_test.go`
- Test: `backend/internal/composeeditor/service_test.go`

**Step 1: Write the failing test**
- Add tests for:
  - Git-backed project can save `.env` while compose files remain read-only
  - env-only save does not rewrite compose files
  - env-only save skips compose snapshot creation

**Step 2: Run test to verify it fails**
Run: `go test ./backend/internal/httpapi -run 'TestCompose(ProjectSave|EnvOnlySave)'`
Expected: FAIL because save is blocked by `SourceWritable` and snapshots always run.

**Step 3: Write minimal implementation**
- Split save permission checks for compose vs env.
- Allow env-only saves when env is editable/creatable.
- Only create compose snapshots when at least one compose file is being written.

**Step 4: Run test to verify it passes**
Run: `go test ./backend/internal/httpapi -run 'TestCompose(ProjectSave|EnvOnlySave)'`
Expected: PASS.

**Step 5: Commit**
```bash
git add backend/internal/composeeditor/service.go backend/internal/httpapi/routes_compose.go backend/internal/httpapi/routes_compose_editor_test.go backend/internal/composeeditor/service_test.go
git commit -m "fix: support env-only compose editor saves"
```

### Task 4: Update validation and conflict handling for env-only drafts

**Files:**
- Modify: `backend/internal/httpapi/routes_compose.go`
- Modify: `backend/internal/composeeditor/service.go`
- Test: `backend/internal/httpapi/routes_compose_editor_test.go`

**Step 1: Write the failing test**
- Add tests that env-only validation works for Git-backed projects and env hash conflicts are reported cleanly.

**Step 2: Run test to verify it fails**
Run: `go test ./backend/internal/httpapi -run 'TestCompose(EnvValidation|EnvHashConflict)'`
Expected: FAIL because current flow assumes project-wide writability.

**Step 3: Write minimal implementation**
- Keep dotenv parsing validation.
- Preserve env hash conflict checks separately from compose hashes.
- Return targeted env conflict errors.

**Step 4: Run test to verify it passes**
Run: `go test ./backend/internal/httpapi -run 'TestCompose(EnvValidation|EnvHashConflict)'`
Expected: PASS.

**Step 5: Commit**
```bash
git add backend/internal/httpapi/routes_compose.go backend/internal/composeeditor/service.go backend/internal/httpapi/routes_compose_editor_test.go
git commit -m "fix: validate and guard compose env drafts"
```

### Task 5: Update Compose Project UI for plain-text env editing

**Files:**
- Modify: `web/src/lib/pages/ComposeProjectDetail.svelte`
- Test: `web/src/lib/pages/ComposeProjectDetail.test.ts` or existing compose UI test file if present

**Step 1: Write the failing test**
- Add UI tests for:
  - Git-backed compose shows compose read-only but env editable
  - env editor remains enabled when compose editor is disabled
  - env save is allowed with compose unchanged

**Step 2: Run test to verify it fails**
Run: `npm --prefix web test -- ComposeProjectDetail`
Expected: FAIL because UI uses a single save-block reason today.

**Step 3: Write minimal implementation**
- Add separate compose/env editability state.
- Render clear read-only messaging for Git-backed compose.
- Keep `.env` editor as plain text, not secret-like.

**Step 4: Run test to verify it passes**
Run: `npm --prefix web test -- ComposeProjectDetail`
Expected: PASS.

**Step 5: Commit**
```bash
git add web/src/lib/pages/ComposeProjectDetail.svelte web/src/lib/pages/ComposeProjectDetail.test.ts
git commit -m "feat: allow env editing for git-backed compose projects"
```

### Task 6: Run full verification and update docs if needed

**Files:**
- Modify: `docs/FEATURES.md` or relevant user-facing docs only if behavior is surfaced there

**Step 1: Run backend test suite for touched areas**
Run: `go test ./backend/internal/composeeditor ./backend/internal/httpapi`
Expected: PASS.

**Step 2: Run frontend verification**
Run: `npm --prefix web test -- ComposeProjectDetail && npm --prefix web run check`
Expected: PASS.

**Step 3: Run targeted API verification**
- Start HarborWatch locally if needed and verify compose project load/save flows with `curl`.

**Step 4: Review for robustness**
- Confirm Portainer-managed stacks remain blocked.
- Confirm env-only saves do not snapshot or rewrite compose files.
- Confirm free-text `.env` values round-trip cleanly.

**Step 5: Commit**
```bash
git add docs/FEATURES.md
git commit -m "docs: describe compose env editing authority"
```
