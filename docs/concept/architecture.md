# Product Architecture

DeltaScope exposes one audit engine through four product surfaces: the `deltascope` CLI, the `deltascope-server` HTTP service, the `deltascope-mcp` MCP stdio service, and the `pkg/deltascope` Go library. They share the same rule evaluation path, the same severity model, and the same offline-first guarantee.

---

## Design Philosophy: One Path, Four Surfaces

The CLI, HTTP server, MCP server, and library are thin adapters over identical application logic. This has several practical consequences:

- A finding produced offline is identical to one produced in metadata-aware mode — same rule ID, same level, same message format.
- Findings are reproducible across surfaces: auditing the same SQL file via CLI and via the Go library produces the same result.
- No "online-only" rules exist. Every rule that uses metadata is safe to register in offline mode; it simply no-ops when no snapshot is attached. Offline audits never fail because a metadata-gated rule is registered.
- Policy config is evaluated identically regardless of surface. A `deltascope.yaml` file controls the same rule set whether consumed by the CLI or the embedded library.

---

## Shared Audit Flow

```text
SQL text / file / stdin / HTTP body
                |
                v
      +----------------------+
      | CLI / HTTP / Library |
      +----------------------+
                |
                v
      +----------------------+
      | Policy / Config Load |
      +----------------------+
                |
                v
      +----------------------+
      | Parser + Extractor   |
      | normalized statements|
      +----------------------+
                |
      +---------+----------+
      |                    |
      v                    v
+----------------+   +----------------------+
| Offline facts  |   | Optional metadata    |
| only           |   | enrichment           |
+----------------+   | instance + schema    |
      |              +----------------------+
      +---------+----------+
                |
                v
      +----------------------+
      | Rule Evaluation      |
      | blocker/warning/...  |
      +----------------------+
                |
                v
      +----------------------+
      | Verdict + Findings   |
      +----------------------+
                |
      +---------+---------+----------------+
      |                   |                |
      v                   v                v
  CLI markdown        CLI JSON        HTTP / library
  or exit code        for agents      structured result
```

---

## Audit Pipeline Stages

### 1. Parse

SQL text is passed to the TiDB parser adapter (`internal/infrastructure/parser/tidb`), which produces a typed AST. The TiDB parser is used for both MySQL and TiDB dialects because it is a strict superset of MySQL syntax. Dialect-specific behavior is controlled by parser mode flags set based on the active dialect.

### 2. Extract

The AST is traversed and normalized into `[]spec.Statement` — a parser-neutral model that decouples downstream rule evaluation from parser internals. Each `spec.Statement` carries:

- The statement type (DDL or DML) and subtype (CREATE TABLE, ALTER TABLE, DELETE, etc.)
- Extracted structural details (columns, indexes, clauses, affected tables)
- The original raw SQL string

### 3. Enrich (optional)

When a `MetadataProvider` is supplied (i.e., metadata-aware mode is active), the enrichment stage attaches live facts to each statement:

- `spec.InstanceFacts` is attached to every statement: MySQL/TiDB version, default charset, InnoDB configuration variables.
- `spec.TableSnapshot` is attached per target table: the current column definitions, index definitions, primary key state, and table options loaded from `information_schema`.

Statements for which no snapshot can be resolved (e.g., tables that do not yet exist) are enriched with what is available. Rules handle missing snapshots gracefully.

### 4. Evaluate

The rule registry applies registered rules to the enriched statements in deterministic order. There are two evaluation scopes:

- **Statement-scoped rules** are applied to each `spec.Statement` independently. Most rules are statement-scoped.
- **Global rules** are applied once with access to all statements in the batch. They detect patterns that span multiple statements, such as multiple ALTER TABLE operations targeting the same table.

The registry runs all statement-scoped rules for each statement before running global rules.

### 5. Report

Findings from all rules are aggregated into a `report.Result`. The result includes:

- **Verdict**: `pass`, `review`, or `reject` (see [Core Concepts — Findings and Verdicts](./core-concepts.md#findings-and-verdicts))
- **Summary**: counts of findings by level
- **Per-statement findings**: findings grouped by statement index
- **Global findings**: findings produced by global rules

The `report.Result` is then rendered by the active output adapter (CLI markdown, CLI JSON, or HTTP/library structured result).

---

## Layer Boundaries

```
cmd/deltascope | cmd/deltascope-server        ← process entrypoints (flag binding only)
internal/interfaces/cli | http                ← transport adapters (request/response translation)
internal/application/audit | policy           ← orchestration: parse → extract → enrich → evaluate
internal/domain/spec | rule | policy | report ← core domain: normalized types and rule semantics
internal/infrastructure/parser | config | metadata | output  ← external adapters
pkg/deltascope                                ← stable public API facade
```

**Key constraint**: domain packages (`internal/domain/...`) must not import infrastructure packages (`internal/infrastructure/...`). Dependencies flow inward only. Infrastructure adapts external dependencies to domain interfaces; it never defines domain behavior.

**Public API boundary**: `pkg/deltascope` is the stable public API facade. New public types belong in `pkg/deltascope`, not in `internal/` packages. The internal packages are free to evolve without breaking the public API.

---

## Severity Model

All four product surfaces use the same three severity levels:

| Level | Meaning |
|---|---|
| `blocker` | The SQL must not be applied as-is. The issue is a policy violation or a pattern with high risk of data loss, outage, or corruption. |
| `warning` | The SQL should be reviewed before applying. The issue is a policy concern, a risky pattern, or something worth explicit sign-off. |
| `notice` | Informational. The issue is worth knowing but does not require action before applying the SQL. |

The severity of a finding is determined by the rule's default level, which can be overridden per-rule in the policy config. This means the same check can be a `blocker` in a strict production environment and a `notice` in a development environment, using only config — no code changes.
