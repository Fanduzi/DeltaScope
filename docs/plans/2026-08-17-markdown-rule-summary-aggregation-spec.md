# Spec: Aggregate Markdown Rule Skip Reasons

## Status

Proposed. This document defines the output contract for Issue #17. It does not
authorize implementation, merge, push, release, or issue closure.

## Problem

The default Markdown audit output renders every entry in
`RuleSummary.Skipped` under `## Skipped Rules`. A small MySQL audit can therefore
print more than 190 PostgreSQL rule IDs with the same dialect-mismatch reason.
The verdict and findings are already visible, so the repeated catalog rows
obscure the useful result for both people and agents.

The complete skipped-rule list remains useful as structured audit evidence.
The problem is presentation density in the default human-readable format, not
rule evaluation or the `rule_summary` data contract.

## Objective

Keep the default Markdown rule summary bounded by the number of skip reasons,
not by the number of shipped rules. Preserve the complete per-rule list in JSON
and preserve every non-Markdown format and evaluation behavior.

## Required Contract

1. Default Markdown retains `## Rule Summary` with `Loaded`, `Applicable`, and
   the count currently derived from `len(RuleSummary.Skipped)`.
2. Markdown labels that last count `Skipped with known reason` because the
   registry records only non-applicability reasons it can infer reliably.
   `Loaded` is not required to equal `Applicable + Skipped with known reason`.
3. When the skipped list is non-empty, Markdown renders `### Skip Reasons` and
   one count per distinct `SkipReason`.
4. Markdown never renders a skipped rule ID and never renders the old
   `## Skipped Rules` section.
5. Reason groups are ordered deterministically by the underlying reason code.
   Known codes use bounded human text; an unknown future code is rendered
   verbatim rather than omitted.
6. When no skipped reason is recorded, Markdown shows the zero count and omits
   `### Skip Reasons`.
7. JSON retains the complete `rule_summary.skipped` array, including every
   `rule_id` and `reason`. No JSON field, version, or serialization behavior
   changes.
8. Quiet, GitHub Actions, GitHub Summary, SARIF, GitLab Code Quality, and every
   other output format remain unchanged.
9. Rule loading, applicability, skip inference, deduplication, sorting, and
   domain types remain unchanged.
10. Do not add `--verbose`, an environment variable, a compatibility switch,
    renderer options, or a new rule-summary model.
11. Update the English and Chinese CLI reference and migration recipe to state
    that Markdown is aggregated while JSON retains the complete list.
12. Do not edit release notes before a release-preparation task owns that work.

## Canonical Markdown Shape

```text
## Rule Summary

- Loaded: 365
- Applicable: 6
- Skipped with known reason: 190

### Skip Reasons

- Not applicable to current dialect: 190
```

The numbers are examples, not fixed catalog facts.

## Explicit Non-Goals

- No change to rule evaluation, applicability, or skip semantics.
- No compaction or version change for JSON.
- No per-rule sample, cap, truncation message, or hidden recovery mode in
  Markdown.
- No change to `deltascope rules list`.
- No new domain term in `CONTEXT.md`.

## Acceptance Evidence

The ADR may become Accepted only after:

- a renderer RED test demonstrates that the old Markdown emits per-rule IDs;
- the renderer contract proves counts, deterministic reason aggregation,
  unknown-reason preservation, and empty-summary behavior;
- a real CLI-path regression proves default Markdown contains the aggregate and
  no skipped rule IDs or `## Skipped Rules` section;
- JSON regression evidence proves the complete per-rule list is unchanged;
- existing Markdown, CLI, JSON, quiet, and output-format tests pass;
- full tests, build, vet, lint, documentation, decision-record, gofmt,
  three-level-documentation, module-tidy, and diff checks pass; and
- fresh Standards and Spec review of a fixed candidate reports no unresolved
  P0, P1, or P2.
