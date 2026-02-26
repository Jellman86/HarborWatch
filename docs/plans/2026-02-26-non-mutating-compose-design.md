# Non-Mutating Local Compose Support Design

## Goal

Support local Docker Compose containers/projects as a first-class orchestration mode while preserving compose files exactly. HarborWatch should be compose-aware (use `docker compose`, respect compose runtime semantics) but must not behave like a compose file manager.

## Scope

- Pivot local compose upgrade execution to a non-mutating workflow.
- Block compose upgrades that require changing the declared compose image reference.
- Add HarborWatch-managed snapshots (zip archives) for local compose config files and project `.env`.
- Surface snapshot/change status in local compose project UI and compose container lifecycle messaging.

Out of scope:

- Editing compose YAML or `.env` files
- User-facing compose editor or restore UI
- Parsing arbitrary `env_file:` entries from compose YAML

## Principles

1. Compose source is authoritative.
2. HarborWatch may orchestrate compose commands but will not rewrite source files.
3. Users without git still need backup/change visibility.
4. Automation must only run when it does not require source mutation.

## Behavior Changes

### Local Compose upgrades

HarborWatch will run compose updates using:

- `docker compose ... pull <service>`
- `docker compose ... up -d --no-deps --force-recreate <service>`

HarborWatch will no longer patch compose YAML image refs.

### Eligibility / gating

For local compose containers:

- If target image ref differs from the declared compose image ref, HarborWatch blocks the upgrade (manual and auto) with a clear precondition error.
- If the target image ref matches the declared compose image ref (including mutable tags like `latest`/`dev`), HarborWatch can run compose pull + up safely.

This preserves compose source and still supports mutable-tag refreshes.

## Snapshots and Change Detection

### Snapshot contents

Before a local compose update action, HarborWatch creates a zip archive containing:

- All compose config files from `com.docker.compose.project.config_files`
- Project `.env` in `com.docker.compose.project.working_dir` (if present)
- `manifest.json` with metadata (project/service, timestamp, file list, hashes, mtimes, sizes)

### Snapshot storage

- HarborWatch-managed snapshot root path (configurable in Settings)
- Per-project/service archive naming for deterministic browsing
- Metadata stored in HarborWatch database/settings store for UI history (or derivable by scanning snapshot manifests)

### Change detection

For each local compose project:

- Compute current fingerprints for compose files + project `.env`
- Compare against latest HarborWatch snapshot manifest
- Report status:
  - `none` (no snapshot)
  - `unchanged`
  - `changed`

## UI Changes

### Container Lifecycle (Compose mode)

- Explicit message: HarborWatch respects compose source files and does not edit them.
- If blocked due to source mismatch, show actionable reason:
  - “Pinned compose image ref must be changed in compose source manually.”

### Stacks page (Local Compose Projects)

Add compose-tailored context:

- snapshot status badge
- changed file count
- last snapshot timestamp
- snapshot path/location hint (global setting if configured)

## Error Handling

- Snapshot creation failure blocks compose update (safe default).
- Compose pull/up failures return command errors as before.
- No source-file rollback is attempted (files are never mutated).

## Testing

- Unit tests for compose eligibility/gating with non-mutating rules
- Unit tests for snapshot archive + manifest creation
- Unit tests for project change detection against latest snapshot
- UI build verification for new status rendering
