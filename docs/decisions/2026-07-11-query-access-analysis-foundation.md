# Decision: Query Access Analysis Foundation

Date: 2026-07-11
Status: Accepted
Related milestone/version: Unassigned; versioning follows implementation evidence
Related commits:
- Task 1: query-access capability census and this decision record
- Planned: parser-neutral query facts and read classification
- Planned: relation, column-use, and output-lineage extraction
- Planned: metadata-backed resolution and access-requirement generation
- Planned: Go SDK, HTTP, and diagnostic CLI surfaces
Related tests:
- `internal/infrastructure/parser/tidb/query_access_ast_census_test.go`
- `internal/infrastructure/parser/postgresql/query_access_ast_census_postgresql_tag_test.go`
- Planned: read-classification table tests
- Planned: relation/column lineage fixtures
- Planned: strict and projection-only access-requirement tests
- Planned: SDK/HTTP/CLI parity and no-leak tests
Related docs:
- Planned: public query-access reference and integration recipe

## Context

DeltaScope's current public audit use case is centered on DDL and mutating DML.
It has no parser-neutral query model:

- `spec.Kind` contains `unknown`, `ddl`, and `dml`, but no query kind.
- `spec.DML.Tables` records mutation targets, not relations read by a query.
- `spec.DML.LookupColumns` records a bounded mutation-predicate fact, not general
  query column usage or output lineage.
- MySQL and TiDB `SELECT` statements parse through the TiDB parser but currently
  reach audit evaluation as `kind=unknown`, with no applicable statement rules.
- PostgreSQL `SELECT` statements are currently represented as an explicit
  unsupported boundary in the PostgreSQL audit adapter.
- The existing metadata provider loads instance facts and one audit target-table
  snapshot. It is not a multi-relation query resolver and does not represent
  principals or grants.

This means an audit `pass` for a MySQL/TiDB `SELECT` is not evidence that the
query is read-only, that its object references were resolved, or that a caller
is authorized to execute it. Query platforms need a separate machine contract
for those questions.

The target scenario is a database query platform that:

- accepts user-authored SQL;
- allows only demonstrably read-only queries;
- restricts access to tables and columns;
- may use a shared database account and therefore cannot delegate all
  application-level authorization to the database account;
- needs deterministic, explainable decisions before execution.

SQL syntax makes this more than a root-node check. A `SELECT` can acquire locks,
write a server-side file, assign session variables, contain data-modifying CTEs,
or call a function whose side effects are unknown. Column confidentiality also
cannot be enforced by checking only final column names: predicates, joins,
grouping, ordering, window expressions, and derived output expressions can all
consume sensitive source columns.

## Decision

Create a new Query Access Analysis use case. It is separate from the existing
DDL/DML audit rule engine and its `pass/review/reject` verdict.

The new use case has three responsibilities:

1. Classify whether every statement is demonstrably read-only.
2. Resolve the relations and source columns read by the query, including how
   each column is used and which source columns contribute to each output.
3. Produce deterministic access requirements under one of two column modes.

The foundation does not authenticate users and does not become an identity or
grant store. The calling platform owns authentication, effective-grant
retrieval, and the final user authorization decision. DeltaScope returns the
requirements that the platform must authorize. A later integration may accept
a caller-provided permission checker, but that is a separate contract and must
not change the analysis result.

### Separate query-access result

Do not reuse audit findings, audit `level`, or audit verdicts for access
analysis. Query access has its own machine concepts:

```text
read_classification: read_only | not_read_only | indeterminate
admission: admissible | rejected | indeterminate
mode: strict | projection_only
```

`admissible` means the SQL is structurally eligible to proceed to the caller's
authorization check. It is possible only when read classification is
`read_only` and all requirements relevant to the selected mode are resolved.
It does not mean that a user has permission. A parser error, unsupported
statement, unknown side-effect boundary, or required unresolved reference must
not become `admissible`.

### Read classification

Read classification is independent of column requirement mode.

`read_only` means DeltaScope can prove that the accepted subset has no
recognized write, lock, session mutation, file-output, or unknown-effect path.

`not_read_only` includes, at minimum:

- DDL, DML, DCL, and transaction/session mutation statements;
- locking reads such as `FOR UPDATE` and dialect equivalents;
- MySQL/TiDB `SELECT ... INTO OUTFILE` and `INTO DUMPFILE`;
- `SELECT ... INTO` forms that mutate session state or create database objects;
- PostgreSQL data-modifying CTEs;
- multi-statement input where any statement is not read-only.

`indeterminate` includes, at minimum:

