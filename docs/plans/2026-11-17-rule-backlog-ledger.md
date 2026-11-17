# Rule-backlog execution ledger

Date: 2026-11-17
Status: Active — no whole family closed by this ledger
Scope source: [2026-11-16 family plan](2026-11-16-next-rule-family-scope.md#family-ledger-all-63-residual-rows)
Implementation baseline: `d8cceea6b71e6ebe76ee0aaabcf3f95c76c665d0`
Authoritative facts: [inventory.yaml](../../testdata/ddl-inventory/inventory.yaml)

## Denominator and completion discipline

This ledger retains **19 families and all original 63 row IDs**. At the fixed
implementation baseline those rows declare **255 authored subactions** and
**203 normalized subaction records**. Their per-row name union is **258**;
these are tracking labels, not 258 independent supported features. The three
extra normalized names are `sql-security` in the three security-bearing MySQL
routine rows; they refer to the authored `SQL SECURITY` dimension.

Each row below explicitly lists every authored dimension. Its scope also
includes every `subaction_status[].subaction_id` of that exact inventory row at
baseline `d8cceea6`; normalized labels must not disappear because the authored
list uses a broader spelling. Later newly recorded dimensions join the ledger
rather than replacing the frozen denominator.

**Initial disposition:** every dimension not explicitly covered by a delivered
slice below is `unreviewed`. An inventory `audited` cell or an old golden gate
is an input to review, not automatic completion under this program. In
particular, the older inventory may be conservative or stale; retain its
status until the exact new evidence justifies changing it.

Allowed per-dimension conclusions after review:

- `implemented`: approved behavior shipped with exact rule/test/corpus/native/
  inventory/decision/commit evidence and the full delivery chain.
- `verified-existing`: existing behavior already meets the approved dimension,
  proven by exact current cases, not a generic finding count.
- `boundary-accepted`: reproducible parser/vendor/fact boundary, remaining scope
  and upgrade condition recorded; a new product deferral requires user
  acceptance. This means boundary characterized, **not supported**.
- `blocked`: exact missing prerequisite and attempted commands recorded; never
  complete while an implementation, approval or verification failure remains.
- `unreviewed`: queued work; no support or completion claim.

A family closes only when **all its recorded dimensions** have a justified
conclusion and all approved implementable work has passed the delivery chain.
A row marked `partial:D1` or `partial:D2` is not a completed family. No rows,
anchors or cases may be deleted to improve the percentage.

## Already delivered slices (bounded evidence, not family closure)

| Slice | Exact scope | Delivery and evidence | Remaining dimensions |
| --- | --- | --- | --- |
| D1 | MySQL CREATE explicit/default DEFINER versus INVOKER; ALTER explicit DEFINER versus INVOKER/omission; account independence | Rules `780585f1`, shipping `795b47b0`; T27/T28; [decision](../decisions/2026-11-16-mysql-routine-security-definer-warn.md) | General routine existence/dependencies, other characteristics, RETURN and dynamic/body boundaries are not closed by security mode evidence |
| D2 | PostgreSQL CREATE TABLE / ADD COLUMN bounded generation/default/NULL/type/sequence-bound validity and explicit identity CYCLE | Extractor `5d5f7ea0`, rules `664c1ed4`, guard `01bc3135`, native/inventory `c233fab7`, final shipping `d8cceea6`; T36/T37; [decision](../decisions/2026-11-16-postgresql-generated-identity-governance.md) | Expression legality/dependencies, existing-column identity transitions, runtime sequence state, partition/LIKE/inheritance and other table attributes remain open |
| D3 | PostgreSQL ALTER PK index-build / FK initial-validation warnings and UNIQUE USING INDEX notice correction | Extractor `37fa8fd1`, rules `f9f3601e`, native/inventory and final shipping `8aa9e3c8`; T37; [decision](../decisions/2026-11-17-postgresql-alter-constraint-build-risk.md) | Identity/default and wider constraint legality remain open; D4 below adds bounded target proof |
| D4 | PG17 exact constraint existence/kind/guard checks and no-op notices, with accepted conservative freshness gaps | Facts `be3a5570`, rules `8482b193`, native/inventory `ed6fe765`, final `aa7f8872`; T37; [decision](../decisions/2026-11-17-postgresql-constraint-target-evidence.md) | Data/dependencies/permissions/inheritance/partition and general PG ordered execution remain outside target proof; other F09 dimensions open |

D1/D2 passed their final single-HEAD TASK=all aggregates and CI. Their
verified implementation baseline is `d8cceea6`: **50 tasks,
3398/3398 cases, 59,716 assertions**. The artifact is
`/tmp/ddl-golden-pg-generated-identity-final/all/artifact.json`;
[Go/PG/lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/43909845975),
[TLS E2E CI](https://github.com/Fanduzi/DeltaScope/actions/runs/43909846063)
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/133448716229) succeeded.
This does not attribute that aggregate to a later docs-only commit.

Inventory aggregates at that baseline: semantically_checked=163,
generic_notice=18, mixed_coverage=45, explicit_unsupported=41,
vendor_not_supported=11, parser_unsupported=8, parse_only=15 (301 total).
The original 63 generic/mixed rows still need dimensional review.

## F09 delivered constraint-risk slice D3

The operator approved two default-enabled warnings and the corrected UNIQUE
USING INDEX silent arm. Extractor `37fa8fd1` retains the explicit reuse fact;
`f9f3601e` adds the shared warnings, config/catalog, corpus, SDK/HTTP/MCP and
bilingual reference coverage. [Decision](../decisions/2026-11-17-postgresql-alter-constraint-build-risk.md).

T37 preserves its 66 prior cases/profiles and adds 13 isolated cases. Final
79/79 execution (836 assertions) proves ordinary/reused PK/UNIQUE, plain/NOT
VALID FK, independent follow-up/forbid, multi-action/disabled/dialect controls,
PG17.7 index OID/constraint catalog/lock oracles and no audit execution. The
partitioned NOT VALID FK driver refusal is SQLSTATE 42809, not support.

Only `postgresql.alter-table.constraints` is touched: ADD PRIMARY KEY and ADD
FOREIGN KEY become audited for these migration-risk signals; UNIQUE retains
its status with corrected reuse semantics. All parent aggregates and the
19-family/63-row dimension denominator are unchanged. Ordinary default-policy
PK can move pass→review; an independently enabled FK forbid still rejects.

D3 was FF-only merged and normally pushed from base `8de713b8` to
`8aa9e3c88a6441c0df0273d03ecf0efcddb6c5d6`. All 50 canonical tasks at
that exact HEAD passed: **3411/3411 cases, 59,870 assertions**; no task had
zero execution or a mismatched HEAD. Aggregate:
`/tmp/ddl-golden-pg-constraint-risk-final/all/artifact.json` (validated).
Build, default/PG tests, default/PG lint, formatting and all inventory/corpus/
coverage-catalog gates passed before and after merge. A separate read-only
review found no unresolved P1/P2; the original worktree status was preserved.
[Go/PG/lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44120347868),
[TLS E2E CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44120345515)
and Cloudflare Pages succeeded for that exact SHA. Milestone diff
`8de713b8...8aa9e3c8` is 47 paths; the final evidence task
`f9f3601e..8aa9e3c8` is seven paths. This records historical code delivery,
not aggregate evidence for a later documentation commit.

**F09 remains open.** Other constraint existence/type/validation-state,
identity/default/attribute/options and rename/owner dimensions remain queued.

### D4 delivered constraint targets and accepted stage boundary

After 15 PostgreSQL 17.7 native shapes and 30 real offline/live CLI probes at
`8aa9e3c8`, the operator approved three new default blockers:
`ddl.pg.alter.validate_constraint.target.require` (exists, FK/CHECK),
`ddl.pg.alter.constraint_deferrability.target.require` (exists, FK), and
`ddl.pg.alter.drop_constraint.target.require` (missing without IF EXISTS).
Missing/ambiguous/incomplete facts retain evidence gaps rather than guessed
absence. This can raise offline pass to review. Proven already-validated and
missing-guard no-op cases stop receiving misleading existing notices.

The accepted **stage boundary** does not enable general PG ordered execution:
related prior DDL invalidates old catalog facts; later target checks retain a
gap rather than falsely rejecting ADD→VALIDATE or trusting DROP→VALIDATE.
This is not permanent deferral or whole-family support. Data, dependencies,
permissions and remaining dimensions are not certified by target checks.
Implementation is now in `be3a5570` (complete private catalog, exact schema/
case/cache routing, AST guards and conservative freshness) and `8482b193`
(rules/config/catalog, SDK provider alias/owned copy, HTTP/MCP/application,
corpus and bilingual references). [Decision](../decisions/2026-11-17-postgresql-constraint-target-evidence.md).
The shared PK matcher also keeps PG `ck` distinct from quoted unique `"CK"`
without relaxing actual PK forbids or MySQL/TiDB behavior.

T37 implements the exact 15 cases frozen before rules: all 79 old cases and
profiles are retained, for **94/94 cases and 1131 assertions** in focused
execution. Native oracles pin 42704/42809, missing CASCADE, catalog states,
quoted identities, no-op silence and no audit execution. Prior DDL/parser
facts remain gaps; independent 23514/23503 refusals do not certify data. Only
four exact constraint-target evidence cells are rebound; no cell/row status,
aggregate count or original dimensional denominator is promoted.

D4 final delivery is `aa7f88723fbc930dacdd451a0a6c772a6617a5a5`, FF-only
merged and normally pushed from `8aa9e3c8`. The initial `ed6fe765` aggregate
passed, but read-only review identified repeated failure scanning; `aa7f8872`
uses a single earliest-failure cutoff with unchanged conservative semantics.
The final **50 tasks, 3426/3426 cases, 60,165 assertions** were fully rerun at
`aa7f8872`, not borrowed from `ed6fe765`. Validated artifact:
`/tmp/ddl-golden-pg-constraint-target-final-aa7f8872/all/artifact.json`.
All per-task HEADs match, all executions are nonzero, and build/default+PG
tests, default+PG lint, formatting and contract gates passed before/after
merge. A separate read-only phase found no unresolved P1/P2 and preserved
the original worktree state. [Go/PG/lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44154482102),
[TLS E2E CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44154482185)
and Cloudflare Pages succeeded for that exact SHA.

Milestone diff `8aa9e3c8...aa7f8872` is 69 paths; the final review-fix task
`ed6fe765..aa7f8872` is four paths (the preceding native-evidence task is
seven). This is historical D4 evidence, not aggregate proof for a later docs
commit. **F09 remains open**; no other dimension or whole row is completed.

## Family index

Family IDs preserve the original ledger order, not implementation priority.
Next scope review is **F09**, then F10/F12 by measured value and model readiness;
new default behavior is approved per family before implementation.

| ID | Family | Rows | Canonical ownership | Initial state |
| --- | --- | ---: | --- | --- |
| F01 | MySQL stored routines | 5 | T27/T28 | D1 delivered; remaining dimensions unreviewed |
| F02 | MySQL trigger/event lifecycle | 5 | T29/T30 | unreviewed |
| F03 | MySQL/TiDB columns/generated/multi-action | 6 | T03/T05/T07/T08 | unreviewed |
| F04 | MySQL/TiDB index lifecycle | 3 | T09/T10 | unreviewed |
| F05 | MySQL/TiDB ALTER options/online DDL | 3 | T15 | unreviewed |
| F06 | TiDB recovery/flashback | 1 | T16 | unreviewed |
| F07 | PostgreSQL schema/collation | 2 | T35 | unreviewed |
| F08 | PostgreSQL CREATE TABLE | 6 | T36 | D2 delivered; remaining dimensions unreviewed |
| F09 | PostgreSQL ALTER TABLE | 5 | T37 | D2 ADD COLUMN, D3 constraint risk and D4 bounded targets delivered; remaining dimensions open |
| F10 | PostgreSQL DROP/TRUNCATE | 2 | T38 | unreviewed |
| F11 | PostgreSQL indexes/operator families | 4 | T39 | unreviewed |
| F12 | PostgreSQL views/materialized views | 5 | T40 | unreviewed |
| F13 | PostgreSQL types/operators | 3 | T41 | unreviewed |
| F14 | PostgreSQL row-security policy | 1 | T42 | unreviewed |
| F15 | PostgreSQL sequences | 2 | T43 | unreviewed |
| F16 | PostgreSQL foreign objects | 2 | T44 | unreviewed |
| F17 | PostgreSQL text search | 2 | T45 | unreviewed |
| F18 | PostgreSQL replication | 3 | T46 | unreviewed |
| F19 | PostgreSQL grants/languages | 3 | T47 | unreviewed |

## Exact row/dimension queue

Column `Disposition` applies to every listed and normalized dimension unless
the delivered slice explicitly says otherwise. It never means the whole row
is semantically checked. Each family review must replace `unreviewed` with
individual findings, boundaries and concrete evidence; a blanket `done` is
insufficient.

<!-- BEGIN ORIGINAL 63 ROWS -->
| Family | Row ID | Authored dimensions at d8cceea6 | Disposition |
| --- | --- | --- | --- |
| F01 | `mysql.create-procedure` | CREATE PROCEDURE ... BEGIN..END; parameters; SQL SECURITY; DETERMINISTIC; body statements | partial:D1; others unreviewed |
| F01 | `mysql.create-function` | CREATE FUNCTION ... RETURNS ... BEGIN..END; DETERMINISTIC; SQL SECURITY | partial:D1; others unreviewed |
| F01 | `mysql.alter-procedure-function` | ALTER PROCEDURE; ALTER FUNCTION; SQL SECURITY; COMMENT | partial:D1; others unreviewed |
| F01 | `mysql.drop-procedure-function` | DROP PROCEDURE; DROP FUNCTION; IF EXISTS | unreviewed |
| F01 | `mysql.stored-body-analysis` | static body SQL; DECLARE/local binding; control flow; PREPARE/EXECUTE dynamic SQL | unreviewed |
| F02 | `mysql.create-trigger` | BEFORE/AFTER INSERT/UPDATE/DELETE; FOR EACH ROW; FOLLOWS/PRECEDES; OLD/NEW bindings | unreviewed |
| F02 | `mysql.drop-trigger` | DROP TRIGGER; IF EXISTS | unreviewed |
| F02 | `mysql.create-event` | ON SCHEDULE AT/EVERY; ON COMPLETION; ENABLE/DISABLE; DO body | unreviewed |
| F02 | `mysql.alter-event` | ALTER EVENT ... ON SCHEDULE/RENAME/ENABLE/DISABLE/DO | unreviewed |
| F02 | `mysql.drop-event` | DROP EVENT; IF EXISTS | unreviewed |
| F03 | `mysql.create-table-generated-column` | GENERATED ALWAYS AS (expr); VIRTUAL; STORED; expression DEFAULT (8.0.13+); INVISIBLE column (8.0.23+) | unreviewed |
| F03 | `mysql.alter-table.add-column` | ADD COLUMN; ADD COLUMN FIRST/AFTER; ADD (col, col, ...) | unreviewed |
| F03 | `mysql.alter-table.drop-column` | DROP COLUMN | unreviewed |
| F03 | `mysql.alter-table.modify-column` | MODIFY COLUMN; type change; NULL/NOT NULL transitions; FIRST/AFTER | unreviewed |
| F03 | `mysql.alter-table.multi-action` | comma-separated action lists | unreviewed |
| F03 | `tidb.alter-table.columns` | ADD/DROP/MODIFY/CHANGE/RENAME COLUMN; FIRST/AFTER; TiDB same-object modification limits | unreviewed |
| F04 | `mysql.create-index` | CREATE INDEX; CREATE UNIQUE INDEX; key parts; prefix lengths | unreviewed |
| F04 | `mysql.drop-index` | DROP INDEX name ON table | unreviewed |
| F04 | `tidb.drop-index` | DROP INDEX name ON table | unreviewed |
| F05 | `mysql.alter-table.online-ddl` | ALGORITHM=INSTANT/INPLACE/COPY/DEFAULT; LOCK=NONE/SHARED/EXCLUSIVE/DEFAULT | unreviewed |
| F05 | `mysql.alter-table.options` | ORDER BY; CONVERT TO CHARACTER SET; DEFAULT CHARACTER SET; ROW_FORMAT; AUTO_INCREMENT=; FORCE; ENABLE/DISABLE KEYS; COMMENT; ENGINE | unreviewed |
| F05 | `tidb.alter-table.online-options` | ALGORITHM= (capability assertion, not selection); LOCK=; multiple actions per ALTER | unreviewed |
| F06 | `tidb.flashback-recover` | FLASHBACK TABLE; FLASHBACK DATABASE; FLASHBACK CLUSTER; RECOVER TABLE | unreviewed |
| F07 | `postgresql.create-schema` | CREATE SCHEMA; IF NOT EXISTS / AUTHORIZATION; schema elements | unreviewed |
| F07 | `postgresql.alter-collation` | REFRESH VERSION; RENAME/OWNER/SET SCHEMA | unreviewed |
| F08 | `postgresql.create-table` | basic columns; IF NOT EXISTS; column defaults; CHECK constraint; PRIMARY KEY / UNIQUE; FOREIGN KEY / REFERENCES; EXCLUDE constraint; table constraint clause; WITH storage parameters; USING access method; ON COMMIT / TABLESPACE; OF composite type; arrays and composite columns; generation-declaration | partial:D2; others unreviewed |
| F08 | `postgresql.create-table-constraints` | PRIMARY KEY variants; UNIQUE NULLS NOT DISTINCT; UNIQUE USING INDEX; EXCLUDE; NOT NULL named constraint; REFERENCES actions | unreviewed |
| F08 | `postgresql.create-table-generated-identity` | GENERATED ALWAYS AS IDENTITY; BY DEFAULT AS IDENTITY; sequence options; GENERATED ALWAYS AS expression STORED; generation-declaration | partial:D2; others unreviewed |
| F08 | `postgresql.create-table-partitioning` | PARTITION BY RANGE; PARTITION BY LIST; PARTITION BY HASH; PARTITION OF; PARTITION OF DEFAULT; INHERITS | unreviewed |
| F08 | `postgresql.create-table-temporary` | TEMP/TEMPORARY; GLOBAL TEMPORARY compat noise; UNLOGGED; ON COMMIT actions | unreviewed |
| F08 | `postgresql.create-table-like` | LIKE basic; INCLUDING options | unreviewed |
| F09 | `postgresql.alter-table.columns` | ADD COLUMN; ADD COLUMN IF NOT EXISTS; DROP COLUMN; DROP COLUMN IF EXISTS CASCADE; RENAME COLUMN; ALTER COLUMN TYPE; ALTER COLUMN TYPE USING; multi-action; add-column-generation-declaration | partial:D2; others unreviewed |
| F09 | `postgresql.alter-table.column-attributes` | SET/DROP NOT NULL; SET/DROP DEFAULT; SET GENERATED / DROP EXPRESSION; ADD/DROP IDENTITY; identity options; RESTART; SET STATISTICS; SET STORAGE / COMPRESSION; SET n_distinct / RESET | unreviewed |
| F09 | `postgresql.alter-table.constraints` | ADD PRIMARY KEY; ADD UNIQUE; ADD FOREIGN KEY; ADD CHECK; ADD CHECK NOT VALID; DROP CONSTRAINT; DROP CONSTRAINT CASCADE; VALIDATE CONSTRAINT; ALTER CONSTRAINT DEFERRABLE; ALTER CONSTRAINT NOT ENFORCED; NOT NULL NOT VALID; ALTER CONSTRAINT INHERIT/NO INHERIT | partial:D3 PK/FK migration risk and UNIQUE reuse correction; D4 bounded targets delivered; remaining dimensions open |
| F09 | `postgresql.alter-table.options` | SET/RESET storage parameters; SET TABLESPACE; ALL IN TABLESPACE SET TABLESPACE; SET ACCESS METHOD; SET LOGGED/UNLOGGED; REPLICA IDENTITY; SET WITHOUT OIDS dead form | unreviewed |
| F09 | `postgresql.alter-table.rename-owner` | RENAME TO; SET SCHEMA; OWNER TO | unreviewed |
| F10 | `postgresql.drop-table` | DROP TABLE; IF EXISTS / multi-target | unreviewed |
| F10 | `postgresql.truncate-table` | TRUNCATE basic; RESTART/CONTINUE IDENTITY; ONLY / CASCADE | unreviewed |
| F11 | `postgresql.create-index` | CREATE INDEX basic; UNIQUE; CONCURRENTLY; USING method; expression index; partial WHERE; INCLUDE / NULLS DISTINCT; opclass/parameters; WITH/TABLESPACE | unreviewed |
| F11 | `postgresql.alter-index` | RENAME TO; SET TABLESPACE; SET/RESET parameters; ATTACH PARTITION; IF EXISTS | unreviewed |
| F11 | `postgresql.create-operator-family` | CREATE OPERATOR FAMILY; AS members | unreviewed |
| F11 | `postgresql.alter-operator-family` | ADD/DROP members; RENAME/OWNER/SET SCHEMA | unreviewed |
| F12 | `postgresql.create-view` | CREATE VIEW; OR REPLACE / TEMP; column list; WITH options; CHECK OPTION; RECURSIVE | unreviewed |
| F12 | `postgresql.alter-view` | RENAME/SET SCHEMA/OWNER; SET/RESET options; ALTER COLUMN DEFAULT; IF EXISTS | unreviewed |
| F12 | `postgresql.drop-view` | DROP VIEW; IF EXISTS CASCADE | unreviewed |
| F12 | `postgresql.alter-materialized-view` | RENAME/SET SCHEMA/OWNER; SET/RESET options; DEPENDS ON EXTENSION | unreviewed |
| F12 | `postgresql.refresh-materialized-view` | REFRESH basic; CONCURRENTLY; WITH NO DATA | unreviewed |
| F13 | `postgresql.create-type` | ENUM; composite; RANGE; base type; shell type | unreviewed |
| F13 | `postgresql.alter-type` | ADD VALUE; RENAME VALUE; RENAME/OWNER/SET SCHEMA; composite attributes; SET properties | unreviewed |
| F13 | `postgresql.alter-operator` | OWNER/SET SCHEMA; SET RESTRICT/JOIN | unreviewed |
| F14 | `postgresql.alter-policy` | USING/WITH CHECK; TO role / RENAME | unreviewed |
| F15 | `postgresql.create-sequence` | CREATE SEQUENCE; options; OWNED BY / TEMP / IF NOT EXISTS | unreviewed |
| F15 | `postgresql.alter-sequence` | RESTART; options; OWNED BY / RENAME / SET SCHEMA; IF EXISTS | unreviewed |
| F16 | `postgresql.alter-foreign-data-wrapper` | HANDLER/VALIDATOR changes; OPTIONS; OWNER/RENAME | unreviewed |
| F16 | `postgresql.alter-foreign-table` | OPTIONS; ADD/DROP COLUMN; RENAME/OWNER/SET SCHEMA | unreviewed |
| F17 | `postgresql.alter-text-search-configuration` | ADD/DROP/ALTER MAPPING; RENAME/OWNER/SET SCHEMA | unreviewed |
| F17 | `postgresql.alter-text-search-dictionary` | option changes; RENAME/OWNER/SET SCHEMA | unreviewed |
| F18 | `postgresql.alter-publication` | ADD/DROP/SET TABLE; SET TABLES IN SCHEMA; SET publish options; OWNER/RENAME | unreviewed |
| F18 | `postgresql.alter-subscription` | ENABLE/DISABLE; CONNECTION/PUBLICATION/SET; REFRESH PUBLICATION; SKIP; OWNER/RENAME | unreviewed |
| F18 | `postgresql.drop-subscription` | DROP SUBSCRIPTION; IF EXISTS; WITH drop_slot | unreviewed |
| F19 | `postgresql.grant` | ON TABLE; ALL TABLES IN SCHEMA; columns; WITH GRANT OPTION; sequences/functions/database/schema/types; role membership; tablespace/large objects; GRANTED BY | unreviewed |
| F19 | `postgresql.revoke` | REVOKE privileges; GRANT OPTION FOR; CASCADE/RESTRICT; role membership; object kinds | unreviewed |
| F19 | `postgresql.create-language` | CREATE LANGUAGE; OR REPLACE TRUSTED; HANDLER form | unreviewed |
<!-- END ORIGINAL 63 ROWS -->

## Evidence required for each dimension review

Record the exact row and authored/normalized subaction names, dialect/version/
anchor, probe SQL or source fixture, normalized facts retained or missing,
existing consuming rules, behavior/default approval, targeted tests and native
oracle case IDs, findings versus independent incomplete evidence, corpus and
inventory selectors, decision record and shipping commit. For a prior-rule
sufficiency conclusion, explain why it checks the actual dimension rather than
merely matching the operation. For a boundary, retain a reproducible failure,
missing fact or vendor refusal and the condition for revisiting it.

Do not upgrade any inventory cell from this queue alone. After implementation,
regenerate `matrix.json`, run required local gates and targeted golden cases,
then validate a complete TASK=all at a single final HEAD before FF-only main
integration, normal push and exact-SHA CI confirmation. Preserve user work and
unrelated services. New default-on or reject/review behavior requires per-family
approval. A separate docs-only closeout records the existing implementation
artifact honestly; it is not a fresh implementation golden result.

## Administrative queue

- #149: check `go list -m -versions github.com/pganalyze/pg_query_go/v7` and
  primary release evidence; comment only for an actual newly released binding.
  Keep the five PG18 parser-error candidates explicit until a separately
  approved upgrade/promotion is verified.
- PR #112: review the unmerged guidance changes against current file-header and
  agent instructions; provide a recommendation without merging or closing.
- PR #150: review the historical T20 readonly report and evidence/privacy
  provenance; provide a recommendation without merging or closing.
- Release/tag decisions are outside this program's push authorization.

## Ledger verification at creation

The 63 row IDs and 19 family memberships are mechanically compared with the
original family table. Every authored dimension is compared, in order, with
its exact inventory row at fixed baseline `d8cceea6`; the normalized record
names and per-row union count are checked separately. Local Markdown links
and existing D1/D2 proof refs are checked. The inventory, matrix, rules, policy,
corpus and golden manifests are unchanged by creating this ledger.
