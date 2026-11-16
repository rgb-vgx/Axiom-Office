# AGENTS.md — internal/store

Rules for `core.db`, migrations, and run/conversation persistence.

## Schema

- Migrations are forward-only. Add a new `schemaVN` block and raise `SchemaVersion`;
  never edit an already-released migration or renumber versions.
- Version is tracked with `PRAGMA user_version`. A database at or above the current
  version is left untouched.
- Keep the schema byte-for-byte compatible with the retired .NET core: both sides
  read the same user `core.db`, so table and column names, indexes, FTS tables, and
  triggers must match what .NET expects.

## SQLite setup

- Open with WAL, foreign keys on, a 5-second busy timeout, and
  `_txlock=immediate`.
- All write transactions go through `DB.BeginWrite` (BEGIN IMMEDIATE plus bounded
  backoff on `SQLITE_BUSY`). Do not use a plain deferred transaction for writes.
- If migration fails, report storage as unavailable; do not continue with a partial
  schema.

## Tests and local data

- Tests must create their own temporary database and clean up only that path.
- Never delete, truncate, or migrate the user's real `core.db`. Do not use a fixed
  path that could point at a real profile.
- Prefer in-process temp DBs over a shared fixture; concurrency tests may open the
  same temp file deliberately.
