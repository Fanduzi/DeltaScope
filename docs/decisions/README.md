# Decision Records

This directory holds committed, durable decision records for DeltaScope.

## Purpose

Decision records capture the **why** behind non-obvious choices — architectural boundaries, feature deferrals, public-contract changes, privacy guarantees, and cross-surface consistency requirements. They exist so that future contributors (human or agent) can understand the rationale without reconstructing it from chat logs, local plans, or commit messages alone.

## When to Create One

Create a decision record when:

- A feature is intentionally deferred
- An unsupported boundary is promoted to supported or finding-covered behavior
- Public output shape changes
- Privacy/no-leak behavior is part of the design
- Cross-surface behavior must stay consistent across SDK, CLI, HTTP, or MCP
- Release docs mention a nuanced non-goal, boundary, or compatibility promise
- A milestone makes multiple related commits whose rationale would otherwise live only in chat or local plans

## What Decision Records Are Not

- **Not full task reports.** Do not paste execution logs, step-by-step task narratives, or agent transcripts.
- **Not transcripts.** Conversational context belongs in `docs/plans/` (local/ignored) or chat history.
- **Not release notes.** User-facing change descriptions belong in `docs/releases/`.
- **Not user guides.** Decision records are written for contributors and architects working on
  DeltaScope itself, not for people using the CLI, server, MCP, or library.

## Audience: Maintainers, Not End Users

Decision records explain internal rationale for future contributors (human or agent). They are not
end-user documentation, and user-facing docs should not link to them as a primary reference.

End-user documentation lives in:

- Top-level `README.md` and `README_ZH.md` — first contact and quick start
- `docs/reference/` — reference (CLI, config, capabilities, HTTP/MCP contracts)
- `docs/recipe/` — end-to-end recipes for common tasks
- `docs/releases/` — release notes

If a user-facing doc needs to point at the engineering background, link the decision record under a
low-prominence "Maintainer notes" or "Engineering notes" line rather than inline in the main flow.

`docs/decisions/` stays committed and durable. Do not add it to `.gitignore`, and do not delete
records — supersede a decision with a new record that links the one it replaces.

## Required Fields

Every decision record must include:

- **Date** and **Status** (Proposed, Accepted, Superseded)
- **Context** — what situation prompted the decision
- **Decision** — what was chosen
- **Rationale** — why this choice over alternatives
- **Public Contract** — what external consumers can rely on
- **Deferred / Out Of Scope** — what was explicitly not done and why
- **Verification Evidence** — test results, corpus coverage, surface tests
- **Consequences** — what future work must account for
- **Links** — commits, tests, docs

## Naming Convention

```
YYYY-MM-DD-<version-or-area>-<short-topic>.md
```

Examples:

- `2026-05-17-v0.100.0-postgresql-ddl-long-tail-boundary.md`
- `2026-06-01-serial-heuristic-removal.md`

## Relationship to Other Artifacts

| Artifact | Purpose | Committed |
|----------|---------|-----------|
| `README.md`, `README_ZH.md` | First contact and quick start | Yes |
| `docs/reference/` | Reference documentation (CLI, config, capabilities, HTTP/MCP) | Yes |
| `docs/recipe/` | End-to-end recipes for common tasks | Yes |
| `docs/releases/` | Public user-facing release notes | Yes |
| `docs/decisions/` | Maintainer-facing durable rationale for non-obvious choices | Yes |
| `docs/plans/`, `docs/plans_zh/` | Local planning and task execution notes | No (gitignored) |
| `testdata/sql-corpus/` | Machine-verifiable SQL behavior contracts | Yes |
| Tests (`*_test.go`) | Machine-verifiable code contracts | Yes |
