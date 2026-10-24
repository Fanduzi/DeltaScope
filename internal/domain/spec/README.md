# Domain Spec Module

Normalized statement specifications used as the stable input for rule evaluation.

## Files

| File | Responsibility |
|------|---------------|
| statement.go | Defines the top-level normalized statement model and parser-neutral extraction interface, including the nonserialized `ResourceLimit` execution marker and its feature/reason constants |
| statement_test.go | Verifies typed statement metadata behavior |
| metadata.go | Defines optional schema context, instance facts, target-table snapshots, object-level validation snapshots, and lookup helpers for metadata-aware auditing |
| version.go | Defines the shared parser-neutral `VersionIdentity` fact (numeric components always serialize, including zero), strict `[v]MAJOR.MINOR.PATCH` target_version parsing plus the shared `ValidateTargetVersion` transport preflight, provider-banner observed canonicalization where the product is derived from the banner itself (including the TiDB compatibility prefix, never from the request dialect), the milestone-validated product/version series, and `RenameColumnVersionSupport`, `RenameColumnVersionSupportFor`, and `RenameColumnMinimumSupportedVersion` for the RENAME COLUMN 8.0.3 applicability check |
| version_test.go | Verifies target_version grammar acceptance/rejection, canonicalization, TiDB observed-banner resolution, product derivation independent of caller dialect, zero-component JSON serialization, `ValidateTargetVersion` preflight semantics, and validated-series classification |
| version_rename_column_test.go | Verifies RENAME COLUMN support, known MySQL incompatibility, and missing or out-of-series version gaps |
| ddl.go | Defines DDL-oriented specification types, including explicit DDL operations, richer column facts, typed index metadata (with the T10 `Visibility`/`VisibilityExplicit` effective-vs-authored pair and `MultiValuedParts` CAST-ARRAY member marks beside the existing part-kind counts), multi-target `Targets` plus `TableTargets()` fallback, create-table/object-lifecycle shape flags, declared-constraint alter payloads, and parsed-but-unmodeled option name evidence (`UnextractedOptions`) plus `OmittedTargets` for collapsed multi-object target lists for offline and metadata-aware DDL rules |
| advanced_indexes.go | Defines the T10 advanced-index assessment — `AssessAdvancedIndexes` is the one spec-owned answer shared by the invisibility/existence/compatibility rules, the ordered-state publication, and the coverage model: it derives the declared feature set (visibility, expression_keys, descending_keys, multi_valued, prefix_length, fulltext, spatial, global), evaluates each fact against the frozen per-product/version responsibility table (MySQL visibility ≥8.0.0, expression keys ≥8.0.13, multi-valued ≥8.0.17, MySQL <8.0 and TiDB descending silently stored ascending, primary-key invisibility refused on both anchors, member-legality checks 1170/1283/1687/1252/1054/3152/8200 that never mint version gaps), resolves the implicit primary-key role on qualifying UNIQUE NOT NULL indexes, and emits feature-scoped issues plus bounded version/fact gaps instead of fabricating compatibility from missing facts |
| primary_key_role.go | Shared implicit-primary-role and TiDB primary-object resolution (T10-R1): `ResolvePrimaryRole`/`ResolveDeclaredPrimaryRole` pick the first qualifying UNIQUE NOT NULL holder (unique kind, all members plain columns, proven NOT NULL, declaration order only on ordered snapshots — provider name-sorted sets emit the ordinal gap), and `ResolveIndexPrimaryObject`/`PrimaryKeyHandle` separate a real PRIMARY index object from a single-integer clustered PKIsHandle row-id promotion |
| advanced_columns.go | Defines the T07 advanced-column model — `GeneratedColumn`, `ExpressionFacts` (with `TopForm` outer-root classification), `ColumnReference`, visibility facts (internal, `json:"-"`) — plus the shared `ColumnDependents` dependency answer (ordered direct/transitive dependents with qualified-reference binding and traversal-limit unknowns), `CloneExpressionFacts`/`CloneGeneratedColumn` deep copies, `TiDBSetDefaultPublication`/`TiDBLiteralSetDefaultPublication` for the setter verbatim/temporal publication boundary, and `AssessAdvancedColumns`, the one spec-owned compatibility/dependency/state-gate assessment consumed by the DDL rules and the ordered-state publication — including the R1 declaration-time generated reference-premise check (complete CREATE declaration set; provable ALTER candidate states) and the TiDB expression-default shape adjudication (bare-call create grammar vs. evaluate-and-store setter semantics) |
| advanced_columns_t07_test.go | Verifies the T07 domain boundaries directly: PostgreSQL never activates the shared assessment (C10), unknown expression facts produce an unknown dependent answer while a reference-free column set stays known-empty (C08), transitive dependents emit in declaration order, reachable cycles and traversal/column-set bounds return unknown, and qualifier binding only proves self-qualified references |
| advanced_columns_t07_r1_test.go | Verifies the R1 premise edges unreachable through AuditSQL: unresolved (duplicated) candidate sets gap instead of guessing, same-statement ALTER binds stay bounded, unknown priors never confirm references, owning-table qualifiers bind while foreign qualifiers gap, and `CloneExpressionFacts` deep-copy isolation covers `TopForm` |
| dml_impact.go | Defines shared DML impact estimation enums and payload types reused across audit layers |
| dml.go | Defines DML-oriented specification types, including operation metadata, mentioned tables, MutationTargets, and MutationTargetTables() fallback to Tables |

