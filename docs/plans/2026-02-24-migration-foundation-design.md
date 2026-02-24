# Migration Foundation Design

**Scope:** Milestone 1 of HarborWatch hardening work (database migration foundation + schema version visibility)

**Goal**

Introduce a centralized migration runner and schema version tracking without breaking existing startup behavior. This milestone establishes the foundation for future schema changes while preserving the current per-store `Init()` migration logic during the transition period.

**Constraints**

- Compatibility target: current release schemas + forward only
- Minimize risk to existing installs
- Do not rewrite all store initialization code in the same milestone
- Keep startup failure modes explicit and diagnosable

**Design Summary**

HarborWatch will gain a new `backend/internal/migrations` package responsible for:

- creating and managing a `schema_migrations` table
- applying ordered versioned migrations (SQL-first, via embedded files)
- reporting current schema version

For Milestone 1, the migration set will contain a single baseline migration that marks migration-system initialization (`legacy schema baseline`). Existing store `Init()` methods remain the authoritative schema creators for application tables. This lets us adopt the migration framework safely before moving table DDL into formal migrations in later milestones.

**Architecture**

- `migrations.Run(ctx, db)` executes:
  - ensure `schema_migrations` table exists
  - validate migration list ordering/uniqueness
  - apply unapplied migrations in order inside transactions (when possible)
  - record applied version + name + timestamp
- `migrations.CurrentVersion(ctx, db)` returns the latest applied version (0 if not initialized)
- `httpapi.NewMuxWithSchedulerE()` runs migrations immediately after opening SQLite and setting pragmas, before any store/service `Init()`
- Diagnostics snapshot includes `schemaVersion` for operator visibility

**Why Hybrid (SQL + Go runner)**

- SQL migration files provide a clear, auditable schema history
- Go runner integrates naturally with startup and diagnostics
- Future data backfills can use Go hooks without replacing the migration system

**Risk Management**

- Baseline migration is intentionally minimal and non-destructive
- Existing store `Init()` calls remain unchanged, so startup compatibility is preserved
- Add tests for idempotent migration runs and diagnostics reporting

**Milestone 1 Non-Goals**

- Moving store schemas out of `Store.Init()`
- Full legacy DB compatibility fixture matrix
- Down-migrations
- UI display of schema version (backend diagnostics JSON only in this milestone)

**Follow-on Milestones**

1. Move store schemas incrementally into versioned SQL migrations (one subsystem at a time)
2. Add migration fixture tests for release-to-release upgrades
3. Expose schema version/status in the UI diagnostics screen

