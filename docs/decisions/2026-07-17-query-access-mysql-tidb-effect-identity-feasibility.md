# Decision: Query Access MySQL/TiDB Builtin Effect Identity Feasibility

Date: 2026-07-17
Status: Proposed
Baseline: v0.400.0 (`e01d7e8`)
Branch: `query-access-mysql-tidb-effect-identity-feasibility`
Related decisions:
- `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
- `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- `docs/decisions/2026-07-16-query-access-common-pure-effects.md`
Related planning:
- `docs/plans/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility-spec.md`
- `docs/plans/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility-design.md`
- `docs/plans/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility-implementation.md`

## Context

v0.400.0 can prove a narrow set of PostgreSQL 17 common aggregates and ranking
windows through the opt-in, caller-owned-session SDK. The proof depends on
session-bound catalog facts and an application-owned PG17 manifest. MySQL and
TiDB still classify function-bearing queries such as `COUNT(*)`,
`SUM(amount)`, and `ROW_NUMBER() OVER (...)` as `indeterminate` with
`unknown_function_effect` on every current surface.

That outcome is intentional, not a parser omission. The current MySQL/TiDB
evidence shows that stored functions can be declared `DETERMINISTIC`, while no
version-scoped, shadowing-safe builtin identity root has been established.
Neither a parser spelling, a schema label, a determinism declaration, nor a
shared parser implementation is sufficient to prove that the server will
execute an audited builtin rather than user or plugin code.

## Decision

Start a feasibility-first milestone for MySQL 8.4 and TiDB 8.5. Each dialect
is an independent proof domain. The milestone must answer whether a bounded,
version-scoped, server-verifiable builtin identity model exists for a small
reporting subset:

- `COUNT(*)` and `COUNT(base_column)`;
- direct-base-column `SUM`, `AVG`, `MIN`, and `MAX`;
- `ROW_NUMBER`, `RANK`, and `DENSE_RANK` with direct physical
  partition/order sources.

No candidate becomes admissible merely because it is on this list. Promotion
for a dialect is allowed only when all of these are true:

1. The server supplies, or can be queried for, facts that uniquely bind the
   parsed candidate to the actual builtin under the supported version and
   session context. A resolver returns facts only, never `Trusted`.
2. The model rejects stored functions, UDFs/plugins, qualification changes,
   overload/coercion ambiguity, and relevant session/compatibility-state
   drift. A function spelling, `DETERMINISTIC`, a schema name, or a parser
   token is never the trust root.
3. Relation metadata, column types, effect identity, and the initial/final
   resolution context are captured on one caller-owned connection and a
   dialect-appropriate consistent-read boundary. No `*sql.DB` fallback may
   mix sessions.
4. Strict requirements are complete for every relation and column that can
   influence the admitted aggregate/window result. Unsupported AST, views,
   wildcard, unresolved metadata, derived/CTE input, casts, literals,
   parameters, modifiers, or a partial proof remain fail-closed.
5. Exact dialect/version facts match an application-owned audited manifest and
   the public result has no proof, connection, SQL, literal, credential, or
   `severity` leak.

The default SDK, CLI, HTTP, and MCP surfaces do not acquire database
connections and do not become more permissive. If live proof is required, it
must be exposed only through a separately designed opt-in caller-owned session
SDK for that dialect. MCP remains without a Query Access tool.

## Valid Outcomes

This is not a commitment to ship MySQL/TiDB promotion. Each dialect has one of
three valid outcomes:

| Outcome | Meaning |
| --- | --- |
| GO | A version-scoped identity and same-connection proof model passes all gates; a later task may implement only the proven subset. |
| DEFER | Evidence is insufficient, but no contradiction requires a permanent conclusion; preserve `unknown_function_effect` and document the gap. |
| KILL | The only possible root is name/determinism/schema/parser allowlisting, or required identity/dependency/context facts cannot be bound; do not implement promotion. |

