# Audit Completion Task Prompts

> For task-by-task implementation and review of the `Audit Completion` milestone.  
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/deltascope`.

## Global Rules

- Keep the current offline-first engine intact.
- Add metadata as optional facts, not as a second audit pipeline.
- Keep rules consuming normalized specs and snapshots instead of raw DB clients.
- Use TDD for every non-trivial rule or orchestration change.
- Keep `three-level-doc` as a hard gate.
- Return files changed, tests run, status, and commit hash for every task.

## Milestone Focus

- capability matrix as the acceptance source of truth
- instance facts and object snapshots
- metadata-aware optional enhancement
- deeper alter compatibility
- remaining important DDL/DML gap closure
- mature product docs and release surface

## Task Intent

### Task 1: Capability Matrix

- Produce a concrete matrix of important audit capabilities.
- Mark each row as covered, replaced, deferred, or out of scope.
- Identify exactly which rows still block milestone completion.

### Task 2: Metadata Domain Abstractions

- Add normalized domain types for instance facts and table snapshots.
- Do not leak SQL clients or parser-specific objects into the domain model.

### Task 3: Metadata Providers

- Add optional provider interfaces and infrastructure-backed implementations.
- Preserve the exact offline path when no metadata provider is configured.

### Task 4: Existence And Snapshot Rules

- Add rules that depend on object existence and current table shape.
- Wire defaults and config-template entries for every shipped rule.

### Task 5: Alter Compatibility

- Deepen source-to-target compatibility checks.
- Stay honest where metadata or source facts are still insufficient.

### Task 6: Remaining Gap Closure

- Use the matrix, not intuition, to choose the remaining rule work.
- Prefer high-value gaps over low-value breadth.

### Task 7: Product Docs

- Rewrite README with shields and quick links.
- Add `README_ZH.md`, `CHANGELOG.md`, and `SECURITY.md`.
- Keep English and Chinese readmes aligned in structure.

### Task 8: Closure

- Re-run full verification.
- Update handoff, autonomous-progress, and decisions.
- Finalize the capability matrix and push the milestone.
