# testdata/ddl-golden

Per-task manifests for the milestone DDL golden-path runner
(`make ddl-golden TASK=<id> ARTIFACT_DIR=<dir>`, `scripts/ddl_golden.py`).

## T37 D6 tuning / DROP guard compatibility

D6 retains all 118 prior IDs and profiles. Its approved proof requirement
corrects only three offline DROP expectations (`alter-table-columns-drop-column`,
`alter-table-columns-drop-column-if-exists-cascade`,
`alter-table-columns-multi-action`): aggregate/statement coverage becomes
unverified and exact gap count, attribution and `instance.version` fact checks
are added. Their SQL/verdict/finding counts/entries and other checks are not
weakened. The first replay's 115/118 failure remains recorded; corrected
candidate replay is 118/118, **1673 assertions** (12 added). These candidate
artifacts do not replace final same-HEAD aggregate evidence. The 18 D6 case IDs
were frozen in the [decision](../../docs/decisions/2026-11-18-postgresql-column-tuning-and-drop-guards.md)
before implementation. All 18 additions now run: **136 required cases,
2186 assertions** after the read-only underflow correction (513 additive).
Eight owned metadata fixtures contain 59 native driver steps (40 successes/19 exact refusals), independent attribute,
file/data/guard and no-audit-execution receipts plus owned absence/cleanup.

The first additive replay's 133/136 failures remain recorded: all-off correctly
omits rule_summary, and DROP's existing global freshness cutoff withholds
later target facts even for another table. Fresh guard/quoted/missing/empty
controls occur before a request's first DROP, while all later gaps are pinned;
no product scope or freshness policy is relaxed. PG17 defaults use NULL catalog
statistics (also for authored -1), not an invented -1 catalog value. Literal
zero, normal/max/clamp, fractions/duplicates/RESET, quoted/nonfinite/overflow,
parser, scope and no-immediate-rewrite controls remain distinct. Original
118 objects/profiles/IDs remain unchanged from the D6 compatibility commit.
Inventory binds exact cells. Final shipped fa37a41b aggregate is 50 tasks,
3468/3468 cases, 61220 assertions, validated at
`/tmp/ddl-golden-pg-column-tuning-final-fa37a41b/all/artifact.json`.
Accurate-SHA Go/PG/lint/TLS/Pages checks passed; see the decision delivery
record. Later historical doc commits are not attributed to that code artifact.

The f97 all-task candidate passed 50 tasks/3468 cases/61202 assertions but did
not ship: review caught Go silently rounding a nonzero Float to zero/subnormal
where native PG17 strtod refuses ERANGE. The existing opaque-value case now
pins the correction, exact gaps, real-zero/zero-exponent/small-normal controls
and native minimum-normal success despite a conservative boundary gap. This
adds 18 assertions without removing any old check. Old f97 aggregate evidence
cannot substitute for the corrected fa37 HEAD's complete rerun, which passed
as recorded above.

## T37 PostgreSQL existing-column generation contract

D5 preserves all 94 prior cases, their required IDs and independent profiles.
The 24 cases frozen before rule implementation add 14 CLI controls and ten
owned PG17.7 metadata fixtures under `pg-column-generation`: **118 required
cases**, with 1661 assertions in focused execution (530 additive assertions).
A parsed-JSON comparison verifies every old case/profile is unchanged.

Offline cases pin observed-version/fact gaps, START versus RESTART, guards,
disabled/non-PG and D2-isolation controls, unmodeled declarations and retained
parser exit 2/incomplete output. Metadata fixtures bind exact identities,
column/sequence types and stable parameters independently of issuance state.
The 62 driver steps split into 31 native successes and 31 refusals; SQLSTATEs
include 55000/22023/42601/42P16/42703 and independent nullable-data 23502.
Renamed sequence binding and a bigint column with a smallint sequence prevent
name/type guessing. Catalog/row/uncalled-sentinel oracles run after audit and
before driver DDL. Only driver oracles inspect current sequence values; the
product never reads or advances them.

RESTART resets issuance and acquires a native ShareRowExclusiveLock on the
sequence; SET START changes recorded START without resetting current state.
CYCLE/NO CYCLE, quoted names, guarded no-ops, retained generated data and valid
ordinary DEFAULT controls remain distinct. Narrowing bounds with a fitting
START can still fail on unread current state; data/current-value, prior-DDL/
parser, inheritance/partition/unlogged scopes remain explicit gaps, not support
claims. The partition-member native refusal is **42P16**. All fixtures prove
namespace absence, claim ownership and verify owned cleanup. New inventory
bindings affect only six column-attribute cells, not the original denominator,
other attributes or whole F09. Final same-HEAD aggregate/CI remain required.

