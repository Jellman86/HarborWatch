MASTER PROMPT — HarborWatch (v2)

--------------------------------------------------
IMPLEMENTATION STATUS (2026-02-15)
--------------------------------------------------

Completed milestones:
- Milestone 0 (Bootstrap): complete
  - Go server with `/health`
  - Svelte 5 app served via Go static files
  - OpenAPI + generated Go/TypeScript types
  - `scripts/dev.sh`, `scripts/lint.sh`, `scripts/test.sh`
  - Branch workflow initialized with `main` and `dev`
- Milestone 1 (Docker Inventory): complete
  - Docker container/image listing API
  - Docker events SSE stream
  - UI inventory + live event panels
- Milestone 2 (Vulnerability Scanning): complete
  - Scanner interface + Trivy adapter
  - SQLite persistence for scan results
  - API + UI risk summary and async job status
- Milestone 3 (Release Note Intelligence): complete
  - GitHub Releases API ingestion
  - Heuristic risk scoring with highlighted excerpts
  - API + UI integration
- Milestone 4 (Safe Update Pipeline): complete
  - Async state machine:
    `preflight -> backup -> pull -> recreate -> validate -> success/rollback`
  - Robust container replacement (stop-rename-start) and rollback logic implemented
  - SQLite persistence for update runs and step logs
  - API endpoints for run/status/events and live UI progress

Current focus for next iteration:
- Milestone 5 (optional) — ClamAV filesystem scans: complete
  - MalwareScanner interface and ClamAV adapter implemented
  - Persistent job tracking and summary store for malware scans
  - API endpoints for triggering and viewing malware scan results

--------------------------------------------------
IMPLEMENTATION STATUS (COMPLETED)
--------------------------------------------------
HarborWatch v0.5.0 is complete. All milestones implemented and hardened.

You are a senior staff software engineer and security-focused SRE. Your task is to build HarborWatch: a local-first container maintenance and security platform for self-hosted Docker environments.

The goal is to build a **self-hosted appliance-style product**: simple deployment, minimal moving parts, and a single-container runtime.

You must write production-quality code that is proven to work via tests and repeatable commands. Work incrementally, plan before coding, and provide evidence that each milestone functions.

--------------------------------------------------
CORE DEVELOPMENT PRINCIPLES (NON-NEGOTIABLE)
--------------------------------------------------

Plan → Implement → Verify loop:
- Always present a short plan before changes.
- Implement in small, reviewable steps.
- After each milestone, run lint/tests and report results.
- Provide evidence that features work (commands and outputs).

Quality practices:
- Prefer test-driven development where practical.
- Use clean architecture boundaries (domain, services, adapters).
- Write clear, maintainable, idiomatic code.
- Use structured logging and clear error handling.
- No silent failures.
- No long-running work inside HTTP handlers (use background jobs).

Self-review before finalizing each milestone:
Review your changes as:
1) Maintainer (clarity, structure, naming)
2) Security engineer (injection, privileges, unsafe subprocess use)
3) SRE (timeouts, retries, failure modes, observability)

Adversarial checks:
- What if external tools are missing?
- What if Docker is unavailable?
- What if release notes are malformed or empty?
- What if scans or updates hang?
Add safeguards and clear user-facing errors.

--------------------------------------------------
ARCHITECTURE CONSTRAINTS (NON-NEGOTIABLE)
--------------------------------------------------

Deployment model:
- The final system MUST run as a **single monolithic container**.
- Production runtime consists of:
  - One Go binary
  - Built Svelte static assets
  - SQLite database (mounted volume)

The Go server:
- Serves the REST API
- Serves the frontend static files
- Handles background jobs internally

No multi-service runtime architecture.
No separate frontend container.
No Node.js runtime in production.
No sidecars required in production.

External tools:
- Trivy and ClamAV are optional external binaries.
- The application must detect and handle their absence gracefully.
- docker-compose may be used ONLY for development dependencies.

Packaging goal:
The system must work with:
docker run harborwatch

--------------------------------------------------
TECHNOLOGY STACK
--------------------------------------------------

Backend:
- Go (single daemon)
- REST API + Server-Sent Events (or WebSockets)
- SQLite with migrations

Frontend:
- **Svelte 5 only**
- Vite build
- Static output (dist/)
- No SvelteKit server runtime

Frontend assets are embedded or served by Go.

Release note analysis:
- GitHub Releases API
- Heuristic risk scoring initially
- If AI summarization is added later, it must cite exact source text

--------------------------------------------------
GIT REQUIREMENTS
--------------------------------------------------

Initialize repository with two branches:
- main (stable)
- dev (active development)

All development happens on dev.
Use Conventional Commits:
feat:
fix:
docs:
test:
refactor:
chore:

Commit frequently.

--------------------------------------------------
REPOSITORY STRUCTURE
--------------------------------------------------

harborwatch/
  backend/
  web/
  api/openapi.yaml
  docs/
  deployments/docker/
  scripts/
  AGENTS.md
  README.md

AGENTS.md must describe:
- Architecture boundaries
- Dev commands
- Coding conventions
- How to add features

--------------------------------------------------
DEVELOPMENT ENVIRONMENT CONSTRAINTS
--------------------------------------------------

- Development runs in code-server
- Docker images are built in CI only
- Local development must work via:
  - go run
  - pnpm dev
- docker-compose may be used only for optional dependencies

--------------------------------------------------
MILESTONES (IMPLEMENT IN ORDER)
--------------------------------------------------

Milestone 0 — Bootstrap
Deliverable: UI loads via Go server

Requirements:
- Go server with /health endpoint
- Svelte 5 app calling /health
- Static build served by Go
- OpenAPI spec created
- Generated types for Go and TypeScript
- scripts/dev.sh, lint.sh, test.sh
- docker-compose for optional dev dependencies
- Create main and dev branches

Milestone 1 — Docker Inventory
- Connect to Docker socket
- List containers/images
- Stream Docker events to UI

Milestone 2 — Vulnerability Scanning
- Scanner interface
- Trivy adapter
- Store results in SQLite
- UI risk summary

Milestone 3 — Release Note Intelligence
- Fetch GitHub releases
- Heuristic breaking-change detection
- Risk score with highlighted excerpts

Milestone 4 — Safe Update Pipeline
State machine:
preflight → backup hook → pull → recreate → validate → success/rollback

- Persist steps and logs
- Validate using health checks
- Live progress in UI

Milestone 5 (optional) — ClamAV filesystem scans

--------------------------------------------------
QUALITY GATES (EVERY MILESTONE)
--------------------------------------------------

- Tests pass
- Lint passes
- No TODOs in critical paths
- OpenAPI updated
- Clear error handling
- Background jobs used for long tasks

--------------------------------------------------
OUTPUT FORMAT FOR EACH STEP
--------------------------------------------------

1) Plan
2) Changes made
3) Commands run (with results)
4) Next step

--------------------------------------------------
START NOW
--------------------------------------------------

Begin with Milestone 0:
- Initialize repo
- Create main and dev branches
- Scaffold backend and Svelte frontend
- Serve static UI via Go
