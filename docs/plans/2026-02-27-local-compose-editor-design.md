# Local Compose Editor Design

## Goal

Add a first-class manual editor for local Docker Compose projects in HarborWatch so operators can:

- View and edit compose YAML files.
- View and edit project `.env` file content.
- Validate syntax and compose semantics before saving.
- Save changes safely, with conflict detection and robust file guards.

The editor is intentionally scoped to local Compose projects only. Portainer stack editing is deferred.

## Background and Constraints

- HarborWatch currently supports local compose project discovery and compose-aware non-mutating automation.
- Existing policy remains: automated update flows do not rewrite compose source.
- New editor capability is a deliberate manual operation by the operator.
- Runtime architecture constraints remain unchanged (single Go backend, static Svelte UI, SQLite).

## Scope

### In Scope (v1)

- Local compose project detail view from Stacks page.
- Editable compose file panel (single or multi-file projects).
- Editable project `.env` panel (when present, optional creation support in project working dir).
- Validate action for compose + env drafts.
- Save action with atomic writes and optimistic concurrency checks.
- Optional prettify action for compose YAML only.

### Out of Scope (v1)

- Portainer stack editor.
- Git-aware merge/conflict tooling beyond hash-based optimistic concurrency.
- Full schema-driven form editor (UI remains text-first).
- Editing arbitrary service `env_file` references not discovered as the project `.env`.

## UX Design

## Navigation

- In `Stacks` local compose cards, add an `Open Editor` action.
- Route to a compose project detail view.

## Layout

Top section:
- Compose source editor card.
- File tabs for each compose file in load order.
- Controls: `Validate`, `Prettify`, `Save`.
- Validation diagnostics panel (errors/warnings with file and location when available).

Second section:
- Project `.env` editor card.
- Controls: `Validate`, `Save`.
- Render current file state (`present`, `missing`, `read-only`).

Third section:
- Existing member containers/services list for project context.

## Edit Semantics

- Dirty-state tracked independently for compose and `.env`.
- Save button disabled while validation is running or when no changes exist.
- If source changed since load, show conflict banner and require reload.

## API Design

All endpoints are local-compose only under `/api/compose/projects`.

### 1) Get Project Editor Data

`GET /api/compose/projects/{projectKey}`

Returns:
- `projectKey`, `projectName`, `workingDir`, `sourceStatus`
- `composeFiles[]`:
  - `path`, `writable`, `sha256`, `sizeBytes`, `content`
- `.env` object:
  - `path`, `exists`, `writable`, `sha256`, `sizeBytes`, `content`
- `members[]` (existing project members)
- snapshot metadata (reuse existing status fields)

### 2) Validate Draft

`POST /api/compose/projects/{projectKey}/validate`

Body:
- `composeFiles[]` drafts (path + content)
- optional `.env` draft content and exists intent

Returns:
- `ok` boolean
- `diagnostics[]`:
  - severity (`error`/`warning`)
  - source (`yaml`, `compose-spec`, `env`, `runtime`)
  - file path and optional line/column
  - message
- optional `normalizedComposeFiles[]` for prettify preview

### 3) Save Draft

`PUT /api/compose/projects/{projectKey}`

Body:
- `composeFiles[]` drafts with `expectedSha256`
- optional `.env` draft with `expectedSha256` (or empty if file absent)
- flags:
  - `validateBeforeSave` (default true)
  - `prettifyCompose` (default false)

Returns:
- `savedAt`
- updated per-file hashes/sizes
- `validationSummary`

Error conditions:
- 409 on hash mismatch (external modification)
- 403/409 for read-only paths
- 400 for invalid payload or failed validation

## Validation and Parsing Strategy

## Compose YAML

- Syntax parse with `gopkg.in/yaml.v3` for clear parse errors.
- Compose model validation with `compose-go` loader:
  - interpolation, schema, consistency checks.
  - environment includes OS env and provided `.env` draft map.

## Env File

- Parse `.env` with `compose-go/v2/dotenv` parser.
- Validation errors returned with source `env`.
- Preserve raw `.env` formatting by default; no auto-rewrite unless explicit normalize behavior is added later.

## Runtime Parity Check (Optional)

- If `docker compose` binary is available, run `docker compose config -q` with temp draft files for an additional compatibility signal.
- Report as warning if unavailable.

## Prettification Rules

- Compose files only: marshal through yaml encoder (`indent=2`) after successful model load for stable formatting.
- `.env` prettify is not done in v1 to avoid destructive normalization of comments/order.

## Safety and Robustness

- Strict file allowlist from discovered local compose metadata:
  - only project `configFiles` and `{workingDir}/.env`.
- Path canonicalization and root checks to block traversal or arbitrary write attempts.
- Atomic writes (`tmp + fsync + rename`) and permission-preserving mode handling.
- Project-level lock around validate+save pipeline to prevent concurrent saves.
- Snapshot creation before write using existing compose snapshot facility; failure blocks save.
- Structured diagnostics logging for save attempts and failures.

## Data and OpenAPI

- Add request/response schemas for new compose editor endpoints in `api/openapi.yaml`.
- Regenerate:
  - `backend/internal/gen/types_gen.go`
  - `web/src/lib/api-types.ts`

## Testing Strategy

## Backend Tests

- Project editor load:
  - single-file and multi-file compose projects.
  - with and without `.env`.
- Validation:
  - invalid YAML.
  - invalid compose schema.
  - invalid `.env`.
- Save:
  - successful save (compose and env).
  - hash conflict.
  - read-only write rejection.
  - path allowlist rejection.
  - snapshot failure blocks save.

## Frontend Tests

- Project editor loads and renders compose + `.env` editors.
- Validate flow surfaces diagnostics.
- Save success refreshes hashes and clears dirty state.
- Conflict path shows reload prompt.

## Verification Commands

- `go test ./internal/httpapi ./internal/composesnapshots`
- `npm --prefix web run check`
- `npm --prefix web run build`

## Rollout Notes

- Existing automation behavior and policy text stays non-mutating.
- Add explicit copy in UI:
  - “Automatic pipelines do not edit compose source.”
  - “Manual stack editor changes source files intentionally.”
