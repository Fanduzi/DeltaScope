# Application Query Access Module

Application-level contracts for query access analysis, defining the schema resolver interface, request/result types, dialect-specific extraction adapters, and metadata-backed resolution.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the queryaccess application package boundary |
| contracts.go | Defines SchemaResolver interface, RelationSchema, ColumnSchema, QueryAccessRequest, QueryAccessResult, and the shared MySQL/TiDB online default-schema binding contract |
| profile.go | Defines the closed analysis-profile values and dialect validation |
| builtin_semantic_manifest.go | Owns immutable MySQL/TiDB builtin semantic entries and session-only capability assembly |
| builtin_semantic_gateway.go | Proves exact candidate closure and strict physical requirement completeness |
| identity_resolver.go | EffectIdentityResolver facts-only contract, resolution context, identity batch helpers, bounded volatility/cast enums |
| phase1_effect_eligibility.go | Fail-closed Phase-1 pure-effect candidate eligibility before identity promotion |
| count_integer_one_proof.go | Narrow PostgreSQL COUNT(integer_one) single-table requirements proof predicate |
| count_integer_one_proof_test.go | Verifies exact COUNT(integer_one) proof boundaries and fail-closed requirements |
| corpus_test.go / corpus_pg_test.go / corpus_session_test.go | Own the offline Query Access corpus contract and fixture/session coverage; transport suites do not replace this semantic evidence |
| identity_resolver_test.go | Contract tests: ordinal uniqueness, status enum, fail-closed mapping, cancellation, no Trusted field |
| identity_resolver_context_test.go | Execution-context policy: unqualified unbound, shadowing, overload, TOCTOU, no public leak |
| identity_resolver_no_invoke_test.go | Freezes Analyze: no identity resolver invocation or public leak in T6 |
| trusted_service_postgresql_tag_test.go | Verifies trusted PostgreSQL Service.Analyze promotion and the foreign-table negative path, which must remain indeterminate |
| extract_tidb.go | Bridges TiDB infrastructure query access facts to domain types with admission computation |
| extract_tidb_test.go | Verifies TiDB extraction bridging: classification, admission, CTE permissions, mode normalization, and column usages |
| extract_postgresql.go | Bridges PostgreSQL infrastructure query access facts to domain types with admission computation |
| extract_postgresql_stub.go | Returns ErrPostgreSQLNotAvailable when built without the `postgresql` tag |
| service.go | Normalizes one leading UTF-8 BOM, then orchestrates query access analysis: extraction by dialect, optional metadata resolution, Promotion Barriers, requirement generation, one proof-orchestration point, final classification/admission computation, sorting, and validation |
| proof_orchestration.go | Single private proof-orchestration point: routes ordinary PostgreSQL, exact COUNT(1), MySQL/TiDB builtin, and no-effect applicability; owns proof-specific reason removal |
| proof_orchestration_contract_test.go | Locks the application proof-orchestration contract at the Service.Analyze seam for MySQL/TiDB builtin, no-effect, and barrier applicability |
| proof_orchestration_contract_postgresql_tag_test.go | Locks the PostgreSQL ordinary/exact COUNT(1) proof-orchestration contract, probe counts, and cancellation at the Service.Analyze seam |
| resolve.go | Implements metadata-backed resolution: request-scoped caching, wildcard expansion, alias resolution, column disambiguation, view detection, and output lineage enrichment |
| resolve_test.go | Verifies resolution logic with a fake resolver: schema defaulting, cache deduplication, qualified/unqualified columns, missing metadata, cancellation, star expansion, views, CTEs, derived tables, aliases, output lineage |
| requirements.go | Generates access requirements based on mode: strict requires all columns, projection-only requires only output-contributing columns with inference risk warning |
| requirements_test.go | Verifies requirement generation: salary threshold, blacklist JOIN, GROUP/HAVING, ORDER BY, hashed output, subquery correlation, mode equality, stable warnings, invalid mode, unresolved references |
| service_test.go | Verifies service integration: offline mode, metadata mode, mode normalization, classification preservation, wildcard expansion |
| final_state_invariant_postgresql_tag_test.go | Verifies final classification/admission invariants and fallback reason normalization across MySQL, TiDB, and PostgreSQL offline and metadata-resolved paths |
| unproven_effect_reasons_postgresql_tag_test.go | Verifies bounded unproven-effect reason codes for PostgreSQL operator/function/cast presence, identity-failure mapping no-leak, mode freeze, and sort determinism |
| unproven_effect_mysql_tidb_regression_test.go | Guards MySQL/TiDB operator-bearing admissible cases against unproven_* reason regression |

