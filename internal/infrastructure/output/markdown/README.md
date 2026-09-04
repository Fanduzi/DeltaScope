# Markdown Output Module

Default human-readable renderer for internal audit results.

## Files

| File | Responsibility |
|------|---------------|
| render.go | Formats audit results into deterministic Markdown, including the derived Action Summary and the aggregated rule skip reasons (`dialect_mismatch`, `fk_forbid`) |
| render_test.go | Verifies summary, statement findings, global finding rendering, the Action Summary section, and the rule summary aggregation |

## Exports

- `Render(result report.Result) ([]byte, error)`

## Action Summary

`Render` derives an **Action Summary** section (`## Action Summary`) from existing
findings plus the shipped rule catalog. It is purely additive to the human Markdown
output:

- Rendered only when the result has findings. Clean results omit the section entirely.
- The CLI may prepend one offline limitation line (`existence not checked (no database connection)`) when mode is offline. That line is adapter-owned and is not part of `markdown.Render`.
- Sits between the summary counts (`Statements / Blockers / Warnings / Notices`) and the
  existing `## Result Explanation`, before per-statement sections, global findings, and the
  rule summary.
- Each rule group shows the `[level]`, `` `rule_id` ``, a singular/plural finding count, an
  optional `Summary:`/`Suggestion:` (preferred from the rule catalog, falling back to
  finding text), an `Explain: deltascope rules explain <rule_id>` line, optional 1-based
  `Statements:` indexes, and an optional `Scope: global` marker.
- Displayed rule groups are capped at 10 via `report.BuildActionSummary(..., ActionSummaryOptions{Limit: 10})`;
  when truncated, a `Showing 10 of N rule groups.` line is appended.

The Action Summary is a human-report derivation. It does **not** change the machine JSON,
SARIF, GitHub Actions, or GitLab Code Quality outputs, it does **not** change audit
behavior or the finding JSON shape, and it never introduces a `severity` field. It carries
no raw SQL and no raw finding metadata.

### Rendered Example

A single `DELETE FROM users` blocker renders one rule group. The `Summary:`/`Suggestion:`
text is taken from the rule catalog when present, the `Statements:` index is 1-based, and no
raw SQL or finding metadata appears in the section:

```text
## Action Summary

- [blocker] `dml.where.require`: 1 finding
  Summary: Require DML where require
  Suggestion: Add the required clause, option, or object explicitly so the rule no longer has to infer intent.
  Explain: deltascope rules explain dml.where.require
  Statements: 1
```

When more than 10 rule groups exist, the section keeps the first 10 and appends a
`Showing 10 of N rule groups.` line.

## Rule Summary

`Render` keeps the `## Rule Summary` section bounded by the number of distinct
skip reasons instead of the rule catalog size:

- Shows `Loaded`, `Applicable`, and `Skipped with known reason` counts. The
  skipped count is the recorded slice size, not the arithmetic complement of
  `Applicable`.
- When the skipped list is non-empty, renders `### Skip Reasons` with one row
  per distinct `SkipReason`, ordered deterministically by the raw reason code.
- Known reasons use bounded human text (for example `Not applicable to current dialect`); an unknown future code is rendered verbatim.
- Emits no skipped rule ID and no `## Skipped Rules` section. The complete
  per-rule list remains available through explicit JSON output.
- When no skip reason is recorded, the zero count is shown and `### Skip Reasons`
  is omitted.

The aggregation is presentation-only: domain types and the JSON
`rule_summary.skipped` array stay unchanged.

## Dependencies
- Upstream: CLI and future API adapters that need human-readable output
- Downstream: `internal/domain/report`, `internal/domain/rule`, `internal/domain/rule/catalog`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