## Exports

- `Statement`
- `AuditResourceLimit`
- `AuditResourceLimitFeature`, `AuditResourceLimitReason`
- `UnsupportedDetail`
- `Diagnostic`
- `DiagnosticParserError`
- `StatementExtractor`
- `Kind`
- `Dialect`
  Includes `DialectPostgreSQL` for PostgreSQL routing support
- `Metadata`
  Now carries `Objects []ObjectSnapshot` for non-table object validation and a `Version *VersionIdentity` fact for version-dependent rules
- `VersionIdentity`
- `ParseTargetVersion`, `ParseObservedVersion`, `ValidateTargetVersion`, `ErrInvalidTargetVersion`
- `RenameColumnVersionSupport`, `RenameColumnVersionSupportFor`, `RenameColumnMinimumSupportedVersion`
- `InstanceFacts` (explicit known-bit pairs: `InnoDBPageSizeKnown`/`InnoDBPageSizeBytes`, `InnoDBLargePrefixKnown`/`InnoDBLargePrefixOn`, `TiDBMaxIndexLengthKnown`/`TiDBMaxIndexLengthBytes` — a zero or absent value stays unknown rather than defaulting)
- `TableSnapshot`
- `ObjectSnapshot`
- `MetadataStatus`
- `DDL`
- `DDLOperation`
- `(*DDL).TableTargets()`
- `Table`
- `Column`
- `Constraint` (carries the referenced target/columns for foreign keys plus bounded `UnmodeledParts`/`UnmodeledReferencedParts`/`UnmodeledReferActions` counts for parsed key-part and ON DELETE/UPDATE/MATCH facts the model does not keep)
- `Index` (carries `Global` for the parsed GLOBAL index modifier, `HasPredicate` for partial-index WHERE clauses, member-level `KeyParts` with per-member ordinal/kind/column/prefix-length/direction/structured expression identity/multi-valued marks (aggregate `PrefixParts`/`DescParts`/`ExpressionCount`/`MultiValuedParts` derived via `SetKeyParts`), `PrimaryKeyType` for TiDB CLUSTERED|NONCLUSTERED, `UnmodeledKeyParts` for provider member rows that cannot classify as plain ascending parts, and `UnmodeledOptions` bounded names for other parsed IndexOption members)
- `IndexKind`
- `ImpactSource`
- `ImpactRisk`
- `ImpactConfidence`
- `PredicateShape`
- `ImpactEstimate`
- `AlterColumnChange`
- `AlterColumn`
- `AlterIndex`
- `Alter`
- `DML`
- `(*DML).MutationTargetTables()`
- `DMLOperation`

## Notes

