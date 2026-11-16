# AGENTS.md

Local invariants for this repository. See `README.md` for product and setup docs;
this file only covers rules agents and contributors must not break.

## Build and test

- `make check-gen` — verify generated files match their sources (run before every handoff).
- `make test-go` — Go unit and race tests (`cd core-go && go test -race ./...`).
- `make test-core` — build the Go Agent Core and run the core e2e suite.
- `make test-wps` — WPS add-in Python and Node checks.
- `make test-catalog` — bridge catalog format and C# consistency checks.
- `make test` runs the full non-interactive suite; it is not a substitute for the
  targeted targets when changing one area.

## Git

- Never `git add -A` or `git add .`; stage explicit paths only.
- Never commit while on `main`; create a branch first.
- Commit messages are Vietnamese with a `feat:`, `fix:`, `docs:`, `refactor:`, or
  `test:` prefix.
- Do not push unless the user explicitly asks.

## Generated files

- Files with generated regions (catalog, WPS command CATALOG, MCP command tables,
  ribbon menus, setup catalog) are not hand-edited. Change the generator input or
  script and rerun the generator; `make check-gen` must stay green.

## Platform limits

- The C# add-in and host target .NET Framework 4.8 and cannot be built on Linux.
  Use `scripts/build.ps1` on Windows.
- The WPS add-in runs in CEF Chrome 87: ES5 only.
- Live Office/WPS tests are manual. They must never attach to, modify, or close the
  user's already-open applications or documents; use a separate temporary profile.

## Verification

- Never weaken a guard or test to make a check pass.
- Keep changes scoped to the task; do not refactor unrelated code.