## Exports

- `SchemaResolver`
- `ResolveMySQLTiDBDefaultSchema()` / `ErrMySQLTiDBDefaultSchemaConflict`
- `ErrBOMOnlySQL`
- `RelationSchema`
- `ColumnSchema` (optional `TypeOID` fact; zero when unknown)
- `QueryAccessRequest`
- `AnalysisProfile`
- `ValidateAnalysisProfile()`
- `BuiltinSemanticManifest` / `BuiltinSemanticEntry`
- `BuiltinSemanticCallClass` / `BuiltinSemanticAggregate` / `BuiltinSemanticWindow` / `BuiltinSemanticScalar`
- `ErrBuiltinSemanticManifestInvalid`
- `NewBuiltinSemanticManifest()` / `NewMySQLTiDBSemanticService()`
- `QueryAccessResult`
- `EffectCandidate` (application-internal copy; untrusted; never public JSON)
- `EffectCandidateKind`
- `EffectIdentityResolver` (facts only; not wired into Analyze in T6)
- `ControlledEffectIdentityResolver` (T8: narrow contract for promotion; requires CaptureExecutionBoundContext)
- `EffectIdentityRequest` / `EffectIdentityBatch` / `EffectIdentityItem` / `EffectIdentityFacts`
- `EffectIdentityResolutionContext` / `EffectIdentityResolutionMode`
- `EffectVolatility` / `EffectCastMethod`
- `ValidateEffectIdentityRequest()` / `NormalizeEffectIdentityBatch()` / `CompleteEffectIdentityBatch()`
- `ValidatePhase1PureEffectCandidates()`
- `IsExactCountIntegerOneCandidate()`
- `ValidateCandidateFactBinding()` / `ValidateFactOperandTypeBinding()`
- `CandidateExplicitlyQualified()` / `CandidateExplicitPgCatalog()` / `ClassifyCandidateResolutionMode()`
- `ResolutionContextSessionComplete()` / `ResolutionContextUsableForUnqualified()`
- `ResolutionContextSessionCompatible()` / `ResolutionContextSearchPathCompatible()` / `ResolutionContextsCompatible()`
- `StampFactsFromResolution()`
- `GateIdentityBatchByResolutionContext()` / `GateIdentityBatchAgainstLiveContext()`
- `BuildUnavailableBatch()` / `MapCatalogErrorToStatus()` / `FailClosedReasonCodes()` / `BatchIsFullyResolved()`
- `Service`
- `NewService()` / `NewTrustedService()` (T8)
- `TrustPolicy` / `TrustDecision` / `TrustedEffectManifest` / `TrustedEffectEntry` (T8)
- `PG17Manifest` (T8)
- `ComputeManifestHash()` / `ValidateManifest()` / `MarshalManifestJSON()` / `UnmarshalManifestJSON()` (T8)
- `ExtractTiDBQueryAccess()`
- `AnalyzePostgreSQL()`
- `ResolveMetadata()` (testing)
- `BuildRequirements()` (testing)

## Notes