- parse errors and parser-recognized forms outside the approved query subset;
- unresolved dynamic SQL;
- user-defined or unknown functions whose effect cannot be proven safe;
- query constructs whose relation or side-effect semantics cannot be derived
  conservatively from the parser AST and available metadata.

Unknown does not mean harmless. Strict admission treats `not_read_only` and
`indeterminate` as non-executable.

### Relation and column model

The analyzer always computes the richest safe structural result available,
regardless of requirement mode.

Relation references distinguish:

- base tables and views, which are permission-bearing objects;
- CTEs and derived tables, which are lexical query-scope objects;
- aliases, which must resolve back to their source relation;
- unresolved relation references, which carry bounded reason codes.

Column references record source lineage and one or more usage contexts:

```text
projection
filter
join
grouping
having
ordering
window
```

Output columns are separate from referenced columns. An output alias is not a
permission source. For example, `SHA2(users.ssn, 256) AS token` has output name
`token` but source column `users.ssn`.

Wildcard expansion and ambiguous unqualified columns require schema metadata.
When metadata is unavailable, the analyzer records an unresolved wildcard or
column rather than guessing.

### Column requirement modes

The public modes are:

```text
strict
projection_only
```

Strict mode is the default.

In both modes:

- every permission-bearing relation read by the query requires table/view read
  permission;
- read classification is identical;
- parser errors, unsafe statements, and unknown function effects are not
  relaxed;
- the analyzer still returns all discovered column usages.

In `strict` mode, every resolved source column used in projection, filtering,
joins, grouping, HAVING, ordering, or window expressions becomes a column-read
requirement. Any required unresolved column makes the decision indeterminate.

In `projection_only` mode, only source columns contributing to final output
expressions become column-read requirements. Columns used only by predicates,
joins, grouping, HAVING, ordering, or window expressions are reported but do
not become column requirements. The result must include a stable warning that
this mode permits inference through non-output columns and is not a complete
confidentiality boundary.

Projection-only mode still follows source lineage. It does not build
requirements from output aliases or output field names.

### Resolution rules

The foundation uses fail-closed resolution rules:

- unresolved permission-bearing relations prevent `admissible` in both modes;
- unresolved output lineage prevents `admissible` in both modes;
- unresolved non-output columns prevent `admissible` in strict mode;
- projection-only mode may exclude unresolved non-output columns from column
  requirements only when relation resolution and read classification are
  complete; the result still reports those unresolved references and the
  inference warning;
- `SELECT *` and `relation.*` require metadata expansion before authorization;
- ambiguous unqualified columns are never assigned to a relation by guesswork;
- deterministic ordering and deduplication are part of the machine contract.

### View boundary

The first version emits a read requirement for a referenced view as a
permission-bearing object. It does not promise expansion through the view
definition to all base tables.
If a deployment requires base-table lineage through views, it must provide a
future view-definition resolver and select an explicit expanded-lineage policy.
The result must identify the object as a view when metadata can prove that fact.

### Surface strategy

The foundation is machine-oriented. The first public surfaces are:

- Go SDK: stable request/result types and an analysis entrypoint;
- HTTP: a JSON analysis endpoint for query-platform integration;
- CLI: a JSON-first diagnostic command for local inspection and contract smoke
  tests.

MCP is intentionally deferred until there is a concrete agent workflow that
needs query-access analysis. Not every CLI or HTTP capability must be copied to
MCP. Any future MCP tool must consume the same application service rather than
implementing its own SQL analysis.

The existing audit SDK, CLI, HTTP, and MCP contracts remain independent. This
foundation must not reinterpret an audit `pass` as an access decision. If
sharing parser orchestration would change existing audit statement kinds,
unsupported behavior, verdicts, or JSON, stop and update this decision before
proceeding.

## Rationale

A separate use case prevents three categories of contract confusion:

- audit findings express SQL quality and operational risk, while query access
  expresses execution admissibility and privilege requirements;
- audit `level` values are not authorization decisions;
- an audit `pass` means no configured audit finding rejected the SQL, not that
  the SQL is safe to execute under a user's permissions.

Always extracting full lineage while applying modes only during requirement
generation preserves evidence. It avoids reparsing and lets a projection-only
caller see which filter/join columns were ignored for authorization.

Strict mode is the default because arbitrary predicates can reveal sensitive
facts without returning the protected value. Projection-only mode is retained
for platforms whose policy goal is display-field control and that consciously
accept inference risk.

The three-state read classification is required because a boolean would force
unknown parser and function-effect boundaries into either a false admissible
result or an overstated definitive rejection. `indeterminate` preserves that
uncertainty while still failing closed.

