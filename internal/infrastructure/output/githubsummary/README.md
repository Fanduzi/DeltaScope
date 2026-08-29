# GitHub Summary Output Module

Renders concise GitHub-flavored Markdown for GitHub Actions job summaries (`$GITHUB_STEP_SUMMARY`).

## Files

| File | Responsibility |
|------|----------------|
| render.go | Formats `report.Result` into a short SQL review summary with verdict/counts, Action Summary, unsupported count, and parser-failed statement count |
| render_test.go | Verifies clean/finding summaries, no-leak behavior, unsupported counts, and parser diagnostics are counted without miscounting aggregate unsupported diagnostics |

## Exports

- `Render(result report.Result) ([]byte, error)`

## Output Contract

`Render` produces a human-readable summary intended for `$GITHUB_STEP_SUMMARY`. It is **not** a
stable machine-readable schema; automation should use `--format json`, `--format sarif`, or
`--format gitlab-codequality` instead.

The output always includes:

- A fixed title: `## DeltaScope SQL Review`.
- The verdict, rendered as the **uppercased canonical** `report.Verdict` value
  (`PASS`, `REVIEW`, or `REJECT`). The job summary does not invent a binary `PASS`/`FAIL`
  term; it stays consistent with DeltaScope's three-valued verdict model and the markdown
  renderer's use of the canonical verdict.
- A right-aligned counts table (`Statements`, `Blockers`, `Warnings`, `Notices`).

When the result has findings, the output additionally includes a `## Action Summary` section
derived via `report.BuildActionSummary(result, catalog.All(), report.ActionSummaryOptions{Limit: 10})`,
identical in grouping/ordering/cap to the markdown Action Summary:

- `[level]` + `` `rule_id` `` + singular/plural finding count.
- Optional catalog-backed `Summary:`/`Suggestion:` (falling back to finding text).
- `Explain: deltascope rules explain <rule_id>`.
- Optional 1-based `Statements:` indexes (omitted for global-only findings).
- Optional `Scope: global` marker.
- A `Showing 10 of N rule groups.` line when more than 10 rule groups exist.

When the result has no findings, the output emits `No findings.` and omits the Action Summary.
When the result carries unsupported statements, a final `Unsupported statements: N` line is
appended so unsupported diagnostics are not silently hidden; only the count is shown.
When diagnostics mark statements `audited=false`, a final `Unaudited statements: N` line is also appended without diagnostic or SQL payloads.

### No-Leak Boundary

The summary carries no raw SQL, normalized SQL, parser `near ...` fragments, database secrets,
connection strings, live metadata payloads, unsupported feature labels, or finding metadata.
It only surfaces rule IDs, `level`, counts, catalog summary/suggestion, the `rules explain`
command, 1-based statement indexes, the global scope marker, and the unsupported count. It
introduces no `severity` field; public priority remains `level`.

### Rendered Example

A single blocker finding renders:

```text
## DeltaScope SQL Review

Verdict: REJECT

| Metric | Count |
| --- | ---: |
| Statements | 1 |
| Blockers | 1 |
| Warnings | 0 |
| Notices | 0 |

## Action Summary

- [blocker] `dml.where.require`: 1 finding
  Summary: ...
  Suggestion: ...
  Explain: deltascope rules explain dml.where.require
  Statements: 1
```

A clean result renders:

```text
## DeltaScope SQL Review

Verdict: PASS

| Metric | Count |
| --- | ---: |
| Statements | 1 |
| Blockers | 0 |
| Warnings | 0 |
| Notices | 0 |

No findings.
```

`Render` does not write to `$GITHUB_STEP_SUMMARY`; callers redirect or append the bytes.

## Dependencies
- Upstream: CLI audit adapter (formats dispatch)
- Downstream: `internal/domain/report`, `internal/domain/rule/catalog`, `internal/infrastructure/output`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
