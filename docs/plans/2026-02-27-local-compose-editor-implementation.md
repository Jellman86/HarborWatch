# Local Compose Editor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a local Compose project editor in HarborWatch that supports compose YAML and project `.env` edit/validate/save with robust safety controls.

**Architecture:** Extend the existing compose project discovery API with editor-focused endpoints, implement validation and save logic in a dedicated backend package, and add a new UI detail/editor view launched from Stacks. Keep automation non-mutating and treat editor writes as explicit manual operations.

**Tech Stack:** Go (`net/http`, `os`, `path/filepath`, `gopkg.in/yaml.v3`, `compose-go`), Svelte 5, existing HarborWatch API typing/openapi generation.

---

### Task 1: Add failing backend tests for compose editor API contract

**Files:**
- Create: `backend/internal/httpapi/routes_compose_editor_test.go`
- Modify: `backend/internal/httpapi/server_test.go`

**Step 1: Write failing test for project detail endpoint**

```go
func TestComposeProjectEditorDetail_ReturnsComposeAndEnv(t *testing.T) {
    // build local compose metadata + files, call GET /api/compose/projects/{projectKey}
    // assert composeFiles and env payload fields exist.
}
```

**Step 2: Run test to verify it fails**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run TestComposeProjectEditorDetail_ReturnsComposeAndEnv -v`
Expected: FAIL with route not found / missing handler.

**Step 3: Write failing tests for validate/save endpoints**

```go
func TestComposeProjectValidate_RejectsInvalidYAML(t *testing.T) {}
func TestComposeProjectSave_RejectsHashConflict(t *testing.T) {}
```

**Step 4: Run tests to verify they fail**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run 'TestComposeProjectValidate_RejectsInvalidYAML|TestComposeProjectSave_RejectsHashConflict' -v`
Expected: FAIL with missing handlers/behavior.

**Step 5: Commit**

```bash
git -C /config/workspace/HarborWatch add backend/internal/httpapi/routes_compose_editor_test.go backend/internal/httpapi/server_test.go
git -C /config/workspace/HarborWatch commit -m "test(httpapi): add failing local compose editor endpoint tests"
```

### Task 2: Implement compose editor backend core package with TDD

**Files:**
- Create: `backend/internal/composeeditor/service.go`
- Create: `backend/internal/composeeditor/types.go`
- Create: `backend/internal/composeeditor/service_test.go`

**Step 1: Write failing service test for allowed-path resolution**

```go
func TestResolveAllowedFiles_OnlyProjectComposeAndDotEnv(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/composeeditor -run TestResolveAllowedFiles_OnlyProjectComposeAndDotEnv -v`
Expected: FAIL (package/functions missing).

**Step 3: Write minimal implementation**

```go
type Service struct{}
func (s *Service) ResolveAllowedFiles(...) (...) { ... }
```

**Step 4: Run test to verify it passes**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/composeeditor -run TestResolveAllowedFiles_OnlyProjectComposeAndDotEnv -v`
Expected: PASS.

**Step 5: Add failing tests for validation pipeline**

```go
func TestValidateDraft_InvalidYAML(t *testing.T) {}
func TestValidateDraft_InvalidDotEnv(t *testing.T) {}
```

**Step 6: Run tests to verify they fail**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/composeeditor -run 'TestValidateDraft_InvalidYAML|TestValidateDraft_InvalidDotEnv' -v`
Expected: FAIL.

**Step 7: Implement minimal validation using yaml.v3 + compose-go + dotenv**

```go
func (s *Service) ValidateDraft(...) ValidationResult { ... }
```

**Step 8: Run tests to verify they pass**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/composeeditor -run 'TestValidateDraft_InvalidYAML|TestValidateDraft_InvalidDotEnv' -v`
Expected: PASS.

**Step 9: Commit**

```bash
git -C /config/workspace/HarborWatch add backend/internal/composeeditor/types.go backend/internal/composeeditor/service.go backend/internal/composeeditor/service_test.go
git -C /config/workspace/HarborWatch commit -m "feat(composeeditor): add draft validation service for compose and env"
```

### Task 3: Implement robust save flow (atomic writes + hash checks + allowlist)

**Files:**
- Modify: `backend/internal/composeeditor/service.go`
- Modify: `backend/internal/composeeditor/service_test.go`

**Step 1: Write failing save tests**

```go
func TestSaveDraft_SucceedsWithMatchingHashes(t *testing.T) {}
func TestSaveDraft_FailsOnHashConflict(t *testing.T) {}
func TestSaveDraft_RejectsUnknownPath(t *testing.T) {}
```

**Step 2: Run tests to verify they fail**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/composeeditor -run 'TestSaveDraft_' -v`
Expected: FAIL.