- The Query Access corpus owns offline semantic fixtures; the unified SDK owns online semantic breadth, with complete replacement evidence recorded in the milestone ledger.
- `ResolveMySQLTiDBDefaultSchema` supplies a named/CLI MySQL/TiDB connection schema when the online request omits `default_schema`, accepts an equal explicit value, and rejects conflicting values without changing PostgreSQL behavior.
- `QueryAccessResult` wraps the domain `Result` for application-layer consumption.
- The shared application input boundary removes exactly one leading UTF-8 BOM before Query Access parsing; BOM-only and BOM-plus-whitespace input is rejected as empty, while BOM-free empty-input result semantics remain unchanged.
- `QueryAccessRequest.Mode` is a string that the domain layer normalizes via `NormalizeMode`.
- `ExtractTiDBQueryAccess` computes admission from read classification: read_only → admissible, not_read_only → rejected, indeterminate → indeterminate.
- `AnalyzePostgreSQL` follows the same admission computation pattern as TiDB.
- CTE relations are marked with `PermissionRequired: false`; base tables and derived tables require permission.
- `Service.Analyze` routes by dialect, applies optional metadata resolution, applies Promotion Barriers, generates requirements before every Effect Proof, calls the single proof-orchestration point (`orchestratePromotionProof`, see `proof_orchestration.go`) for ordinary PostgreSQL, exact COUNT(1), MySQL/TiDB builtin, and no-effect applicability, then normalizes the final classification, admission, and reason-code contract once, sorts output, and validates the result.
- PostgreSQL unproven-effect reason codes (`unproven_operator_effect`, `unproven_function_effect`, `unproven_cast_effect`) are presence-only machine identifiers emitted by the parser adapter; they explain indeterminate classification without embedding SQL, OIDs, or effect spellings.
- PostgreSQL `EffectCandidates` on `QueryAccessResult` are **internal-only and untrusted** (future catalog identity resolver input). They are not placed on `domain.Result` and must not appear in SDK/CLI/HTTP JSON. `QueryAccessRequest` has no candidate/trust injection fields.
- **T6 EffectIdentityResolver** is an internal facts-only batch contract: per-ordinal `IdentityStatus` + optional OIDs/volatility/cast method/canonical signature. No `Trusted`, admission, reason text, or free-text status. Batch semantics: unique ordinals, deterministic sort, partial failure via status (not omission), cancel as batch-level `context` error. T6 does **not** call the resolver from `Service.Analyze`, does **not** implement pg_catalog SQL, and does **not** promote admission. Public SDK/CLI/HTTP request schemas intentionally omit the resolver field until a complete end-to-end path exists.
- **Operand-type binding (binary operators):** `ValidateFactOperandTypeBinding` cross-checks the atomic resolver's per-ordinal type map against returned fact `OperandTypeOIDs`. For resolved binary operator candidates, the map entry must exist, have exactly two nonzero OIDs, and equal the fact's operand OIDs. Nil, empty, missing, unexpected, malformed, or mismatched entries fail closed (`lookup_failed`). Functions, casts, and arity-zero candidates are untouched. This is defense-in-depth against contract-violating adapter output, not hostile in-process resolver protection.
- **Cast fields removed:** `CastSourceTypeName` and `CastTargetTypeName` were removed from `EffectIdentityFacts`. Phase 1 does not trust casts; cast candidates remain outside the manifest proof boundary.
- **T6 P1 execution resolution context:** `EffectIdentityRequest.Resolution` is an internal `EffectIdentityResolutionContext`. Phase-1 promotion-ready binding requires **all** of: `Bound`, non-empty `SessionBinding`, non-zero `PathEpoch`, `DatabaseOID`, `RoleOID`, `ServerVersionNum`. Unqualified also needs non-empty `NamespaceSearchOIDs`. Explicit schema may skip search_path ranking but **not** session/database/role/server checks. Resolved facts must be stamped (`StampFactsFromResolution`) with matching database/server pins. Live gate: session mismatch strips **all** candidates (including explicit); path-only mismatch strips unqualified only. Context never appears on `domain.Result` or public JSON.
- **T7 catalog adapter** lives in `internal/infrastructure/metadata/postgresql` (`PinnedSession` + `EffectIdentityAdapter`). It is **facts-only** and implements `ControlledEffectIdentityResolver`. `CaptureExecutionBoundContext` returns the pinned session's live resolution context so the application can set explicit Resolution on the request. Callers that use it must pin one session, run live→lookup→live+gate, and must not promote admission until T8 manifest proof.
- **T8 manifest proof** enables PostgreSQL admission promotion when all effect candidates are exactly proven by an audited manifest. `NewTrustedService()` accepts `ControlledEffectIdentityResolver` (not generic `EffectIdentityResolver`) so only controlled implementations can trigger promotion. The application captures execution-bound context explicitly via `CaptureExecutionBoundContext()` before resolution. `TrustPolicy` evaluates resolved facts against the versioned `PG17Manifest`. `TrustDecisionAllProven` is the sole path to `read_only + admissible` for PostgreSQL. The PG hard-stop in `reclassifyAfterResolution` is replaced with manifest-gated promotion. Without a trusted bundle, PostgreSQL remains fail-closed (indeterminate). Phase-1 provable queries: `SELECT count(*) FROM users` (arity-0 aggregate, no type inference). Operators with literals (`id = 1`) remain indeterminate (literal type unknown → coercion_gap).
- Identity-failure categories map only through `domain.ReasonForIdentityFailure` / `ReasonForIdentityStatus`; free-text errors cannot be injected as trusted reasons. Manifest trust policy and admission promotion remain T8.
- Callers cannot supply `ReasonCodes` on `QueryAccessRequest`; transports passthrough the single application domain result.
- `buildRequirements` generates access requirements based on mode: strict requires all resolved columns, projection-only requires only output-contributing columns and emits inference_risk warning.
- Both modes require every permission-bearing relation (PermissionRequired: true).
- Required unresolved references produce indeterminate requirements.
- Resolution caches relation schemas per request (key: schema.name). CTEs and derived tables bypass resolution.
- Views are detected from metadata and marked as `RelationView` kind without definition expansion.
- Unqualified columns resolve only when exactly ONE source relation has the column.
- Wildcards expand in deterministic ordinal order when metadata is available.
- **Unbound relation safety (PostgreSQL):** When `Service.Analyze` detects unqualified base relations in PostgreSQL with a trusted bundle, it marks those relations as `Unbound` and adds a bounded `unqualified_relation` indeterminate requirement. Unbound relations are excluded from the resolution state (`nameMap`, `aliasMap`, `relationOrder`) so the resolver never calls `DefaultSchema` on them. `resolveQualifiedColumn`, `expandTableStar`, and `expandTableWildcard` skip resolution when the relation is unbound and has no qualified entry in `nameMap`. `buildRequirements` skips columns with empty Schema when unbound relation names exist, preventing unresolved references from producing physical `read_column` requirements. `resolveSourceKeys` and `sourceIsUnbound` treat schema-qualified references (3-part keys with non-empty schema) as non-unbound, preserving requirements for qualified relations that share a table name with an unbound relation.
- The PostgreSQL parser resolves aliases to table names, so `SELECT p.id FROM public.users p JOIN users u` produces both columns with `Table: "users"`. The unbound check uses `resolveRelationRef` → `nameMap` to distinguish: if `nameMap` has a qualified entry, resolution proceeds; if not (all entries unbound), resolution is skipped.
- **Same-connection metadata resolver (T15):** `QueryAccessConnResolver` in `internal/infrastructure/metadata/postgresql` wraps a single `*sql.Conn` directly (no `*sql.DB` field). It satisfies `SchemaResolver` and ensures metadata queries run on the same backend as the identity adapter. The public SDK wrapper (`PostgreSQLQueryAccessSession` in `pkg/deltascope`) creates all resolvers from the same caller-owned `*sql.Conn`. The assembly helper `newTrustedServiceFromSession` lives in `pkg/deltascope` (postgresql-tagged) to avoid import cycles.
- MySQL/TiDB builtin semantic proof is independent from PostgreSQL catalog trust. Its production registry is populated for `mysql-5.7`, `mysql-8.0`, `mysql-8.4`, and `tidb-8.5`; only the explicit same-connection SDK session can construct the private capability. Default SDK/CLI/HTTP remain offline and fail-closed for function-bearing MySQL/TiDB queries. Test-owned manifests may also exercise the gateway without mutating the production registry.
- COUNT(integer_one) proof is narrower than generic physical requirements: only a single schema-qualified base table, one exact `read_table` requirement, no columns/unresolved references, and the parser's complete unqualified `COUNT(1)` statement envelope may enter the dedicated `pg_catalog.count(any)` proof path. Other literals, modifiers, joins, relationless queries, views, CTEs, derived tables, and unresolved relations remain indeterminate.

## Dependencies
- Upstream: `internal/interfaces/*`
- Downstream: `internal/application`, `internal/domain/queryaccess`, `internal/infrastructure/parser/tidb`, `internal/infrastructure/parser/postgresql`, `internal/infrastructure/metadata/mysql`, `internal/infrastructure/metadata/postgresql`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
