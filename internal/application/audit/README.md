# Application Audit Module

Application orchestration for parsing and, later, evaluating SQL audit requests.

## Files

| File | Responsibility |
|------|---------------|
| parse.go | Normalizes one leading UTF-8 BOM, parses bounded top-level statement slices independently by dialect, and translates successful locations from each slice so failed text cannot capture a valid sibling's source match |
| statement_boundary.go | Splits top-level SQL only at semicolons outside supported dialect strings, quoted identifiers, comments, and PostgreSQL dollar-quoted bodies (including Unicode identifier tags) while distinguishing dollar signs inside unquoted identifiers; it does not infer statement semantics |
| parse_pg.go | Implements PostgreSQL parsing when built with the `postgresql` tag |
| parse_pg_stub.go | Returns the PG-capable build guidance error when PostgreSQL support is not compiled in |
| parse_test.go | Verifies that application parsing hides parser-specific AST details |
| extract.go | Converts parsed statements into first-pass domain `Statement` values by invoking parser-neutral extractors and attaching shape-only DML impact facts |
| extract_test.go | Verifies representative DDL and DML extraction behavior, including create-like/create-as/partition flags plus enriched create-table facts, preserved backticked-keyword and unnamed-index names, extracted column charset/collation facts, normalized row-format and auto-increment-init options, explicit DDL lifecycle operations, MySQL/TiDB ALTER index/constraint action normalization with ALGORITHM/LOCK preservation, richer alter-table detail including explicit nullability and other statement-local change facts, multi-column add expansion, non-index constraint handling, mutation-target-only DML tables, and predicate-shape facts |
| corpus_helpers_test.go | Provides shared corpus metadata conversion and semantic assertions, including optional statement-level impact contracts |
| corpus_postgresql_tag_test.go | Runs PostgreSQL corpus fixtures, including the representative modern-shape pack and tagged partial parser-error checks, through the full audit pipeline and semantic extraction checks |
| corpus_test.go | Runs MySQL/TiDB corpus fixtures through the full audit pipeline and semantic extraction checks |
| corpus_testdata_test.go | Validates corpus fixture shape and supported expected-field enums, including impact expectations |
| corpus_inventory_test.go | Prints the supported-rule and dialect corpus inventory |
| corpus_coverage_test.go | Verifies every default rule has checked-in SQL corpus coverage for its supported dialects |
| cross_dialect_ddl_coverage_census_test.go | Classifies representative MySQL/TiDB DDL forms for coverage and generated catalog evidence |
| postgresql_ddl_coverage_census_postgresql_tag_test.go | Classifies representative PostgreSQL DDL forms and records which forms have corpus fixtures for catalog generation |
| postgresql_ddl_consolidated_census_postgresql_tag_test.go | Verifies the per-source and consolidated PostgreSQL DDL census totals |
| ddl_coverage_catalog_test.go | Generates and validates the checked-in DDL coverage catalog from the cross-dialect and PostgreSQL census sources |
| ddl_inventory_contract_test.go | Contract gate for the official DDL acceptance inventory (`testdata/ddl-inventory/inventory.yaml`): unique row IDs, required fields, version/source applicability, status classification, owner assignment to declared milestone tasks with real issue numbers, locked denominator integrity (`rows` == `required_row_ids` == `required_rows.txt` baseline + per-product minimums, with a simultaneous-shrink negative), and concrete evidence refs for `semantically_checked` rows |
| ddl_coverage_catalog_query_test.go | Verifies embedded and checked-in catalog parity plus catalog query behavior |
| impact_postgresql_tag_test.go | Verifies PostgreSQL offline primary-key equality and planner impact-source precedence |
| impact.go | Maps extracted DML predicate shapes to conservative offline impact estimates, populates statement `impact` objects with `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes`, upgrades shape-derived sources to metadata after enrichment, and refines the narrow primary-key-on-`id` case when metadata snapshots confirm `PRIMARY(id)` plus optional `table_rows` facts |
| impact_test.go | Verifies shape-only impact estimation plus post-enrichment metadata source upgrades, unique-equality refinement, and offline preservation behavior for representative UPDATE and DELETE shapes and their additive `impact` payloads |
| impact_optin_test.go | Verifies default UPDATE/DELETE audits keep the statement impact object without `dml.impact.*` findings, and that enabling those rules in config still emits findings from that object |
| coverage.go | Classifies per-statement coverage for MySQL/TiDB: parser-attached unsupported boundaries stay incomplete, recognized-but-unaudited aspects (ALTER sub-actions with no rule consumer, unextracted options, extracted-but-unconsumed options such as `placement_policy`, schema `charset`/`collate`, and sequence/placement `has_options` markers, temporary-table scope, `AUTO_RANDOM` and dropped column options, procedure `has_body`, the GLOBAL index modifier on column/table/ALTER/CREATE-INDEX paths (vendor boundary under MySQL), parsed-but-unmodeled index expression/prefix/descending key parts (explicit zero prefixes included), index WHERE predicates (MySQL vendor boundary / TiDB unaudited) and other unmodeled IndexOption members, collapsed multi-object statement targets including `ALTER USER USER()`/`CURRENT_USER()` current-user forms, account secondary-option families, foreign-key key-part/refer-action gaps, nested partition/sub-partition options plus SPLIT INDEX and UPDATE INDEXES markers, non-audited constraint/index kinds, parsed-but-unmodeled CREATE VIEW clauses (or_replace/view_columns/view_algorithm/definer/sql_security/check_option), GRANT/REVOKE clauses (column_privileges/routine_object — vendor boundary under TiDB / require_tls/with_grant), CREATE/DROP INDEX ALGORITHM=/LOCK= (lock_algorithm), CREATE PROCEDURE parameter lists, the CREATE SEQUENCE table-option tail, per-spec account auth presence (identified/dual_password on CREATE/ALTER USER including the `USER()` current-auth and current-dual-password paths), `or_replace` on CREATE PLACEMENT POLICY, CTAS duplicate-key handling (on_duplicate), DROP HYPO INDEX (hypo_index — vendor boundary under MySQL), inline PRIMARY KEY CLUSTERED|NONCLUSTERED (primary_key_type — vendor boundary under MySQL on column and index paths), and statement-level executable or server-wide markers (do/binlog/select_into including parenthesized set-operation lists/set_global)) become bounded `spec.UnsupportedDetail` evidence with stable feature IDs, and fully covered statements stay complete |
| coverage_t03_test.go | Golden-path regression for issue #82/T03: MySQL `CREATE SEQUENCE` + supported `ALTER TABLE ... ADD COLUMN` returns retained incomplete coverage, bounded unsupported evidence, the review floor, and unchanged parser-failure/mixed-batch/no-leak contracts; rework regressions lock unaudited ALTER actions, nested placement bindings, TiDB `DROP PROCEDURE`, execution-capable `EXPLAIN ANALYZE`/`TRACE`/`EXECUTE` vs read-only `EXPLAIN`/`PREPARE` exclusions, temporary-table scope (create/drop, local/global incl. the MySQL vendor boundary), `AUTO_RANDOM` create/alter column forms, MySQL procedure bodies, and dropped column options (generated/inline REFERENCES/CHECK/UNIQUE), the GLOBAL index modifier vendor-vs-unaudited split, parenthesized `ADD (...)` constraint expansion, index key-part facts (expr/prefix/desc unaudited on both dialects, explicit zero prefixes included), index WHERE predicates and unmodeled IndexOption members, `OmittedTargets` evidence on collapsed account/sequence lists including `ALTER USER USER()`/`CURRENT_USER()` current-user forms, account secondary-option families, foreign-key key-part/refer-action evidence, nested partition options plus SPLIT INDEX and UPDATE INDEXES markers, exact (index, feature, reason) paired-association assertions, exact bounded-metadata pins, and field-conditional statement markers (set_global on `SET GLOBAL`/`@@global`, select_into through parenthesized `SetOprSelectList` nesting, on_duplicate CTAS handling, hypo_index vendor split, inline primary_key_type, current-user dual_password) |
| evaluate.go | Applies registered rules to supported statements, retains recognized-but-unsupported statements as top-level `StatementResult` entries with `coverage.status=incomplete`, merges aspect-gap unsupported evidence, collects rule-declared `EvidenceGap` records from `EvidenceReporter` rules and lowers an otherwise-complete statement's coverage to `unverified`, enriches findings with explanation metadata, and aggregates statement/global findings into report output while preserving statement-level DML `impact` estimates |
| evidence_gap_t04_test.go | Golden-path regression for issue #83 T04-A: isolated `ddl.alter.modify_column.compatibility.require` with `requires_metadata` emits statement evidence gaps (`missing_source_column`/`incomplete_source_column`, bounded `required_facts`), unverified coverage, the review floor that never overrides a proven blocker, gap+unsupported and parser-priority coexistence, provider-error preservation, and the requires_metadata/required opt-in boundaries |
| evaluate_test.go | Verifies application-owned report-flow integration and explanation enrichment over the rule registry |
| explain.go | Joins evaluated findings with shipped catalog metadata and statement metadata availability notes |
| service.go | Normalizes one leading UTF-8 BOM before empty-input validation, then orchestrates policy loading, per-statement parser recovery, extraction, metadata enrichment, impact refinement, evaluation, preserved partial results, a review floor for otherwise-passing partial parser failures and structured unsupported statements, coverage marking on parser-failure return paths, and fail-closed diagnostics for parser-error and unsupported outcomes |
| mysql_tidb_ddl_normalized_silent_metadata_census_test.go | Censuses MySQL/TiDB normalized statements whose rule-relevant metadata is silently absent after extraction, including per-case unsupported expectations for recognized boundaries and per-case incomplete expectations for extracted-but-unaudited option aspects |
| service_basic_schema_postgresql_tag_test.go | Characterizes PostgreSQL schema-aware FK facts and mixed supported/unsupported statement results with retained incomplete-coverage statements |
| service_object_lifecycle_postgresql_tag_test.go | Verifies PostgreSQL object-lifecycle rules plus cross-dialect hygiene, tolerating vendor-boundary unsupported sentinels under MySQL |
| service_policy_dialect_postgresql_tag_test.go | Verifies PG-only rules stay silent on MySQL/TiDB statements, including ones carrying unaudited-aspect unsupported evidence |
| ddl_parser_error_unsupported_contract_postgresql_tag_test.go | Verifies parser-error and unsupported diagnostics stay distinct in mixed batches, retained statements keep indexes/locations, and reject stays ahead of the review floor |
| service_test.go | Verifies parser recovery, diagnostics, the partial-parser review floor, defaults/config overrides, metadata enrichment including MySQL/TiDB MODIFY nullability state, DML impact, schema plumbing, per-target denylist coverage for multi-target DROP/RENAME and ALTER rename destinations (including request-schema resolution for unqualified destinations and distinct dotted qualified identities), and existing unsupported contracts |
| service_multi_target_denylist_postgresql_tag_test.go | Verifies PostgreSQL multi-target DROP keeps distinct dotted qualified identities as separate denylist findings |
| service_unsupported_verdict_floor_postgresql_tag_test.go | Verifies the application unsupported-statement review floor for PostgreSQL `SELECT 1`, mixed audited siblings, unchanged `review`/`reject`, and metadata-aware inheritance |
| corpus_coverage_test.go | Verifies every non-deferred shipped rule has corpus coverage for its supported dialect targets, including MySQL/TiDB-only metadata rules |
| metadata.go | Defines the optional metadata-provider, index-owner resolver, plan estimator, and object-resolver seams, then attaches resolved target schema, instance, Mutation Target, and non-table object snapshots to statements before evaluation |
| dml_table_existence_test.go | Verifies MySQL/TiDB missing and existing DML target behavior, qualified-schema enrichment, joined mutation-target extraction, and metadata lookup error propagation |
| diagnostics.go | Defines diagnostic evidence constants (classification aliases `spec.DiagnosticParserError`, reason, action_hint, guidance codes, evidence refs) and helpers for constructing parser-error and unsupported statement diagnostics with optional guidance classification |
| ddl_coverage_catalog_query.go | Defines CatalogEntry, CatalogQuery, CatalogResult, LoadEmbeddedCatalog, LoadCatalogFile, LoadCatalog, QueryCatalog, and Validate for reading the generated (embedded) DDL coverage catalog and filtering it without invoking the audit engine |
| catalogdata/ddl-coverage-catalog.json | Generated catalog copy compiled into release binaries; kept byte-identical to docs/reference/ddl-coverage-catalog.json |

