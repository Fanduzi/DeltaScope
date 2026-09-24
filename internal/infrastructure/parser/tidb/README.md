# TiDB Parser Module

TiDB-backed parser adapter for multi-statement SQL parsing, parser-warning collection, and query access extraction.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses SQL text and preserves raw statement nodes plus parser warnings |
| extractor.go | Wraps TiDB AST nodes in parser-neutral extractors and performs TiDB-specific statement extraction including MutationTargets, mutation-target-only DML table facts, normalized ALTER index/constraint actions with declared-constraint payloads, inline/table-level primary-key metadata with implicit primary-key nullability, multi-target `DDL.Targets` preservation for DROP TABLE lists, RENAME TABLE source/destination pairs (destinations keep as-written qualifiers; unqualified resolves to the current schema downstream), and ALTER TABLE RENAME TO destinations, plus database/schema lifecycle DDL (CREATE/DROP DATABASE/SCHEMA normalized to create_schema/drop_schema with ObjectType="database"), parsed-but-unmodeled option names recorded on `UnextractedOptions` (including CREATE/ALTER DATABASE attributes such as placement_policy/encryption), and `has_options` markers on CREATE/ALTER SEQUENCE and CREATE/ALTER PLACEMENT POLICY only when an option list is present, temporary-table scope projection (`TemporaryScope`/`OnCommitDelete` on create and drop table), typed `Column.AutoRandom` plus `Column.UnextractedOptions` for recognized-but-dropped column options, and `has_body` on CREATE PROCEDURE only when `ProcedureBody` is present |
| coverage_boundary.go | Classifies parser-recognized statements that sit outside the supported vendor/semantic boundary — MySQL sequences and placement policies, TiDB CREATE/DROP PROCEDURE, TiDB resource groups, TiDB flashback/recovery/admin statements, execution-capable `EXPLAIN ANALYZE`/`EXPLAIN EXPLORE`/`TRACE`/`EXECUTE`, unhandled placement/range/layout operations, and any other parser-recognized mutating or administrative statement the extractor does not model — into `spec.UnsupportedDetail` evidence with stable feature IDs and bounded reasons instead of letting them pass silently; read-only `EXPLAIN`/`EXPLAIN FOR CONNECTION`, transaction control, and session-scope `PREPARE`/`DEALLOCATE` stay outside the audit surface |
| query_access.go | Extracts query access facts from TiDB AST: lexical scope system, relation/column/output extraction, read classification, and candidate handoff |
| query_access_effect_candidates.go | Defines bounded internal candidate facts, operand hints, and copy-safe candidate accumulation |
| query_access_effect_collector.go | Traverses query locations, scopes, subqueries, CTEs, derived tables, set operations, and LIMIT/OFFSET expressions |
| query_access_effect_builders.go | Builds function, aggregate, window, cast, and unsupported-traversal candidate facts |
| query_access_effect_candidates_test.go | Characterizes deterministic candidate closure and fail-closed AST boundaries |
| parser_test.go | Verifies multi-statement parsing, parse-failure behavior, and extractor-backed wrapping |
| query_access_test.go | Verifies query access extraction for SELECT, JOIN, CTE, subquery, wildcard, function, locking, DDL, and multi-statement forms |
| query_access_ast_census_test.go | Characterization matrix for TiDB parser AST fields and classification invariants |

## Exports

- `Parser`
- `Result`
- `ExtractedStatement`
- `New()`
- `WrapStatements()`
- `QueryAccessExtractor`
- `QueryAccessFacts`
- `EffectCandidate` (internal-only; untrusted; never public JSON)
- `RelationFact`
- `ColumnFact`
- `OutputFact`
- `UnresolvedFact`

## Dependencies
- Upstream: `internal/application/audit`, `internal/application/queryaccess`, `internal/domain/spec`
- Downstream: `github.com/pingcap/tidb/pkg/parser`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
