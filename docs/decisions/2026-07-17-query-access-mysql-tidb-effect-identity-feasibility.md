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

That outcome is intentional, not a parser omission. Live Docker probes against
MySQL 8.4 and TiDB 8.5 (see Evidence below) establish that no
version-scoped, shadowing-safe builtin identity root is available on either
dialect among the investigated facilities. Neither a parser spelling, a schema
label, a determinism declaration, `mysql.func`,
`performance_schema.user_defined_functions`, `information_schema.KEYWORDS`,
`EXPLAIN` plan text, nor a shared parser implementation is sufficient to prove
the server will execute an audited builtin rather than user or plugin code.

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
- MySQL: `mysql:8.4` container, actual server version `8.4.10` (probed), port 3406
- TiDB: `pingcap/tidb:v8.5.0` container, actual version
  `8.0.11-TiDB-v8.5.0` (probed), port 4400

The live probe evidence below was captured by executable Go tests in
`internal/infrastructure/metadata/mysql/builtin_effect_identity_live_probes_test.go`
(build tag `integration`). The tests open real `*sql.Conn` connections to the
Docker services and assert server-returned evidence. They FAIL if the Docker
server returns materially different evidence. They skip (not fail) only when
the Docker service is confirmed unreachable; any other connection/query error
FAILS the test. The disposition tests execute real probes via shared helpers
and do NOT pass when their probes did not run.

Run command:

```
docker compose -f docker/cli-e2e-compose.yaml up -d --wait
go test -tags integration -count=1 -v \
  -run 'TestMySQL84|TestTiDB85|TestMySQLTiDB' \
  ./internal/infrastructure/metadata/mysql/
```

### MySQL 8.4 — DEFER

Evidence captured against the live MySQL 8.4.10 Docker service on a
caller-owned `*sql.Conn`:

| Probe | Live server-returned result | Test |
| --- | --- | --- |
| `SELECT VERSION()` | `8.4.10` | `TestMySQL84_LiveProbes_ServerVersion` |
| `CREATE FUNCTION my_sum(a INT, b INT) RETURNS INT DETERMINISTIC RETURN a + b` | Succeeds; `information_schema.ROUTINES.IS_DETERMINISTIC = YES` for `my_sum` (unrelated stored-function support; not builtin shadowing proof) | `TestMySQL84_LiveProbes_StoredFunctionDeterministic` |
| `CREATE FUNCTION count(a INT) RETURNS INT DETERMINISTIC RETURN a` (lowercase) | Rejected — syntax error 1064 (builtin name reserved) | `TestMySQL84_LiveProbes_BuiltinNameShadowingRejected` |
| `CREATE FUNCTION COUNT(a INT) RETURNS INT DETERMINISTIC RETURN a` (uppercase) | Rejected — same syntax error 1064 | `TestMySQL84_LiveProbes_BuiltinNameShadowingRejected` |
| `mysql.func` table | Exists; columns exactly `name, ret, dl, type`; row count 0 (asserted, not merely logged) | `TestMySQL84_LiveProbes_MysqlFuncScope` |
| `performance_schema.user_defined_functions` | 16 plugin UDFs (`innodb_redo_log_*`, `mysqlx_*`, `asynchronous_connection_failover_*`); NO builtins listed — the test fails if any builtin name appears | `TestMySQL84_LiveProbes_PerfSchemaUDFsScope` |
| `SELECT app.my_sum(1, 2)` | Returns `3` — schema-qualified stored function call succeeds | `TestMySQL84_LiveProbes_SchemaQualifiedStoredFunction` |
| `EXPLAIN SELECT COUNT(*) FROM users` | Plan columns have no `oid`/`impl`/`identity` column | `TestMySQL84_LiveProbes_ExplainRevealsNameNotIdentity` |
| `EXPLAIN ANALYZE SELECT COUNT(*) FROM users` | Plan text references count by name only; no `oid:` or `impl_class` field | `TestMySQL84_LiveProbes_ExplainRevealsNameNotIdentity` |
| `information_schema` builtin-identity catalog candidates (`FUNCTIONS`, `BUILTIN_FUNCTIONS`, `BUILTINS`, `PG_PROC`, `PG_BUILTIN`, `SYS_FUNCTIONS`) | None exist among the probed candidates — the test fails if any appears | `TestMySQL84_LiveProbes_NoBuiltinIdentityCatalog` |
| `information_schema.TABLES/COLUMNS` for `app.users` | Readable on a `*sql.Conn`; columns `id, name, created_at, updated_at` | `TestMySQL84_LiveProbes_MetadataReadable` |

