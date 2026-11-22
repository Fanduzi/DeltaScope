# Coding Standards

## Decisions

Commit an ADR in `docs/decisions/` with the task for milestone work or a non-obvious product, architecture, parser, rule, metadata, privacy, or public-contract boundary. Required cases include:

- Feature deferral; promotion of unsupported behavior to supported or finding-covered behavior.
- Public output shape changes; privacy/no-leak design; SDK/CLI/HTTP/MCP consistency decisions.
- Nuanced release non-goals, boundaries, or compatibility decisions; multi-commit milestone rationale otherwise confined to chat or local plans.

Record the decision, durable rationale, public contract, deferred scope, verification evidence, and links to relevant tests, corpus fixtures, docs, and commits—not task logs.

Every task's final report must state whether an ADR was required and explain why if not. Distinguish task diffs (`<previous-task>..HEAD`) from milestone diffs (`main...HEAD`).

## Changes

- Preserve layer ownership: `cmd/` holds thin flag/startup entrypoints; `internal/interfaces/` translates CLI/HTTP/MCP requests; `internal/application/` orchestrates; `internal/domain/` owns rules/specs; `internal/infrastructure/` implements external adapters. `pkg/deltascope` is the stable public API.
- Preserve the file-header convention. Update a package's README with changes to its exports, responsibilities, or dependencies.
- Synchronize `README.md`, `README_ZH.md`, and relevant release docs when installation or release documentation changes.
- Rule behavior changes require fixtures in `testdata/sql-corpus/`.
- Run focused Go tests during development; finish with `make test`.
- PostgreSQL: use canonical `make pg-unit-test-gates` for tagged unit tests without Docker, or `make pg-confidence-gates` for those tests plus Docker-backed CLI/HTTP/MCP E2E. Their recipes own the required `postgresql` build tag and other flags.
- Metadata CLI/HTTP/MCP E2E: select relevant `test-e2e-*` targets in [Makefile](Makefile); Docker is required, plus Python 3 for CLI suites.
- Node launcher: run `npm test --prefix packages/deltascope-mcp`.

## Milestones

- Start a multi-task milestone from current local `main` on one branch/worktree. Run each task's gates and make its focused commit before starting the next task.
- Separate task worktrees require explicitly parallel work with disjoint write sets; reintegrate before sequential work resumes.
- Merge into local `main` only when the whole milestone is ready. Prefer `--ff-only`; stop if it fails.
- Final milestone reports must give the actual branch, HEAD, base, working-tree status, and `git diff --name-only <base>...HEAD` output.

## Delivery

- Keep commits focused on a surface or layer, with short conventional subjects (`feat`, `fix`, `test`, `docs`, or `chore`; scopes allowed).
- PR descriptions must identify the affected surface, link the issue when applicable, and list exact verification commands.
- Push, tag, or release only with explicit instruction.

## Releases

- Handoff prompts must explicitly instruct the executor to invoke and follow the `go-release` skill; a repository checklist cannot substitute. Handoff authors need not execute the release.
- Repository context, version, safety, and reporting requirements may be acceptance criteria. The skill wins conflicts unless the checklist adds a stricter safety guardrail, including no-co-author, readiness, AI-attribution checks, assets, and binary-version smoke tests.
