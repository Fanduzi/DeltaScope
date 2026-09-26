# Decision: Retained incomplete-coverage contract for recognized-but-unaudited DDL

Date: 2026-09-23
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T03/#82)
Related commits: this task's commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/application/audit/coverage_t03_test.go`, `internal/interfaces/{cli,http,mcp}/audit_coverage_t03_test.go`, `make ddl-golden TASK=T03 ARTIFACT_DIR=...`
Related docs: `testdata/ddl-golden/T03.json`, `docs/decisions/2026-09-22-ddl-inventory-and-golden-baseline.md`

## Context

The milestone denominator (T02/#81) made silent passes visible: DDL the
parser recognized but rules never audited previously completed as `pass`
with no evidence. Under the all-rules-off policy, `CREATE SEQUENCE
golden_seq START WITH 1;` under MySQL exited 0 with verdict `pass` — MySQL
has no SEQUENCE feature (vendor boundary), yet the result claimed a clean
audit. Task T03 (#82) requires recognized-but-unaudited statements to
return an explicit incomplete audit result instead of silently passing.

## Decision

1. **Coverage is a capability fact on every result scope.**
   `report.Coverage{Status}` and the public `deltascope.Coverage` carry
   `complete` | `unverified` | `incomplete` on both `StatementResult` and
   `Result`. `Aggregate()` rolls statement coverage up (`incomplete` >
   `unverified` > `complete`). Coverage is computed from extraction facts
   and cannot be changed by policy configuration: disabling every finding
   rule never turns an incomplete statement complete. `unverified` is
   reserved for #83 (missing metadata/version evidence) and is not emitted
   by this task.
2. **Recognized-but-unsupported statements are retained, not dropped.**
   `EvaluateStatements` keeps them as top-level `StatementResult` entries
   with `coverage.status=incomplete`, preserving index, raw/normalized SQL,
   and source locations; they are excluded from rule evaluation while
   supported siblings still audit normally. This changes the prior
   PostgreSQL contract where unsupported statements were absent from
   `statements`; the retained contract is now uniform across dialects.
3. **Shared semantic classification, one place.** Boundary decisions live
   in `internal/infrastructure/parser/tidb/coverage_boundary.go`
   (parser-attached statement-level boundaries) and
   `internal/application/audit/coverage.go` (aspect gaps: unhandled ALTER
   sub-actions, unextracted options, non-audited constraint/index kinds).
   The MySQL/TiDB audited-aspect whitelist applies only to those dialects;
   PostgreSQL keeps its existing statement-level mechanism. CLI, HTTP, and
   MCP contain no boundary logic — they map the shared result and error.
4. **Vendor boundaries are product facts, not gaps.** MySQL `CREATE
   SEQUENCE`/`DROP SEQUENCE`, placement policies, and TiDB-side procedures,
   resource groups, and flashback/recovery/admin forms classify as
   `unsupported_statement` with stable feature IDs and bounded fixed
   reasons — not as parser errors and not as silently-passed statements.
   The same treatment covers every parser-recognized mutating or
   administrative statement the extractor does not model (DCL role/proxy
   lifecycle, stats/load/import, region/topology operations, session
   admin, procedure bodies): read-only queries and transaction/session
   statements stay outside the audit surface, but anything that mutates
   server or schema state without audited semantics reports incomplete.
   Inventory `status_evidence` records the observed classification per row.
5. **Verdict floor only.** Incomplete coverage floors `pass` to `review`;
   existing `review` and `reject` are never downgraded. Parser-error
   semantics are unchanged: genuine syntax failures keep `parser_error`
   diagnostics and CLI exit 2, and can coexist with unsupported diagnostics
   in one mixed batch.
6. **Golden runner gains `cli_cases` without weakening anchors.**
   `testdata/ddl-golden/T03.json` adds manifest `cli_cases` asserting
   expected exit codes and recomputed `coverage`/`unsupported` evidence
   from real CLI JSON across all four pinned anchors (5.7.44, 8.0.46,
   8.4.10, TiDB v8.5.0). The validator recomputes from raw stdout; recorded
   expected/parsed fields are corroborative only.

## Rationale

- A retained statement result preserves statement identity, order, and
  source locations — dropping unsupported statements would silently
  renumber the batch and lose the audit trail the milestone exists to
  expose.
- One shared classifier prevents three transport copies of the boundary
  from drifting; adapters only translate the result.
- Vendor boundaries differ from unaudited aspects: `CREATE SEQUENCE` under
  MySQL can never be audited (the product has no such feature), while a
  TiDB partition sub-action may be promoted by later tasks (#92, #101).
  Feature IDs name the deferred owner explicitly.
- The floor cannot weaken real findings: `reject` outranks the incomplete
  floor so a batch with a blocker finding stays `reject`.

## Public Contract

- `Result.coverage.status` and `StatementResult.coverage.status` are
  additive JSON fields with values `complete`/`unverified`/`incomplete`.
- Unsupported statements appear in `statements[]` with their original
  index, `raw_sql`, `normalized_sql`, empty findings, and
  `coverage.status=incomplete`; `Result.unsupported[]` carries
  `{index, feature, sql, reason}` per detail.
- SDK: `Audit` returns `ErrUnsupportedStatement` (wrappable via
  `errors.Is`) together with the populated partial `Result`.
- CLI: exit 1 for incomplete coverage even with `--fail-on none`; exit 2
  stays reserved for parser/user errors; JSON/markdown still render the
  partial result.
- HTTP: `400` with the partial result, coverage, diagnostics, and
  unsupported evidence in the error envelope.
- MCP: `isError=true` with the same structured partial result.
- Unsupported diagnostics never contain raw SQL fragments beyond the
  statement text field, passwords, parser internals, or option values.

## Deferred / Out Of Scope

- `coverage.status=unverified` (metadata/version evidence missing) is
  reserved for #83.
- Promoting deferred boundaries to audited semantics belongs to their
  owner tasks (e.g. #92 partition sub-actions, #100 sequences, #101
  resource groups, #103 MySQL resource groups); their expected fixtures
  and inventory `status_evidence` must be updated when they land.
- PostgreSQL boundary semantics beyond the retained-statement contract
  are unchanged.

## Amendment 2026-09-24 — rework of recognized-but-unaudited classification

Post-merge probing showed several parser-recognized statements still
reported `coverage=complete`/`pass`/exit 0. Root cause: the audited ALTER
whitelist treated *extracted* action names as *audited* even when no rule
consumed them (`algorithm`, `lock`, `alter_index`, `drop_check`), the
nested `PLACEMENT POLICY=<name>` create-table option was extracted but
never classified, `DROP PROCEDURE` was missing from the TiDB vendor
boundary, and execution-capable unknown statements (`EXPLAIN ANALYZE`,
`EXPLAIN EXPLORE`, `TRACE`, `EXECUTE`) fell through to the default-complete
unknown path. Corrections:

- The audited-action whitelist now means "at least one rule consumes this
  action name" — extraction alone never implies audited semantics.
  `ALTER TABLE ... ALGORITHM/LOCK/ALTER INDEX INVISIBLE` (both dialects)
  and TiDB `DROP CHECK` now produce `alter_table.<action>` unsupported
  evidence and `coverage=incomplete`.
- `CREATE TABLE ... PLACEMENT POLICY=<name>` records
  `create_table.option.placement_policy`: a vendor boundary under MySQL
  (no placement feature) and unaudited analysis under TiDB (binding
  extracted, no rule consumer). The same `alter_table.placement_policy`
  action is a vendor boundary under MySQL and stays audited under TiDB
  (notice-level coverage).
- TiDB `DROP PROCEDURE` joins `CREATE PROCEDURE` as a vendor boundary;
  `ddl.drop_procedure.notice` no longer fires under TiDB.
- `ExplainStmt.Analyze`, `TraceStmt`, and `ExecuteStmt` are
  parser-attached boundaries (`explain_analyze`, `trace`,
  `execute_prepared`) because they execute the wrapped or dynamic
  statement — they can never be treated as read-only inspection.
  `ExplainStmt.Explore` (`explain_explore`) submits the statement to the
  TiDB explore/plan-search path whose internal candidate evaluation the
  audit cannot reason about, so it is marked unaudited rather than assumed
  read-only. Plain `EXPLAIN`, `EXPLAIN FOR CONNECTION`, `PREPARE`, and
  `DEALLOCATE` remain named out-of-surface exclusions (read-only or
  session scope).
- Extracted-but-unconsumed option facts are unaudited aspects too:
  schema `charset`/`collate` on `CREATE/ALTER DATABASE`, create-table
  `collate`, and sequence/placement-policy option lists now surface
  `<op>.option.<name>` or `<op>.options` evidence. `ALTER DATABASE`
  additionally records every unparsed attribute as an unextracted option,
  so TiDB placement bindings and MySQL encryption attributes cannot fall
  through silently. The `has_options` marker is emitted only when an
  option list is actually present — bare `CREATE SEQUENCE seq1` stays
  complete.
- Terminology: `incomplete` now means two distinct sub-classes with the
  same contract — vendor boundary and unaudited analysis. "Audit evidence
  gap" (`unverified`) remains reserved for #83: an understood operation
  whose required metadata facts are missing. Unmodeled actions/options are
  unaudited analysis, not evidence gaps; generic notices do not imply
  semantic coverage.
- `cli_cases` in `testdata/ddl-golden/T03.json` now lock each rework input
  (14 cases total) into the required denominator.

## Amendment 2026-09-24 (second rework) — temporary scope, AUTO_RANDOM, procedure bodies, dropped column options

A second probing round found four more parsed-but-unaudited families still
reporting `coverage=complete`: temporary tables, `AUTO_RANDOM`, MySQL
procedure bodies, and column options the extractor silently dropped.
Corrections, all driven by AST facts only (no raw-SQL scanning):

- `spec.DDL.TemporaryScope`/`OnCommitDelete` now project
  `CreateTableStmt.TemporaryKeyword`/`OnCommitDelete` and the same field on
  `DropTableStmt`. `CREATE/DROP TEMPORARY TABLE` records
  `<op>.temporary` (unaudited, both dialects — owner T16); the `GLOBAL`
  form records `<op>.temporary.global`, which is a vendor boundary under
  MySQL (no global temporary feature) and unaudited under TiDB.
- `spec.Column.AutoRandom` projects `ColumnOptionAutoRandom`. CREATE and
  `ALTER ... ADD COLUMN` forms record `<op>.column.auto_random` /
  `alter_table.<action>.column.auto_random`: unaudited under TiDB, vendor
  boundary under MySQL (owner T23).
- `extractCreateProcedure` now emits `has_body` only when
  `ProcedureBody != nil`; a parsed body records `create_procedure.body`
  (unaudited, owner T27). Unparseable bodies keep the parser-error
  contract (CLI exit 2) — no body effects are inferred from failed text.
- `spec.Column.UnextractedOptions` records every recognized-but-dropped
  column option (`generated`, `reference`, `check`, `unique`, `fulltext`,
  `column_format`, `storage`, `secondary_engine_attribute`) as
  `<op>.column.<name>` evidence; inventory rows already documented these
  as unchecked aspects (owners T07/T23/T27 etc.). `ColumnOptionNull` is a
  no-op marker and stays excluded; column `COLLATE`/`COMMENT` are consumed
  typed facts (`column.Collation`/`column.Comment` feeds charset rules),
  not gaps; `SERIAL`'s implicit UNIQUE now honestly surfaces as
  `create_table.column.unique`. Unmapped future option types synthesize a
  fail-closed `column_option_<n>` name like `tableOptionName`.
- `spec.Alter.HasColumnPosition` projects the parsed `FIRST|AFTER`
  position clause; `alter_table.<action>.column_position` is unaudited
  under both dialects (valid MySQL and TiDB syntax, unaudited ordering
  semantics).
- TiDB-only parsed table options (`auto_random_base`, `auto_id_cache`,
  `shard_row_id_bits`, `pre_split_regions`, the TTL family, `stats_*`,
  `affinity`) classify as vendor boundaries under MySQL in both the
  create-table and alter unextracted-option paths — matching the column
  `AUTO_RANDOM` form instead of under-claiming unaudited analysis.
- `cli_cases` grow to 35 (21 new): all four repro inputs plus MySQL
  temporary/global/AUTO_RANDOM vendor variants, temporary drops in both
  dialects, alter AUTO_RANDOM in both dialects, `FIRST|AFTER` position
  forms, TiDB-only option vendor marking, the dropped column-option
  family, the procedure parser-error contract case (also pinned by a fast
  unit test, since the `BEGIN..END` failure currently comes from the
  statement splitter rather than the parser), and ordinary create/drop
  contrasts.

## Amendment 2026-09-25 — GLOBAL index modifier and evidence-fidelity follow-up

External review of the second rework found the parser's `GLOBAL` index
modifier was still being dropped on every path before this fix:

- Column-level `UNIQUE [KEY] GLOBAL` and `PRIMARY KEY GLOBAL` carry the
  modifier in `ColumnOption.StrValue`; extraction now records bounded
  `unique_global` / `primary_key_global` option names. `LOCAL` parses to
  an empty `StrValue` and remains ordinary `unique` (default scope, no
  additional semantics).
- Table-level and `ALTER ... ADD` constraints carry the modifier in
  `Constraint.Option.Global`; standalone `CREATE INDEX ... GLOBAL` in
  `CreateIndexStmt.IndexOption.Global`. Both now project the new
  `spec.Index.Global` fact.
- Classification: `create_table.index.global`,
  `alter_table.<action>.index.global`, `create_index.create_index.index.global`,
  and the column-level names above are vendor boundaries under MySQL
  (MySQL has no `GLOBAL` index modifier) and unaudited under TiDB.
- `cli_cases` grow to 42 (7 new). The golden runner additionally gained
  optional `unsupported_reasons` and `statement_sql` expect keys, so
  cases now pin the exact vendor-vs-unaudited reason list and returned
  statement identity, not just feature names.

Open contract question (unchanged by this diff): `unsupported[].sql`
carries the original statement text per the documented public contract.
A stricter no-raw-SQL reading would remove that field; doing so is a
public-shape change deferred to an explicit decision.

## Amendment 2026-09-25 (part 2) — parenthesized ADD constraint expansion

Review of the GLOBAL fix found a deeper residual omission: the
parenthesized `ALTER TABLE t ADD (<column>, <constraint>)` form parses
into `AlterTableAddColumns` with constraints on `NewConstraints`, which
extraction never consumed — so every constraint in the list (UNIQUE,
PRIMARY KEY, FOREIGN KEY, CHECK, including GLOBAL variants) was silently
dropped and the statement could still report `coverage=complete`.

- `extractAlterSpecs` now expands `NewConstraints` through the same
  `extractAlterSpec` path used by standalone `ADD <constraint>` clauses.
  Index kinds take the `add_index` action and PK/FK/CHECK take
  `add_constraint`, matching standalone equivalents; the AST does not
  record a `CONSTRAINT` keyword inside the list, so `ADD (CONSTRAINT uq
  UNIQUE ...)` also normalizes to `add_index` (documented ambiguity).
- Consequences are wider than GLOBAL coverage: parenthesized CHECK now
  surfaces `alter_table.add_constraint.check` under MySQL, parenthesized
  FK reaches the `add_constraint` action like the standalone form, and
  GLOBAL variants emit `alter_table.<action>.index.global`.
- Golden `cli_cases` assertions were strengthened again: the optional
  `unsupported_entries` key compares sorted (statement index, feature,
  reason) tuples instead of independent sorted lists, so a swapped-reason
  report is rejected (covered by a dedicated validator mutation case);
  `statement_sql` pins returned raw-SQL identity.
- `cli_cases` grow to 50 (8 new, 58 cases total): parenthesized GLOBAL/check mixed cases
  in both dialects, a mixed unique/unique_global paired-reason case, and
  complete controls for column-only, index-only, and foreign-key lists.

## Amendment 2026-09-25 (part 3) — index key-part facts and unaudited statement targets

Review of the parenthesized-ADD fix found one introduced defect and two
same-class residual omissions, all now resolved:

- Introduced defect (fixed): the synthetic grouped-constraint spec reused
  the parent clause text, so a sibling column `COMMENT 'ADD CONSTRAINT'`
  could flip an index constraint's action to `add_constraint`. Grouped
  constraints now derive their action from constraint type only.
- `IndexPartSpecification.Expr`, `Length`, and `Desc` were silently
  dropped by `extractIndexColumns`. `spec.Index` now reuses the existing
  `HasExpressionKeys`/`ExpressionCount` fields plus new bounded
  `PrefixParts`/`DescParts` counts, projected on table-level constraints,
  ALTER index definitions, and standalone CREATE INDEX. Classification:
  `<op>.index.expr` is unaudited on both dialects — TiDB 8.5 documents
  expression indexes (`LOWER()` among the allowed functions) and MySQL
  8.0.13+ ships functional key parts; presence/count alone cannot decide
  per-expression engine legality, so the aspect stays incomplete rather
  than vendor-boundary (part-4 correction). `<op>.index.prefix` and
  `<op>.index.desc` are unaudited under both.
- Multi-target account/sequence lists were collapsed to their first
  member or ignored: `spec.DDL.OmittedTargets` now counts parsed targets
  absent from the normalized model — extras in DROP USER/ROLE/SEQUENCE
  lists, extra CREATE/ALTER USER specs, and the entire user list on
  GRANT/REVOKE (all unaudited as `<op>.unaudited_targets`, single-target
  grants included since no target identity is modeled).
- `cli_cases` grow to 60 (10 new, 68 cases total with the 8 DB cases):
  index-part classifications in both
  dialects, unaudited-target forms, the comment-interference regression,
  TiDB grouped PRIMARY KEY GLOBAL, and a nonzero-index batch proving
  `unsupported_entries` binds entries to the right statement.

## Amendment 2026-09-25 (part 4) — current-user ALTER USER, zero-length prefixes, secondary collections

A fourth review found one residual coverage hole and four same-class
omissions around secondary collection fields, plus a reason-fidelity
defect:

- `ALTER USER USER()` current-user forms carry their target on
  `CurrentAuth` / `CurrentDualPasswordOption` while `Specs` stays empty;
  `len(Specs)-1` produced `-1` and the statement stayed complete. The
  extractor now counts the unmodeled current-user target as one omitted
  target.
- `index.expr` was misclassified as a TiDB vendor boundary on the
  assumption the engine lacks expression indexes. TiDB 8.5 documents
  them (`LOWER()` allowed; `tidb_allow_function_for_expression_index`
  gates only experimental expressions), so the aspect is unaudited on
  both dialects — presence/count cannot decide per-expression legality.
- Explicit zero-length prefixes (`c(0)`) are indistinguishable from
  unspecified ones when testing `Length > 0`. Column parts now test
  `Length != types.UnspecifiedLength`, counting `c(0)` and `c(8)` alike
  while leaving expression parts (Length unset) out.
- Foreign-key key lists are not column-only: local and referenced
  `IndexPartSpecification` lists accept expressions, prefixes, and
  DESC, and `OnDelete`/`OnUpdate`/`Match` options were dropped.
  `spec.Constraint` now projects the referenced target/columns and
  carries bounded `UnmodeledParts`/`UnmodeledReferencedParts`/
  `UnmodeledReferActions` counts; coverage emits
  `<op>[.<action>].constraint.<type>.parts` unaudited evidence. CHECK
  carries `Expr` rather than key parts and is not affected.
- CREATE/ALTER USER secondary option lists
  (`AuthTokenOrTLSOptions`/`ResourceOptions`/`PasswordOrLockOptions`/
  `CommentOrAttributeOption`/`ResourceGroupNameOption`) were parsed but
  never projected; bounded family names now flow through
  `UnextractedOptions` as `<op>.option.<family>` unaudited evidence.
- Nested `PARTITION p0 ... OPTIONS` and sub-partition option lists were
  invisible to `extractTableOptions`, which only sees top-level table
  options. `partitionOptionNames` now appends deduplicated bounded names
  to `UnextractedOptions`, so a nested `PLACEMENT POLICY` reaches the
  existing MySQL vendor-boundary classifier.
- `unsupported_entries` expectations may pin `metadata` for exact
  bounded-metadata comparison; new Go rows and golden cases assert
  `expression_count`, `prefix_parts`, `omitted`, and
  `local_parts`/`referenced_parts`/`refer_actions` values.
- `cli_cases` grow to 70 (10 new, 78 cases total with the 8 DB cases):
  current-user forms, account options, zero prefixes, FK key-part and
  refer-action forms, and nested partition options.

## Amendment 2026-09-25 (part 5) — CURRENT_USER(), IndexOption members, SPLIT/UPDATE INDEXES

A fifth review found one residual coverage hole, two introduced defects,
and two pre-existing omissions, plus two validator edge cases:

- `ALTER USER CURRENT_USER()` resolves through `Specs` with
  `UserIdentity.CurrentUser=true` and an empty username — unlike the
  literal `USER()` form it is not carried on `CurrentAuth`. Specs[0]
  current-user identities now count as one omitted target on
  CREATE/ALTER USER and CREATE ROLE paths; extras still count via
  `len(Specs)-1`.
- Introduced defect (fixed): `extractAlterSpec` fed index-producing
  constraints through `extractConstraint`, so `ALTER TABLE t ADD INDEX
  ix (c(8))` emitted both `...constraint.index.parts` and
  `...index.prefix` for the same fact. Constraint part counters are
  zeroed when the constraint produces an index definition.
- `IndexOption.Condition` (partial-index WHERE) and all other unmodeled
  `IndexOption` members (comment, key_block_size, index_type,
  with_parser, visibility, primary_key_type, split_opt,
  secondary_engine_attr, columnar_replica) were silently dropped.
  `spec.Index` now carries `HasPredicate` (wired from `Condition`) plus
  a bounded `UnmodeledOptions` name list on all four index
  construction sites. `*.index.predicate` is a MySQL vendor boundary
  (no partial indexes) and TiDB unaudited (documented in the 8.5
  grammar); `*.index.option.<name>` is unaudited except TiDB-only
  members (split/engine/columnar), which stay vendor under MySQL.
- `CreateTableStmt.SplitIndex` (SPLIT ... BETWEEN ... REGIONS) and
  partition `UPDATE INDEXES` had no evidence; bounded markers
  `create_table.option.split_index` and
  `create_table.option.partition_update_indexes` now classify as MySQL
  vendor boundaries (TiDB extensions) / TiDB unaudited.
- `CREATE USER u RESOURCE GROUP rg` reaches
  `create_user.option.resource_group_name`: MySQL vendor (the account
  binding is TiDB-only) / TiDB unaudited.
- Golden validator: metadata pins now consume actual entries one-to-one
  (repeated `(index,feature,reason)` tuples are legal — two expr
  indexes on one table emit two entries), and `unsupported[].index`
  must reference a retained statement before the omitted-SQL exception
  applies, closing the invalid-index-plus-missing-sql bypass.
- `cli_cases` grow to 83 (13 new, 91 cases total with the 8 DB cases).

## Amendment 2026-09-25 (part 6) — exhaustive statement/field census

Successive review rounds kept finding parsed-but-unprojected facts
because the boundary was defined by case-by-case review rather than an
enumerated census. This amendment closes the two remaining enumeration
axes and pins them mechanically.

Statement-type axis: `unhandledStatementFeature` returning "" used to
admit every unlisted parsed type into complete coverage. A full census
of the pinned parser's `*ast.*Stmt` registry classified all 108 types:
27 extracted, 65 named unsupported features, 16 deliberately exempt
(read-only query forms, session state, transaction control). Three
executable markers joined the set: `do` (`DO expr` evaluates
server-side expressions), `binlog` (`BINLOG 'base64'` replays row
events — mutating), and `select_into` (`SELECT|UNION ... INTO OUTFILE`
writes server files; the clause lands on the inner trailing select for
set operations, so it is a field marker on `SelectStmt`/`SetOprStmt`
rather than a type entry). `stmt_disposition_test.go` pins the table
and scans
the parser module for `type XxxStmt struct` declarations, failing the
build when a parser upgrade adds or renames a type before a
disposition is chosen.

Field axis (extracted statements): previously-dropped AST fields now
carry bounded evidence —

- `CreateViewStmt`: `create_view.option.{or_replace, view_columns,
  view_algorithm, definer, sql_security, check_option}`. The parser
  fills MySQL-compatible defaults even when clauses are omitted
  (DEFINER→CURRENT_USER, SECURITY→DEFINER, ALGORITHM→UNDEFINED, CHECK
  OPTION→CASCADED), so only forms that provably deviate from defaults
  emit evidence; explicit default-writes stay silent by design.
- `GrantStmt`/`RevokeStmt`: `*.option.{column_privileges,
  routine_object, require_tls, with_grant}` — `routine_object`
  (FUNCTION|PROCEDURE grants) is the first MySQL-only surface
  classification: vendor boundary under TiDB, unaudited under MySQL.
- `CreateIndexStmt`/`DropIndexStmt` `LockAlg`: `*.option.lock_algorithm`
  — real MySQL ALGORITHM=/LOCK= syntax, unaudited on both dialects.
- `ProcedureInfo.ProcedureParam`: `create_procedure.option.params`
  beside the existing `has_body` marker.
- `CreateSequenceStmt.TblOptions` (shared table-option tail):
  `create_sequence.option.<name>` under TiDB; MySQL stays boundary-marked.

Deferred scope (documented, not fixed): DML modifier fields
(`INSERT IGNORE/LOW_PRIORITY/PARTITION(...)` / `UPDATE`/`DELETE`
priority and hints, `RETURNING`, row aliases) — the aspect mechanism is
DDL-scoped and DML rows are rule-audited; `DropIndexStmt.IsHypo`,
`AlterDatabaseStmt.AlterDefaultDatabase`, and
`CreateTableStmt.OnDuplicate` are unreachable from the pinned grammar
(probed); scalar existence flags (`IF [NOT] EXISTS`, `OrReplace` on
non-view objects) stay exempt per the established boundary precedent.

`cli_cases` grow to 95 (12 new, 103 cases total with the 8 DB cases);
the census test guards statement-type drift across parser upgrades.

## Amendment 2026-09-26 (part 7) — census hardened to the three review connections

Independent review accepted the census direction but required proof of
three connections instead of more ad-hoc field review. This amendment
replaces the naming-based enumeration with structural ones and adds the
missing field-level table.

**AST full set → census.** Naming was never a completeness proof:
`ProcedureInfo` is a real statement node without a `Stmt` suffix and the
original `type XxxStmt struct` regex missed it. `stmt_disposition_test.go`
now scans parser-module struct declarations and takes the transitive
embedding closure of `stmtNode`/`ddlNode`/`dmlNode` plus any type
defining `statement()` directly — 128 candidates. Reflection decides
membership: 123 satisfy `ast.StmtNode` and must carry a disposition; 5
structural carriers (`SplitOption`, `SplitIndexOption`,
`QueryWatchOption`, `DynamicCalibrateResourceOption`,
`ProcedureErrorCondition`) embed a statement base for visitor plumbing
without satisfying the interface — they stay in the candidate registry so
a parser upgrade that promotes them to statements cannot slip past the
census, but they are never treated as parsed statements.

**Census → actual dispatch.** Every row is verified reflectively:
`extracted` types must reach a `classify` kind other than unknown, named
rows must produce a feature via `unhandledStatementFeature`, and an
extracted type carrying a boundary feature fails the test. The 19
procedure helper nodes (`ProcedureInfo` family) classify as
`procedure_body` defense-in-depth behind the extractor's own dispatch.

**Field full set → field disposition table.** `field_census_test.go`
pins a disposition for every exported field of every census-scope struct
(28 statement types + 41 option/fact carriers): `projected`,
`projected_evidence`, `evidence:<name>`, `carried:<parent>`,
`subsumed:<scope>`, `exempt:<reason>`, or `deferred:<reason>`. An
unlisted field fails the test — a parser upgrade adding a field cannot
silently become complete coverage. A second test closes the carrier
boundary: any ast-package struct reachable through a census field must
join the scope or carry an explicit reach exemption (DML-context clause
trees, `CIStr` identifier pairs, subsumed value carriers).

**New evidence found by the field census** (fields the table enumerated
that had no prior disposition):

- `CreateUserStmt`/`AlterUserStmt` `UserSpec.AuthOpt`: per-target
  `IDENTIFIED BY/WITH` produced only a static `has_auth` option with the
  auth content dropped — now `*.option.identified` presence evidence;
  credential values still never travel.
- `AlterUserStmt` `UserSpec.DualPasswordOption` (and the `USER()`-path
  `CurrentDualPasswordOption`): `RETAIN/DISCARD OLD PASSWORD` — now
  `*.option.dual_password`; the `USER()` dual-password form keeps its
  omitted-target evidence.
- `CreatePlacementPolicyStmt.OrReplace`: `CREATE OR REPLACE PLACEMENT
  POLICY` — now `create_placement_policy.option.or_replace`.

`cli_cases` grow to 99 (107 cases total with the 8 DB cases).

## Verification Evidence

- `make ddl-golden TASK=T03 ARTIFACT_DIR=/tmp/ddl-golden`: 91 cases, 1034
  assertions, PASS on all four anchors — `cli_cases` prove MySQL `CREATE SEQUENCE` + `ALTER TABLE
  ... ADD COLUMN` exits 1 with `coverage.status=incomplete`, bounded
  `create_sequence` evidence, and a `review` verdict under the
  all-rules-off policy; rework cases lock `alter_table.alter_index`,
  `alter_table.drop_check`, `create_table.option.placement_policy`,
  `drop_procedure`, `explain_analyze` (both dialects), `execute_prepared`,
  `trace`, `create_sequence.options`, `alter_table.algorithm`/
  `alter_table.lock`, `alter_table.placement_policy`, the read-only
  `EXPLAIN` counter-example, and the second-rework set: `create_table.temporary[.global]`
  (both dialects), `drop_table.temporary`, `create_table.column.auto_random`
  (both dialects) plus `alter_table.add_columns.column.auto_random`,
  `create_procedure.body`, `create_table.column.generated`/`reference`/
  `check`/`unique`, the procedure parser-error exit-2 case, and ordinary
  create/drop-table contrasts; the fourth-rework set: `alter_user.unaudited_targets`
  on `ALTER USER USER()` current-user forms, account-option families,
  `create_index.create_index.index.prefix` on `c(0)`, FK key-part and
  refer-action `*.constraint.foreign_key.parts`, and nested partition
  options, with exact-metadata pins on representative entries.
- `make ddl-golden TASK=T02 ARTIFACT_DIR=/tmp/ddl-golden`: 10 cases, 57
  assertions, PASS — baseline behavior unregressed.
- `make ddl-golden-validator-test`: 26 contract cases — `cli_cases`
  coverage downgrade, wrong-feature tampering, and swapped-reason
  entries are rejected; `unsupported_entries` metadata pins compare
  bounded maps exactly.
- `make ddl-inventory-gate`, `make sql-corpus-gates`, `make test`, and
  `make pg-unit-test-gates` all pass; PostgreSQL-tagged suites verify the
  uniform retained-statement contract.
- Focused transport tests: CLI exit 1 vs exit 2, HTTP 400, MCP
  `isError=true`, mixed reject>review, and no-leak assertions.

## Consequences

- Any future `statements[]` consumer must tolerate incomplete entries with
  empty findings; `summary.statements` counts retained unsupported
  statements.
- Promoting a boundary row to audited semantics requires updating its
  feature-ID expectation, corpus fixtures, census classifications, the
  coverage catalog, and inventory `status_evidence` in the same change.
- New parser-recognized statement types must be classified in
  `coverage_boundary.go` (boundary) or `coverage.go` (aspect gap) rather
  than letting them fall through silently.

## Links

- Commits: this task's commit; prerequisites #80 `1768f05`, #81 `b2956b3`
- Tests: `internal/application/audit/coverage_t03_test.go`, `internal/application/audit/coverage.go`, `internal/infrastructure/parser/tidb/coverage_boundary.go`, `internal/interfaces/{cli,http,mcp}/audit_coverage_t03_test.go`, `pkg/deltascope/audit_unsupported_verdict_floor_postgresql_tag_test.go`
- Docs: `testdata/ddl-golden/T03.json`, `testdata/ddl-inventory/inventory.yaml`, `docs/decisions/2026-09-22-ddl-inventory-and-golden-baseline.md`