## Exports

- `Parse(sql string, dialect spec.Dialect)`
- `Extract(parsed ParsedSQL)`
- `EvaluateStatements(registry, statements)`
- `AuditSQL(ctx, request)`
- `Request`
- `MetadataRequest`
- `MetadataProvider`
- `IndexOwnerResolver`
- `PlanEstimator`
- `ObjectResolver`
- `Service`
- `NewService()`
- `Service.Audit(ctx, request)`
- `ParsedStatement`
- `ParsedSQL`
- `PostgreSQLCapabilityBoundaryError`
- `CatalogEntry`
- `CatalogQuery`
- `CatalogResult`
- `LoadEmbeddedCatalog() (string, []CatalogEntry, error)`
- `LoadCatalogFile(path string) (string, []CatalogEntry, error)`
- `LoadCatalog(path string) ([]CatalogEntry, error)`
- `QueryCatalog(entries []CatalogEntry, q CatalogQuery) CatalogResult`
- `CatalogQuery.Validate() error`

## Notes

- `report.Result` carries additive diagnostics. A parser failure affects only its bounded top-level statement: valid siblings continue through extraction, metadata, evaluation, findings, impact, and source-location attachment in original order. Each failed statement produces one `audited=false` parser diagnostic with optional 1-based `line`/`column`; an otherwise-`pass` partial result is floored to `review`, while existing `review`/`reject` results remain unchanged. The overall call still returns an error so process surfaces exit 2. Structured unsupported statements likewise floor an otherwise-`pass` result to `review` before returning `ErrUnsupportedStatement`; they stay in `Unsupported` and `Diagnostics` and are not counted as audited `Statements`. Diagnostics never contain raw SQL text, parser `near ...` fragments, or other forbidden payload.
- Statement-boundary recovery is lexical and deliberately narrow. It recognizes supported quote/comment forms needed to avoid false semicolon boundaries, then delegates every slice to the existing dialect parser. It does not add grammar fallbacks or guess semantics for failed text.

## Dependencies
- Upstream: future CLI and public audit entrypoints
- Downstream: `context`, `embed`, `fmt`, `internal/application`, `internal/application/policy`, `internal/domain/report`, `internal/domain/rule`, `internal/domain/rule/ddl`, `internal/domain/rule/dml`, `internal/domain/spec`, `internal/infrastructure/parser/postgresql`, `internal/infrastructure/parser/tidb`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
