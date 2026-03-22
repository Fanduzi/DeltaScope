# CLI Metadata E2E Task Prompts

> For task-by-task implementation and review of the `CLI Metadata E2E` milestone.  
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/deltascope`.

## Global Rules

- Keep the existing CLI contract unchanged; this milestone validates it through real targets.
- Test the public CLI only; do not bypass it through internal helpers.
- Keep containerized e2e separate from normal `go test` workflows.
- Use stable fixture schemas and tables so ambiguity and inference behavior are deterministic.
- Prefer JSON assertions over markdown string matching.
- Keep `three-level-doc` as a hard gate for any code or docs touched.
- Return files changed, commands run, status, and commit hash for every task.

## Milestone Focus

- real MySQL metadata-aware CLI smoke
- real TiDB metadata-aware CLI smoke
- schema inference and ambiguity coverage
- qualified-schema SQL coverage
- metadata-backed existence and compatibility coverage
- local developer ergonomics through Make targets

## Task Intent

### Task 1: Planning Artifacts

- Save the agreed design, implementation plan, and prompts.

### Task 2: Docker And Fixtures

- Add reproducible MySQL and TiDB services plus fixture SQL that creates both unique and ambiguous target-table situations.

### Task 3: E2E Script

- Add one script that can execute the metadata-aware CLI suite against MySQL, TiDB, or both.
- Keep cleanup and readiness handling deterministic.

### Task 4: MySQL Coverage

- Prove the MySQL path with real CLI invocations and JSON assertions.

### Task 5: TiDB Coverage

- Prove the TiDB path with real CLI invocations and JSON assertions.

### Task 6: Makefile And Docs

- Add simple local entrypoints and document how to run the e2e suite.

### Task 7: Closure

- Re-run verification.
- Remove the remaining “no live smoke yet” risk from handoff/progress docs.
- Leave the milestone in a shippable state.