**Step 3: Implement minimal save flow**

```go
func (s *Service) SaveDraft(...) (SaveResult, error) {
    // verify hashes, allowlist, write tmp->rename atomically
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/composeeditor -run 'TestSaveDraft_' -v`
Expected: PASS.

**Step 5: Commit**

```bash
git -C /config/workspace/HarborWatch add backend/internal/composeeditor/service.go backend/internal/composeeditor/service_test.go
git -C /config/workspace/HarborWatch commit -m "feat(composeeditor): add guarded atomic save with hash conflict detection"
```

### Task 4: Wire HTTP routes for detail, validate, and save

**Files:**
- Modify: `backend/internal/httpapi/routes_compose.go`
- Modify: `backend/internal/httpapi/server.go`
- Modify: `backend/internal/httpapi/routes_compose_editor_test.go`

**Step 1: Implement GET editor detail endpoint**

```go
r.Get("/projects/{projectKey}", ...)
```

**Step 2: Run detail endpoint test**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run TestComposeProjectEditorDetail_ReturnsComposeAndEnv -v`
Expected: PASS.

**Step 3: Implement validate endpoint**

```go
r.Post("/projects/{projectKey}/validate", ...)
```

**Step 4: Run validate endpoint test**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run TestComposeProjectValidate_RejectsInvalidYAML -v`
Expected: PASS.

**Step 5: Implement save endpoint**

```go
r.Put("/projects/{projectKey}", ...)
```

**Step 6: Run save endpoint test**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run TestComposeProjectSave_RejectsHashConflict -v`
Expected: PASS.

**Step 7: Commit**

```bash
git -C /config/workspace/HarborWatch add backend/internal/httpapi/routes_compose.go backend/internal/httpapi/server.go backend/internal/httpapi/routes_compose_editor_test.go
git -C /config/workspace/HarborWatch commit -m "feat(httpapi): add local compose editor detail validate and save endpoints"
```

### Task 5: Add snapshot-before-save safety and failure handling

**Files:**
- Modify: `backend/internal/httpapi/routes_compose.go`
- Modify: `backend/internal/httpapi/compose_backup_scheduler_test.go`
- Modify: `backend/internal/httpapi/routes_compose_editor_test.go`

**Step 1: Write failing test that snapshot failure blocks save**

```go
func TestComposeProjectSave_BlocksWhenSnapshotFails(t *testing.T) {}
```

**Step 2: Run test to verify it fails**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run TestComposeProjectSave_BlocksWhenSnapshotFails -v`
Expected: FAIL.

**Step 3: Implement snapshot hook before file writes**

```go
// call composesnapshots.CreateProjectSnapshot before save
```

**Step 4: Run test to verify it passes**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run TestComposeProjectSave_BlocksWhenSnapshotFails -v`
Expected: PASS.

**Step 5: Commit**

```bash
git -C /config/workspace/HarborWatch add backend/internal/httpapi/routes_compose.go backend/internal/httpapi/routes_compose_editor_test.go backend/internal/httpapi/compose_backup_scheduler_test.go
git -C /config/workspace/HarborWatch commit -m "fix(compose): require snapshot creation before editor save"
```

### Task 6: Update OpenAPI and regenerate shared API types

**Files:**
- Modify: `api/openapi.yaml`
- Modify: `backend/internal/gen/types_gen.go`
- Modify: `web/src/lib/api-types.ts`

**Step 1: Add endpoint schemas and routes to OpenAPI**

```yaml
/api/compose/projects/{projectKey}:
  get:
  put:
/api/compose/projects/{projectKey}/validate:
  post:
```

**Step 2: Regenerate API types**

Run: `cd /config/workspace/HarborWatch && ./scripts/generate-types.sh`
Expected: generated Go and TS types updated.

**Step 3: Run compile check for generated types**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run TestDummy -count=1`
Expected: package compiles (no type errors).

**Step 4: Commit**