A GO for MySQL does not imply a GO for TiDB, and vice versa. A DEFER/KILL is a
successful safety result, not a reason to broaden the allowlist.

## Public Contract If a Dialect Reaches GO

- Admission is opt-in and based on complete static requirements, not grants,
  authorization, RLS, masking, rewrite, or a guarantee about a later execution
  snapshot.
- Only exact audited candidate shapes and supported server versions may return
  `read_only` plus `admissible`.
- Default `AnalyzeQueryAccess`, CLI, HTTP, and MCP behavior stays unchanged.
- Resolver/manifest/session/type facts remain internal. Public errors and JSON
  use bounded information only.

## Deferred and Out of Scope

- Generic builtin name lists, determinism lists, or static parser allowlists.
- Arbitrary scalar functions, stored functions, UDFs/plugins, procedures,
  dynamic SQL, file/network/session-mutating functions, and user extensions.
- Casts/coercions, literals, parameters, nested expressions, `DISTINCT`,
  aggregate-local ordering, `FILTER`, named windows, explicit frames,
  windowed aggregates, ordered-set aggregates, views, wildcard, and
  derived/CTE effect inputs.
- Default-surface database connections, CLI/HTTP trusted promotion, an MCP
  Query Access tool, and audit-rule behavior changes.
- A claim of MySQL/TiDB feature parity with PostgreSQL.

## Evidence Required Before Acceptance

- Docker-backed MySQL 8.4 and TiDB 8.5 probes record exact server version,
  builtin/stored/UDF behavior, qualification/shadowing behavior, relevant
  session state, type facts, and negative cases.
- A per-dialect ledger records the proposed trust root or the DEFER/KILL
  evidence. It must not infer TiDB behavior from MySQL.
- Candidate collection and strict requirements are characterized across every
  relevant expression holder. Any incomplete traversal fails closed.
- If a live provider is proposed, public SDK E2E proves one caller-owned
  `*sql.Conn` supplies metadata, types, identity, and context; the caller can
  still use and close it after analysis.
- Manifest, adversarial, no-leak, corpus, default-surface non-regression, and
  independent read-only audit tests pass for every promoted dialect.

## Acceptance Rule

Keep this decision `Proposed` until the final audit reports no P1/P2 findings
and all claimed dialect/version evidence is executable. Update it to
`Accepted` only with the actual GO/DEFER/KILL disposition for MySQL and TiDB;
do not mark it accepted merely because planning or parser characterization was
completed.

## Evidence and Disposition

### Docker Probe Environment

- Compose: `docker compose -f docker/cli-e2e-compose.yaml up -d --wait`
- MySQL: `mysql:8.4` container, actual server version `8.4.10`, port 3406
- TiDB: `pingcap/tidb:v8.5.0` container, actual version
  `8.0.11-TiDB-v8.5.0`, port 4400

### MySQL 8.4 — KILL

Evidence captured against MySQL 8.4.10 on a caller-owned connection:

| Probe | Result |
| --- | --- |
| Candidate execution (COUNT/SUM/AVG/MIN/MAX/ROW_NUMBER/RANK/DENSE_RANK) | All execute correctly |
| `CREATE FUNCTION my_sum(a INT, b INT) RETURNS INT DETERMINISTIC` | Succeeds; `information_schema.ROUTINES.IS_DETERMINISTIC = YES` |
| `CREATE FUNCTION count(a INT) RETURNS BIGINT DETERMINISTIC` | Rejected (syntax error — builtin name reserved) |
| `CREATE FUNCTION COUNT(a INT)` (uppercase) | Rejected (same syntax error) |
| `mysql.func` table | Exists; columns `name, ret, dl, type`; lists loadable UDFs only |
| `performance_schema.user_defined_functions` | Lists plugin UDFs (`innodb_*`, `mysqlx_*`, etc.); does NOT list builtins like COUNT/SUM |
| `EXPLAIN SELECT COUNT(*) FROM users` | Plan text shows `Count rows in users`; no OID, no implementation class |
| `EXPLAIN ANALYZE` | Execution stats; no builtin identity |
| Schema-qualified stored function call (`app.my_sum(1, 2)`) | Succeeds — proving schema qualification can select stored functions |
| `information_schema.TABLES/COLUMNS` | Relation/column metadata available on same connection |
| Builtin OID-equivalent identity table | **Does not exist** — no `INFORMATION_SCHEMA` table provides OID/implementation-class identity for builtins |

