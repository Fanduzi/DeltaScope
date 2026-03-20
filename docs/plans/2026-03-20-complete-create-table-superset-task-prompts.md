# Complete Create-Table Superset Task Prompts

> For task-by-task implementation and review of the `Complete Create-Table Superset` milestone.  
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/deltascope`.

## Global Rules

- Follow the design in `docs/plans/2026-03-20-complete-create-table-superset-design.md`.
- Follow the implementation sequence in `docs/plans/2026-03-20-complete-create-table-superset-implementation.md`.
- Preserve the current parser-neutral DDL flow.
- Do not introduce live metadata dependencies.
- Keep `three-level-doc` as a hard gate.

## Task Themes

- identifier and keyword governance
- wider type-family and charset/collation rules
- deeper redundant-index analysis
- remaining create-table object-shape gaps

## Validation Pattern

Every task must return:
- files changed
- tests run
- status
- commit hash

Target validations are defined in the implementation plan and should be used as written.
