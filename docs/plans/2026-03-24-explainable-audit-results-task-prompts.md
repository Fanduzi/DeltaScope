# Explainable Audit Results Task Prompts

> For task-by-task implementation and review of the `Explainable Audit Results` milestone.
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/DeltaScope`.

## Global Rules

- Keep the existing offline-first audit engine intact.
- Keep verdict semantics, finding counts, and `--fail-on` behavior unchanged.
- Treat explanation enrichment as a shared post-evaluation capability, not a CLI-only formatting trick.
- Keep rule execution and explanation metadata as separate concerns linked by `rule_id`.
- Use TDD for every non-trivial model, enrichment, transport, or UX change.
- Keep `three-level-doc` as a hard gate.
- Return files changed, tests run, status, and commit hash for every task.

## Milestone Focus

- stable explanation structure on audit findings
- rule catalog as the explanation source of truth
- post-evaluation finding enrichment
- CLI human-readable explanation output
- structured explanation output in HTTP and `pkg/deltascope`
- honest metadata-aware explanation notes
- docs that teach users how to read enriched findings

## Task Intent

### Task 1: Planning Artifacts

- Save the design, implementation plan, and task prompts for the milestone.
- Keep naming and scope aligned with `Explainable Audit Results`.

### Task 2: Shared Explanation Model

- Add a stable explanation structure to shared result types.
- Do not change verdict semantics or make explanation data mandatory for correctness.

### Task 3: Rule Catalog Expansion

- Extend the shipped rule catalog so it can explain findings, not just list rules.
- Prefer concise, stable metadata over verbose prose.

### Task 4: Explanation Enrichment

- Enrich findings after evaluation using catalog metadata and current audit context.
- Missing explanation metadata must degrade gracefully.

### Task 5: Public Package Exposure

- Expose explanation data through `pkg/deltascope` so downstream callers can use it directly.
- Keep the public API coherent and documented.

### Task 6: CLI Output

- Add a detailed explanation-oriented CLI view without breaking compact default output.
- Ensure JSON output exposes the structured explanation data cleanly.

### Task 7: HTTP Output

- Return structured explanation fields from `POST /v1/audit`.
- Keep the existing response contract stable apart from the additive explanation fields.

### Task 8: Metadata Transparency

- Make explanation output honest about whether metadata enhanced or limited the result.
- Do not imply confidence that the engine did not actually have.

### Task 9: Docs

- Update reference and recipe docs to teach users how to read enriched findings.
- Keep CLI, HTTP, and library examples aligned.

### Task 10: Closure

- Re-run full verification.
- Confirm explanation enrichment does not change existing verdict outcomes.
- Update handoff, autonomous progress, and decisions.
- Leave the milestone in a shippable state.
