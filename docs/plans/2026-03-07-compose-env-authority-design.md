# Compose Env Authority Design

**Goal:** Make HarborWatch treat compose-adjacent `.env` files as normal editable project files while preserving source authority for compose files managed locally, by GitOps, or by Portainer.

## Problem

Current compose-project editing assumes a single `SourceWritable` flag for both compose files and `.env`. That is too coarse.

Operationally, we want:
- local compose projects: compose files editable, adjacent `.env` editable
- Git-backed compose projects: compose files read-only, adjacent `.env` editable and creatable
- Portainer-managed stacks: unchanged safety behavior

We also want `.env` handling to behave like a normal dotenv editor, not like a secret vault:
- no secret masking semantics
- allow free-text values and paste-heavy editing
- auto-discover `.env` sitting beside compose files

## Design

### 1. Source authority becomes file-type aware

Compose authority must be tracked separately for compose files and `.env` files.

Project metadata should expose:
- `composeEditable`
- `envEditable`
- `envCreatable`
- `envPath`
- `envExists`

`sourceStatus` remains useful for overall display, but save permissions should no longer depend on a single writable bit.

### 2. Adjacent `.env` is the default env model

For compose editing, HarborWatch should resolve the project env path as `workingDir/.env`.

Behavior:
- if `.env` exists: load and populate it
- if `.env` does not exist and env creation is allowed: return an empty editable env draft bound to that path
- if working dir is unavailable or unreadable: env editing is disabled with a clear reason

This keeps HarborWatch aligned with common Docker Compose workflows.

### 3. Git-backed compose stays authoritative

Git-managed compose files remain read-only in HarborWatch.

HarborWatch may still create or update a local `.env` alongside those compose files when the deployment directory is writable. This preserves:
- Git as the source of truth for compose manifests
- local operator control for machine-specific overrides and secrets

### 4. `.env` values are plain text, not secrets

The compose editor should not apply secret-style treatment to `.env` values.

Implications:
- values render plainly
- values accept arbitrary text and paste input
- validation remains dotenv parsing, not secret classification

### 5. Save flow becomes env-aware

Save behavior should be split:
- local compose project: save compose files and `.env`
- Git-backed project: save only `.env`
- Portainer-managed project: unchanged; no compose editor bypass

Snapshot behavior should remain compose-focused. We should not create a compose snapshot for an env-only edit if no compose file is being written.

## Data Flow

1. HarborWatch discovers a compose project from Docker metadata.
2. Source authority is resolved from orchestration metadata.
3. Adjacent `.env` path is derived from the working directory.
4. The editor API returns compose files plus env authority metadata.
5. The UI enables compose editing only when `composeEditable` is true.
6. The UI enables env editing or creation when env authority allows it.
7. Save requests persist only the file classes allowed by source authority.

## Error Handling

- unreadable or missing working directory: env editing disabled with explicit reason
- env creation failure: targeted `env_file_create_failed`
- env parse failure: validation error without masking content
- compose read-only + env editable: compose save rejected, env save allowed
- portainer-managed stacks: editor remains blocked by existing safety gates

## Testing

Backend:
- local compose with existing `.env`
- local compose with missing `.env` and creation allowed
- Git-backed compose with compose read-only and env editable
- env-only save path does not rewrite compose files
- env-only edits do not force compose snapshots

Frontend:
- plain-text env editing
- Git-backed compose read-only messaging
- env editor enabled when compose editor is read-only

## Non-Goals

- secret vault semantics for `.env`
- editing Portainer-managed compose/env directly
- supporting arbitrary external env-file paths in v1 of this change
