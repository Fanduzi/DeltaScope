# Implementation Plan: Aggregate Markdown Rule Skip Reasons

## Status

Proposed. This is one implementation issue on one isolated branch. It does not
authorize merge, push, release, or issue closure.

## 1. Establish the Fixed Point

- Start from current `main` and record the full base SHA.
- Confirm Issue #17 remains open, the ADR remains Proposed, the task worktree is
  clean, and root user WIP is untouched.
- Use CodeGraph and source inspection to confirm the Markdown renderer, CLI
  render path, JSON renderer, quiet renderer, and existing test owners.
- Capture the current noisy default Markdown output as RED evidence.

## 2. Lock the Public Contract RED

- Replace the old per-rule Markdown expectation with the canonical aggregate
  shape from the spec.
- Add the minimum cases for multiple IDs with one reason, deterministic multiple
  reasons, an unknown reason, and no skipped items.
- Add or adapt one CLI-path regression proving default Markdown omits every
  skipped rule ID and the `## Skipped Rules` heading.
- Pin JSON preservation of the complete per-rule list.

## 3. Implement Renderer-Local Aggregation

- Keep `markdown.Render(report.Result)` unchanged.
- Count skipped entries by `rule.SkipReason`, sort distinct raw reason codes,
  and render the aggregate under `### Skip Reasons`.
- Rename the Markdown count to `Skipped with known reason`.
- Omit the reason subsection for an empty skipped slice.
- Delete the per-rule rendering loop; add no cap, sample, option, flag, or new
  domain type.

## 4. Synchronize Documentation

- Update the Markdown renderer L3 header and its module README according to the
  three-level-documentation protocol.
- Update English and Chinese CLI reference text.
- Update English and Chinese migration recipe text.
- Keep JSON, quiet, CI-native, release, and `CONTEXT.md` contracts unchanged.
- Keep the ADR Proposed until a fixed candidate passes all review requirements.

## 5. Verify

- Run focused Markdown renderer, CLI render-path, JSON, and quiet tests.
- Run a real CLI audit that previously produced the expanded PostgreSQL list;
  assert bounded Markdown output and preserved verdict/finding.
- Run default and PostgreSQL-tagged full tests, build, vet, and lint.
- Run relevant output/CLI integration gates and documentation examples.
- Run decision-record, gofmt, three-level-documentation, module-tidy, and diff
  checks.
- Confirm no rule evaluation, domain, SDK, HTTP, MCP, workflow, fixture,
  dependency, version, or release file changed.

## 6. Independent Review and Acceptance

- Freeze a full candidate SHA and immutable review range.
- Request fresh read-only Standards and Spec reviews.
- Treat Markdown rule-ID leakage, JSON list loss, count-semantic drift,
  nondeterministic output, new CLI flags, or unrelated format changes as
  blocking.
- Fix every P1/P2, rerun affected and full gates, and repeat review.
- Only after no unresolved P0/P1/P2 remains, change the ADR from Proposed to
  Accepted in a focused final commit citing the fixed reviewed candidate.

## 7. Delivery Closure

- Fast-forward local `main` only with human authorization and rerun required
  gates on the merged SHA.
- Push only with separate authorization and verify exact-SHA CI.
- Close only Issue #17 after merge, push, and required CI are green.
- Do not tag, release, publish, force-push, or delete branches/worktrees unless
  separately authorized.

## Suggested Commits

1. `docs(output): propose aggregated Markdown rule summary`
2. `test(output): characterize Markdown skipped-rule noise`
3. `fix(output): aggregate Markdown skip reasons`
4. `docs(output): accept aggregated Markdown rule summary`
