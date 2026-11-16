# AGENTS.md — internal/agent

Rules for the run lifecycle and orchestration layer.

## Run lifecycle

- A run is created only through `Manager.TryStart`, lives as `running`, and finishes
  through exactly one terminal path: completed, failed, cancelled, timed out, or
  stopped. `finish()` is the single place that releases per-run state such as the
  WPS selection lock; keep every exit path going through it.
- One running run per office port. The manager rejects a second start on a busy port
  with a `busy:` error. Do not add a side channel that starts work outside the
  manager.
- `CancelRunning` is called on Core shutdown and must stay idempotent (second call
  returns zero).
- Results are retained for `RetentionMinutes` (30) so clients can reconnect to SSE
  and read the final result.

## Safety path

- Policy checks, user confirmation, and audit logging stay in the execution path.
  Never call a mutating bridge command directly from orchestration code and skip
  them.
- `Interactive=false` (for example `ai.ask`) means no human can confirm: commands
  that require confirmation must be refused, not auto-approved.

## Context and summarization

- The context assembler estimates tokens conservatively and keeps the newest turns
  within budget; the summary represents everything cut off. Older turns must not
  silently reappear.
- The first turn sent to Anthropic must be a `user` turn; drop a leading assistant
  turn when truncation leaves one.
- Summarize only when the conversation exceeds its threshold; the summary is stored
  on the conversation and reused. Do not recompute it on every round.

## Add-in boundary

- The add-in never calls an LLM directly. All model calls go through Core so prompts,
  policy, audit, and token accounting stay in one place.
