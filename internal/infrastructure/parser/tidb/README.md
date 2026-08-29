# TiDB Parser Module

TiDB-backed parser adapter for multi-statement SQL parsing, parser-warning collection, and query access extraction.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses SQL text and preserves raw statement nodes plus parser warnings |
| extractor.go | Wraps TiDB AST nodes in parser-neutral extractors and performs TiDB-specific statement extraction including inline/table-level primary-key metadata and database/schema lifecycle DDL (CREATE/DROP DATABASE/SCHEMA normalized to create_schema/drop_schema with ObjectType="database") |
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
