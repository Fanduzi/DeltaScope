# DeltaScope v0.300.0 Release Notes

## Summary — Audit Action Summary

v0.300.0 adds an **Action Summary** to the default markdown audit report. When an audit produces findings, the report now opens with a `## Action Summary` section that groups findings by rule, orders them by remediation priority, and gives a direct `deltascope rules explain <rule_id>` next step for each group. This release does **not** add new audit rules, change rule evaluation behavior, change audit behavior, change finding JSON shape, add parser support, or change SDK/HTTP/MCP/SARIF/GitHub Actions/GitLab Code Quality outputs. There is no `severity` field; DeltaScope uses `level`.

## Action Summary in Markdown Audit Output

The default `deltascope audit` path and `--format markdown` now render an `## Action Summary` section near the top of the report whenever findings exist:

```bash
deltascope audit --file ./migration.sql
deltascope audit --format markdown --file ./migration.sql
```

The section is derived from existing audit findings and rule catalog metadata. It does **not** re-run the audit, parse SQL, evaluate rules, inspect raw SQL, or add raw SQL beyond what the existing per-statement sections already show. The summary references findings by 1-based statement index instead of SQL snippets.

### Grouping and Ordering

Each action item groups findings by `rule_id`. For each group the summary shows:

- `[level] \`rule_id\`: N finding(s)`, where `level` is the highest-priority level observed for that rule (`blocker`, `warning`, or `notice`).
- Catalog-backed `Summary:` and `Suggestion:` text, falling back to the existing finding message/suggestion when a rule is not in the shipped catalog.
- `Explain: deltascope rules explain <rule_id>` as the next step.
- Deduplicated 1-based `Statements:` indexes (omitted for global-only findings), plus an optional `Scope: global` marker.

Groups are sorted deterministically by `level` priority (`blocker` → `warning` → `notice`), then finding count descending, then `rule_id` ascending.

### Truncation and Clean Audits

- Markdown output renders at most 10 rule groups. When more groups exist, a final `Showing 10 of N rule groups.` line is emitted.
- Clean audits (no findings) omit the `## Action Summary` section entirely.

### Privacy

The Action Summary carries no raw SQL, no normalized SQL, no parser `near ...` text, no object names derived only from user SQL, no metadata values from a live database, and no raw finding metadata maps. The fallback text for rules outside the catalog is limited to the existing finding message and suggestion already produced by the audit result.

## Unchanged Outputs (Machine Contracts)

- Audit JSON output is unchanged. There is **no** `action_summary` field in audit JSON.
- Finding JSON shape is unchanged. No finding fields are added or renamed.
- `level` remains the public priority field (`blocker`, `warning`, `notice`). There is no `severity` field anywhere in the public output; DeltaScope continues to use `level`, not `severity`.
- SDK, HTTP, MCP, SARIF, GitHub Actions, and GitLab Code Quality outputs are unchanged.

## Rule Catalog Facts

The rule catalog is unchanged from v0.290.0. The Action Summary is a rendering change, not a rule change.

| Metric | Count |
|--------|------:|
| Total rules | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 361 |
| dml | 10 |

## Non-Goals

- Not new audit rules.
- Not rule evaluation behavior changes.
- Not audit behavior changes.
- Not finding JSON shape changes.
- No `severity` field is introduced.
- Not parser support changes.
- Not SDK/HTTP/MCP/SARIF/GitHub Actions/GitLab Code Quality output changes.
- Not a new `report` subcommand.
- Not complete remediation of all migration risks; the summary points at the next step, it does not auto-fix anything.

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE config entries: **53** (unchanged).
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).
- DDL coverage catalog: **400** entries (61 MySQL / 54 TiDB / 285 PostgreSQL / 18 parser_upgrade_candidate) (unchanged).
- Parser-error total: **29** cases across all dialects (unchanged).

## Decision Record

`docs/decisions/2026-06-13-v0.300.0-audit-action-summary.md`
