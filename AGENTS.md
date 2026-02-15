# HarborWatch Agent Guide

## Architecture Boundaries
- `backend/`: Go API server, background jobs, persistence, integrations.
- `web/`: Svelte 5 UI source and frontend tooling.
- `api/`: OpenAPI contract used to generate shared types.
- `deployments/docker/`: Development/runtime container manifests.
- `scripts/`: Project automation scripts.

Production model is a single container running one Go binary that serves API and static web assets.

## Development Commands
- `scripts/dev.sh`: Start local development services/processes.
- `scripts/lint.sh`: Run backend/frontend lint checks.
- `scripts/test.sh`: Run backend/frontend tests and builds.
- `scripts/generate-types.sh`: Regenerate API types from `api/openapi.yaml`.

## Coding Conventions
- Keep changes small and testable.
- Use clear names and explicit error handling.
- Avoid long-running work in HTTP handlers.
- Use Conventional Commits: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`.

## Adding Features
1. Update `api/openapi.yaml` first for contract changes.
2. Regenerate shared API types.
3. Implement backend handlers/services.
4. Integrate frontend UI with typed API models.
5. Add tests and update docs.
