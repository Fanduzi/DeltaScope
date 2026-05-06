# Domain Spec Module

Normalized statement specifications used as the stable input for rule evaluation.

## Files

| File | Responsibility |
|------|---------------|
| statement.go | Defines the top-level normalized statement model and parser-neutral extraction interface |
| statement_test.go | Verifies typed statement metadata behavior |
| metadata.go | Defines optional schema context, instance facts, target-table snapshots, and lookup helpers for metadata-aware auditing |
| ddl.go | Defines DDL-oriented specification types, including explicit DDL operations, richer column facts, typed index metadata, and create-table/object-lifecycle shape flags for offline and metadata-aware DDL rules |
| dml_impact.go | Defines shared DML impact estimation enums and payload types reused across audit layers |
| dml.go | Defines DML-oriented specification types, including operation metadata and extracted target tables for rule applicability |

## Exports

- `Statement`
- `UnsupportedDetail`
- `StatementExtractor`
- `Kind`
- `Dialect`
  Includes `DialectPostgreSQL` for PostgreSQL routing support
- `Metadata`
- `InstanceFacts`
- `TableSnapshot`
- `DDL`
- `DDLOperation`
- `Table`
- `Column`
- `Constraint`
- `Index`
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
- `DMLOperation`

## Notes

- `Statement` may now carry optional metadata-aware context through `Metadata` and an additive `Unsupported` payload for recognized-but-unsupported statements so mixed PostgreSQL results can preserve supported statements while surfacing structured unsupported details.
- `UnsupportedDetail` carries the unsupported statement index, feature name, original SQL, and reason so CLI/API surfaces can render machine-readable partial-support outcomes.
- `Statement` may now carry optional metadata-aware context through `Metadata`:
  - `Schema` for request-level schema context even when no provider is attached
  - `Instance` for normalized server-level facts such as version and InnoDB defaults
  - `TargetTable` for the current metadata-backed shape of the table being audited
- `TableSnapshot` includes convenience lookups for case-insensitive column/index existence checks so future rules do not need to duplicate iteration logic.
- `DML.Tables` preserves the parser-neutral set of mutation target tables so denylist and future metadata-aware DML rules do not need to rediscover them from AST nodes.

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
- `DDL` also carries create-table shape flags for:
  - `CREATE TABLE ... LIKE`
  - `CREATE TABLE ... AS SELECT`
  - partitioned tables
- `DDL` has optional `ObjectName` / `ObjectType` for object lifecycle DDL (schema, sequence, materialized view, extension create/drop/alter).
- `DDL.Operation` now distinguishes `create_table`, `create_view`, `alter_table`, `drop_table`, `drop_index`, `drop_view`, `truncate_table`, `create_schema`, `drop_schema`, `create_sequence`, `alter_sequence`, `drop_sequence`, `create_materialized_view`, `drop_materialized_view`, `refresh_materialized_view`, `create_type`, `alter_type`, `drop_type`, `create_domain`, `alter_domain`, `drop_domain`, `create_extension`, `alter_extension`, and `drop_extension` so lifecycle rules do not rely on structural guesswork.
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
  - `Index` carries `OldName` plus an optional target `Definition` reused from `Index`
  - `Options` is intentionally a flat normalized subset of table options, not a full option AST or ordering-preserving model

## Dependencies
- Upstream: application extraction and domain rule evaluation
- Downstream: none inside the domain core

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
