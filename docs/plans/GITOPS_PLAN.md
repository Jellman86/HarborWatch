# HarborWatch GitOps Implementation Plan

## Overview
A native GitOps engine for standalone Docker hosts, replacing the need for external tools like Portainer for Git-based deployments. HarborWatch will synchronize remote Git repositories to a master directory on the host and automatically deploy selected compose files.

## Design Decisions
1.  **Deployment Scope:** Explicit selection. The user defines *exactly* which `docker-compose.yml` file(s) within the repository should be deployed, rather than auto-discovering and deploying everything.
2.  **Environment Variables (.env):** HarborWatch will support three modes of secret management:
    *   *Committed:* `.env` files are tracked in the Git repository (not recommended for secrets, but supported).
    *   *Docker Secrets:* Relying natively on Docker Swarm/Compose secret files.
    *   *Managed UI:* HarborWatch provides a UI to inject and manage an overriding `.env` file that lives alongside the synced compose file on the host.
3.  **Target Environment:** Exclusively for standalone Docker environments (bypassing Portainer).

## Implementation Phases

### Phase 1: Data Model & Infrastructure (Backend)
1.  **Global Settings:** Add `GitOpsMasterDirectory` (e.g., `/data/gitops`) to global settings.
2.  **SQLite Schema:** Create `git_sources` and `git_deployments` tables.
    *   `git_sources`: Tracks repos, auth (SSH/HTTP), polling interval.
    *   `git_deployments`: Maps a `git_source_id` + a relative file path (e.g., `media/docker-compose.yml`) + managed `.env` overrides.
3.  **go-git Integration:** Create `internal/gitops` package. Implement functions for `Clone`, `Fetch`, and `Pull` using `github.com/go-git/go-git/v5`.
4.  **Security/Auth:** Implement robust secret storage for SSH keys and HTTP tokens within the SQLite database.

### Phase 2: Engine & Scheduler (Backend)
1.  **File System Sync:** Ensure pulled files are written to the correct sub-directory inside the `GitOpsMasterDirectory`.
2.  **Deployment Execution:** Use `os/exec` to execute `docker compose -f <path> up -d --remove-orphans` against the local Docker socket.
3.  **Environment Injection:** Logic to write the UI-managed `.env` overrides to disk immediately before executing `docker compose up`.
4.  **Scheduler Task:** Create `gitops_sync_task` to poll repositories on their configured intervals.
5.  **API Endpoints:** Build the REST API for creating sources, listing deployments, updating env vars, and forcing a sync.

### Phase 3: User Interface (Frontend Svelte)
1.  **GitOps Page:** A new top-level page or nested tab inside "Stacks".
2.  **Connection Wizard:**
    *   Add Repository (URL, Branch, Auth).
    *   Test Connection button (validates credentials via `go-git` without pulling).
3.  **Deployment Builder:**
    *   Browse the repository tree (via API) to select a specific `docker-compose.yml` file.
    *   Configure UI-managed `.env` variables.
4.  **Status Dashboard:** Display active Git deployments, current git SHA, deployment status, and "Sync Now" controls.

### Phase 4: AI & Edge Cases
1.  **AI Compose Doctor Integration:** Allow the AI to read the synced compose files and propose modifications (which would then be committed and pushed back to the Git remote).
2.  **Failure Rollback:** If a pull + up results in a crashed container, automatically revert the `go-git` tree to the previous SHA and redeploy.