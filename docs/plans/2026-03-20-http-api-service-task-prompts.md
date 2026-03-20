# HTTP API Service Task Prompts

> For task-by-task implementation and review of the `HTTP API Service` milestone.  
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/deltascope`.

## Global Rules

- Keep the HTTP layer thin and adapter-only.
- Reuse the existing audit core instead of forking CLI logic.
- Keep `three-level-doc` as a hard gate.
- Return files changed, tests run, status, and commit hash for every task.

## Milestone Focus

- JSON audit endpoint
- health/version endpoints
- config-backed long-running server wiring
- docs and verification closure
