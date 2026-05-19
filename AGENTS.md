# Repository Guidelines

## Project Structure & Module Organization

DeltaScope is a layered Go repository. Keep entrypoints thin in `cmd/` (`deltascope`, `deltascope-server`, `deltascope-mcp`), transport adapters in `internal/interfaces/`, use-case orchestration in `internal/application/`, core audit logic in `internal/domain/`, and external adapters in `internal/infrastructure/`. The stable public API lives in `pkg/deltascope/`. Supporting material lives in `configs/`, `docs/`, `scripts/`, `docker/`, and `testdata/sql-corpus/`. The npm launcher for MCP clients is in `packages/deltascope-mcp/`.

## Build, Test, and Development Commands

- `make test`: fast default verification; runs `go test ./...`.
- `make build`: builds all local binaries into `bin/`.
- `make build-cli`, `make build-server`, `make build-mcp`: build one surface at a time.
- `make build-linux`: cross-compiles Linux amd64 binaries.
- `make test-e2e-cli`, `make test-e2e-http-mysql`, `make test-e2e-mcp-postgresql`: Docker-backed metadata e2e suites.
- `make pg-unit-test-gates` and `make pg-confidence-gates`: canonical PostgreSQL verification targets.
- `npm test --prefix packages/deltascope-mcp`: runs the Node launcher tests (`node --test`).

## Coding Style & Naming Conventions

Use standard Go formatting with `gofmt`; keep package names lowercase and exported identifiers in `CamelCase`. Match the existing architecture: `cmd` binds flags and startup only, interfaces translate requests, application coordinates work, domain owns rules/specs, and infrastructure wraps third-party dependencies. Follow the repo’s file-header convention where present. Test files should stay adjacent to code and use the usual `*_test.go` naming.

## Testing Guidelines

Prefer focused `go test ./path/... -run TestName` during development, then finish with `make test`. PostgreSQL-tagged coverage uses `-tags postgresql` and the Make targets above. Metadata-aware CLI/HTTP/MCP suites require Docker; CLI metadata scripts also require Python 3. Add or update fixtures under `testdata/sql-corpus/` when rule behavior changes.

## Commit & Pull Request Guidelines

Recent history follows short conventional subjects such as `feat:`, `fix:`, `test:`, `docs:`, and `chore:`; scoped forms like `feat(http): ...` fit well. Keep commits focused by surface or layer. PRs should explain the affected surface, link the issue when applicable, and list the exact verification commands run. If you change package exports, module responsibilities, or dependencies, update the affected package `README.md` in the same change. When install or release-facing docs change, sync `README.md`, `README_ZH.md`, and relevant release docs together.

## Decision Record Discipline

For milestone work or any change that introduces a non-obvious product, architecture, parser, rule, metadata, privacy, or public-contract boundary, create or update a committed decision record under `docs/decisions/`.

Decision records are for durable rationale, not task logs. Do not paste full task reports. Summarize the decision, rationale, public contract, deferred scope, verification evidence, and links to tests, corpus fixtures, docs, and commits.

A decision record is required when:
- a feature is intentionally deferred
- an unsupported boundary is promoted to supported or finding-covered behavior
- public output shape changes
- privacy/no-leak behavior is part of the design
- cross-surface behavior must stay consistent across SDK, CLI, HTTP, or MCP
- release docs mention a nuanced non-goal, boundary, or compatibility promise
- a milestone makes multiple related commits whose rationale would otherwise live only in chat or local plans

Before completing a task, explicitly state whether a decision record was required. If required, commit it with the task. If not required, explain why in the final report.

## Milestone Branch Discipline

For multi-task milestones, default to one milestone branch/worktree, not one branch per task. Start the milestone from current local `main`; implement each task as a focused commit on the milestone branch. Before moving to the next task, commit the current task and run its required gates. Use separate task worktrees only for explicitly parallel work with disjoint write sets, then integrate them back into the milestone branch before continuing sequential work. Merge the milestone branch into local `main` only after the whole milestone is ready; use `--ff-only` when possible, and stop if it fails. Do not push, tag, or release unless explicitly instructed.

