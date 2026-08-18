# Decision: Aggregate Markdown Rule Skip Reasons

- Date: 2026-08-17
- Status: Accepted
- Issue: [#17](https://github.com/Fanduzi/DeltaScope/issues/17)
- Spec: `docs/plans/2026-08-17-markdown-rule-summary-aggregation-spec.md`
- Design: `docs/plans/2026-08-17-markdown-rule-summary-aggregation-design.md`
- Implementation: `docs/plans/2026-08-17-markdown-rule-summary-aggregation-implementation.md`

## Context

Default Markdown currently expands every reliably inferred skipped rule. As the
catalog grew, a small MySQL audit began printing more than 190 PostgreSQL rule
IDs with the same dialect-mismatch reason. The list hides the verdict and
findings that the default format exists to communicate.

The skipped-rule identities remain useful structured evidence. JSON already
owns that complete representation, while Markdown serves direct human and
agent reading. The two formats do not need the same presentation density.

## Proposed Decision

Keep loaded, applicable, and skipped-with-known-reason counts in
`## Rule Summary`. Replace the per-rule `## Skipped Rules` section with a
`### Skip Reasons` aggregate ordered by the underlying reason code. Render
known reasons as bounded human text and preserve an unknown future code
verbatim. Omit the subsection when no skipped reason exists.

Keep the complete `rule_summary.skipped` array in JSON. Do not change quiet or
CI-native formats, rule evaluation, skip inference, or domain types. Do not add
`--verbose` or another compatibility mechanism; exact per-rule evidence remains
available through explicit JSON output.

## Rationale

Markdown output size should scale with distinct explanations, not catalog size.
Reason aggregation preserves the operational signal that a high skipped count
may indicate a wrong dialect without flooding the primary result with repeated
IDs.

Keeping aggregation inside the Markdown adapter preserves the domain's complete
facts and avoids a JSON migration. Reusing JSON for details is smaller and more
durable than adding a second Markdown mode.

## Contract

- Markdown shows `Skipped with known reason`, not an implied arithmetic
  partition of all loaded rules.
- Markdown emits no skipped rule ID and no `## Skipped Rules` section.
- Reason aggregates are deterministic and unknown reason codes remain visible.
- JSON continues to emit every skipped `rule_id` and `reason`.
- Quiet and all other formats remain unchanged.
- No rule evaluation, CLI flag, domain model, or release behavior changes.

## Consequences

Positive:

- Default output stays bounded as the rule catalog grows.
- People and agents see the verdict, findings, and skip explanation without a
  catalog dump.
- Machine consumers retain complete structured evidence.

Costs:

- Consumers scraping per-rule IDs from Markdown must switch to JSON.
- Markdown output snapshots must adopt the new heading and count label.

## Alternatives Rejected

- Add `--verbose`: rejected because JSON already carries the complete list and
  the flag would widen the public CLI contract.
- Show a capped sample: rejected because cap and sample ordering become new
  policy while repeated IDs remain.
- Aggregate in `RuleSummary`: rejected because presentation concerns should not
  replace or duplicate complete domain evidence.
- Keep both aggregate and full list for one release: rejected because it does
  not solve the current default-output problem.

## Deferred Scope

- JSON compaction or format-version changes.
- New skip-reason inference or complete accounting of every non-applicable rule.
- Changes to `deltascope rules list`.
- Release-note wording, owned by the next release-preparation milestone.

## Acceptance Criteria

This decision remained Proposed until a fixed candidate on the refresh branch
satisfied the spec's renderer, CLI, and JSON evidence; all required gates
passed; and fresh Standards and Spec review reported no unresolved P0, P1, or
P2. Acceptance evidence cites immutable base and candidate SHAs.

Fixed refresh base: `9f98315f14a25bd0bac2218e0d079001348f10f0`
(`origin/main` at refresh start). The previous reviewed candidate
`e28f6d5efcf52fc9f88bf96979ac94ecce30aa04` (branch
`feat/markdown-rule-summary-aggregation-issue-17-20260817`, base
`34d6c6d9e33f75e7cb298ff96324413b4f320b28`) was superseded because its base
predated the #18-#26 merges; it remains historical evidence only and did not
authorize this acceptance.

Accepted on branch `feat/markdown-rule-summary-aggregation-issue-17-refresh-20260818`
with base `9f98315f14a25bd0bac2218e0d079001348f10f0` and fixed reviewed
candidate `0adbeeaa0e7d85a6ff333fcc6a07226486089ee6`, covering commits
`e33a1ef` (docs proposal import with ADR Proposed), `1c293eb` (renderer/CLI
RED characterization), `d41952a` (aggregated `### Skip Reasons` implementation
and doc sync), and `2b8068c`/`0adbeea` (review P1 remediation: exact aggregate
row count pinned to the JSON skipped list, then exact-line matching). Fresh
three-round two-axis review (Standards, then Spec) over `9f98315...0adbeea`
reported zero unresolved P0, P1, and P2 after the final fix, so acceptance is
flipped. Verification evidence: focused markdown/CLI/JSON/quiet tests, real
CLI audit regression, full default and PostgreSQL-tagged tests, build, default
and tagged vet, lint, CLI/MySQL/TiDB/PostgreSQL E2E suites (CLI metadata,
PostgreSQL CLI metadata + objects, HTTP MySQL/PostgreSQL, MCP PostgreSQL, CLI
TLS), sql-corpus and query-access corpus gates, docs-example, decision-record,
gofmt, three-level-documentation, module-tidy, and diff-scope checks all
green.
