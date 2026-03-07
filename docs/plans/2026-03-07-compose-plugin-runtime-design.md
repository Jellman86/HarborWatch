# Compose Plugin Runtime Design

**Goal:** Make HarborWatch GitOps deploys use the supported Docker Compose plugin (`docker compose`) instead of the legacy standalone `docker-compose` v1 binary.

**Problem:** Live redeploys are currently failing in the HarborWatch container with the legacy `docker-compose` recreate bug:

- `ERROR: for yawamf-backend 'ContainerConfig'`

HarborWatch already prefers `docker compose` in code via `composecli.Resolve()`, but the runtime image only ships `docker-compose` v1. That means deploys fall back to the buggy runtime even though the selection logic is correct.

**Approach:** Replace the legacy standalone runtime in the HarborWatch image with the Docker Compose CLI plugin. Keep the existing runtime resolver logic. Once the plugin is present, HarborWatch will automatically prefer `docker compose`.

## Architecture
- Keep `backend/internal/composecli/composecli.go` as the single source of runtime selection.
- Update the Docker image so `docker compose version` succeeds at runtime.
- Remove the legacy `docker-compose` package from the image to avoid falling back to v1 and to keep the runtime unambiguous.

## Runtime Packaging
- Continue to install `docker.io` for the Docker CLI.
- Install the Docker Compose plugin into Docker's CLI plugin directory rather than using the standalone `docker-compose` package.
- Keep `sqlite3` in the image for operational debugging.

## Testing
- Add a regression test that asserts the Dockerfile no longer installs the legacy `docker-compose` package and does install the Compose plugin.
- Rebuild the HarborWatch image and verify:
  - `docker compose version`
  - no `docker-compose` dependency is required for HarborWatch to function

## Error Handling
- If the plugin is unavailable in the image build, fail the build rather than silently keeping v1.
- Keep the existing `composecli.Resolve()` fallback to `docker-compose` for other environments, but the HarborWatch image itself should no longer rely on it.
