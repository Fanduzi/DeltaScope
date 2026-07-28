# Decision: Relationless Literal-Only Query Access

- Date: 2026-07-28
- Status: Accepted
- Baseline: `main@d584084`
- Related: [literal-only and reversed operands](2026-07-26-query-access-literal-only-and-reversed-operands.md)
- Spec: `docs/plans/2026-07-28-query-access-relationless-literal-selects-spec.md`
- Design: `docs/plans/2026-07-28-query-access-relationless-literal-selects-design.md`
- Implementation: `docs/plans/2026-07-28-query-access-relationless-literal-selects-implementation.md`

## Context

The existing MySQL/TiDB online manifest can prove a finite set of literal-only
function shapes only when a resolved physical base relation creates a table
requirement. Relationless forms such as `SELECT LOWER('x')` remain
indeterminate even though they do not read a database object.

That is a separate contract decision: an empty requirement set must not be
mistaken for authorization, but it can accurately express that this bounded
static analysis identified no table or column read.

## Proposed Decision

Permit relationless promotion only for existing exact literal-only manifest
entries on online MySQL/TiDB sessions. The result may be `read_only` and
`admissible` with no requirements, relations, or referenced columns.

The result is not an authorization decision and does not authorize a caller to
connect, execute SQL, or access data. It only states that the analyzed query,
under the selected bounded profile, identified no database object read.
Candidate-free relationless statements, including `SELECT 1`, are not part of
this decision and retain their existing analyzer behavior.

## Boundaries

- MySQL 5.7/8.0/8.4 and TiDB 8.5 online sessions only.
- Existing exact `[const]` and `[const,const]` manifest shapes only.
- Default SDK, CLI, HTTP, PostgreSQL, and MCP remain fail-closed.
- Any column, relation, parameter, expression, cast, nested call, UDF,
  unsupported modifier, noncanonical call, or non-manifest shape remains
  indeterminate.
- The existing result schema is retained. An empty SDK requirements slice may
  be omitted from JSON; no new authorization or permission-free flag is added.

## Alternatives

### Keep relationless queries indeterminate

Safest but unnecessarily rejects a closed set of database-object-free calls.

### Relax physical requirements globally

Rejected. It would change relation-bearing proof and risk allowing empty
requirements where a database read was unresolved.

### Add a general expression evaluator

Rejected. This would expand beyond manifest-backed static proof into a broad
SQL semantic engine.

## Acceptance Evidence

Acceptance is backed by real tests across every layer (parser → gateway →
service → SDK/CLI/HTTP → Docker live e2e), plus no-leak and no-exec proofs.

- Exact shape matching and rejection: parser
  `TestQueryAccessRelationlessLiteralAdmittedShapes` and
  `TestQueryAccessRelationlessRejectedShapes`
  (`internal/infrastructure/parser/tidb/query_access_relationless_literal_test.go`);
  predicate `TestRelationlessLiteralRequirementsComplete_AdmitsExactLiteralShape`
  and `..._FailClosedTable`, gateway
  `TestBuiltinSemanticGateway_RelationlessLiteralOnlyProven`,
  `..._RelationlessCountLiteralProven`, `..._RelationlessTwoConstProven`, and
  `..._RelationlessRejectsMalformedCandidates`
  (`internal/application/queryaccess/builtin_semantic_gateway_relationless_test.go`).
- No requirements / relations / referenced columns / unresolved on promotion:
  asserted in the gateway proven tests above, regression
  `TestBuiltinSemanticProfileRegression_RelationlessLiteralPositive`
  (`internal/application/queryaccess/builtin_semantic_profile_regression_test.go`),
  and SDK `TestMySQLTiDBQueryAccessSession_PromotesRelationlessLiteralShapes`
  (`pkg/deltascope/query_access_session_mysql_tidb_test.go`).
- Default offline, PostgreSQL, and cross-dialect regressions stay fail-closed:
  `TestBuiltinSemanticService_RelationlessDefaultServiceStaysIndeterminate`,
  `TestBuiltinSemanticGateway_RelationlessPostgreSQLNotProven`,
  `..._RelationlessEmptyProfileNotProven`,
  `..._RelationlessRejectsTiDBProfileOnMySQLDialect`, and
  `TestBuiltinSemanticProfileRegression_BoundariesStayIndeterminate`.
- Candidate-free no-change behavior (`SELECT 1`):
  `TestBuiltinSemanticService_RelationlessCandidateFreeSelectOneUnchanged` and
  parser `TestQueryAccessRelationlessCandidateFreeSelectOneUnchanged`.
- Relation-bearing proof unaffected:
  `TestBuiltinSemanticGateway_RelationBearingStillRequiresPhysicalRequirements`
  and `TestBuiltinSemanticService_RelationBearingMixedConstKeepsRequirements`.
- No-leak and user-SQL-never-executed:
  `TestMySQLTiDBQueryAccessSession_DoesNotExecuteUserSQL` (recording driver) and
  the `SECRET_LITERAL` scans in SDK, CLI
  (`TestQueryAccessOnline_MixedLiteralScalars`), and HTTP responses/access logs.
- Docker-backed SDK/CLI/HTTP coverage on all supported MySQL/TiDB profiles:
  `TestBuiltinSemanticService_RelationlessLiteralShapesAcrossProfiles`, live
  `TestLiveProfile_AssertsVersionAndAdmitsAggregates`
  (`pkg/deltascope/query_access_session_mysql_tidb_live_e2e_test.go`), and the
  integration `TestQueryAccessOnline_MixedLiteralScalars` suites in the CLI and
  HTTP interfaces across MySQL 5.7/8.0/8.4 and TiDB 8.5.