## T37 PostgreSQL constraint-target contract

The target tranche preserves all 79 prior required cases and independent
profiles, adding the 15 cases frozen in the decision before rule implementation
(**94 required cases**): eight CLI controls and seven owned PG17.7 metadata
cases under `pg-constraint-target`.

The CLI cases pin offline unverified gaps, disabled/non-PG controls and parser
retention. Metadata cases independently prove missing/wrong-kind 42704/42809,
validated/guarded no-op silence, case-sensitive selection, preservation of the
real PK forbid without misidentifying a quoted unique name, and an explicit
missing CASCADE target refusal. Catalog flags and uncalled sequences prove
no audit mutation before the driver runs. Prior ADD/DROP/parser failures keep
catalog gaps rather than predicting post-state; parser-error exit remains the
existing CLI code 2 with incomplete coverage, distinct from unverified facts.

CHECK/FK data probes independently refuse 23514/23503 while the bounded target
check is satisfied. These cases are **not data-legality or successful full-SQL
audit certificates** and do not promote runtime/permission/dependency/parser
boundaries. Every new metadata fixture proves prior namespace absence and
uses ownership-guarded, verified cleanup. Inventory evidence is bound only to
four exact constraint-target subactions; no row/cell status is upgraded.

## T37 PostgreSQL constraint migration-risk contract

T37 preserves all 66 prior required cases and adds eleven CLI controls plus
 two owned metadata cases (**79 before the target tranche above**). Existing `pg-acceptance`
 and generation profiles stay unchanged. The isolated `pg-constraint-risk`
 profile pins PK build/reuse, FK initial-validation/NOT VALID and corrected
 UNIQUE build/reuse arms; separate profiles prove the existing VALIDATE
 follow-up and blanket FK forbid remain independent. Disabled, multi-action
 and non-PG controls do not borrow generic-notice evidence.

On PostgreSQL 17.7, the driver separately creates eligible unique indexes
 CONCURRENTLY, records their OIDs and proves USING INDEX attaches those same
 objects. Catalog oracles pin constraint kinds and `convalidated`; transaction
 probes observe PK ACCESS EXCLUSIVE and FK SHARE ROW EXCLUSIVE locks. Audit
 precedes native execution and changes neither constraints, original indexes,
 guarded data nor sentinel sequence state. A separate partitioned-table
 native refusal pins SQLSTATE **42809**, not a parser or product success.
 Schema absence/ownership descriptors and verified cleanup protect fixtures.
 These are declaration-risk cases, not constraint/index eligibility or whole
 F09 completion claims.

## T36/T37 PostgreSQL generation-declaration contract

T36 retains its original 48 cases and adds nine CLI controls plus two owned
metadata cases (59 required cases). T37 retains its original 62 cases and adds
three CLI controls plus one owned metadata case (66 before the constraint-risk
tranche, then 94 after the target tranche above). Existing
`pg-acceptance` profiles are unchanged; `pg-generation-declaration` isolates the
two new rules. Fixed cases pin CREATE/ADD COLUMN parity, mode/cycling silence,
int64 preservation, native refusal categories, duplicates/unmodeled gaps,
disabled rules and non-PG controls.

PostgreSQL 17.7 catalog oracles read `pg_attribute`, `pg_sequence` and sequence
`is_called` without INSERT or nextval: the audit creates no tables/columns and
does not advance sentinel sequences. Native driver-only CREATE/ADD COLUMN
then proves identity modes/cycling, stored generation and large sequence
bounds. Seven negative driver statements independently pin SQLSTATE 22023 /
42601 rather than borrowing a generic notice or treating refusal as success.
All owned schemas are removed with absence checks. Generated expressions and
existing-column identity transitions are not promoted by these cases.

## T27/T28 MySQL routine-security contract

[T27.json](T27.json) retains its original 30 required cases and independent
`t27-where`/`t27-routine` profiles. The `t27-sql-security` profile adds 18 CLI
controls and three anchor-owned metadata cases (51 required cases total).
CREATE/ALTER × procedure/function × DEFINER/INVOKER/omission is pinned without
borrowing lifecycle-notice evidence. Unknown/repeated characteristics remain
unverified, disabled and SONAME controls stay silent, and TiDB retains its
vendor boundary.

