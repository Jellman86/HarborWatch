# GitOps Deploy Jobs Design

**Goal:** Turn GitOps stack deploys into first-class background jobs so HarborWatch can show reliable progress in the global progress bar and on the GitOps page without blocking the request lifecycle.

**Problem:** `POST /api/gitops/deployments/{id}/deploy` currently runs `docker-compose up -d --remove-orphans` inline. That makes deploys look stalled, causes request timeouts for long-running stacks, and gives the UI no durable stage or result model beyond a final success/failure toast.

**Approach:** Reuse HarborWatch's existing in-memory job manager instead of inventing a second progress system. A deploy request should enqueue a GitOps deploy job, return immediately, and let a background worker run the existing compose apply path while publishing coarse stage updates. The GitOps page should show per-deployment active job state and recent result details, while the existing global progress bar should automatically pick up active deploy jobs through the shared diagnostics path.

## Architecture
- Add a new job type for GitOps deploy work in the shared job manager.
- Split GitOps deploy execution into two layers:
  - enqueue/track path in the HTTP route
  - reusable execution path in the GitOps service that updates job state during deploy stages
- Extend Git deployment persistence with explicit runtime status fields so the GitOps page can show durable deploy state and recover cleanly after process restarts.
- Keep deploy output bounded and summary-oriented; do not store full compose logs in SQLite.

## Job Model
Each deploy job should track:
- `job id`
- `deployment id`
- `source id`
- `target name` derived from the compose path or deployment id
- `status`: `queued`, `running`, `completed`, `failed`
- `message`: current stage summary
- `progress`: coarse measured values only

Recommended stage mapping:
- `queued` / `0`: waiting for worker slot
- `running` / `10`: validating deployment
- `running` / `25`: resolving compose runtime and env sources
- `running` / `50`: running compose apply
- `running` / `90`: persisting deploy result
- `completed` / `100`: deploy finished
- `failed` / `100`: deploy failed

## Persistence Model
Extend `git_deployments` with durable runtime metadata:
- `last_job_id`
- `deploy_status`
- `deploy_status_message`
- `deploy_started_at`
- `deploy_finished_at`
- `deploy_output_summary`

Rules:
- `last_deployed_at` updates only on successful deploy completion
- `last_error` updates only on failure
- `deploy_status` reflects current or most recent deploy attempt
- on HarborWatch startup, any persisted `queued` or `running` deploy status is reconciled to `failed` with an `interrupted` message

This avoids stale forever-running deploy badges after a restart.

## Duplicate Protection
Use the existing `RegisterJobIfNoDuplicate` behavior with a dedicated GitOps job type and deployment id target so repeated clicks on the same deployment while it is queued/running return the active job instead of spawning overlapping compose applies.

Different deployments remain independent.

## API Changes
`POST /api/gitops/deployments/{id}/deploy`
- current: blocks until deploy finishes
- new: enqueues deploy and returns immediately

Response should include:
- `ok`
- `jobId`
- `status`
- `duplicate` flag when an active deploy already exists

Deployment list/get responses should include the new runtime fields so the GitOps page can render live and recent status without a second ad hoc API.

## UI Changes
### GitOps Page
For each deployment row, show:
- active or latest deploy state badge
- latest status message
- last finished timestamp
- last failure summary if present

`Deploy Now` behavior:
- enqueue deploy
- show immediate acknowledgment toast
- refresh deployment state and rely on shared active-jobs polling for live progress

### Global Progress Bar
No special-case GitOps bar is needed. Once deploy jobs are registered with the shared job manager, the existing diagnostics snapshot and `GlobalProgress` component should surface them automatically.

Minor UI adjustment may be needed so deploy jobs render with a sensible verb/tag, e.g. `Deploying Stack`.

## Error Handling
- Repeated clicks on the same deployment while active return the existing job
- Authorization remains identical to the current admin-only deploy path
- Failed compose apply stores a bounded output summary in deployment state and marks the job failed
- Unexpected worker error must still mark the deployment/job failed; no silent abandonment
- Restart reconciliation marks any in-flight deploy state as interrupted instead of leaving stale running status

## Testing
Backend:
- enqueue returns immediately with job id
- duplicate deploy requests return the active job instead of creating a second one
- successful worker execution updates job and deployment state correctly
- failed worker execution stores bounded error summary and does not update `last_deployed_at`
- restart reconciliation converts stale queued/running deploy states to failed/interrupted

Frontend:
- GitOps page renders queued/running/completed/failed deploy state from deployment fields
- active deploy jobs appear in global progress with the correct verb/target
- repeated clicks while active do not create a second pending state in the UI

## Scope Boundaries
Included:
- deploy job tracking
- global progress integration
- GitOps deployment row status
- restart-safe deploy state reconciliation

Excluded:
- streaming full compose logs to the browser
- granular per-service deployment progress
- changing sync-source auto-deploy semantics in this pass
