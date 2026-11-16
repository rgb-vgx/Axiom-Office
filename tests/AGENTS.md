# AGENTS.md — tests

Rules for the test suites in this repository.

## Isolation

- Prefer a fake bridge and fake LLM for automated tests. The default core e2e suite
  already does this; keep it that way so tests run without Office.
- Use a temporary HOME and temporary working directory. Never read or write the
  real `HKCU` configuration, and never delete or migrate the user's real `core.db`.
- Each test creates its own fixtures/ports and cleans up only what it created.

## Live tests

- Live Office and live WPS tests are manual. They are intentionally not in `make
  test`/CI and must not attach to the user's already-open applications.
- A live test starts its own app instance in a separate profile (and separate Xvfb
  for WPS) and closes only that instance.
- Do not weaken or skip a live test to make an automated target pass.

## Commands

- `make test-core` — build Core and run the core e2e suite (fake bridge/LLM).
- `make test-wps` — WPS add-in Python and Node tests.
- `make test-catalog` — catalog format and C# consistency checks.
- `make test-go` — Go unit tests, including `internal/tools/needs_test.go`.
- Run the targeted make target for the area you changed; full `make test` is broader
  but still not a substitute for manual live testing.