- `Statement` may now carry optional metadata-aware context through `Metadata` and an additive `Unsupported` payload for recognized-but-unsupported statements so mixed PostgreSQL results can preserve supported statements while surfacing structured unsupported details.
- `UnsupportedDetail` carries the unsupported statement index, feature name, original SQL, and reason so CLI/API surfaces can render machine-readable partial-support outcomes.
- `Statement.ResourceLimit` (`json:"-"`) is a nonserialized execution marker stamped by the application ordered-state admission budget on retained statements beyond the limit. It is internal-domain only — never a public SDK field, request field, or serialized output — and evaluation projects it as ordinary `UnsupportedDetail` evidence with `Feature` `audit.resource_limit` and the fixed `ordered-state statement budget exhausted` reason.
- `DDL.UnextractedOptions` and `Alter.UnextractedOptions` name parsed statement/table-option clauses the extractor recognized but did not model, so coverage classification can flag an unaudited aspect (incomplete coverage) instead of silently passing. This is distinct from `coverage.status=unverified`, which is reserved for understood operations missing required metadata/version evidence.
- `DDL.TemporaryScope` records the parsed temporary-table scope (`"local"`/`"global"`) on create and drop table statements, and `DDL.OnCommitDelete` records the global temporary transaction-scope marker. No rule audits temporary identity or scope yet, so the fact exists purely to drive incomplete-coverage evidence.
- `Column.UnextractedOptions` names recognized-but-dropped column option clauses (`generated`, `reference`, `check`, `unique`, `fulltext`, `column_format`, `storage`, `secondary_engine_attribute`) so coverage classification flags them instead of silently passing; `Column.AutoRandom` carries the typed AUTO_RANDOM fact for the same reason.
- `Alter.HasColumnPosition` records a parsed `FIRST|AFTER` column-position clause, and `Alter.Position` (`*AlterPosition`) carries the normalized `{kind, after}` fact when the extractor resolves it (non-nil exactly when `HasColumnPosition` is true). A parenthesized multi-column `ADD (...) FIRST|AFTER` is not projected per column: it stays an unextracted spec-level `column_position` option on the first emitted alter instead.
- `AlterColumnChange.DeclaresPrimaryKey` (`json:"-"`) records that a MODIFY or CHANGE column definition itself writes `PRIMARY KEY`. It is parser-owned presence for state decisions, not a public result field and not an added-primary-key post-state. `NotNull` together with `TouchesNullability` does not encode that fact.
- `Alter.Constraint` carries the declared constraint payload for constraint-bearing alter specs (e.g. `ADD CONSTRAINT CHECK`) even when no index definition is produced, letting coverage classification see the constraint type.
- `IndexKind` gains `IndexKindSpatial`, `IndexKindVector`, and `IndexKindColumnar` so index forms outside audited semantics are modeled explicitly rather than dropped.
- `Diagnostic` carries structured evidence about unaudited or unsupported outcomes with stable `classification`, safe `reason`, generic `action_hint`, `audited=false`, and selected `dialect`. Optional `line` and `column` identify the 1-based start of a bounded parser-failed statement; optional `guidance_code` and `evidence_ref` classify documented parser boundaries. The no-leak contract prohibits raw SQL text, parser `near ...` fragments, routine bodies, and inferred object names.
- `Statement` may now carry optional metadata-aware context through `Metadata`:
  - `Schema` for request-level schema context even when no provider is attached
  - `Instance` for normalized server-level facts such as version and InnoDB defaults
  - `Version` for the canonical `VersionIdentity` fact (issue #83 T04-B): `product`/`version`/`major`/`minor`/`patch`/`source` (`target` or `observed`)/`validated_range`. Offline, an explicit `target_version` becomes the fact; online, the provider-observed identity is authoritative and a caller `target_version` may only constrain it. The raw provider banner stays internal and is never projected into public output. Validated series: MySQL 5.7.x/8.0.x/8.4.x, TiDB 8.5.x — syntactically valid versions outside that series keep `validated_range=false` so version-dependent rules emit bounded evidence gaps instead of guessing.
  - `TargetTable` for the current metadata-backed shape of the table being audited
  - `Objects` for non-table object validation snapshots (types, domains, extensions, publications, subscriptions, foreign objects, event triggers, rewrite rules, annotation targets)
- `TableSnapshot` includes convenience lookups for case-insensitive column/index existence checks so future rules do not need to duplicate iteration logic. It also carries the internal completeness markers `PrimaryKeyUnknown` / `IndexesUnknown` / `ConstraintsUnknown` (all `json:"-"`, never serialized): the ordered-state projections and partial provider snapshots use them to distinguish "collection not provided" from "collection loaded and confirmed empty", so member rules never interpret a withheld collection as confirmed absence (issue #84 T05-A1). `PrimaryKeyIsHandle` marks TiDB PKIsHandle tables whose logical PK is row-id storage with no operable PRIMARY index object; `IndexOrderCatalog` marks derived create-table snapshots whose index order is declaration order (provider snapshots are name-sorted and leave it false); `Partitioned *bool` records the PARTITIONS catalog answer with nil meaning unanswerable (issue #89 T10-R1).
- `ObjectSnapshot` carries metadata-validated state for non-table database objects with `Status` (confirmed/not_found/unavailable/ambiguous), `Exists`, `Schema`, `Type`, `Name`, safe `Attributes`, and optional `AmbiguousCandidates`.
- `MetadataStatus` identifies the outcome of a metadata object lookup: `confirmed` (object exists), `not_found` (object absent), `unavailable` (lookup not performed), `ambiguous` (identity not uniquely resolved).
- `Metadata.FindObject` and `Metadata.FindObjectsByType` provide case-insensitive lookup across attached object snapshots.
- `ObjectSnapshot.SafeAttributes` filters out sensitive attribute keys (password, secret, token, connection, body, definition, etc.) to prevent leaking secrets through metadata projection.
- `DML.Tables` preserves mentioned relations. `DML.MutationTargets` names the tables a statement writes. `MutationTargetTables()` returns MutationTargets when filled and otherwise falls back to Tables so denylist, metadata, and existence rules do not rediscover targets from AST nodes.
- `DDL.Targets` preserves every table-level object identity a statement names in source order: all `DROP TABLE`/`DROP VIEW`/`TRUNCATE` relations, every `RENAME TABLE` source and destination pair, and `ALTER TABLE ... RENAME TO` destinations after the altered subject. Entries keep their as-written qualifiers — an unqualified rename destination stays unqualified because it resolves to the current schema under MySQL/TiDB semantics, not the source table's schema. `DDL.Table` remains the primary/first target for compatibility. `TableTargets()` returns `Targets` when populated and otherwise falls back to `Table`, so the denylist checks every named target without order-dependent gaps; metadata enrichment still resolves the primary `DDL.Table` only.
- `DiagnosticParserError` is the named classification for a statement the dialect parser could not parse. Output adapters compare this constant instead of the string `"parser_error"`.
- `DML.HasReturning` records whether a real DML `RETURNING` clause was parsed. It is a structural parser fact projected from the AST (`len(stmt.Returning) > 0`), not a raw token scan, so an identifier or table alias named `returning` does not set it. It captures clause presence only; it does not carry returned column names, expressions, aliases, or any parser subtree.

- `Column` now carries offline-governance facts needed by column-focused DDL rules:
  - `Length`
  - `Charset`
  - `Collation`
  - `Unsigned`
  - `NotNull`
  - `AutoIncrement`
  - `HasDefault`
  - `DefaultValue`
  - `DefaultIsNull`
  - `DefaultIsCurrentTimestamp`
  - `OnUpdateCurrentTimestamp`
  - `DefaultLiteral` (internal, `json:"-"`)
- `Column.DefaultParenthesized` (`json:"-"`) records that the `DEFAULT` clause was authored as `DEFAULT (<expr>)` — syntax evidence only, never stored-value evidence. It is set on the parser supplement path for CREATE and ALTER `SET DEFAULT` sites and on the metadata path for catalog `default_generated` rows (catalog-stored expression defaults are always parenthesized), and it is what lets the assessment distinguish a grammar-legal parens wrapper from the same shape without parens.
- `Column` default facts are source-layered, not provenance claims: on the input-AST path `HasDefault` marks an explicit `DEFAULT` clause, literals keep their quoting in `DefaultValue`, and `DefaultIsNull` records the typed `DEFAULT NULL`; on the MySQL/TiDB metadata path `HasDefault` marks a non-NULL `COLUMN_DEFAULT`, `DefaultValue` is the stored representation verbatim, and `DefaultIsNull` is never set — a NULL catalog value cannot distinguish an omitted clause from an explicit `DEFAULT NULL`, and neither field may be read as recovering the original declaration (issue #85 T06-A4).
- `Column.DefaultLiteral` (`json:"-"`) is the T08-R1 recorded literal identity `LiteralDefaultFacts{Kind, Value}`: `Kind` names the value domain (`string`/`numeric`) and `Value` carries the decoded literal bytes of each source — declaration side the typed AST value, provider side the verbatim `COLUMN_DEFAULT` bytes. A recorded fact is not automatically a comparable pair: numeric values compare directly only when both sides carry canonical-integer text — catalog scale text (`1.00`) vs AST `1` stays unproven (T08-R2). `nil` is the honest unknown for non-literal classes and unprovable literal shapes (decimal/bit/hex/byte forms); `DefaultValue` stays display text and is never read as identity. `CloneLiteralDefaultFacts` deep-copies it alongside `CloneExpressionFacts` on every clone/publication seam (issue #87 T08-R1/R2).
- `ExpressionFacts.TopForm` classifies the paren-transparent outer root (`call`/`literal`/`column`/`arithmetic`/`unary`/`case`/`subquery`/`aggregate`/`other`) so assessments adjudicate the complete expression shape — a recognized function name inside `Functions` never stands in for the whole construct. `RootParenthesized` records the authored outermost node as a parens wrapper separately, distinguishing `(uuid())` from `uuid()` — the TiDB setter evaluates the first and stores the second verbatim. `CallCount` counts the function-call nodes so the single-bare-call boundary can be proven — a deduplicated `Functions` set cannot (`uuid(uuid())` is not `uuid()`); it proves nothing about evaluation, arguments, or results.
- `AssessAdvancedColumns` runs a declaration-time generated reference-premise check before the dependency-scope assessment: CREATE evaluates the complete declaration column set and ALTER evaluates the provable candidate set (prior snapshot plus same-statement declarations). Self-reference, references to absent names, and references to generated columns ordered later are determined issues (`generated_self_reference`/`generated_missing_column`/`generated_column_order`) — issues always set `InvalidTransition` so the publication gate tombstones the identity independently of policy switches. Ordinary-column references at any ordinal stay legal, qualifiers bind only to the owning table/schema, and same-statement binds, `FIRST|AFTER` positions, name-space-touching clauses, and unresolvable candidate sets stay bounded gaps.
- `TiDBSetDefaultPublication`/`TiDBLiteralSetDefaultPublication` mirror the setter-side verdict for the ordered-state seam: TiDB `SET DEFAULT` stores a verbatim expression for a single bare allowed call on any non-BLOB/TEXT/JSON target and a verbatim temporal default for `(current_timestamp)` on TIMESTAMP/DATETIME/DATE — every evaluated-shape cell (CASE roots, wrapped roots, arithmetic/unary/other, multi-call roots, non-temporal current_timestamp) is the `tidb_set_default_eval_unproven` boundary because the fold verdict is value-dependent; publication records the proven catalog shape, never a computed value, and unproven setters withhold the default shape through `UnresolvedTransition`.
- `DDL` also carries create-table shape flags for:
  - `CREATE TABLE ... LIKE`
  - `CREATE TABLE ... AS SELECT`
  - partitioned tables
- `DDL` has optional `ObjectName` / `ObjectType` for object lifecycle DDL (schema, sequence, materialized view, extension create/drop/alter). `ObjectType` is `"database"` for MySQL/TiDB database/schema statements and `"schema"` for PostgreSQL schema statements.
- `DDL.Operation` now distinguishes `create_table`, `create_view`, `alter_view`, `alter_table`, `drop_table`, `drop_index`, `drop_view`, `truncate_table`, `create_schema`, `drop_schema`, `create_sequence`, `alter_sequence`, `drop_sequence`, `create_materialized_view`, `drop_materialized_view`, `refresh_materialized_view`, `create_type`, `alter_type`, `drop_type`, `create_domain`, `alter_domain`, `drop_domain`, `create_extension`, `alter_extension`, `drop_extension`, `create_function`, `drop_function`, `create_procedure`, `drop_procedure`, `grant_table`, `revoke_table`, `create_event_trigger`, `alter_event_trigger`, `drop_event_trigger`, `create_rule`, `alter_rule`, `drop_rule`, `create_collation`, `alter_collation`, `drop_collation`, `create_statistics`, `alter_statistics`, and `drop_statistics` so lifecycle rules do not rely on structural guesswork.
- `DDL` preserves explicit naming-governance subjects directly on the normalized model:
  - `Table.Name` for table-level rules
  - `Column.Name` for column-level rules
  - `PrimaryKey.Name` plus `PrimaryKey.Kind`
  - `Indexes[].Name` plus `Indexes[].Kind` for unique, secondary, and fulltext index rules
  - `Indexes[].AccessMethod` for access method (defaults to `btree` when empty)
  - `Indexes[].IncludedColumns` for INCLUDE clause column names
  - `Indexes[].HasPredicate` for partial index predicate presence
  - `Indexes[].HasExpressionKeys` for expression index key presence
  - `Indexes[].ExpressionCount` for count of expression key entries
  - `PrimaryKey.Cardinality` plus `Indexes[].Cardinality` for additive metadata-aware selectivity hints, where `nil` means unknown and a present `0` remains distinguishable at the JSON boundary
  - `Constraints[].Name` plus `Constraints[].Type` for non-index constraints such as foreign keys and checks when extraction provides explicit names
- `DML` now preserves additive impact-estimation facts without changing existing rule inputs:
  - `PredicateShape` for parser-neutral predicate classification
  - `LookupColumns` for normalized lookup-column tracking
  - `MatchedKeyName` and `MatchedKeyKind` for the best matching index hint
  - `IsSingleTable` to distinguish single-table from join or multi-target mutations
  - `Impact` for the final conservative estimate payload with `estimated_rows`, `estimated_ratio`, `risk_level`, `confidence`, `source`, `reason_codes`, and optional notes
  - offline mode derives the initial estimate from SQL shape only
  - metadata-aware mode may refine that estimate with read-only table statistics without executing the DML
- `Alter` now has room for richer normalized payloads and may also carry standalone DDL action subjects, such as PostgreSQL `DROP INDEX`, when no table object exists:
  - `Name` is the canonical subject identifier:
    - existing-object actions use the pre-change name
    - pure-add actions use the created object name
    - table-option actions leave it empty
  - `Column` carries:
    - `OldName` for the existing source-side identifier when the statement names one
    - an optional target `Definition` reused from `Column`
    - rename intent is inferred from `OldName` plus `Definition.Name`, not a separate boolean
    - an optional `Change` block with statement-local relation facts only for semantics the statement explicitly spells out, such as nullability, default, and auto-increment
    - target type and unsigned shape still live on `Definition`, but are not separately labeled as touched change facts
  - `Index` (carries `Global` for the parsed GLOBAL index modifier, `HasPredicate` for partial-index WHERE clauses, member-level `KeyParts` with per-member ordinal/kind/column/prefix-length/direction/structured expression identity/multi-valued marks (aggregate `PrefixParts`/`DescParts`/`ExpressionCount`/`MultiValuedParts` derived via `SetKeyParts`), `PrimaryKeyType` for TiDB CLUSTERED|NONCLUSTERED, `UnmodeledKeyParts` for provider member rows that cannot classify as plain ascending parts, and `UnmodeledOptions` bounded names for other parsed IndexOption members) carries `OldName` plus an optional target `Definition` reused from `Index`
  - `Options` is intentionally a flat normalized subset of table options, not a full option AST or ordering-preserving model

## Dependencies
- Upstream: application extraction and domain rule evaluation
- Downstream: none inside the domain core

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
