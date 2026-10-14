# Coding Standards

## Before code or documentation changes

- Preserve the layered boundaries: `cmd/` holds thin flag/startup entrypoints; `internal/interfaces/` translates CLI/HTTP/MCP requests; `internal/application/` orchestrates; `internal/domain/` owns rules/specs; `internal/infrastructure/` implements external adapters. `pkg/deltascope` is the stable public API.
- Preserve the existing file-header convention.
- When a package's exports, responsibilities, or dependencies change, update its README in the same change.
- When installation or release documentation changes, synchronize `README.md`, `README_ZH.md`, and the relevant release docs.
- For rule configuration, consult [configs/deltascope.example.yaml](configs/deltascope.example.yaml).
- Rule behavior changes require fixtures in `testdata/sql-corpus/`.

## During development and verification

- Run focused Go tests during development, then finish with `make test`.
- PostgreSQL coverage requires the `postgresql` build tag. Use `make pg-unit-test-gates` for tagged unit tests without Docker, or `make pg-confidence-gates` for unit tests plus Docker-backed CLI/HTTP/MCP E2E coverage. These are the canonical PostgreSQL entrypoints; their recipes own the build flags.
- For metadata CLI/HTTP/MCP E2E verification, select the relevant `test-e2e-*` targets in [Makefile](Makefile). These need Docker; CLI suites also need Python 3.
- For Node launcher verification, run `npm test --prefix packages/deltascope-mcp`.
- Look up build commands and other verification recipes in [Makefile](Makefile), rather than maintaining a separate command catalog here.

## Decision records for every task

Commit a record in `docs/decisions/` for milestone work or a non-obvious product, architecture, parser, rule, metadata, privacy, or public-contract boundary. Required cases include:

- Intentionally deferring a feature.
- Promoting unsupported behavior to supported or finding-covered behavior.
- Changing output shape or designing privacy/no-leak behavior.
- Deciding SDK/CLI/HTTP/MCP consistency.
- Establishing a nuanced release non-goal, boundary, or compatibility decision.
- Preserving multi-commit milestone rationale that would otherwise exist only in chat or local plans.

Record durable rationale, the decision, public contract, deferred scope, verification evidence, and links to relevant tests, corpus fixtures, docs, and commits—not task logs.

## Before commits or pull requests

- Use short conventional commit subjects (`feat`, `fix`, `test`, `docs`, or `chore`; scopes allowed), with commits focused on a surface or layer.
- PR descriptions must identify the affected surface, link the issue when applicable, and list exact verification commands.

## When planning or executing a milestone

- For a multi-task milestone, use one branch/worktree starting from current local `main`. Make a focused commit per task; run its gates and commit before beginning the next task.
- Separate task worktrees are only for explicitly parallel work with disjoint write sets. Reintegrate that work before sequential work continues.
- Merge into local `main` only when the whole milestone is ready. Prefer `--ff-only`; stop if it fails.

## Before pushing, tagging, or releasing

Push, tag, or release only with explicit instruction.

## Before every final report

- Every task's final report must explicitly state whether an ADR was required. If required, commit it with the task; otherwise explain why it was not required.
- Distinguish a task diff (`<previous-task>..HEAD`) from a milestone diff (`main...HEAD`). A final milestone report must give the actual branch, HEAD, base, working-tree status, and `git diff --name-only <base>...HEAD` output.

## When writing a release handoff

- The release handoff prompt must explicitly instruct its executor to invoke and follow the `go-release` skill; a repository checklist is not a substitute. Writing the handoff does not require its author to execute a release.
- Repository context, version, safety, and reporting requirements may be acceptance criteria. The skill wins conflicts unless the checklist adds a stricter safety guardrail, including no-co-author, readiness, AI-attribution checks, assets, and binary-version smoke tests.