DEFER rationale (per decision §Valid Outcomes and the task rule):

MySQL 8.4 supports unrelated stored functions, but the investigated builtin
name `count` is rejected for `CREATE FUNCTION`. The live evidence therefore
does **not** demonstrate that a supported builtin can be shadowed, so it does
not refute name-based resolution for the candidate under investigation. Live
probes also found no server-verifiable non-name identity root among the
investigated facilities, but the probe set is not exhaustive enough to
establish that name is the only possible root. The evidence is insufficient
for KILL and does not support GO, so MySQL is DEFER.

DEFER necessary conditions (all locked by live evidence via
`TestMySQL84_Disposition_DEFER`, which executes the probes and fails if Docker
is unreachable but the probes did not run):

1. Stored-function support is observed, but the investigated `count` name is
   rejected; supported-builtin shadowing is not demonstrated.
2. `mysql.func` lists only loadable UDFs (0 rows on the compose image,
   asserted).
3. `performance_schema.user_defined_functions` lists no investigated
   builtins.
4. `EXPLAIN` reveals only the function name, never an OID/implementation
   class.
5. No builtin identity catalog found among the probed candidates (NOT a
   universal proof; locks only the investigated candidates).

Supporting negative evidence (not a KILL necessary condition):

- `DETERMINISTIC` is a stored-function declaration, not a trust root.
- Builtin-name rejection is supporting evidence only, not an identity root.
- Schema-qualified stored-function calls prove qualification selects stored
  functions, but do not prove a supported builtin can be shadowed.

MySQL 8.4 is **DEFER**. `unknown_function_effect` is retained for all
function-bearing queries.

### TiDB 8.5 — DEFER

Evidence captured independently against the live TiDB v8.5.0 Docker service
(TiDB evidence was NOT inferred from MySQL):

| Probe | Live server-returned result | Test |
| --- | --- | --- |
| `SELECT VERSION()` | `8.0.11-TiDB-v8.5.0` | `TestTiDB85_LiveProbes_ServerVersion` |
| `CREATE FUNCTION my_sum(a INT, b INT) RETURNS INT DETERMINISTIC RETURN a + b` | Rejected entirely — syntax error 1064 (TiDB does not support stored functions) | `TestTiDB85_LiveProbes_StoredFunctionRejected` |
| `CREATE AGGREGATE FUNCTION foo RETURNS INT SONAME 'foo.so'` | Rejected — syntax error 1064 (no loadable UDF support) | `TestTiDB85_LiveProbes_LoadableUDFRejected` |
| `SELECT COUNT(*) FROM mysql.func` | Fails — table `mysql.func` does not exist (error 1146) | `TestTiDB85_LiveProbes_LoadableUDFRejected` |
| `information_schema.PLUGINS` row count | 0 (empty on the compose image) | `TestTiDB85_LiveProbes_PluginsEmpty` |
| `information_schema.KEYWORDS` | 654 words; reserved window names `ROW_NUMBER`/`RANK`/`DENSE_RANK` present (RESERVED=1); aggregate names `AVG`/`COUNT`/`SUM`/`MIN`/`MAX` absent (keyword catalog, not a function catalog); no `oid`/`impl`/`identity` column | `TestTiDB85_LiveProbes_KeywordsNotIdentityCatalog` |
| `EXPLAIN SELECT COUNT(*) FROM users` | Plan columns have no `oid`/`impl`/`identity` column | `TestTiDB85_LiveProbes_ExplainRevealsNameNotIdentity` |
| `EXPLAIN ANALYZE SELECT COUNT(*) FROM users` | Plan columns have no `oid`/`impl`/`identity` column | `TestTiDB85_LiveProbes_ExplainRevealsNameNotIdentity` |
| `information_schema` builtin-identity catalog candidates (`FUNCTIONS`, `BUILTIN_FUNCTIONS`, `BUILTINS`, `PG_PROC`, `PG_BUILTIN`, `SYS_FUNCTIONS`) | None exist among the probed candidates | `TestTiDB85_LiveProbes_NoBuiltinIdentityCatalog` |
| `information_schema.ROUTINES` row count | 0 (no stored-function catalog entries) | `TestTiDB85_LiveProbes_RoutinesEmpty` |
| `information_schema.TABLES/COLUMNS` for `app.users` | Readable on a `*sql.Conn`; columns `id, name, created_at, updated_at` | `TestTiDB85_LiveProbes_MetadataReadable` |