Table permission remains strict in both modes because a table used only in a
join, subquery, or predicate still contributes information to the result.

## Public Contract

After the foundation is accepted, consumers may rely on these principles:

- Query access is a separate API from SQL audit.
- Strict mode is the default.
- Projection-only mode relaxes only column requirements; it does not relax
  table requirements or read classification.
- Every analysis reports all discovered relation and column references, even
  when a mode does not turn all of them into requirements.
- Output lineage identifies source columns; aliases do not hide sources.
- `admissible` is never returned for `not_read_only` input.
- `admissible` means ready for caller-owned authorization, not user-authorized.
- Required unresolved references produce `indeterminate`, not `admissible`.
- Results use bounded reason codes and deterministic ordering.
- Results do not include `severity`; query access is not an audit finding.
- Results do not echo raw SQL, normalized SQL, literal values, comments,
  credentials, connection strings, or parser near-text by default.
- Relation and column identifiers are intentionally present because they are
  the core access-analysis output.

The exact Go type and HTTP field names remain Proposed until the Task 1 AST and
surface census confirms they can be implemented consistently. Renaming fields
before acceptance is allowed; changing the principles above requires updating
this decision.

## Deferred / Out Of Scope

- Authentication, login, token validation, session management, and identity
  storage.
- A built-in organization, role, or grant database.
- A final user authorization decision in the foundation API.
- Executing user SQL to discover behavior or lineage.
- Treating static analysis as the only production security boundary; database
  read-only credentials, read-only transactions, timeouts, and result limits
  remain defense in depth.
- Row-level security policy evaluation.
- Data masking or SQL rewriting.
- Statistical disclosure prevention, differential privacy, minimum-group-size
  enforcement, or query-rate controls.
- Automatic expansion through view definitions in the first version.
- Dynamic SQL analysis inside stored programs.
- A promise that every built-in or user-defined function is pure.
- MCP surface parity without a demonstrated consumer need.
- Reusing audit `level`, findings, rule IDs, or verdicts.
- A `severity` field.
- A release/version bump before implementation and evidence justify one.

## Verification Evidence

Task 1 characterization tests confirm the following.

### AST Fields Verified

**TiDB `*ast.SelectStmt`** exposes: `From`, `Fields`, `Where`, `GroupBy`,
`Having`, `OrderBy`, `WindowSpecs`, `LockInfo`, `SelectIntoOpt`, `With`,
`Limit`, `Distinct`, `AfterSetOperator`, `Kind`. Set operations produce
`*ast.SetOprStmt` with `SelectList` containing child `SelectStmt` nodes.

**PostgreSQL `*pg_query.SelectStmt`** exposes: `TargetList`, `FromClause`,
`WhereClause`, `GroupClause`, `HavingClause`, `SortClause`, `WindowClause`,
`LockingClause`, `IntoClause`, `WithClause`, `LimitCount`, `LimitOffset`,
`Op` (set operation type), `All`, `Larg`, `Rarg`, `ValuesLists`.

### Classification Matrix

