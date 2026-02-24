# Migration Foundation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a centralized migration runner, initialize a baseline schema migration on startup, and expose the active schema version in diagnostics.

**Architecture:** Introduce `backend/internal/migrations` as a Go runner with embedded SQL migrations and a `schema_migrations` table. Keep existing store `Init()` schema creation in place for this milestone, and run the migration runner before store initialization to establish version tracking safely.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), embedded SQL files, existing `httpapi` diagnostics snapshot.

---

### Task 1: Add Migration Runner Package (TDD)

**Files:**
- Create: `backend/internal/migrations/migrations.go`
- Create: `backend/internal/migrations/runner.go`
- Create: `backend/internal/migrations/sql/0001_legacy_schema_baseline.sql`
- Create: `backend/internal/migrations/runner_test.go`

**Step 1: Write failing tests**

- Test first-run behavior:
  - `schema_migrations` table is created
  - baseline migration is recorded
  - `CurrentVersion()` returns expected version
- Test idempotency:
  - second `Run()` call applies nothing
  - no duplicate migration rows

**Step 2: Run tests to verify failure**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/migrations -v`

Expected: FAIL (package/files missing)

**Step 3: Implement minimal runner**

- Define migration list with embedded SQL
- Ensure table exists
- Apply ordered unapplied migrations
- Record version/name/applied timestamp
- Expose `CurrentVersion(ctx, db)`

**Step 4: Run tests to verify pass**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/migrations -v`

Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/migrations
git commit -m "feat: add schema migration runner foundation"
```

### Task 2: Integrate Migration Runner Into Startup (TDD)

**Files:**
- Modify: `backend/internal/httpapi/server.go`
- Test: `backend/internal/httpapi/server_test.go`

**Step 1: Write failing test**

- Add test coverage at a seam that verifies diagnostics snapshot includes schema version (after Task 3) and/or that migration version can be queried when DB is present.
- For startup integration, prefer a focused unit seam if possible rather than full `NewMuxWithSchedulerE()` integration.

**Step 2: Run targeted test to verify failure**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run 'TestDiagnosticsSnapshot.*SchemaVersion' -v`

Expected: FAIL (field not present yet)

**Step 3: Implement startup migration execution**

- In `NewMuxWithSchedulerE()`, run `migrations.Run(context.Background(), db)` after PRAGMAs and before store initialization
- Fail startup if migrations fail
- Optionally log applied migration count/version to diagnostics logger when available

**Step 4: Run relevant tests**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -v`

Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/httpapi/server.go backend/internal/httpapi/server_test.go
git commit -m "feat: run migrations during startup"
```

### Task 3: Add Schema Version To Diagnostics Snapshot (TDD)

**Files:**
- Modify: `backend/internal/httpapi/diagnostics.go`
- Modify: `backend/internal/httpapi/server.go`
- Modify: `backend/internal/httpapi/server_test.go`

**Step 1: Write failing tests**

- `collectDiagnosticsSnapshot()` returns `schemaVersion` when a DB is provided and migrations table exists
- endpoint smoke test confirms `schemaVersion` key appears in `/api/diagnostics/snapshot`

**Step 2: Run targeted tests and verify failure**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run 'TestDiagnosticsSnapshot.*' -v`

Expected: FAIL on missing field/assertions

**Step 3: Implement diagnostics schema version reporting**

- Add `db *sql.DB` to `diagnosticsDeps`
- Add `SchemaVersion` field to `diagnosticsSnapshot`
- Query `migrations.CurrentVersion()` during snapshot collection (non-fatal: append error in `Errors`)

**Step 4: Run tests**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/httpapi -run 'TestDiagnosticsSnapshot.*' -v`

Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/httpapi/diagnostics.go backend/internal/httpapi/server.go backend/internal/httpapi/server_test.go
git commit -m "feat: expose schema version in diagnostics"
```

### Task 4: Verification Pass

**Files:**
- Modify (if needed): `CHANGELOG.md`

**Step 1: Run package tests**

Run: `cd /config/workspace/HarborWatch/backend && go test ./internal/migrations ./internal/httpapi ./internal/jobs ./internal/scanning -v`

Expected: PASS

**Step 2: Run frontend build smoke (to ensure no accidental API type breakage in current UI)**

Run: `npm --prefix /config/workspace/HarborWatch/web run build`

Expected: PASS

**Step 3: Summarize outcomes and residual risks**

- Note that stores still own table creation/micro-migrations this milestone
- Document next milestone to move store schemas to migration files

**Step 4: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: note migration foundation milestone"
```