Each MySQL 5.7.44/8.0.46/8.4.10 metadata case audits before native definition
creation, verifies existing modes and a body-write sentinel are unchanged,
then the driver applies the exact header statements and independently reads
`information_schema.ROUTINES.SECURITY_TYPE`. COMMENT-only ALTER preserves an
existing DEFINER mode without the audit inferring it. Functions use root-owned
`DETERMINISTIC NO SQL` definitions; binary-logging/trust values are recorded,
not altered to hide a privilege precondition. No routine is CALLed or invoked.

[T28.json](T28.json) retains its original 29 cases/profile and adds one isolated
security-warning plus dynamic PREPARE/EXECUTE retention control (30 required
cases). Existing protected-marker expectations are unchanged. These manifests
are expectations, not standalone claims of executed or aggregate readiness.

## T25 MySQL tablespace family contract

[T25.json](T25.json) pins the issue-#104 tablespace surface end to end:
the strict recognizer for the standalone MySQL family the locked TiDB
grammar cannot parse (`CREATE/ALTER/DROP TABLESPACE`, `CREATE UNDO
TABLESPACE`, `CREATE/ALTER/DROP LOGFILE GROUP` — memberless creates,
unsupported `IF` guards, and trailing junk stay parser diagnostics rather
than half-claims), the eight InnoDB rules
(`ddl.mysql.tablespace.{create.warn,alter.warn,drop.forbid,applicability}`
and `ddl.alter.{import,discard}_tablespace.{warn/forbid,applicability}`),
and the honest-premise contract — an authored non-InnoDB/non-NDB engine,
a proven below-floor version (memberless general CREATE and UNDO SQL =
8.0.14, ENCRYPTION = 8.0.13 — a memberless InnoDB CREATE executes, never
refuses, from 8.0.14), an InnoDB datafile move, a proven-absent
ALTER/DROP target, a proven-present CREATE target, a proven-occupied
DROP, and a proven kind mismatch all refuse on evidence while unproven
existence/occupancy/kind/engine/partition/version premises owe bounded
gaps (`unknown_tablespace_state`, `unknown_table_state`,
`target.version`, `tablespace.drop_eligibility` for undo drops whose
runtime eligibility stays unmodeled) instead of inferred verdicts. File
paths are counted but never retained.
The NDB/logfile-group family and every TiDB form stay vendor boundaries
with zero MySQL semantic findings. Metadata cases are `pending_native`
authorization candidates — provider-backed existence/kind/occupancy
through the `mysql_tablespace` object seam (INNODB_TABLESPACES +
INNODB_TABLES association count), including the occupied-drop refusal and
the proven-absent ALTER refusal.

## T19 TiDB TTL contract

[T19.json](T19.json) pins the TiDB 8.5 TTL surface end to end: the
metadata-backed `ddl.tidb.ttl.warn` path (provider `SHOW CREATE` TTL read
plus the TiDB-only `KEY_COLUMN_USAGE` reverse-edge read), the
`checkTTLInfoValid` refusal boundary (temporary destination, invalid
interval, inbound foreign keys), the `checkTableForeignKey` parent-TTL
refusal on both CREATE and ALTER edges, `REMOVE TTL` silence, and the
MySQL feature-comment no-op control. Offline CLI cases pin the three
outcomes — proven apply warns once, proven refusal stays silent, and an
unproven prerequisite set owes a `target_table.ttl` evidence gap instead
of a finding. `SHOW CREATE TABLE` native receipts travel as `execute`
observations; information_schema scalars pin every mutable fact. The
manifest declares expectations only — it is not a passing-result claim.

## T20 placement binding contract

[T20.json](T20.json) retains the original 25 case definitions. T20-R4 adds
seven live metadata cases and three offline controls: CREATE with a later
missing policy or repeated policy on distinct members; ADD/REORGANIZE member
refusal and successful binding followed by a bound-policy DROP; unknown
catalog controls for all three forms; and the TiDB 8.5.0 ignored-subpartition
boundary with a retained parent binding.

Member findings pin policy and schema/table/partition identity. Applied
bindings retain a separate topology gap; known refusals have no contradictory
placement warning. Native queries prove server catalog behavior, while
`batch_state_t20r4_test.go` companions prove private ordered projections.
The [contract](../../docs/work-plans/t20-tidb-placement-lifecycle/CONTRACT.md)
defines the distinction and version scope. Historical artifacts remain
unchanged; assertion counts are computed from each completed run.

## T17 schema-lifecycle contract

