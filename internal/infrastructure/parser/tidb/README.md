# TiDB Parser Module

TiDB-backed parser adapter for multi-statement SQL parsing, parser-warning collection, and query access extraction.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses SQL text and preserves raw statement nodes plus parser warnings |
| extractor.go | Wraps TiDB AST nodes in parser-neutral extractors and performs TiDB-specific statement extraction including MutationTargets, mutation-target-only DML table facts, normalized ALTER index/constraint actions with declared-constraint payloads, inline/table-level primary-key metadata with implicit primary-key nullability, multi-target `DDL.Targets` preservation for DROP TABLE lists, RENAME TABLE source/destination pairs (destinations keep as-written qualifiers; unqualified resolves to the current schema downstream), and ALTER TABLE RENAME TO destinations, plus database/schema lifecycle DDL (CREATE/DROP DATABASE/SCHEMA normalized to create_schema/drop_schema with ObjectType="database"), parsed-but-unmodeled option names recorded on `UnextractedOptions` (including CREATE/ALTER DATABASE attributes such as placement_policy/encryption), and `has_options` markers on CREATE/ALTER SEQUENCE and CREATE/ALTER PLACEMENT POLICY only when an option list is present, temporary-table scope projection (`TemporaryScope`/`OnCommitDelete` on create and drop table), typed `Column.AutoRandom` plus `Column.UnextractedOptions` for recognized-but-dropped column options, and `has_body` on CREATE PROCEDURE only when `ProcedureBody` is present, and `Index.Global` projection for the parsed GLOBAL index modifier on table/ALTER constraints and standalone CREATE INDEX plus bounded column-option names `unique_global`/`primary_key_global`, and expansion of parenthesized `ALTER ... ADD (...)` `NewConstraints` into the same normalized alter actions as standalone ADD clauses (index kinds -> add_index, PK/FK/CHECK -> add_constraint; synthetic actions derive from constraint type only), bounded index key-part counts (expression/prefix/descending, with explicit zero-length prefixes counted via `Length != UnspecifiedLength`) on all index definitions, `OmittedTargets` for collapsed DROP USER/ROLE/SEQUENCE, CREATE/ALTER USER, and GRANT/REVOKE target lists including the `ALTER USER USER()`/`CURRENT_USER()` current-user alternatives, normalized foreign-key reference targets plus bounded `Constraint.Unmodeled*` counts for key-part and refer-action facts, bounded account-option family names on CREATE/ALTER USER, nested partition/sub-partition option names plus the SPLIT INDEX collection surfaced through `UnextractedOptions`, and `Index.HasPredicate`/`UnmodeledOptions` projection of IndexOption members across all index construction sites, parsed-but-unmodeled clause names on `UnextractedOptions` for CREATE VIEW (or_replace/view_columns/view_algorithm/definer/sql_security/check_option — only forms that deviate from parser-filled defaults), GRANT/REVOKE (column_privileges/routine_object/require_tls/with_grant), CREATE/DROP INDEX statement-level ALGORITHM=/LOCK= (lock_algorithm), CREATE PROCEDURE parameter lists (params), the shared table-option tail on CREATE SEQUENCE, per-spec account auth presence (identified/dual_password on CREATE/ALTER USER spec lists including the USER() current-auth and current-dual-password paths), `or_replace` on CREATE PLACEMENT POLICY, CTAS duplicate-key handling (on_duplicate for IGNORE|REPLACE SELECT), DROP HYPO INDEX (hypo_index), and inline PRIMARY KEY CLUSTERED|NONCLUSTERED (primary_key_type mirroring the index path) |
| coverage_boundary.go | Classifies parser-recognized statements that sit outside the supported vendor/semantic boundary — MySQL sequences and placement policies, TiDB CREATE/DROP PROCEDURE, TiDB resource groups, TiDB flashback/recovery/admin statements, execution-capable `EXPLAIN ANALYZE`/`EXPLAIN EXPLORE`/`TRACE`/`EXECUTE`, unhandled placement/range/layout operations, executable `DO`/`BINLOG` statements, `SELECT|UNION ... INTO` server-file forms (select_into marker on the statement carrying the clause, including parenthesized SetOprSelectList nesting), server-wide `SET GLOBAL`/`@@global` assignments (session-scope SET stays exempt), and any other parser-recognized mutating or administrative statement the extractor does not model — into `spec.UnsupportedDetail` evidence with stable feature IDs and bounded reasons instead of letting them pass silently; read-only `EXPLAIN`/`EXPLAIN FOR CONNECTION`, transaction control, session-scope `PREPARE`/`DEALLOCATE`, plain `SELECT`/set operations, `USE`/session-scope `SET`/session-state, and `SHOW`/`HELP` stay outside the audit surface |
| query_access.go | Extracts query access facts from TiDB AST: lexical scope system, relation/column/output extraction, read classification, and candidate handoff |
| query_access_effect_candidates.go | Defines bounded internal candidate facts, operand hints, and copy-safe candidate accumulation |
| query_access_effect_collector.go | Traverses query locations, scopes, subqueries, CTEs, derived tables, set operations, and LIMIT/OFFSET expressions |
| query_access_effect_builders.go | Builds function, aggregate, window, cast, and unsupported-traversal candidate facts |
| query_access_effect_candidates_test.go | Characterizes deterministic candidate closure and fail-closed AST boundaries |
| parser_test.go | Verifies multi-statement parsing, parse-failure behavior, and extractor-backed wrapping |
| query_access_test.go | Verifies query access extraction for SELECT, JOIN, CTE, subquery, wildcard, function, locking, DDL, and multi-statement forms |
| query_access_ast_census_test.go | Characterization matrix for TiDB parser AST fields and classification invariants |
| stmt_disposition_test.go | Statement-type disposition census: enumerates statement-node candidates via `go/parser` (anonymous-field struct analysis — transitive `stmtNode`/`ddlNode`/`dmlNode` embedding plus direct `statement()` receivers seeded before closure and local type-alias edges resolved through the graph; naming, comments, one-line decls, generic receivers, and alias-mediated embeds cannot evade), verifies each `ast.StmtNode` implementer reflectively against `classify`/`unhandledStatementFeature`, checks the extracted set against the real `Extract` type-switch cases, and fails when a parser upgrade adds or renames a type before a disposition is chosen |
| field_census_test.go | Field-level disposition census: pins projected/evidence/carried/subsumed/exempt/deferred for every exported field of every census-scope struct (extracted statement types and their option/fact carriers), and closes the carrier boundary by requiring any ast-package struct reachable through a census field to join the scope or carry a reach exemption |

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
