# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is DeltaScope

DeltaScope is an offline-first SQL audit engine for MySQL and TiDB. It ships three binaries (`deltascope`, `deltascope-server`, `deltascope-mcp`) and a stable Go library (`pkg/deltascope`).

## Build & Test Commands

```bash
# Run all unit tests
make test           # equivalent: go test ./...

# Run a single test package or test function
go test ./internal/domain/rule/... -run TestRegistryDeterministic

# Build all binaries → bin/
make build

# Build individual binaries
make build-cli
make build-server
make build-mcp

# Cross-compile Linux amd64 binaries
make build-linux

# Metadata E2E tests (require Docker + Go; CLI targets also need Python 3)
make test-e2e-cli          # CLI against both MySQL and TiDB
make test-e2e-cli-mysql
make test-e2e-cli-tidb
make test-e2e-mcp-mysql
make test-e2e-mcp-tidb
make test-e2e-http-mysql
make test-e2e-http-tidb
```

All builds use `CGO_ENABLED=0` by default.

## Architecture

The codebase follows a strict layered architecture. Dependencies only flow downward:

```
cmd/                        → thin entrypoints, bind flags/startup only
internal/interfaces/        → transport adapters (cli, http, mcp)
internal/application/       → use-case orchestration (audit, auditmeta, policy)
internal/domain/            → core logic: spec, rule, report, policy
internal/infrastructure/    → adapters to external deps (parser/tidb, config/viper, metadata/mysql, output/json|markdown)
pkg/deltascope/             → stable public library facade (Audit, Request, Result, ...)
```

Key boundaries:
- `cmd` packages stay thin — mostly flag binding and binary startup.
- `internal/interfaces` owns request/response translation for CLI, HTTP, and MCP. Delegates real work to `internal/application` or `pkg/deltascope`.
- `internal/application/audit` runs the main offline audit path: parse → enrich → evaluate rules → build report.
- `internal/domain/spec` defines normalized statement models (`Statement`, `DDL`, `DML`, `Alter`, etc.) that rules consume.
- `internal/domain/rule` owns `Finding`, `Registry`, `StatementRule`, and `GlobalRule` — the rule contract layer.
- `internal/infrastructure/parser/tidb` wraps the TiDB parser for both MySQL and TiDB dialects.
- `pkg/deltascope` is the stable public API (`Audit(ctx, request)`) that library consumers import.

## Rule Configuration

Rules are configured via YAML. See `configs/deltascope.example.yaml` for all rule IDs, default levels (`blocker`/`warning`/`notice`), and params. The config is loaded through `internal/infrastructure/config/viper`.

## GitNexus Code Intelligence

This repo is indexed by GitNexus. Before editing any symbol:

1. Run `gitnexus_impact({target: "symbolName", direction: "upstream"})` to check blast radius.
2. Warn the user if risk is HIGH or CRITICAL before proceeding.
3. Use `gitnexus_rename` for all renames — never raw find-and-replace.
4. Run `gitnexus_detect_changes()` before committing to verify scope.

After committing, re-index with `npx gitnexus analyze` (add `--embeddings` if `.gitnexus/meta.json` shows `stats.embeddings > 0`).

## Module README Update Rule

Each package under `internal/` and `pkg/` has a `README.md` that lists exports and dependencies. **When changing a package's exported symbols or inter-package dependencies, update that package's `README.md` in the same commit.**

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

For multi-task milestones, default to one milestone branch/worktree, not one branch per task. Start from current local `main`; implement each task as a focused commit on the milestone branch. Before moving to the next task, commit the current task and run its gates. Use separate task worktrees only for explicitly parallel work with disjoint write sets, then integrate them back into the milestone branch before continuing. Merge the milestone branch into local `main` only after the whole milestone is ready; use `--ff-only` when possible, and stop if it fails. Do not push, tag, or release unless explicitly instructed.

## Task Report Accuracy

Task reports must distinguish task-commit diff (`<previous-task>..HEAD`) from milestone diff (`main...HEAD`). Final milestone reports must include actual branch, HEAD, base, working tree status, and `git diff --name-only <base>...HEAD` scope.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **DeltaScope** (11292 symbols, 24044 relationships, 300 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

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