[T17.json](T17.json) retains the original 47 required cases and adds five
T17-R1 cases (52 total): three offline conditional-create cases and two
metadata cases for the MySQL/TiDB tiflash boundary. For `TASK=T17`,
`make ddl-golden` first runs the independent ordered-projection and provider
error and T17-R1-tail attribute Go proofs, then runs the real CLI/database cases
and artifact validator.
Neither the supplemental projections nor offline CLI behavior substitute
for native receipts; they do not change the 52 CLI/database case count.

## T07 advanced-column contract

[T07.json](T07.json) materializes the independent
[`scripts/t07_contract.py`](../../scripts/t07_contract.py) oracle: 207 cases
(8 database, 2 baseline CLI, 34 offline CLI, 163 metadata). The contract and
versioned/native source distinctions are indexed in
[CONTRACT.md](../../docs/work-plans/t07-advanced-columns/CONTRACT.md).
T06 remains 496; pre-contract observations and controlled sensitivity checks
have separate denominators. Exact T07 SQL/profile/version/native structures
are frozen before production implementation and must not be regenerated from
product actuals. T07 fixture credentials are environment-only names, not values.
Its `exec_password_env` anchor key passes a password to the mysql client through
Docker's inherited `MYSQL_PWD` environment, never an argv value. T07 fixture
steps mark `requires_absence`, `creates_fixture` and `drops_fixture`; the runner
fails closed on missing prior COUNT0 and never drops a fixture it did not create.
The existing runner chooses `cli_cases` instead of expanding `cli_audit` when
both exist, so T07 explicitly lists its two baseline cases plus 34 semantic
roles (36 list entries). Approved T03/T04/T05 generated promotions are enumerated
in [EXPECTED-UPGRADES](../../docs/work-plans/t07-advanced-columns/EXPECTED-UPGRADES.md);
their SQL, IDs, profiles and unrelated unsupported controls stay unchanged.

## T08 column-governance contract

[T08.json](T08.json) materializes the independent
[`scripts/t08_contract.py`](../../scripts/t08_contract.py) oracle: 171 cases
(8 database baselines, 39 offline CLI, 121 metadata, 3 `cli_error` records —
the 162-role denominator maps to 161 executable cases plus the unit-proven
`err.cancel` role, with the 8 baseline db and 2 baseline CLI dialect cases
added on top). The
contract and versioned/native source distinctions are indexed in
[CONTRACT.md](../../docs/work-plans/t08-column-governance/CONTRACT.md). Exact
T08 SQL/profile/version/native structures are frozen before production
implementation and must not be regenerated from product actuals. T08 reuses
the T07 fixture-credential shape: `exec_password_env` passes the MySQL
password through Docker's inherited `MYSQL_PWD` environment, never an argv
value; the run requires `T08_MYSQL_PASSWORD` to be exported. Approved upgrades
over earlier frozen expectations are enumerated in
[EXPECTED-UPGRADES](../../docs/work-plans/t08-column-governance/EXPECTED-UPGRADES.md).

## Manifest contract

- `task_id`: milestone task id (e.g. `T02`)
- `issue`: originating GitHub issue URL
- `policy_profile`: name recorded into the artifact for the isolated policy
  used by CLI cases (a temporary all-rules-off profile; the repository
  default policy is never modified)
- `description`: human-readable scope
- `anchors`: required database anchors keyed by name. Each anchor declares
  its compose `service`/`container`, pinned `image`, `product`,
  `version_contains` (matched against the server-reported version — a
  mismatch fails the run), target `database`, `exec_client` argv, and
  `needs_database_create`. A missing, unreachable, or unhealthy anchor fails
  the run; it is never skipped.
- `ddl_steps`: ordered DDL steps. Each step has `name`, `sql`, `expect_rc`,
  and `verify` assertions executed as live `information_schema` queries with
  expected scalar results (e.g. table exists, primary key exists, column
  exists, table absent).
- `syntax_negative`: `sql` that must fail for a syntax reason. `expect`
  carries `rc_nonzero`, `error_class` (e.g. `1064`), and `forbidden_markers`
  (connectivity/permission/missing-schema/collision markers that would
  disqualify the negative).
- `cli_audit`: `dialects`, the same `sql` batch, and `expect` on `exit`,
  `verdict`, `statements`, `findings`, `diagnostics`, `unsupported`.
