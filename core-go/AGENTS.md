# AGENTS.md — core-go

Rules for the Go Agent Core. See `README.md` for architecture and the package map.

## Toolchain

- Go 1.26 (`go.mod` declares `go 1.26.0`).
- No cgo: build with `CGO_ENABLED=0`. SQLite is `modernc.org/sqlite` (pure Go) so
  cross-builds from one machine stay possible.
- Run `go test ./...` in this directory for the full package suite; use
  `go test -race ./...` when touching concurrent run state.

## Compatibility contract

- `core.db` must stay schema-compatible with the retired .NET core. Both read the
  same user database, so schema changes are forward-only and tracked with
  `PRAGMA user_version` (currently `SchemaVersion = 4`).
- Core API v1 is append-only: add fields/endpoints, never change or remove existing
  behavior, field names, or response shapes. The add-in, LibreOffice extension, and
  MCP bridge depend on the old contract.
- Config sources keep their precedence (`AXIOM_*` → HKCU or
  `~/.config/axiom-office/config.json` → defaults).

## Concurrency

- One running run per office port. `agent.Manager.TryStart` rejects a second run on
  a busy port; do not bypass this with a parallel path or goroutine.
- Cancelling on Core shutdown is part of the run lifecycle; keep it idempotent.

## Generated files

- Generated Go files (for example the setup catalog) are not hand-edited. Change the
  generator input/script and rerun it; `make check-gen` must pass.
- `prompts/extract.txt` is the single embedded source for memory extraction.

## WPS and bridge

- The WPS channel (`internal/wpsbridge`, `/wps/*`) is part of API v1 and must keep
  working for Linux WPS without a Node runtime.
- Never move policy, confirmation, or audit outside the command path.