| SQL Form | TiDB | PostgreSQL | Notes |
|---|---|---|---|
| Simple SELECT | approved | approved | FROM, Fields, Where accessible |
| Aliases | approved | approved | TableSource/RangeVar with alias |
| Schema-qualified | approved | approved | TableName.Schema/RangeVar.Schemaname |
| INNER/LEFT/RIGHT/CROSS JOIN | approved | approved | Join/JoinExpr with type and quals |
| FULL OUTER JOIN | indeterminate | approved | TiDB parser rejects; PG accepts |
| USING/NATURAL JOIN | approved | approved | Join.UsingClause/JoinExpr.isNatural |
| LATERAL join | N/A | approved | PG-specific; RangeSubselect.lateral=true |
| WHERE predicates | approved | approved | BinaryOperationExpr/AExpr |
| WHERE EXISTS subquery | approved | approved | SubqueryExpr/SubLink(EXISTS) |
| GROUP BY | approved | approved | GroupBy/GroupClause |
| HAVING | approved | approved | Having/HavingClause |
| ORDER BY | approved | approved | OrderBy/SortClause |
| Window functions | approved | approved | WindowFunc/WindowDef |
| Scalar subquery | approved | approved | SubqueryExpr/SubLink(EXPR) |
| Correlated subquery | approved | approved | Inner ref to outer column |
| Derived table | approved | approved | SubqueryExpr/RangeSubselect |
| Simple CTE | approved | approved | With/WithClause |
| Recursive CTE | approved | approved | With.IsRecursive/WithClause.recursive |
| Data-modifying CTE | N/A | not_read_only | PG-specific; CTE body is DELETE |
| UNION/INTERSECT/EXCEPT | approved | approved | SetOprStmt/SelectStmt.op |
| SELECT * | approved | approved | Wildcard; needs metadata expansion |
| Qualified wildcard | approved | approved | Table-qualified wildcard |
| FOR UPDATE | not_read_only | not_read_only | LockInfo/LockingClause |
| FOR SHARE | not_read_only | not_read_only | LockInfo/LockingClause |
| FOR NO KEY UPDATE | N/A | not_read_only | PG-specific locking |
| FOR KEY SHARE | N/A | not_read_only | PG-specific locking |
| SELECT INTO OUTFILE | not_read_only | N/A | TiDB-specific; writes file |
| SELECT INTO DUMPFILE | indeterminate | N/A | TiDB parser rejects |
| SELECT INTO @var | indeterminate | N/A | TiDB parser rejects |
| SELECT INTO (table) | N/A | not_read_only | PG-specific; creates table |
| generate_series | N/A | indeterminate | V1: function → indeterminate |
| NOW()/CONCAT() | indeterminate | indeterminate | V1: function → indeterminate |
| Unknown function | indeterminate | indeterminate | V1: unknown → indeterminate |
| Multi-statement SELECT | approved | approved | Per-statement classification |
| Mixed DML+SELECT | not_read_only | not_read_only | Any non-read-only → not_read_only |
| Parser errors | indeterminate | indeterminate | Fail-closed |
| EXPLAIN | approved | approved | Read-only plan analysis |
| EXPLAIN ANALYZE | N/A | not_read_only | PG-specific; executes query |
| VALUES | approved | approved | Table value constructor |
| DDL statements | not_read_only | not_read_only | KindDDL |
| DML statements | not_read_only | not_read_only | KindDML |

### Audit Regression Evidence

- TiDB SELECT reaches audit as `kind=unknown` with no applicable rules
  (confirmed by `TestQueryAccessClassificationConsistency`).
- PostgreSQL SELECT reaches audit as `kind=unknown` with unsupported boundary
  (confirmed by `TestPGQueryAccessClassificationConsistency`).
- Existing DDL/DML classification unchanged
  (confirmed by `TestQueryAccessAuditRegression` and `TestPGQueryAccessAuditRegression`).
- No existing `spec.Kind`, `spec.StatementExtractor`, or audit extraction code
  was modified.

### Build-Tag Behavior

- TiDB tests run without build tags (pure Go parser).
- PostgreSQL tests require `//go:build postgresql` tag.
- Non-PG builds return `PostgreSQLCapabilityBoundaryError` from `parsePostgreSQL()`.
- Audit regression tests pass under both tagged and untagged builds.

### Seam Approach Comparison

| Approach | Decision | Rationale |
|---|---|---|
| Add `query` to `spec.Kind` | **REJECTED** | Would change audit contract; risk of activating DML rules on SELECT statements |
| Extend audit parsed statements | **REJECTED** | Couples query access to audit pipeline; violates separation of concerns |
| Query-access-owned parser dispatch | **ACCEPTED** | Reuse parser adapters' raw `Parse()` entrypoints only; own the extraction in a new application boundary |

The accepted approach reuses `tidbparser.New().Parse()` and `pgparser.New().Parse()`
for raw parsing, but query-access extraction is owned by the new query-access
application layer, not by the audit extractor. This preserves the audit contract
completely while enabling independent query-access development.

## Consequences

- Query analysis becomes a new domain and application boundary, not another
  DML rule pack.
- Parser adapters will need bounded SELECT extraction in addition to current
  audit extraction.
- Metadata resolution must support multiple relations and column lookup rather
  than one mutation target table.
- Public SDK and HTTP schemas need compatibility tests from their first commit.
- Existing audit behavior requires explicit regression tests during any shared
  parser refactor.
- Security tests must prove fail-closed behavior and no raw-SQL/literal leak.
- Projection-only mode must always disclose its inference-risk warning.
- Documentation must describe DeltaScope as an analysis/admission component,
  not a replacement for database privileges and runtime controls.

## Links

- Commits:
  - Planned: Task 1 capability census and decision record
- Tests:
  - Planned: `testdata/query-access/` fixtures and cross-surface contract tests
- Docs:
  - Planned: query-access reference and query-platform integration recipe
- External references:
  - MySQL `SELECT ... INTO`: https://dev.mysql.com/doc/refman/8.0/en/select-into.html
  - PostgreSQL `SELECT`: https://www.postgresql.org/docs/current/sql-select.html
  - PostgreSQL privileges: https://www.postgresql.org/docs/current/ddl-priv.html