```bash
git -C /config/workspace/HarborWatch add api/openapi.yaml backend/internal/gen/types_gen.go web/src/lib/api-types.ts
git -C /config/workspace/HarborWatch commit -m "docs(api): define local compose editor endpoints and regenerate types"
```

### Task 7: Add frontend compose project editor UI

**Files:**
- Create: `web/src/lib/pages/ComposeProjectDetail.svelte`
- Modify: `web/src/lib/pages/Stacks.svelte`
- Modify: `web/src/App.svelte`

**Step 1: Add route wiring for compose project detail**

```svelte
{:else if currentRoute === 'compose-project-detail'}
```

**Step 2: Implement compose file tabs and editor panel**

```svelte
{#each composeFiles as file}
  <!-- tab + textarea -->
{/each}
```

**Step 3: Implement `.env` editor panel**

```svelte
<textarea bind:value={envDraft} />
```

**Step 4: Hook validate and save actions to API**

```ts
await fetch(`/api/compose/projects/${projectKey}/validate`, {...})
await fetch(`/api/compose/projects/${projectKey}`, {...})
```

**Step 5: Keep containers/services list under editor**

```svelte
{#each project.members as m}
```

**Step 6: Build frontend to verify**

Run: `cd /config/workspace/HarborWatch/web && npm run build`
Expected: PASS.

**Step 7: Commit**

```bash
git -C /config/workspace/HarborWatch add web/src/lib/pages/ComposeProjectDetail.svelte web/src/lib/pages/Stacks.svelte web/src/App.svelte
git -C /config/workspace/HarborWatch commit -m "feat(web): add local compose and env editor view from stacks"
```

### Task 8: Add frontend correctness polish and conflict UX

**Files:**
- Modify: `web/src/lib/pages/ComposeProjectDetail.svelte`
- Modify: `web/src/lib/components/PaginationBar.svelte` (only if needed for consistency)

**Step 1: Add dirty-state and disable save when no changes**

```ts
const hasDirtyChanges = ...
```

**Step 2: Add hash-conflict warning banner**

```svelte
{#if conflictError}
  <div>Source changed externally...</div>
{/if}
```

**Step 3: Add read-only and unavailable state messaging**

```svelte
{#if !file.writable} ... {/if}
```

**Step 4: Run frontend checks**

Run: `cd /config/workspace/HarborWatch/web && npm run check`
Expected: PASS.

**Step 5: Commit**

```bash
git -C /config/workspace/HarborWatch add web/src/lib/pages/ComposeProjectDetail.svelte web/src/lib/components/PaginationBar.svelte
git -C /config/workspace/HarborWatch commit -m "fix(web): harden compose editor conflict and read-only UX"
```

### Task 9: Full backend and frontend verification

**Files:**
- No file modifications required.

**Step 1: Run backend test set for changed areas**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/composeeditor ./internal/httpapi ./internal/composesnapshots`
Expected: PASS.

**Step 2: Run full backend smoke**

Run: `cd /config/workspace/HarborWatch/backend && go test ./...`
Expected: PASS.

**Step 3: Run frontend check and build**

Run: `cd /config/workspace/HarborWatch/web && npm run check && npm run build`
Expected: PASS.

**Step 4: Manual API smoke with curl**

Run: `curl -sS http://localhost:8080/api/compose/projects | jq`
Expected: list includes local compose projects.

**Step 5: Commit verification notes (if captured in docs)**

```bash
git -C /config/workspace/HarborWatch commit --allow-empty -m "chore: verify local compose editor end-to-end"
```

### Task 10: Documentation and changelog updates

**Files:**
- Modify: `CHANGELOG.md`
- Modify: `docs/FEATURES.md`
- Modify: `docs/CONFIGURATION.md`

**Step 1: Add changelog entry under Unreleased**

```md
- Added local Compose source editor with compose and .env validation/save.
```

**Step 2: Document behavior split**

```md
- Automated updates remain non-mutating.
- Manual editor can update compose/.env source intentionally.
```

**Step 3: Verify docs build/lint expectations (if any)**

Run: `cd /config/workspace/HarborWatch && rg -n "local Compose source editor|non-mutating" docs CHANGELOG.md`
Expected: matches in all intended docs.

**Step 4: Commit**

```bash
git -C /config/workspace/HarborWatch add CHANGELOG.md docs/FEATURES.md docs/CONFIGURATION.md
git -C /config/workspace/HarborWatch commit -m "docs: document local compose editor and source mutation policy"
```