## Task Report Accuracy

Task reports must distinguish task-commit diff (`<previous-task>..HEAD`) from milestone diff (`main...HEAD`). Final milestone reports must include actual branch, HEAD, base, working tree status, and `git diff --name-only <base>...HEAD` scope.

## Release Prompt Discipline

Any DeltaScope release handoff prompt must be skill-first. It must explicitly require the executor to call and follow the `go-release` skill instead of treating hand-written shell steps as a substitute release workflow. Repository-specific context, version numbers, safety constraints, and final reporting requirements may be included after that requirement, but they are acceptance criteria and guardrails, not a replacement for the skill. If the `go-release` skill and a hand-written command checklist differ, follow the skill unless the checklist is a stricter safety requirement such as `no-co-author`, release-readiness verification, AI-attribution checks, asset validation, or binary version smoke tests.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **DeltaScope** (11271 symbols, 23972 relationships, 276 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## When Debugging

1. `gitnexus_query({query: "<error or symptom>"})` — find execution flows related to the issue
2. `gitnexus_context({name: "<suspect function>"})` — see all callers, callees, and process participation
3. `READ gitnexus://repo/DeltaScope/process/{processName}` — trace the full execution flow step by step
4. For regressions: `gitnexus_detect_changes({scope: "compare", base_ref: "main"})` — see what your branch changed

## When Refactoring

- **Renaming**: MUST use `gitnexus_rename({symbol_name: "old", new_name: "new", dry_run: true})` first. Review the preview — graph edits are safe, text_search edits need manual review. Then run with `dry_run: false`.
- **Extracting/Splitting**: MUST run `gitnexus_context({name: "target"})` to see all incoming/outgoing refs, then `gitnexus_impact({target: "target", direction: "upstream"})` to find all external callers before moving code.
- After any refactor: run `gitnexus_detect_changes({scope: "all"})` to verify only expected files changed.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Tools Quick Reference

| Tool | When to use | Command |
|------|-------------|---------|
| `query` | Find code by concept | `gitnexus_query({query: "auth validation"})` |
| `context` | 360-degree view of one symbol | `gitnexus_context({name: "validateUser"})` |
| `impact` | Blast radius before editing | `gitnexus_impact({target: "X", direction: "upstream"})` |
| `detect_changes` | Pre-commit scope check | `gitnexus_detect_changes({scope: "staged"})` |
| `rename` | Safe multi-file rename | `gitnexus_rename({symbol_name: "old", new_name: "new", dry_run: true})` |
| `cypher` | Custom graph queries | `gitnexus_cypher({query: "MATCH ..."})` |

## Impact Risk Levels

| Depth | Meaning | Action |
|-------|---------|--------|
| d=1 | WILL BREAK — direct callers/importers | MUST update these |
| d=2 | LIKELY AFFECTED — indirect deps | Should test |
| d=3 | MAY NEED TESTING — transitive | Test if critical path |

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/DeltaScope/context` | Codebase overview, check index freshness |
| `gitnexus://repo/DeltaScope/clusters` | All functional areas |
| `gitnexus://repo/DeltaScope/processes` | All execution flows |
| `gitnexus://repo/DeltaScope/process/{name}` | Step-by-step execution trace |

## Self-Check Before Finishing

Before completing any code modification task, verify:
1. `gitnexus_impact` was run for all modified symbols
2. No HIGH/CRITICAL risk warnings were ignored
3. `gitnexus_detect_changes()` confirms changes match expected scope
4. All d=1 (WILL BREAK) dependents were updated

## Keeping the Index Fresh

After committing code changes, the GitNexus index becomes stale. Re-run analyze to update it:

```bash
npx gitnexus analyze
```

If the index previously included embeddings, preserve them by adding `--embeddings`:

```bash
npx gitnexus analyze --embeddings
```

To check whether embeddings exist, inspect `.gitnexus/meta.json` — the `stats.embeddings` field shows the count (0 means no embeddings). **Running analyze without `--embeddings` will delete any previously generated embeddings.**

> Claude Code users: A PostToolUse hook handles this automatically after `git commit` and `git merge`.

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
