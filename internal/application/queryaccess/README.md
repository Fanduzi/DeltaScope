# Application Query Access Module

Application-level contracts for query access analysis, defining the schema resolver interface, request/result types, dialect-specific extraction adapters, and metadata-backed resolution.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the queryaccess application package boundary |
| contracts.go | Defines SchemaResolver interface, RelationSchema, ColumnSchema, QueryAccessRequest, and QueryAccessResult |
| extract_tidb.go | Bridges TiDB infrastructure query access facts to domain types with admission computation |
| extract_tidb_test.go | Verifies TiDB extraction bridging: classification, admission, CTE permissions, mode normalization, and column usages |
| extract_postgresql.go | Bridges PostgreSQL infrastructure query access facts to domain types with admission computation |
| extract_postgresql_stub.go | Returns ErrPostgreSQLNotAvailable when built without the `postgresql` tag |
| service.go | Orchestrates query access analysis: extraction by dialect, optional metadata resolution, requirement generation, sorting, and validation |
| resolve.go | Implements metadata-backed resolution: request-scoped caching, wildcard expansion, alias resolution, column disambiguation, view detection, and output lineage enrichment |
| resolve_test.go | Verifies resolution logic with a fake resolver: schema defaulting, cache deduplication, qualified/unqualified columns, missing metadata, cancellation, star expansion, views, CTEs, derived tables, aliases, output lineage |
| requirements.go | Generates access requirements based on mode: strict requires all columns, projection-only requires only output-contributing columns with inference risk warning |
| requirements_test.go | Verifies requirement generation: salary threshold, blacklist JOIN, GROUP/HAVING, ORDER BY, hashed output, subquery correlation, mode equality, stable warnings, invalid mode, unresolved references |
| service_test.go | Verifies service integration: offline mode, metadata mode, mode normalization, classification preservation, wildcard expansion |
| unproven_effect_reasons_postgresql_tag_test.go | Verifies bounded unproven-effect reason codes for PostgreSQL operator/function/cast presence, identity-failure mapping no-leak, mode freeze, and sort determinism |
| unproven_effect_mysql_tidb_regression_test.go | Guards MySQL/TiDB operator-bearing admissible cases against unproven_* reason regression |

## Exports

- `SchemaResolver`
- `RelationSchema`
- `ColumnSchema`
- `QueryAccessRequest`
- `QueryAccessResult`
- `EffectCandidate` (application-internal copy; untrusted; never public JSON)
- `EffectCandidateKind`
- `Service`
- `ExtractTiDBQueryAccess()`
- `AnalyzePostgreSQL()`
- `ResolveMetadata()` (testing)
- `BuildRequirements()` (testing)

## Notes

- `SchemaResolver` is an optional interface; callers may pass `nil` when schema metadata is unavailable.
- `QueryAccessResult` wraps the domain `Result` for application-layer consumption.
- `QueryAccessRequest.Mode` is a string that the domain layer normalizes via `NormalizeMode`.
- `ExtractTiDBQueryAccess` computes admission from read classification: read_only → admissible, not_read_only → rejected, indeterminate → indeterminate.
- `AnalyzePostgreSQL` follows the same admission computation pattern as TiDB.
- CTE relations are marked with `PermissionRequired: false`; base tables and derived tables require permission.
- `Service.Analyze` routes by dialect, applies optional metadata resolution, generates requirements based on mode, sorts output, and validates the result.
- PostgreSQL unproven-effect reason codes (`unproven_operator_effect`, `unproven_function_effect`, `unproven_cast_effect`) are presence-only machine identifiers emitted by the parser adapter; they explain indeterminate classification without embedding SQL, OIDs, or effect spellings.
- PostgreSQL `EffectCandidates` on `QueryAccessResult` are **internal-only and untrusted** (future catalog identity resolver input). They are not placed on `domain.Result` and must not appear in SDK/CLI/HTTP JSON. `QueryAccessRequest` has no candidate/trust injection fields.
- Identity-failure categories map only through `domain.ReasonForIdentityFailure`; free-text errors cannot be injected as trusted reasons. Effect-identity resolver / admission promotion remain out of scope for this layer until later tasks.
- Callers cannot supply `ReasonCodes` on `QueryAccessRequest`; transports passthrough the single application domain result.
- `buildRequirements` generates access requirements based on mode: strict requires all resolved columns, projection-only requires only output-contributing columns and emits inference_risk warning.
- Both modes require every permission-bearing relation (PermissionRequired: true).
- Required unresolved references produce indeterminate requirements.
- Resolution caches relation schemas per request (key: schema.name). CTEs and derived tables bypass resolution.
- Views are detected from metadata and marked as `RelationView` kind without definition expansion.
- Unqualified columns resolve only when exactly ONE source relation has the column.
- Wildcards expand in deterministic ordinal order when metadata is available.

## Dependencies
- Upstream: `internal/interfaces/*`
- Downstream: `internal/domain/queryaccess`, `internal/infrastructure/parser/tidb`, `internal/infrastructure/parser/postgresql`, `internal/infrastructure/metadata/mysql`, `internal/infrastructure/metadata/postgresql`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
