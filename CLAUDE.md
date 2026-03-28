# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
make build            # produces bin/deltascope, bin/deltascope-server, bin/deltascope-mcp
make build-cli        # CLI only
make build-server     # HTTP server only
make build-mcp        # MCP server only
make build-linux      # Linux amd64 binaries under bin/

# Test
make test             # go test ./... (fast, no Docker)
make test-e2e-cli     # Docker-backed metadata e2e: MySQL + TiDB
make test-e2e-cli-mysql
make test-e2e-cli-tidb
make test-e2e-mcp-mysql  # MCP metadata e2e (Docker + Go only)
make test-e2e-mcp-tidb

# Run a single test package
go test ./internal/domain/rule/ddl/...

# Run a single test by name
go test ./internal/domain/rule/ddl/... -run TestTableCommentRule
```

CLI metadata e2e tests require Docker, Go, and Python 3. MCP metadata e2e tests require Docker and Go only.

## Architecture

DeltaScope is an offline-first SQL audit engine for MySQL and TiDB. One audit path, four product surfaces: `deltascope` CLI, `deltascope-server` HTTP service, `deltascope-mcp` MCP server, and `pkg/deltascope` library.

### Layer Boundaries

```
cmd/deltascope | cmd/deltascope-server | cmd/deltascope-mcp  ← thin process entrypoints (flag binding only)
       ↓
internal/interfaces/cli | http | mcp     ← transport adapters (request/response translation)
       ↓
internal/application/audit | policy      ← orchestration: parse → extract → enrich → evaluate
       ↓
internal/domain/spec | rule | policy | report  ← core domain: normalized types and rule semantics
       ↓
internal/infrastructure/parser/tidb      ← TiDB parser adapter (converts AST → spec)
internal/infrastructure/config/viper     ← YAML policy loader
internal/infrastructure/metadata/mysql   ← live schema/instance facts provider
internal/infrastructure/output/json | markdown  ← renderers
       ↓
pkg/deltascope                           ← stable public API facade
```

**Key constraint**: `internal/infrastructure` adapts external dependencies. Domain packages must not import infrastructure. Rules live in `internal/domain/rule/ddl` and `internal/domain/rule/dml`.

### Audit Flow

1. **Parse** (`application/audit/parse.go`): SQL text → `ParsedSQL` via TiDB parser adapter.
2. **Extract** (`application/audit/extract.go`): `ParsedSQL` → `[]spec.Statement` (parser-neutral normalized model).
3. **Enrich** (`application/audit/metadata.go`): optional `MetadataProvider` attaches live `TableSnapshot` and `InstanceFacts` to each statement.
4. **Evaluate** (`application/audit/evaluate.go`): registered rules applied to each statement; findings aggregated into `report.Report`.

### Key Domain Types

- `spec.Statement` — normalized statement with `DDL`, `DML`, and optional `Metadata` (schema, instance facts, target table snapshot).
- `rule.Finding` — one rule result with stable ID, `Level` (blocker/warning/notice), and message.
- `report.Report` / `Verdict` — aggregated outcome; verdict maps to CLI exit codes via `--fail-on`.
- `rule.Registry` — holds statement-scoped and global rules; evaluated deterministically by registration order.
- `policy.Policy` — per-rule on/off + params; `policy.Default()` is the shipped baseline.

### Rule Organization

- **DDL rules** (`internal/domain/rule/ddl/`): CREATE TABLE governance, ALTER TABLE restrictions, object lifecycle (CREATE VIEW, DROP TABLE, TRUNCATE), merge-alter guidance, metadata-backed existence/sizing checks.
- **DML rules** (`internal/domain/rule/dml/`): WHERE/LIMIT/ORDER BY requirements, subquery/join-on guards, INSERT row-count and pattern restrictions, table denylists.
- Each package exposes `Register(registry, policy)` which registers only enabled rules.
- Rule IDs follow the pattern `ddl.<area>.<check>` or `dml.<area>.<check>`.

### Module README Contract

Every `internal/` and `pkg/` package has a `README.md` listing files, exports, and dependencies. **When adding or changing exports or dependencies, update the package README in the same commit.** This is the authoritative index for understanding a package without reading source.

### Metadata-Aware Mode

Rules that require live schema facts are metadata-gated: they are registered via the normal `Register(...)` path but no-op when no `TargetTable` snapshot or `InstanceFacts` is attached. Offline audits are always safe to run without a database connection.

## Execution Preferences

- For UI/design update requests in a specific node/screen, implement the requested change directly in the named target first. Do not switch into brainstorming, direction selection, or extra clarification unless a required constraint is truly missing.

## Documentation Workflow

- When asked to do documentation rewrites or audits, follow a docs/plans-first workflow: create the review/implementation plan documents first, then make content changes, and use an isolated worktree when the scope is large.

## Configuration

- Default policy: `internal/domain/policy/defaults.go` — `policy.Default()`.
- User config: YAML file, loaded via `internal/infrastructure/config/viper`. Example: `configs/deltascope.example.yaml`.
- Config is re-read per request in the HTTP server (no restart needed for policy changes).
- `deltascope config init` emits a default YAML template. `deltascope config lint` validates a config file.

## Public API

The stable library surface is `pkg/deltascope`. Do not add new public types to `internal/` packages and expose them directly; new public types belong in `pkg/deltascope`.

```go
result, err := deltascope.Audit(ctx, deltascope.Request{
    SQL:     "delete from users",
    Dialect: deltascope.DialectMySQL,
})
```

## HTTP Endpoints

- `GET /healthz`
- `GET /version`
- `POST /v1/audit` — same JSON shape as library `Result`

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **DeltaScope** (4858 symbols, 8891 relationships, 176 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

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