Kill criterion met: the best available identity is the function name.
`DETERMINISTIC` is a stored-function declaration, not a trust root. No
OID-equivalent identity binding exists. A name-based allowlist is explicitly
forbidden by the decision. MySQL 8.4 is **KILL**.

### TiDB 8.5 — KILL

Evidence captured independently against TiDB v8.5.0:

| Probe | Result |
| --- | --- |
| Candidate execution | All execute correctly |
| `CREATE FUNCTION my_sum(...)` | Rejected entirely — TiDB does not support stored functions |
| `CREATE AGGREGATE FUNCTION ... SONAME` | Rejected — no loadable UDF support |
| `mysql.func` table | Does not exist |
| `information_schema.PLUGINS` | Returns 0 rows |
| `information_schema.KEYWORDS` | Lists reserved/non-reserved words only (ROW_NUMBER/RANK/DENSE_RANK reserved, AVG non-reserved); not a function identity catalog |
| `EXPLAIN SELECT COUNT(*) FROM users` | Plan text shows `funcs:count(1)`; no OID, no implementation class |
| `EXPLAIN ANALYZE` | Execution stats; no builtin identity |
| `information_schema.TABLES/COLUMNS` | Relation/column metadata available on same connection |
| Builtin OID-equivalent identity table | **Does not exist** |

Kill criterion met independently: the only available identity is the function
name. No stored-function/UDF/plugin system exists, but no OID-equivalent
identity binding exists either. A name-based allowlist is explicitly forbidden.
TiDB 8.5 is **KILL**.

### Independent Proof Domains

MySQL and TiDB reached KILL through different evidence paths:
- MySQL has stored functions (shadowing risk) but blocks builtin-name creation
- TiDB has no stored functions at all (different gap)

Neither dialect's evidence was inferred from the other. Both retain
`unknown_function_effect` for all function-bearing queries.

### Tasks 5–7 Disposition

Skipped. No dialect reached GO, so no caller-owned session foundation,
proof gateway, manifest, or public opt-in SDK E2E was implemented. The
default SDK, CLI, HTTP, and MCP surfaces remain unchanged and fail-closed.

### Test Evidence

- Task 1 characterization: `builtin_effect_identity_feasibility_characterization_test.go`
- Task 2 traversal lock: `builtin_effect_identity_feasibility_traversal_test.go`
- Task 3/4 ledger: `internal/infrastructure/metadata/mysql/builtin_effect_identity_feasibility_ledger_test.go`
- Corpus: 43 MySQL fixtures (9 new fail-closed matrix + 4 projection_only) pass `make query-access-corpus-gates`
- Existing defer evidence: `pure_effect_feasibility_test.go`, `pure_effect_defer_test.go` still pass

### Remaining Indeterminate Boundaries

All MySQL/TiDB function-bearing queries remain `indeterminate` with
`unknown_function_effect`. This includes but is not limited to:
`COUNT(*)`, `COUNT(column)`, `SUM/AVG/MIN/MAX(column)`, `ROW_NUMBER/RANK/DENSE_RANK`,
stored functions, UDFs, `DISTINCT`, aggregate-local ordering, named windows,
explicit frames, nested expression operands, and any schema-qualified
function call.

A future GO would require a separately designed, audited identity model that
does not exist in MySQL 8.4 or TiDB 8.5 as probed.