- `cli_cases`: named CLI audit cases (`id`, `dialect`, `sql`, optional
  `args`, optional per-case `policy` profile selection, `expect`). Besides
  the base keys, `expect` may pin `coverage`/`statement_coverage`,
  `unsupported_features`/`unsupported_entries` (with exact bounded
  `metadata` maps), `statement_sql` identity, `evidence_gaps`/
  `evidence_gap_entries` (`index`/`rule_id`/`reason_code` plus exact
  `required_facts`), `finding_entries`/`finding_metadata`/
  `finding_locations`, and `fail_on_triggered`.
- `policy`: optional isolated-policy declaration. `policy.enable` maps rule
  IDs to `{enabled, level, params}`; the runner asserts each enabled rule
  exists in the live `rules list` catalog, renders the isolated profile
  under `policy_profile`'s name plus a generated `all-rules-disabled`
  profile, and records sha256 per policy file into the artifact. Per-case
  `policy` selects which generated profile a case pins. `policy.profiles`
  names additional isolated profiles; a `profiles` key equal to
  `policy_profile` or `all-rules-disabled` fails the run instead of
  silently redefining the default.
- `metadata_cases`: live-database audit cases. Each binds `anchor`,
  `dialect`, `sql`, a `connect` block (`password` travels only via
  `--password-env`/`--password-file` and is never recorded), ordered
  `setup` steps (with `verify` metadata assertions), the same `expect`
  vocabulary as `cli_cases`, `post_verify` assertions proving the audited
  object was not mutated, optional driver-side `execute` steps applying
  the audited SQL natively (each recorded step re-derived by the
  validator; a step may pin `expect` exit/`stderr_contains` when a native
  negative like ERROR 1171 is required), `structure` oracle queries
  pinning the resulting `information_schema` facts, and `teardown`
  steps.
- `error_cases`: CLI invocations expected to fail before producing an audit
  result (for example a real connection refusal). `expect` pins `exit` and
  `stderr_contains` markers only; stdout is never parsed as a result. An
  anchored error case (`anchor` key) additionally records the live observed
  banner into `version_evidence`, and the validator requires it — a
  product/version mismatch can never be laundered into a connection failure
  or lack its observed identity proof (#83 T04-B).
- `metadata_cases` may declare `instance_facts` (`innodb_page_size`,
  `tidb_max_index_length`, `sql_require_primary_key`,
  `sql_generate_invisible_primary_key`,
  `show_gipk_in_create_table_and_information_schema`): the runner live-reads
  each declared fact from the anchor (`SHOW VARIABLES` for MySQL page size
  and the GIPK variables, `SHOW CONFIG` for TiDB `max-index-length`) and the
  validator rejects deleted, emptied, or tampered values — the manifest can
  never substitute for live evidence. Task-scoped auxiliary anchors (e.g.
  `mysql84-4k`, `tidb85-12288`) pin the fact in compose instead of the
  manifest so live reads prove it. GIPK facts prove a server-generated
  invisible primary key was not in play, so a driver-side CREATE cannot
  launder a server-added PK into input-declared evidence (T06-A1).
- `required_case_ids`: exact case IDs that must appear as executed in the
  artifact; any missing required case is a violation.

## Locked baseline

`anchors-baseline.json` is checked in **independently of any task manifest**
and pins the milestone denominator: the four anchors (`mysql:5.7.44`,
`mysql:8.0.46`, `mysql:8.4.10`, `pingcap/tidb:v8.5.0`) and the required CLI
dialects (`mysql`, `tidb`). The runner refuses a manifest that drops or
rewrites a baseline anchor, and the validator requires every baseline anchor's
`db_ddl`/`syntax_negative` cases and every baseline dialect's `cli_audit` case
to have executed — so shrinking a manifest and artifact together still fails.
Editing the baseline is a visible review diff, never a silent shrink.

Database steps and CLI audit are deliberately separate evidence: real
execution proves fixture legality and live metadata; CLI audit proves parser
and audit behavior. Neither substitutes for the other. The artifact validator
recomputes expectations from the manifest and results from raw evidence —
it rejects zero/missing/unexecuted cases, stale binaries, version mismatches,
external blockers, deleted or failed metadata-query records, non-JSON CLI
stdout, parsed/stdout disagreement, artifact-internal expected tampering,
missing or mis-attributed `evidence_gaps` entries (including a gap smuggled
into `findings`), tampered policy files or `enabled_rules`, leaked fixture
passwords in recorded commands, tampered metadata `setup`/`post_verify`/
`teardown` records, and manifests/artifacts shrunk below the locked baseline —
proven offline by `make ddl-golden-validator-test` (`scripts/test_ddl_golden.py`).