DEFER rationale (per decision §Valid Outcomes and the task rule "If live
probes plus version-scoped authoritative evidence cannot establish that the
only available root is a forbidden name-based model, downgrade that dialect
to DEFER"):

TiDB 8.5 has NO stored functions and NO loadable UDFs, so the name-based
trust model is **NOT refuted** by shadowing (unlike MySQL). Live probes found
no server-verifiable non-name identity root among the investigated facilities,
but the absence of shadowing means the evidence is **INSUFFICIENT** to
establish that "the only possible root is name-based" — a future, unprobed
facility could expose a non-name binding without contradicting the shadowing
evidence. Per the task rule, this insufficiency downgrades TiDB to DEFER
(not KILL), because no contradiction requires a permanent conclusion.

DEFER is not a commitment to ship promotion. `unknown_function_effect` is
retained for all function-bearing queries. The default SDK/CLI/HTTP/MCP
surfaces remain unchanged and fail-closed.

DEFER necessary conditions (all locked by live evidence via
`TestTiDB85_Disposition_DEFER`, which executes the probes and fails if Docker
is unreachable but the probes did not run):

1. No stored-function shadowing (CREATE FUNCTION rejected → name model NOT
   refuted → cannot establish "only possible root is name-based").
2. No loadable UDFs (CREATE AGGREGATE FUNCTION SONAME rejected).
3. `mysql.func` does not exist.
4. `information_schema.PLUGINS` is empty.
5. `information_schema.KEYWORDS` is a keyword catalog, not an identity
   catalog.
6. `EXPLAIN` reveals only the name, never an OID/implementation class.
7. No builtin identity catalog found among the probed candidates (NOT a
   universal proof; locks only the investigated candidates).

TiDB 8.5 is **DEFER**.

### Independent Proof Domains

MySQL and TiDB retain independent dispositions through different live evidence
paths:

- MySQL supports unrelated stored functions but rejects the investigated
  `count` name, so supported-builtin shadowing is not demonstrated → DEFER.
- TiDB has no stored functions, no loadable UDFs, and no investigated
  non-name identity root → DEFER because the evidence is insufficient to
  establish "only possible root is name-based".

The `TestMySQLTiDB_IndependentLiveEvidencePaths` test executes both probe
suites and compares server-returned observations; it does NOT compare
hardcoded string literals.

### Tasks 5–7 Disposition

Skipped. No dialect reached GO, so no caller-owned session foundation,
proof gateway, manifest, or public opt-in SDK E2E was implemented. The
default SDK, CLI, HTTP, and MCP surfaces remain unchanged and fail-closed.

### Test Evidence

- Task 1 characterization: `internal/application/queryaccess/builtin_effect_identity_feasibility_characterization_test.go`
- Task 2 traversal lock: `internal/application/queryaccess/builtin_effect_identity_feasibility_traversal_test.go`
- Task 3/4 live Docker probes: `internal/infrastructure/metadata/mysql/builtin_effect_identity_live_probes_test.go` (build tag `integration`)
- Task 3/4 static Phase-1 assumption (superseded by live probes, retained as documented reasoning): `internal/infrastructure/metadata/mysql/pure_effect_feasibility_test.go`, `internal/infrastructure/metadata/mysql/pure_effect_defer_test.go`
- New no-leak regression for the probe boundary:
  - SDK normal path: `pkg/deltascope/query_access_probe_boundary_no_leak_test.go`
  - CLI normal path: `internal/interfaces/cli/query_access_probe_boundary_no_leak_test.go`
  - HTTP normal path + error-boundary marker injection: `internal/interfaces/http/query_access_probe_boundary_no_leak_test.go`
- Corpus: MySQL fixtures pass `make query-access-corpus-gates`
- Existing static Phase-1 assumption tests: `pure_effect_feasibility_test.go`, `pure_effect_defer_test.go` (honestly labeled as static assumptions, superseded by live probes) still pass

### No-Leak Evidence (Probe Boundary)

The live Docker probes do NOT introduce a new public SDK/CLI/HTTP/MCP path.
The probe tests are test-only (`integration` build tag) and never reach any
public surface. The new no-leak regression tests exercise the UNCHANGED
public surfaces under MySQL/TiDB function-bearing SQL with injected markers.

Marker coverage by surface (honestly scoped):

- **SDK** (`pkg/deltascope/query_access_probe_boundary_no_leak_test.go`):
  covers the NORMAL path. Injects function-name (`my_secret_udf`) and literal
  (`SECRET_LITERAL`) markers via SQL. DSN/credential/driver-error markers are
  NOT injected through the SDK normal path because `AnalyzeQueryAccess` does
  not expose a live-connection error path; those markers are covered by the
  HTTP error-boundary test. The marker scan still includes all markers as a
  defense-in-depth check that no hardcoded DSN/credential/driver text appears
  in the SDK result.
- **CLI** (`internal/interfaces/cli/query_access_probe_boundary_no_leak_test.go`):
  covers the NORMAL path. Injects function-name and literal markers via SQL.
  DSN/credential/driver-error markers are NOT injected through the CLI normal
  path (same reason as SDK); covered by HTTP error-boundary test. Marker scan
  includes all markers as defense-in-depth. Raw SQL is scanned on BOTH stdout
  and stderr (no raw-SQL leak on any CLI surface).
- **HTTP** (`internal/interfaces/http/query_access_probe_boundary_no_leak_test.go`):
  covers the NORMAL path AND the error boundary. The error-boundary test
  (`TestHandlerQueryAccess_MySQLTiDBProbeBoundary_DriverErrorNoLeak`) is the
  ONLY test that injects DSN/credential/driver-error markers through a real
  error path (by replacing `analyzeQueryAccess` with an error carrying all
  markers). It asserts those markers, plus identity/candidate/manifest/session/
  severity/raw_sql/dsn/driver_error fields, are absent from the HTTP response
  body.

Every no-leak test asserts the injected markers, identity facts, candidates,
session/context data, manifest data, raw SQL, and `severity` are absent from
the public result/JSON/output. MCP has NO Query Access tool and was not
modified. The default SDK, CLI, HTTP, and MCP paths open NO live database
connection. PostgreSQL trusted SDK behavior is unchanged.

### Remaining Indeterminate Boundaries

All MySQL/TiDB function-bearing queries remain `indeterminate` with
`unknown_function_effect`. This includes but is not limited to:
`COUNT(*)`, `COUNT(column)`, `SUM/AVG/MIN/MAX(column)`, `ROW_NUMBER/RANK/DENSE_RANK`,
stored functions, UDFs, `DISTINCT`, aggregate-local ordering, named windows,
explicit frames, nested expression operands, and any schema-qualified
function call.

A future GO would require a separately designed, audited identity model that
does not exist in MySQL 8.4 or TiDB 8.5 as probed. For either DEFER dialect, a
future investigation could probe additional facilities (session modes,
type/coercion resolution, additional catalogs) to either establish a non-name
root (→ GO) or establish that the only possible root is name-based (→ KILL).
For MySQL, the investigated `count` name was rejected, so supported-builtin
shadowing was not demonstrated and no non-name root was found among the
investigated facilities.

### Status Rationale

This decision is `Proposed` (not `Accepted`) because the final independent
read-only audit (Oracle security/design review and Momus diff/acceptance
review) has not yet completed with no P1/P2 findings. Per the Acceptance
Rule, the decision becomes `Accepted` only after executable live evidence,
full audit, and no P1/P2 findings. The live Docker evidence above supports
the per-dialect DEFER (MySQL and TiDB) disposition exactly as defined by
the decision; the `Proposed` status reflects the pending audit, not a
weakness in the evidence.

Momus limitation: the Momus plan-critic agent is designed to review
`.omo/plans/*.md` files, not arbitrary diffs. It rejected the diff/acceptance
review request because no `.omo/plans/*.md` path was provided. Per the task
rule "If a reviewer tool is unavailable, report the limitation honestly and
do not claim its approval", Momus approval is NOT claimed. Oracle's
read-only security/design review was completed; its P1/P2 findings were fixed
(see the commit history for the fix diff).
