# Decision: Normalize MySQL/TiDB ALTER TABLE Index Additions

Date: 2026-08-30
Status: Accepted
Related milestone/version: bar70 / GitHub issue #49
Related commits:
  `df86703b2401ec481061fd88cf2aa35578469741`
  `0009e52a5d8f4f83f7034a7d192e169d14eab1a8`
Related tests:
  `internal/application/audit/extract_test.go`
  `internal/infrastructure/parser/tidb/extractor_test.go`
  `pkg/deltascope/audit_ddl_lifecycle_mysql_test.go`
  `testdata/sql-corpus/{mysql,tidb}/ddl/findings/alter_table_{index_additions,constraints}.*`
Related docs:
  `internal/infrastructure/parser/tidb/README.md`
  `internal/domain/rule/ddl/README.md`

## Context

The TiDB parser represents `ALTER TABLE ADD INDEX`, `ADD KEY`, and supported
unique/full-text index forms with the same `AlterTableAddConstraint` AST type
used for explicit `ADD CONSTRAINT`. The existing extractor therefore emitted
`add_constraint` for index additions, causing the constraint notice to fire and
skipping the intended index lifecycle semantics.

## Decision

Keep the top-level normalized operation as `alter_table`, but derive the
normalized action from the parsed constraint subtype and the original ALTER
clause. Supported index-form additions emit `add_index`; explicit `ADD
CONSTRAINT` and primary-key, foreign-key, and CHECK constraint forms emit
`add_constraint`. Existing ALTER index selectors accept both actions, and the
existing `ddl.create_index.notice` rule recognizes nested `add_index` actions
with its existing notice text, level, and rule ID. `ALGORITHM` and `LOCK` remain
accepted and preserved in normalized SQL without a new policy rule.

## Rationale

The parser AST does not retain whether the optional `CONSTRAINT` keyword was
present on the child node. Clause text is therefore used only at the extractor
boundary to preserve that distinction, while the parser-neutral action remains
available to all downstream rules. This avoids changing public operation shape,
adding duplicate rule IDs, or changing existing lifecycle wording.

## Public Contract

MySQL and TiDB `ALTER TABLE` index additions expose bounded `action:
add_index` and `index` metadata and use `ddl.create_index.notice`. True `ADD
CONSTRAINT` forms continue to use
`add_constraint` and `ddl.alter.add_constraint.notice`. Existing renderers
receive the same bounded finding message and metadata through the shared audit
result.

## Deferred / Out Of Scope

- No ALGORITHM/LOCK safety policy or finding is added.
- No parser support is added for syntax the current parser rejects, including
  unsupported ALTER SPATIAL forms.
- No rule IDs, levels, unrelated lifecycle wording, or renderer contracts are
  changed.

## Verification Evidence

- Focused extractor tests cover MySQL/TiDB INDEX, KEY, UNIQUE, FULLTEXT,
  explicit constraint, mixed-clause, and ALGORITHM/LOCK forms.
- Public SDK tests verify the existing create-index notice, `add_index` action
  metadata, identifier rendering, and absence of the constraint notice.
- Dedicated MySQL/TiDB corpus fixtures cover index additions and true
  constraints; the DDL coverage catalog is regenerated from the updated census.

## Consequences

Future ALTER rules that target index additions must match `add_index` and retain
`add_constraint` only for actual constraint semantics. If the parser later
exposes a structured keyword-presence field, the extractor clause scan can be
replaced with that parser-owned fact without changing the normalized contract.

## Links

- Commits:
  - `df86703b2401ec481061fd88cf2aa35578469741`
  - `0009e52a5d8f4f83f7034a7d192e169d14eab1a8`
- Tests: `make sql-corpus-gates`, `make ddl-coverage-catalog-test`
- Docs: `docs/decisions/README.md`
