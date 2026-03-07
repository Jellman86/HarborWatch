# GitOps Pull-On-Deploy Design

**Goal:** Allow each GitOps deployment to opt into pulling newer images before redeploying, while keeping the default behavior unchanged.

**Problem:** HarborWatch GitOps deploy currently runs `docker-compose up -d --remove-orphans` only. That reconciles the stack but does not reliably fetch newer images for existing tags. Users need an explicit per-stack policy to refresh images when redeploying.

**Approach:** Add a per-deployment `pullOnDeploy` flag, default `false`. When enabled, HarborWatch will run `docker-compose pull` before `docker-compose up -d --remove-orphans`. This applies to both manual deploys and sync-triggered auto-deploys. The existing GitOps deploy job model remains in place and gains one extra progress stage for image pulling.

## Architecture
- Extend `git_deployments` with a durable `pull_on_deploy` column.
- Expose the field through GitOps create/update/list APIs.
- Update the executor to run `pull` before `up` when enabled.
- Keep strict semantics: if `pull` is requested and fails, the deploy fails.

## Runtime Behavior
- `pullOnDeploy = false`
  - `docker-compose up -d --remove-orphans`
- `pullOnDeploy = true`
  - `docker-compose pull`
  - `docker-compose up -d --remove-orphans`

This uses the already-supported `docker-compose 1.29.2` behavior rather than relying on `--pull always`.

## UI
- Add a per-deployment toggle in add/edit deployment forms.
- Show current deploy image policy on the deployment row, e.g. `Pull Before Deploy` vs `Use Local Images`.

## Error Handling
- If pull is enabled and `pull` fails, stop and mark deploy failed.
- Persist bounded pull/apply output as the deployment summary.
- Duplicate-job prevention and restart recovery remain unchanged.

## Testing
- migration/store coverage for `pull_on_deploy`
- API coverage for create/update/list
- executor coverage for:
  - disabled: no pull
  - enabled: pull then up
  - pull failure: no up
- frontend build verification after wiring the toggle
