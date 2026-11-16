# AGENTS.md — internal/tools

Rules for tools, the office action allowlist, and bridge command declarations.

## Command allowlist

- `office_action` accepts only the actions in the session bridge catalog
  (`catalog/live-commands.json`, sent by the bridge). Do not hard-code a wider
  allowlist or let the model name an action outside it.
- A tool that calls bridge commands directly (not through the model's
  `office_action`) must implement `Commands()` in `needs.go` and declare every
  command as `Required` or `Optional`.
  - `Required`: the tool cannot work without it; Core hides the tool when the
    backend catalog lacks it.
  - `Optional`: only used on a branch; the tool stays available and that branch
    reports a clear error.
- Register the tool and its source files in `toolSources` in `needs_test.go`.
  `needs_test.go` must pass: it cross-checks source literals against declarations
  and the canonical catalog. Do not suppress it or weaken the regex to avoid fixing
  a declaration.

## Flags

- `Mutating`, `Verifies`, `Final`, and `Abort` change how a tool is scheduled,
  verified, and terminated. Set them deliberately; a missing or wrong flag can skip
  read-back verification or prevent the run from stopping.

## Guards

- Never bypass the policy, overwrite, sort, or undo guards. If a command is refused,
  surface the refusal to the model/user; do not reimplement it in a tool with fewer
  checks.
- Limits and truncation must keep their documented behavior; a tool cannot opt out
  of call limits by calling the bridge elsewhere.
