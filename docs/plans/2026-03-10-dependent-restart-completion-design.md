# Dependent Restart Completion Design

## Problem

Portainer-managed updates can finish the main redeploy and visibly restart dependent containers, but the HarborWatch update job can remain stuck in `restart_dependents` with the initial message `Discovering dependent containers`.

This is operationally wrong in two ways:

- the update should be considered complete once the main redeploy succeeds
- dependent restarts are explicitly best-effort and must never hold the job open indefinitely

## Design

Keep the existing best-effort restart behavior, but make the dependent-restart phase self-bounding and terminal:

- run dependent discovery and restarts under a dedicated internal timeout
- guarantee the phase emits a terminal `completed` step message on every path
- guarantee the outer update pipeline continues to `completed` after the bounded phase returns
- preserve warning detail for discovery failures, timeout, or partial restart failures

## Approach

Add a small internal helper layer in the update service:

- service-level timeout value for dependent restarts
- service-level hooks for dependent discovery and individual restarts to enable deterministic tests
- restart phase wrapper that converts hangs, timeouts, and partial failures into completion messages instead of a stuck job

This is intentionally not a policy change. Dependent restarts remain best-effort and do not fail the update.

## Testing

Add regression coverage for:

- a blocked dependent discovery path timing out and still allowing the update to complete
- a normal dependent-restart path still producing completion details

