# Compose Execution Context Design

## Problem

HarborWatch currently assembles Docker Compose commands in multiple places. GitOps deploys and compose-managed updates each build their own argument lists. That duplication already caused one production bug: GitOps `pullOnDeploy` omitted the deployment env-file arguments that the later `up` command used, so compose interpolation failed during `pull`.

The update path has a related architectural gap. It can execute compose `pull` and `up`, but it does not model compose env authority centrally, so support for project `.env` files and HarborWatch-managed overrides is incomplete and vulnerable to drift.

## Design

Introduce a shared compose execution context helper in the backend that resolves:

- compose config files
- compose working directory
- ordered env files
- the common compose command prefix

The helper must be usable by both GitOps deploy execution and the compose-managed update executor.

## Env authority

Compose env resolution should support both pathways:

1. Project `.env`
2. HarborWatch-managed override env file

Ordering matters. HarborWatch should pass env files in stable order:

1. project `.env`
2. HarborWatch-managed override env file(s)

That preserves the conventional local project baseline while still allowing HarborWatch-managed values to override when needed.

## Scope

In scope:

- shared compose execution context helper
- GitOps migration to the helper
- update executor/request migration to the helper
- targeted regression tests for shared env-file propagation

Out of scope:

- broader Portainer execution changes
- redesigning compose project discovery
- changing the semantics of existing stored deployment env data

## Implementation notes

- Keep the helper small and explicit. It should build arguments, not hide command execution.
- GitOps can still write managed inline env files where it already does today, but command assembly should move to the shared helper.
- The update path needs request metadata for compose env files so discovered `.env` and HarborWatch-managed overrides can flow into the executor.
- Tests must verify both `pull` and `up` use the identical compose prefix.

## Verification

- unit tests for helper arg ordering
- GitOps regression test for env-file propagation to `pull`
- update executor tests for env-file propagation to both `pull` and `up`
