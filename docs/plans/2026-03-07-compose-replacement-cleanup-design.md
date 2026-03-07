# Compose Replacement Cleanup Design

**Goal:** Prevent GitOps deploy failures caused by stale Docker Compose replacement containers left behind from prior failed recreate attempts.

**Problem:** After moving HarborWatch from legacy `docker-compose` v1 to `docker compose` v5, redeploys can fail with name conflicts like:

- `Conflict. The container name "/a116645eca1b_yawamf-backend" is already in use`

These containers are not active services. They are failed replacement containers created during `docker compose up` with labels such as `com.docker.compose.replace`.

**Approach:** Before running `docker compose up`, HarborWatch will remove only stale, non-running replacement containers that belong to the exact target compose project instance.

## Scope
- Cleanup applies only to GitOps deploys.
- Cleanup targets containers that:
  - are not running
  - have a non-empty `com.docker.compose.replace` label
  - match both:
    - `com.docker.compose.project.config_files = <compose file path>`
    - `com.docker.compose.project.working_dir = <work dir>`

## Safety
- Do not delete active/running containers.
- Do not delete containers from other projects.
- Do not delete ordinary exited containers that are not Compose replacements.
- If cleanup cannot initialize a Docker client, fail the deploy rather than proceeding into a known-conflict state.

## Testing
- Add unit tests for the cleanup selector:
  - removes only matching non-running replacement containers
  - skips running replacements
  - skips non-replacement containers
  - surfaces Docker client removal errors

## Runtime Behavior
- Deploy flow becomes:
  - validate deployment
  - resolve env/runtime
  - optional pull
  - clean stale replacement containers
  - `docker compose up -d --remove-orphans`

This keeps the fix narrow to the actual failure mode instead of adding broad Docker cleanup behavior.
