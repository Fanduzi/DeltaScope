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
| D5 | PG17 ordinary-table existing-column generation target/options blockers and RESTART/CYCLE risks with explicit data/current/freshness gaps | Facts `1e8c773d`, rules `23c9f4f9`, native/inventory and final `26dde9ad`; T37; [decision](../decisions/2026-11-18-postgresql-column-generation-transitions.md) | Broader data/expression/current/permission/partition/inheritance and other original F09 dimensions remain open |
| D6 | PG17 statistics/options legality and guarded DROP target proof, preserving underflow and freshness gaps | Facts `2299f216`, rules `65387b5f`, evidence `f97c7398`, corrected final `fa37a41b`; T37; [decision](../decisions/2026-11-18-postgresql-column-tuning-and-drop-guards.md) | Broader table options/kinds, dependencies/permissions and general order remain open |
| D7 | PG17 ordinary-table STORAGE/COMPRESSION validity, real type/system/build facts and shared D5/D6 system-target correction | Facts `4f157146`, rules `631575a6`, evidence `9c0cf73a`, source corrections `d445cc5b`/final `b966f73c`; T37; [decision](../decisions/2026-11-18-postgresql-column-storage-compression-targets.md) | Other F09 dimensions remain open; no complete low-privilege, no-lz4 native platform or global historical-query hardening claim |

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

### D5 delivered column-generation transitions and accepted stage boundary

After 45 PG17.7 shapes, 90 actual offline/metadata-aware CLI probes and six
normalized-spec collapse counterexamples at `aa7f8872`, the operator approved
four enabled-by-default, parameterless rules: blocker
`ddl.pg.alter.column.generation.transition.valid`, blocker
`ddl.pg.alter.identity.sequence_options.valid`, warning
`ddl.pg.alter.identity.restart.warn`, and warning
`ddl.pg.alter.identity.cycle.warn`. [Accepted decision and frozen 24 additive
T37 IDs](../decisions/2026-11-18-postgresql-column-generation-transitions.md).

The accepted stage uses exact fresh PG17 facts for ordinary persistent,
non-inheritance/non-partition tables and dependency-bound stable sequence
parameters. No expression/permission/data analysis, current-counter read or
general PG post-state is authorized. Unknown/stale/ambiguous facts, bounds
changes needing an unread current value, and nullable SET NOT NULL data
proof remain explicit gaps. Existing blanket forbids stay independent; D2
CREATE/ADD COLUMN predicates remain unchanged. Stage boundaries are not
permanent deferrals or completed dimensions.

Fact commit `1e8c773d` preserves guards, duplicate modes/options, int64 values
and START versus RESTART without changing JSON projections; the private
directory and nested definitions are cloned by application/SDK and invalidated
after unapplied related DDL/parser failures. Tests and a native renamed/
mismatched-type sequence oracle validate this seam. The subsequent rule
candidate implements all four shared rules, configuration/catalog, six corpus
pairs and application/SDK/HTTP/MCP parity. A 45-shape/90-CLI candidate replay
retains native outcomes and audit non-execution: known illegal transitions and
options reject, valid ADD/int64 stay pass, CYCLE/RESTART review, START/NO CYCLE/
guard controls add no risk finding; offline proof gaps remain review. Old
blanket forbids remain independent. Build/default+PG tests, default+PG lint,
formatting and inventory/corpus/coverage-catalog gates pass.

The frozen 24 canonical additions now pass: **T37 118/118, 1661 assertions**,
retaining all old 94 case objects/IDs/profiles. Ten owned metadata fixtures
contain 62 independent native steps (31 successes, 31 refusals), post-audit
catalog/data/sequence/sentinel controls, and RESTART sequence-lock evidence.
They pin renamed/smallint sequence binding behind a bigint column, precise
quoted guards, mode/options conflicts, retained generated data, START versus
RESTART and data/current-value/prior-DDL/parser/scope gaps. Partition-member
ADD refuses 42P16; other scoped native successes are not claimed as audit
support. Initial fixture reason-code/first-refusal assertions were corrected,
not deleted or weakened.

Six exact existing column-attribute cells gain selectors: SET GENERATED /
DROP EXPRESSION and RESTART become audited for bounded predicates; NOT NULL,
DEFAULT and identity options are mixed because broader data/expression/current
proof is absent. ADD/DROP IDENTITY keeps audited status. Parent/other cells,
301-row aggregates and the original dimension sets remain unchanged; matrix
is generated.

D5 final delivery is `26dde9adbec574ddaeae5d01b461d15c7fa2065a`, FF-only
merged and normally pushed from `aa7f88723fbc930dacdd451a0a6c772a6617a5a5`.
Final **50 tasks, 3450/3450 cases, 60,695 assertions** all use that exact HEAD,
with nonzero execution and no failed case/assertion. Artifact:
`/tmp/ddl-golden-pg-column-final-26dde9ad/all/artifact.json`. The first all-run
hit the 1800-second tool limit after 45 passing tasks; the unchanged HEAD
completed T51/T52/T53/T54/T58, then reaggregation/validation passed without
mixed-HEAD or zero-execution substitution. Build/default+PG tests/default+PG
lint/format/contracts passed before and after merge; artifact validation was
rechecked after push. Separate read-only verification found no unresolved
P1/P2, matched local/server refs and preserved the original status snapshot.
[Go/PG/lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44228829754),
[TLS E2E CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44228829863) and
Cloudflare Pages succeeded for the exact final SHA. Milestone
`aa7f8872...26dde9ad` has **66 paths**; last evidence task
`23c9f4f9..26dde9ad` has seven. These are historical delivery facts, not
same-HEAD proof for a later docs commit. **F09 and the full goal remain open.**

### D6 column tuning and DROP guards (bounded slice delivered)

At D5-history baseline `4eac7c11`, 72 PG17.7 native shapes and 144 real offline/
metadata-aware CLI calls covered remaining tuning/storage/options/RID/rename/
owner/guard/dependency dimensions; eight normalized collapse pairs prove lost
values/guards. The operator approved default blocker
`ddl.pg.alter.column.statistics.target.valid`, default warning
`ddl.pg.alter.column.statistics.clamp.warn`, default blocker
`ddl.pg.alter.column.options.valid`, and preservation of existing DROP COLUMN
IDs with exact fresh PG targets/guard/no-op/gap correction. MySQL/TiDB and
independent forbids remain unchanged. [Decision and 18 frozen additive T37
IDs](../decisions/2026-11-18-postgresql-column-tuning-and-drop-guards.md).

STATISTICS below -1 refuses; DEFAULT/-1/0..10000 are valid, and >10000 warns
about native clamping rather than blocking valid SQL. Known builtin n_distinct/
n_distinct_inherited SET values and duplicates are checked; unknown/extension,
quoted/nonfinite/unmodeled values remain gaps, and unknown RESET names are not
inherently errors. Existing DROP IF EXISTS false rejection is corrected only
with proven absence; unknown/stale/misbound proof remains review. Source AST
facts precede rules, and generic-notice counts do not substitute for validation.
Fillfactor/access-method copy is corrected without another default predicate.

The fact seam preserves statistics DEFAULT/value, ordered exact option keys/
namespaces and value kinds, finite numeric literals and DROP guard/CASCADE;
quoted payloads/expressions never enter it. No parser expansion or data/
permission/expression/general-PG-state analysis is approved. The shared rules,
existing-ID DROP/no-op correction, config/catalog, unit/application/SDK/HTTP/MCP,
six corpus pairs and bilingual/module docs are implemented. An 80-shape/160-CLI
candidate replay preserves all 72 baseline native SQLSTATEs and audit
non-execution; quoted/nonfinite/overflow inputs remain honest gaps. Three old
offline DROP expectations are corrected from false complete to unverified with
12 additional exact gap assertions: all 118 IDs/profiles/SQL/verdict/finding
checks remain; corrected T37 is 118/118, 1673 assertions. Neither candidate is
final delivery. The 18 frozen additions now pass 136/136, 2168 assertions,
including 51 native actions (36 successes/15 exact refusals) with independent
attribute/file/data/guard/no-execution and ownership-cleanup receipts. All old
118 case objects/profiles remain unchanged. First additive 133/136 fixture
failures are preserved and explained: omitted all-off summary and global
freshness after DROP, not product changes or weaker oracles. Fresh controls
precede DROP; every later gap stays pinned. PG17 default statistics are catalog
NULL, including authored -1, as verified natively and in its catalog manual.
Seven exact subactions across columns/attributes/options bind this evidence;
only multi-action and n_distinct/RESET become honestly mixed, and layout is
copy-only. Original row/subaction denominators and 301 row statuses remain.
The f97 fixed candidate passed 50 tasks/3468 cases/61202 assertions and
validate, but was not merged: independent review found Go-rounded nonzero
Float literals could become modeled zero/subnormal while PG17 strtod refuses
22023. The fact seam now quarantines underflow/minimum-normal boundary values
under the already-approved unmodeled gap, preserving genuine zero and small
normal values. Parser/application/SDK/HTTP/MCP/corpus regression and the same
136 frozen IDs now pass 2186 focused assertions; 59 native steps include 40
successes/19 refusals. No existing check/ID/profile is removed, no new blocker
is added, and no source/HEAD changed during f97 TASK=all. Its artifact remains
historical. Corrected final code **fa37a41b46ffbf280c71114eb85441381c57c85f**
passed all 50 tasks, **3468/3468 cases, 61220 assertions**, with all per-task
artifacts at fa37, nonzero counts and no failure; validate passed. Final
artifact: `/tmp/ddl-golden-pg-column-tuning-final-fa37a41b/all/artifact.json`.
FF-only merged/pushed main from 26dde9ad to fa37; merged-root foundation gates
and final independent read-only checks passed, zero unresolved P1/P2. Accurate
SHA CI [Go/PG/lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44267963771),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44267963701) and
Pages succeeded (all five checks). Main/origin/actual remote all equaled fa37
at verification; complete original workspace snapshot/release worktree and
unrelated services were preserved. `26dde9ad...fa37` milestone scope is 62
paths; `f97c7398..fa37` corrective task is 25, native-evidence task is seven.
The decision record was required and committed with the bounded implementation;
this later historical doc HEAD does not inherit fa37's aggregate attribution.
Other original F09 dimensions continue in the queue, not permanent deferral.

### D7 storage/compression and system targets (bounded slice delivered)

Approved default-on, parameterless blockers:
`ddl.pg.alter.column.storage.valid` and `ddl.pg.alter.column.compression.valid`.
Facts **4f157146** retain exact compression AST, actual typstorage, real negative-
attnum members separately from user known-empty, and lz4 enum known/support.
Shared rules **631575a6** add config/catalog, bounded findings/gaps, unit and
application/SDK/HTTP/MCP, seven corpus pairs and bilingual/module docs. D5/D6
IDs remain; guarded existing ctid is not no-op/absence, and inapplicable deletion/
clamp advice is suppressed without disabling independent forbids or other dialects.
Fixed-type compression default and STORAGE PLAIN toastable text remain legal;
method/native-storage case and Unicode lookalikes stay distinct. Public enum
observation proves build support, not a selected pglz default or pg_config read.
Unknown/stale/misbound/ambiguous/type/build/scope facts remain gaps.

All previous 136 T37 objects/profiles/IDs remain unchanged. The frozen 20 additions
pass **156/156, 2772 assertions**, with **52 native steps: 25 successes/27 exact
refusals** and independent attributes/files/exact data/compressed rows/guard and
owned schema/role cleanup. System ADD IDENTITY is exact 22023, not a guessed
0A000. The NOLOGIN reader's SET ROLE enum probe is not a full low-privilege audit
login or DDL-permission claim; no-lz4 native binaries have not been run, and
negative unit/corpus/source facts are not relabeled as native platform evidence.

Candidate artifact
`/tmp/ddl-golden-pg-column-format-canonical-v2-631575a6/T37/artifact.json`
belongs to **631575a6 + dirty evidence**, not final-HEAD all-task proof. The
partial required-ID and first full 154/156 fixture failures and their exact,
non-weakening corrections remain in the
[decision](../decisions/2026-11-18-postgresql-column-storage-compression-targets.md).
Exactly ten existing cells in two mixed parents gain relevant evidence; all
301 parent/child statuses, original subaction sets and the 19-family/63-row
backlog denominator remain unchanged. Matrix follows the YAML generator.
Read-only review rejected 9c0cf73a for unqualified instance sources: same-named
version/settings and text equality operators could falsify real version/lz4.
That old all-runner was interrupted (143), with owned-only official cleanup and
unchanged source/HEAD until quiescence; it is not a passed aggregate. The minimum
system function/view/operator qualification and committed regression are being
verified. Direct actual-provider native red/green proves both shadow cases;
the same frozen enum-reader now has owned namesake/ordinary-role controls,
**156/156, 2774 assertions**, still 52 native DDL steps, at
`/tmp/ddl-golden-pg-column-format-source-fixed-9c0cf73a/T37/artifact.json`
(9c + dirty fixed source, never a corrected 9c). No rule/default/ID expansion.
The subsequent d445 candidate passed all 50 tasks, 3488/3488 and 61808 assertions
plus validate, but read-only review rejected D7's new attribute/type comparisons:
shadow smallint/integer <> fabricated known-empty; OID equality misbound actual
text storage x as p despite native EXTENDED success. Only those new comparisons
are now system-qualified, with permanent source-query and actual-provider native
red/fixed regressions. The frozen enum-reader retains old checks and adds 0:7 /
p:x controls; focused candidate is 156/156, 2776 assertions at
`/tmp/ddl-golden-pg-column-format-catalog-source-d445cc5b/T37/artifact.json`
(d445 + dirty corrected source), still 52 native steps. d445's full aggregate is
historical, not the corrected HEAD's proof. No source/HEAD changed during its run.
Final **b966f73c76352507a8dcf03b01063e0f43255564** passed all **50 tasks,
3488/3488 cases, 61810 assertions** and validate, with all per-task/CLI builds
at that same SHA, nonzero/unique required IDs, no failed/skipped cases and all
owned cleanup rc0/residual[]. T37 is **156/156, 2776 assertions**, with unchanged
52 native steps (25 success/27 exact refusal). Candidate and merged-root full
build/default+PG tests/contracts/both lint lanes/format/diff passed. Independent
read-only acceptance had zero unresolved P1/P2. From fa37, FF-only merge and
normal push yielded main/origin/actual remote b966. Exact-SHA
[Go/PG/lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44305816088),
[TLS](https://github.com/Fanduzi/DeltaScope/actions/runs/44305816080) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/134807127588) succeeded.
Original complete status, c096 release worktree/branch and 16 unrelated service
name/IDs were retained. Task diff d445..b966 is10 paths; milestone fa37...b966
is68. These results belong to b966, never a later historical-doc HEAD.
**D7 is delivered; F09 and the total goal remain open.**
Other original table options, rename/owner/type, dependency/permission,
inheritance/partition and general-order dimensions continue, not silent deferral.

### D8 PostgreSQL column identity and existence guards

User-approved reuse of three column target IDs and the existing table-existence
ID. `5b96f34f` retains distinct private AST guards and exact relation observations.
`c94a15da` consumes PG17 ordinary-table column facts, preserves separate policy
findings, and gaps stale or unproven cases. Seventeen new T37 cases passed in
the focused replay (173/173, 3390 assertions and 37 native steps, including
typed-table scope and exact relation-binding refusals found in read-only review). The RENAME
child is mixed after evidence binding; the parent and original denominators
stay intact. TYPE/USING, general ordering and action eligibility for other
relation kinds remain in scope for future slices. See the
[decision](../decisions/2026-11-18-postgresql-alter-column-identity.md).

D8 shipped at `8cff67d49058b339f5cea96f85f03a2e548f9418` from base
`b966f73c76352507a8dcf03b01063e0f43255564` via fast-forward and normal push.
The final canonical run passed 50 tasks, 3505/3505 cases and 62429 assertions,
with zero failures or skips. Every task and CLI build used the shipped SHA.
Artifact validation and all owned cleanup passed. T36 passed 59/59 with 660
assertions. T37 passed 173/173 with 3390 assertions, including 37 new native
steps (17 successes and 20 precise refusals). The independent read-only
review, candidate gates and merged-root gates passed. The original workspace,
protected release worktree and 16 unrelated container identities remained
intact. The last task changed seven paths and the milestone changed 59.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44360977800)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44360977946)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135002167971)

Final artifact: `/tmp/ddl-golden-pg-column-identity-final-8cff67d4/all/artifact.json`.
Task counts and CI refer to the shipped product SHA. The three interrupted
candidates remain historical. Other F09 dimensions stay open.

### D9 PostgreSQL replica identity index attachment

Approved default blocker for exact ordinary persistent non-typed PG17 table
bindings. `69cf55bd` binds private catalog evidence and `97fa6b64` implements
native attachment eligibility. T37 retains all 173 earlier cases and adds 16
frozen IDs. The focused candidate passed 189/189 with 3922 assertions and 31
native controls. This adds evidence only to the original REPLICA IDENTITY
cell. Runtime usability was left open by the approved scope, as were other
F09 options/type/rename-owner and general dependency/order dimensions.
See the [decision](../decisions/2026-11-18-postgresql-replica-identity-index.md).

D9 shipped at `cb2e7c360944a8716a20a8c73c378bc25440ae24` from base
`8cff67d49058b339f5cea96f85f03a2e548f9418` via fast-forward and normal push.
The exact SHA passed 50 canonical tasks, 3521/3521 cases and 62961 assertions,
with zero failures or skips. T37 passed 189/189 with 3922 assertions and 31
native steps (15 successes and 16 precise refusals). Validation, independent
read-only review, candidate and merged-root gates, and owned cleanup passed.
The original workspace status, protected worktrees and 16 unrelated container
identities remained intact. The final task changed seven paths and the
milestone changed 41.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44372672858)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44372672933)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135043622559)

Final artifact: `/tmp/ddl-golden-pg-replica-index-final-cb2e7c36/all/artifact.json`.
Coverage is limited to index attachment. Replication usability and other F09
dimensions remain open.

### D10 PostgreSQL heap table SET and RESET

Approved default blocker for the two bounded heap SET parameters and valued
RESET. Facts commit `1d088d18` and rule commit `6efd1dce` preserve argument
presence, exact order and private bounded facts. Focused T37 passed 203/203 and
4352 assertions, including 27 native controls. Existing profiles and cases
stay frozen. The option cell gains exact PG17 evidence while broader table
kinds, namespaces, dependencies and other F09 options remain in the queue.
See the [decision](../decisions/2026-11-18-postgresql-table-reloptions.md).

D10 shipped at `61cb93b1c69840281d75f5ee4f58e0ddf1f996b0` from
`cb2e7c360944a8716a20a8c73c378bc25440ae24` via fast-forward and normal push.
The fixed SHA passed 50 canonical tasks, 3535/3535 cases and 63391 assertions,
with zero failures or skips. T37 passed 203/203 with 4352 assertions and 27
native steps (16 successes and 11 precise refusals). The validator, independent
read-only review, candidate and merged-root gates, and owned cleanup passed.
The original workspace, protected worktrees and 16 unrelated container
identities remained intact. The final task changed seven paths and the
milestone changed 45.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44431053316)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44431053280)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135248351056)

Final artifact: `/tmp/ddl-golden-pg-table-reloptions-final-61cb93b1/all/artifact.json`.
The bounded heap declarations are delivered. Other F09 dimensions stay open.

### D11 PostgreSQL single-table tablespace destinations

Approved default blocker for proven missing destinations and `pg_global` on
exact fresh ordinary persistent non-typed nonshared and nonmapped PG17 tables.
`34ec33f6` binds the private observation, including effective current space,
and `c319179e` implements the rule and truthful same-space notice. Focused T37
passed 213/213 with 4655 assertions and 11 native controls. Only the original
SET TABLESPACE evidence cell changes. Bulk, permissions and lock conditions
remain in the queue. See the
[decision](../decisions/2026-11-18-postgresql-table-tablespace.md).

D11 shipped at `42259221dd283c5a46f87934a9806716ad44fae2` from
`61cb93b1c69840281d75f5ee4f58e0ddf1f996b0` via fast-forward and normal push.
The final fixed SHA passed 50 canonical tasks, 3545/3545 cases and 63694
assertions, with zero failures or skips. T37 passed 213/213 with 4655 assertions
and 11 native steps (8 successes and 3 precise refusals). Validator, independent
read-only review, candidate and merged-root gates, and owned cleanup passed.
The original workspace, protected worktrees and 16 unrelated container
identities remained intact. The final task changed seven paths and the
milestone changed 39.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44443726717)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44443726908)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135290776391)

Final artifact: `/tmp/ddl-golden-pg-table-tablespace-final-42259221/all/artifact.json`.
These results belong to the shipped D11 SHA. Bulk movement and the other F09
dimensions stay open.

### D12 PostgreSQL ordinary-table access method destinations

Approved default blocker for observed missing or wrong-kind methods on fresh,
exact qualified ordinary persistent non-typed, nonshared and nonmapped PG17
tables. `e64fd4bb` binds the exact named or effective DEFAULT observation.
`683ffd81` adds the shared blocker and preserves the warning level with
same-method wording. Focused T37 passed 223/223 and 4943 assertions with eleven
native controls. Only the original SET ACCESS METHOD evidence cell changes.
See the [decision](../decisions/2026-11-18-postgresql-table-access-method.md).

D12 shipped at `28de6dc545bb007246513f606e9ed802903515f4` from
`42259221dd283c5a46f87934a9806716ad44fae2` via fast-forward and normal push.
The final SHA passed 50 canonical tasks, all 3555 cases and 63982 assertions,
with zero failures or skips. T37 passed 223/223 with 4943 assertions and 11
native steps, including seven successes and four precise refusals. Validator,
independent read-only review, candidate and merged-root gates, and owned
cleanup passed. Worktrees, original workspace status and 16 unrelated
container identities were preserved. The final task changed seven paths and
the milestone changed 39.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44465411159)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44465411176)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135361995843)

Final artifact: `/tmp/ddl-golden-pg-table-access-method-final-28de6dc5/all/artifact.json`.
These results belong to the shipped D12 SHA. Other F09 dimensions stay open.

### D13 PostgreSQL ordinary-table persistence transitions

`a8767e9c` binds exact ordinary permanent/unlogged PG17 table evidence and
`dc2a3bbb` adds the approved default blocker. It covers direct publications
and direction-specific foreign-key refusals, with same-persistence advice.
Focused T37 passed 236/236 and 5333 assertions with fourteen native controls.
Only the original SET LOGGED/UNLOGGED cell gains selectors. Other kinds,
permissions, locking and runtime replication remain open. See the
[decision](../decisions/2026-11-18-postgresql-table-persistence.md).

D13 shipped at `33f87fd76b8b7e8e780d0165f3df77ec1c8ae5f9` from
`28de6dc545bb007246513f606e9ed802903515f4` via fast-forward and normal push.
The exact SHA passed 50 canonical tasks, all 3568 cases and 64372 assertions,
with zero failures or skips. T37 passed 236/236 with 5333 assertions and 14
native steps, including ten successes and four precise refusals. Validator,
independent read-only review, candidate and merged-root gates, and owned
cleanup passed. Original workspace status, protected worktrees and 16
unrelated container identities were preserved. The final task changed seven
paths and the milestone changed 40.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44477122026)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44477122064)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135400501667)

Final artifact: `/tmp/ddl-golden-pg-table-persistence-final-33f87fd7/all/artifact.json`.
These results belong to the shipped D13 SHA. Other F09 dimensions stay open.

### D14 PostgreSQL ALTER COLUMN TYPE source preconditions

`588f0cd1` uses existing PG17 column and AST facts for the approved source
blocker. It checks exact missing or system sources and rejects USING on a
stored generated source. Focused T37 passed 245/245 and 5589 assertions with
nine native controls. Only the original TYPE and TYPE USING anchors gain
evidence. Destination types, expressions, dependencies and rewrite outcomes
remain open. See the
[decision](../decisions/2026-11-18-postgresql-column-type-source.md).

D14 shipped at `7ba14c4c9ed75444f269f15a25ad63942233a9a6` from
`33f87fd76b8b7e8e780d0165f3df77ec1c8ae5f9` via fast-forward and normal push.
The exact SHA passed 50 canonical tasks, all 3577 cases and 64628 assertions,
with zero failures or skips. T37 passed all 245 cases and 5589 assertions.
Nine native steps included four successes and five precise refusals. Validator,
independent read-only review, candidate and merged-root gates, and owned cleanup
passed. Original workspace status, protected worktrees and 16 unrelated
container identities were preserved. The final task changed seven paths and
the milestone changed 26.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44487916234)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44487916208)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135436041896)

Final artifact: `/tmp/ddl-golden-pg-column-type-source-final-7ba14c4c/all/artifact.json`.
These results belong to the shipped D14 SHA. Destination types, conversion and
other F09 dimensions remain open.

### D15 PostgreSQL ordinary-table rename and schema destinations

`35157dfe` binds the primary destination namespace, relation and row-type
observations. `c71c189a` checks proven destination conflicts under the two
approved defaults and preserves the existing rename prohibition. Focused T37
passed 258/258 and 6001 assertions, including seventeen native steps. Only
RENAME TO and SET SCHEMA gain evidence. Dependent moves and OWNER TO remain
open. See the
[decision](../decisions/2026-11-18-postgresql-table-namespace-destination.md).

D15 shipped at `a303a8b414b8179b9371cc3a6924019f39464cc8` from
`7ba14c4c9ed75444f269f15a25ad63942233a9a6` via fast-forward and normal push.
The fixed SHA passed 50 canonical tasks, all 3590 cases and 65040 assertions,
with zero failures or skips. T37 passed all 258 cases and 6001 assertions.
Seventeen native steps included nine successes and eight precise refusals.
Validation, independent review, candidate and merged-root gates, and owned
cleanup passed. Original workspace status, protected worktrees and 16 unrelated
container identities were preserved. The final task changed seven paths and
the milestone changed 41.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44500330984)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44500331544)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135478226302)

Final artifact: `/tmp/ddl-golden-pg-table-namespace-destination-final-a303a8b4/all/artifact.json`.
Dependent moves and other F09 dimensions remain open.

### D16 PostgreSQL ordinary-table owner targets

`b0f20d4a` preserves exact RoleSpec and bound owner facts privately. `21ef828e`
checks the approved absent-owner and PUBLIC refusals, with contextual session
sameness in the existing notice. Focused T37 passed 270/270 and 6362 assertions,
including thirteen native controls. Only OWNER TO gains evidence. Permission,
execution and the other relation kinds remain open. See the
[decision](../decisions/2026-11-18-postgresql-table-owner-target.md).

Delivered at `1a03cd0d3ff1d89f8a37fb70149dbae1635c8b9e` with all 50 canonical
tasks, 3,602/3,602 cases and 65,401 assertions. Validator, merged-root gates
and all five CI checks passed. [Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44542728143),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44542728376),
[Pages](https://github.com/Fanduzi/DeltaScope/runs/135625272372).

### D17 PostgreSQL bulk table tablespace targets

`bb9f3050` records exact endpoint and ordered role facts privately. `72472c0e`
adds the PG17 blocker and corrects the bulk notice to describe the transactional
request. T37 focused passed 282/282 cases and 6,979 assertions. Fifteen native
commands supplied four successes and eleven exact refusals. Thirteen selectors
bind the original ALL IN TABLESPACE cell. Execution rights, broader object
selection, capacity, physical moves and current locks remain open. See the
[decision](../decisions/2026-11-19-postgresql-bulk-table-tablespace-targets.md).

Delivered at `6b394bd8645b428114f16d96e8451d8217c9b72a` with all 50 canonical
tasks, 3,614/3,614 cases and 66,018 assertions. Validator and all five CI checks
passed. [Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44561730379),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44561730094),
[Pages](https://github.com/Fanduzi/DeltaScope/runs/135692783952).

### D18 PostgreSQL column type destinations

`36b6957a` binds exact private type and array catalog observations after the D14
source premise. `e124c553` checks approved direct destination failures.
Focused T37 passed 299/299 cases and 7,604 assertions. Eighteen native
commands produced six successes and twelve exact refusals. Seventeen cases
and one corpus selector attach to the existing TYPE and TYPE USING cells.
Conversion, modifiers, collations, dependencies, permissions, actual rewrites
and wider identity resolution remain open. See the
[decision](../decisions/2026-11-19-postgresql-column-type-destination.md).

Delivered at `1bc8709c7f18072ca24f1f5d597cb3a5403e88df` with 50 canonical tasks,
3,631/3,631 cases and 66,643 assertions. Validator, merged-root gates and all
five CI checks passed. [Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44622791333),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44622791505), and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/135908765356).

### D19 PostgreSQL named CHECK constraints

`99162180` binds the existing constraint catalog reads to system comparison
operators. `9055f9fd` checks exact same-table CHECK name occupation. The
scoped CHECK no longer reaches the existing case-folded index-name rule.
Focused T37 passed 309/309 and 7,870 assertions with ten native commands,
six successful and four precise `42710` refusals. Six evidence references
attach to each original CHECK cell. Expression legality, existing rows,
inheritance and wider table shapes retain separate evidence needs.
[Decision](../decisions/2026-11-19-postgresql-check-constraint-name.md)

Delivered at `64a1fd4e77556f54dce08f6f84ce57ba73ab92cd` with 50 canonical tasks, 3,641/3,641 cases and 66,909
assertions. Validator, merged-root gates and all five CI checks passed.
[Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44652137227),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44652137340) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/136009522204).

### D20 PostgreSQL DROP NOT NULL replica identity keys

`f8695f4d` observes the live attached index's exact key slots with the column
catalog. `ef8836ab` extends the existing PG17 column-transition blocker.
Focused T37 passed 317/317 and 8,105 assertions, with four native successes
and four exact refusals. Nine selectors bind the original SET/DROP NOT NULL
cell. INCLUDE slots, FULL and unattached indexes have independent controls.
Existing-row SET NOT NULL validation and wider relation kinds retain their
own evidence requirements.
[Decision](../decisions/2026-11-20-postgresql-drop-not-null-replica-key.md)

Delivered at `5799cd4d61f6145803f4bf65e78975472aa1a444` with 50 canonical tasks, 3,649/3,649 cases and 67,144
assertions. Validator, merged-root gates and all five CI checks passed.
[Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44663528166),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44663528266) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/136047329510).

### D21 PostgreSQL constraint attachment using an index

`4f91cc2f` binds the exact index privately. `a47025da` checks the approved
attachment premises and corrects the existing name check for same-name reuse.
Focused T37 passed 336/336 and 8,751 assertions with eighteen native commands.
Five selectors attach to ADD PRIMARY KEY and fifteen to ADD UNIQUE. Default
operator classes, data, permissions, dependent effects and wider table shapes
retain separate evidence requirements.
[Decision](../decisions/2026-11-20-postgresql-constraint-using-index.md)

Delivered at `53cc0592051ceca225b1463d69b1d8b0ce1ebf3c` with 50 canonical tasks, 3,668/3,668 cases and 67,790
assertions. Validator, merged-root gates and all five CI checks passed.
[Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44685529669),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44685529712) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/136118934239).

### D22 PostgreSQL foreign key targets

`d4ccb0c0` retains the exact foreign key identities and attaches private
reference facts. `d0501883` checks relation kind and the declared columns
on both sides. Focused T37 passed 354/354 and 9,344 assertions with 17 native
commands. Nineteen selectors attach to ADD FOREIGN KEY. Referenced key
eligibility, conversion, row validity, permissions and wider table shapes
have separate evidence requirements.
[Decision](../decisions/2026-11-20-postgresql-foreign-key-targets.md)

Delivered at `529d10b1b548a277ca74eb1200b2b1380ae8f721` with 50 canonical tasks, 3,686/3,686 cases and 68,383
assertions. Validator, merged-root gates and all five CI checks passed.
[Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44694809416),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44694809506) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/136149005473).

### D23 PostgreSQL foreign-key referenced keys

`83577403` reads private key facts for the exact referenced ordinary table.
`75319194` extends the existing blocker with eligibility, repeated referenced
columns and arity. Focused T37 passed 368/368 and 9,807 assertions with twelve
native commands. Fifteen selectors attach to ADD FOREIGN KEY. Casts,
operator-class/equality compatibility, referential actions, permissions,
row validity and wider relation shapes retain separate evidence requirements.
[Decision](../decisions/2026-11-20-postgresql-foreign-key-referenced-keys.md)

Delivered at `c34c6e8a35121d751516c3dee228d3ed4da5e783` with 50 canonical tasks, 3,700/3,700 cases and 68,846
assertions. Validator, merged-root gates and all five CI checks passed.
[Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44737041836),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44737041754) and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/136291155785).

### D24 PostgreSQL direct foreign-key dependencies on constraint drops

`508d69e3` observes direct foreign-key dependencies on an exact supporting
index. `cfd0b1ab` uses them for PostgreSQL 17 DROP CONSTRAINT RESTRICT on an
ordinary table. T37 focused passed 377/377 and 10,087 assertions. Ten
selectors attach to DROP CONSTRAINT. Broad dependency closure, inheritance,
privileges, locks and CASCADE effects retain distinct premises.
[Decision](../decisions/2026-11-20-postgresql-drop-constraint-foreign-key.md)

Delivered at `a9b8065dd35959760529cc01e2a7edabd66b948c` with 50 canonical tasks, 3,709/3,709 cases and 69,126
assertions. All task receipts use the same CLI build hash. Docker Desktop
port-forwarding conflicts interrupted the first run. The first isolated
attempt stopped after an image-tag preflight failure. In the final isolated
run, T24 needed write access to its generated evidence directory. T24 then
passed 24/24 with 292 assertions using the original CLI hash. The reaggregated
artifact passed the validator, the independent read-only review and cleanup
checks. All 16 protected Docker identities and the release worktree matched
baseline. Fourteen other Desktop entries were absent at the final snapshot.
Five removal events were captured, with nine entries left unattributed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44769974126/job/136404691433),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44769974126/job/136404692266),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44769974126/job/136404691899),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44769973882/job/136404690740),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136404906482) passed after the merged-root gates.

### D25–D26 PostgreSQL inherited CHECK drops and partition-parent ONLY

`4f9d49e7` observes exact CHECK inheritance and child presence and retains
ALTER traversal. `9695769e` extends the existing DROP target blocker. An
inherited CHECK refuses direct DROP, including locally merged CHECKs and
requests with IF EXISTS or CASCADE. For a local CHECK that propagates,
ONLY on a partitioned parent with direct children has a separate reason.
The rule records unavailable inheritance or child facts as evidence gaps.

The temporary T37 preview passed 395/395 with 10,655 assertions. Its 17
added native commands returned eight successes and nine `42P16` refusals.
Two earlier batches keep their original SQL and native controls and now
assert the precise stale-inheritance gap. Evidence adds 17 selectors to
DROP CONSTRAINT and three to DROP CONSTRAINT CASCADE at PostgreSQL 17.
Foreign-table and non-CHECK inheritance, recursive dependency closure,
permissions and locks remain separate premises. The final commit passed
verification.
The first fixed run at `6c351ff3` was interrupted to update the SDK
metadata-provider contract for the new `PGConstraintState` fields. The
next candidate, `71552bf3`, was interrupted after native PostgreSQL 17.7
confirmed that a detach-pending child is skipped by the ONLY traversal.
The provider excludes these catalog entries. The attached-child blocker
still matched the native refusal in a paired CLI comparison. Both stopped
runs have owned Compose cleanup receipts. The corrected candidate ran
a new full verification from an exact bundled checkout in the guest.
[D25](../decisions/2026-11-20-postgresql-drop-inherited-check.md)
[D26](../decisions/2026-11-20-postgresql-drop-check-only.md)

Delivered at `7af6f84ace0fcd97c44f6fd4bbe9f3676a501415` with 50 canonical tasks, 3,727/3,727 cases and 69,694 assertions. All tasks used the same CLI hash. The artifact validator, independent read-only review, owned cleanup and merged-root gates passed. T24 generated its deployment evidence in the guest checkout and passed its 24 cases without a rerun. The PostgreSQL 17 T37 run passed 395/395 with 10,655 assertions. The detach-pending comparison adds independent native evidence to the D26 decision record.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44782282469/job/136447838317),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44782282469/job/136447837864),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44782282469/job/136447838373),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44782282463/job/136447837391),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136448028882).

### D27 PostgreSQL direct DROP COLUMN dependencies

`6819eaa3` observes direct dependencies on an exact user column on a fresh
ordinary PostgreSQL 17 table. `26e53477` extends the existing DROP COLUMN
blocker under RESTRICT. The checks cover views, materialized views, stored
generated columns and referencing foreign keys. A self-referential foreign
key using the same source column follows PostgreSQL's automatic removal path.
Missing applicable dependency proof produces `column.pg_dependencies`.

The T37 preview passed 409/409 with 11,137 assertions. Thirteen new native
commands returned six successes and seven `2BP01` refusals. Fifteen evidence
selectors attach to the PostgreSQL 17 DROP COLUMN subaction. The earlier
tuning batch retains its SQL and native controls and now asserts the stale
dependency gap at statement index 11. CASCADE effects, wider relation kinds
and indirect dependencies remain separate. The fixed candidate passed verification.
[Decision](../decisions/2026-11-21-postgresql-drop-column-dependencies.md)

Delivered at `b6d73f0407958d6bc6747a5f5b406e0b501dc9fd` with 50 canonical tasks, 3,741/3,741 cases and 70,176 assertions. The validator, independent read-only review, owned cleanup and merged-root gates passed. T37 passed 409/409 with 11,137 assertions. All five CI checks succeeded at the delivered SHA.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44790017575/job/136474898816),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44790017575/job/136474899214),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44790017575/job/136474899143),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44790017587/job/136474898518),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136475059050).

### D28–D30 PostgreSQL schema moves and column type prerequisites

`576c9a2a` retains exact type modifiers and collation AST facts and reads
schema companion and type prerequisites from PostgreSQL 17 catalogs.
`f0f7b0b8` extends the existing blockers for three independent premises.

- D28 checks associated array types, attached indexes and column-owned
  sequences for name collisions during SET SCHEMA. It adds 12 selectors
  to that subaction. [Decision](../decisions/2026-11-21-postgresql-table-schema-companions.md)
- D29 checks exact SET WITHOUT OIDS target kind and guarded absence.
  The PostgreSQL 17 compatibility operation still has no column effect.
  Four selectors bind the subaction's mixed evidence. [Decision](../decisions/2026-11-21-postgresql-set-without-oids.md)
- D30 checks bounded character and numeric type modifiers, qualified
  collations and scalar assignment for TYPE without USING. Existing-value
  conversion and USING expressions require separate evidence. Fifteen
  selectors attach to TYPE and TYPE USING. [Decision](../decisions/2026-11-21-postgresql-column-type-coercion.md)

T37 combined preview passed 437/437 with 12,138 assertions and 30 added
native commands. Three existing expectations record the index conflict,
missing offline version and unresolved domain assignment. The previous
SQL, profiles, case IDs and native controls were preserved. The 301 parent
classifications and 19-family/63-row denominator remain in place. The
fixed candidate passed verification.

Delivered at `36ef07e478a6883ea0214b48819be72c231abf99` with 50 canonical tasks, 3,769/3,769 cases and 71,177 assertions. T37 passed 437/437 with 12,138 assertions. The aggregate validator, independent read-only review, owned cleanup and merged-root gates passed. All five CI checks succeeded at the delivered SHA.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44797329748/job/136500156332),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44797329748/job/136500156270),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44797329748/job/136500155755),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44797329708/job/136500155747),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136500314759).

### D31–D33 PostgreSQL constraint declarations and column traversal

`acd669f7` adds exact PostgreSQL 17 catalog facts and retains authored
ON DELETE SET columns and RENAME traversal. `f2e0aeb5` extends the existing
blockers with three bounded checks.

- D31 checks the selected index key's operator classes for PRIMARY KEY
  or UNIQUE USING INDEX attachment. A nondefault installed class or a
  proved mismatch with the unique exact-type default blocks. Binary
  compatibility and polymorphic default selection retain their evidence
  gap. Five references bind the two constraint cells.
  [Decision](../decisions/2026-11-21-postgresql-constraint-default-opclass.md)
- D32 checks the exact ON DELETE SET NULL or SET DEFAULT column subset,
  including NOT VALID declarations. Native duplicates are accepted.
  Missing, system and out-of-key columns have separate reasons. Seven
  references bind ADD FOREIGN KEY.
  [Decision](../decisions/2026-11-21-postgresql-foreign-key-delete-set-columns.md)
- D33 checks direct inherited DROP and RENAME, direct partition keys for
  DROP and attached-child traversal for ONLY. Pending detach entries are
  excluded. Fourteen references bind DROP COLUMN, guarded DROP COLUMN
  CASCADE and RENAME COLUMN.
  [Decision](../decisions/2026-11-21-postgresql-column-traversal.md)

T37 combined preview passed 459/459 with 12,904 assertions. Twenty-two
added native commands yielded eight successes and fourteen refusals. The
legacy scope case preserves its SQL and native checks and expects the
proved inherited RENAME refusal at index zero. The first combined preview
stopped before case execution when PostgreSQL's published port was occupied.
After owned cleanup and port release, the full focused preview passed.
Inventory adds 26 selectors with the same parent classifications and the
original 19-family and 63-row denominator. The fixed candidate passed verification.

Delivered at `b49be2d43ada8a35ae1dad7da520894cbe0c3c66` with 50 canonical tasks, 3,791/3,791 cases and
71,943 assertions. T37 passed 459/459 with 12,904 assertions. The validator,
independent read-only review, owned cleanup and merged-root gates passed.
Fifteen historical Docker Desktop container identities matched. The
running `h1run21-restore` identity changed from `2debbf4304e1` to
`d81e18eb58ab` on Docker Desktop, separate from the Golden guest daemon.
Available records did not identify the actor. The owned guest ended with
zero containers, Golden networks and Golden volumes.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44806793504/job/136532798473),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44806793504/job/136532798263),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44806793504/job/136532798445),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44806793432/job/136532797997),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136532937032) succeeded at the delivered SHA.

### D34–D36 PostgreSQL TYPE dependencies and inherited column actions

`226e8b73` binds applicable column dependencies and traversal facts.
`208ec31b` extends the existing blockers for three independent premises.

- D34 checks direct view, materialized-view and stored generated-column
  dependencies after the exact TYPE destination prerequisites pass.
  Missing applicable facts produce `column.pg_type_dependencies`.
  [Decision](../decisions/2026-11-21-postgresql-column-type-dependencies.md)
- D35 checks inherited sources and direct partition keys for TYPE and TYPE
  USING. It checks direct inherited DROP EXPRESSION and an ONLY request
  that needs attached-child traversal.
  [Decision](../decisions/2026-11-21-postgresql-inherited-type-and-drop-expression.md)
- D36 checks direct partition ADD, ordinary-parent traversal and identity
  ADD on ordinary inheritance parents. Eligible IF NOT EXISTS duplicates
  retain the no-op. The finding uses the observed column presence.
  [Decision](../decisions/2026-11-21-postgresql-add-column-traversal.md)

The corrected preview passed 481/481 with 13,697 assertions. Twenty-two
added native commands returned five successes and seventeen refusals.
The existing TYPE scope case retains its SQL and native refusal and now
expects the direct partition-key finding. Inventory adds 25 references
on six exact PostgreSQL 17 cells. The parent classifications and original
19-family and 63-row denominator stay in place. The fixed candidate passed verification.

Delivered at `441e6b3462cbe4835e935df42471725d37dbbcf4` with 50 canonical tasks, 3,813/3,813 cases and
72,736 assertions. T37 passed 481/481 with 13,697 assertions. T24 passed
24/24 with 292 assertions. The validator, read-only review and merged-root
gates passed. The isolated guest ended with zero containers, Golden
networks and Golden volumes.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44815378603/job/136562222365),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44815378603/job/136562222048),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44815378603/job/136562222158),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44815378506/job/136562221652),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136562352876) succeeded at the delivered SHA.

### D37–D39 PostgreSQL foreign-key equality and constraint traversal

`90be4c87` observes bounded scalar equality, CHECK validation traversal
and partition nullability. `a1809635` consumes those facts under the
existing blocker IDs.

- D37 checks equality operators or implicit casts for the selected
  reference key and ordered FK columns. Domain, array and polymorphic
  cases require additional facts.
  [Decision](../decisions/2026-11-21-postgresql-foreign-key-scalar-equality.md)
- D38 checks whether validating an exact unvalidated CHECK with ONLY
  requires attached children. Existing row validity remains a separate
  requirement.
  [Decision](../decisions/2026-11-21-postgresql-validate-check-only.md)
- D39 checks partition-parent NOT NULL requirements and ONLY traversal
  for SET and DROP NOT NULL. Data-dependent SET retains its evidence gap.
  [Decision](../decisions/2026-11-21-postgresql-partition-nullability.md)

The joint T37 preview passed all 505 cases and 14,539 assertions. The 24
added cases exercised 25 native commands. Twenty-seven selectors bind
three exact PostgreSQL 17 subactions. The fixed candidate passed verification.

Delivered at `683d84acd94aada39c5e1982ae4849eef8e8f489` with 50 canonical tasks, 3,837/3,837 cases and
73,578 assertions. T37 passed 505/505 with 14,539 assertions. T24 passed
24/24 with 292 assertions. The validator, read-only review and merged-root
gates passed. The isolated guest ended with zero containers, Golden
networks and Golden volumes.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44822676180/job/136587283195),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44822676180/job/136587283471),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44822676180/job/136587283302),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44822676034/job/136587282297),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136587444478) succeeded at the delivered SHA.

### D40–D43 PostgreSQL defaults, key types and TYPE ONLY

`d4e6e681` checks column references and subqueries in SET DEFAULT after
checking the target. `b7f9a436` checks native default operator class selection
for constraint attachment and adds bounded foreign-key type comparisons for
domains, enums and arrays. `0ccabf37` checks TYPE ONLY after proving the exact
destination and scalar assignment.

- [D40 SET DEFAULT decision](../decisions/2026-11-21-postgresql-set-default-expression-references.md)
- [D41 operator class decision](../decisions/2026-11-21-postgresql-constraint-default-opclass-fallback.md)
- [D42 foreign-key type decision](../decisions/2026-11-21-postgresql-foreign-key-domain-enum-array.md)
- [D43 TYPE ONLY decision](../decisions/2026-11-21-postgresql-column-type-only-traversal.md)

The isolated preview passed 535/535 T37 cases and 15,589 assertions. It added
30 native controls, with 14 successes and 16 refusals. Three earlier cases
now consume known facts in place of their former gaps. Their SQL and native
controls are identical. Thirty-two selectors bind six PostgreSQL 17 cells.
The fixed candidate passed verification.

Delivered at `243894152ccb4bc10f77448133d2d9e2f27c3d44` with 50 canonical tasks, 3,867/3,867 cases and
74,628 assertions. T37 passed 535/535 with 15,589 assertions. T24 passed
24/24 with 292 assertions. The validator, read-only review and merged-root
gates passed. The isolated guest ended with zero containers, Golden
networks and Golden volumes.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44830434298/job/136613697694),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44830434298/job/136613697942),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44830434298/job/136613697877),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44830434310/job/136613698161),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136613827571) succeeded at the delivered SHA.

### D44–D47 PostgreSQL descendant and CHECK checks

- D44 checks recursive RENAME COLUMN for a destination collision or an
  inherited definition from outside the selected descendant tree.
  [Decision](../decisions/2026-11-21-postgresql-recursive-column-rename.md)
- D45 checks direct catalog dependencies that block column type changes
  after the bounded conversion prerequisites are established.
  [Decision](../decisions/2026-11-21-postgresql-column-type-additional-dependencies.md)
- D46 checks subqueries in single ADD CHECK predicates, including unnamed
  constraints and NOT VALID. The target guard and existing name check apply.
  [Decision](../decisions/2026-11-21-postgresql-check-predicate-subqueries.md)
- D47 checks outside inheritance for a recursive scalar TYPE change after
  destination and assignment checks.
  [Decision](../decisions/2026-11-21-postgresql-column-type-outside-inheritance.md)

The corrected joint preview passed 560/560 T37 cases with 16,507 assertions.
Twenty-five new native controls returned nine successes and sixteen refusals.
The first preview's privacy assertion matched a substring of the authored
schema. The corrected fixture uses a distinct private relation name and keeps
its SQL and native execution control. The earlier recursive rename case keeps
its SQL and native control and now expects the observed catalog result. The
inventory adds 29 selectors to five PostgreSQL 17 subactions. The fixed candidate passed verification.

Delivered at `df33755efbff25997336842fae0e17c101672c2d` with 50 canonical tasks, 3,892/3,892 cases and
75,546 assertions. T37 passed 560/560 with 16,507 assertions. T24 passed
24/24 with 292 assertions. The validator, read-only review and merged-root
gates passed. The isolated guest ended with zero containers, Golden
networks and Golden volumes.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44837862287/job/136638653780),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44837862287/job/136638654282),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44837862287/job/136638654247),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44837862258/job/136638653894),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136638814192) succeeded at the delivered SHA.

## Family index

Family IDs preserve the original ledger order, not implementation priority.
Next scope review is **F09**, then F10/F12 by measured value and model readiness;
new default behavior is approved per family before implementation.

| ID | Family | Rows | Canonical ownership | Initial state |
| --- | --- | ---: | --- | --- |
| F01 | MySQL stored routines | 5 | T27/T28 | Stored routine target and guard checks delivered at `a2f73b0c`. Runtime effects and unmodeled bodies retain evidence boundaries. |
| F02 | MySQL trigger/event lifecycle | 5 | T29/T30 | Qualified trigger and event checks delivered at `d49b805c`. Invocation and scheduler effects retain boundaries. |
| F03 | MySQL/TiDB columns/generated/multi-action | 6 | T03/T05/T07/T08 | Repeated ADD declarations and the bounded column evidence delivered at `cec48651`. Expression and execution limits remain recorded. |
| F04 | MySQL/TiDB index lifecycle | 3 | T09/T10 | Standalone index scope delivered at `3b0cd01b`. Positive foreign key dependency evidence and TiDB guarded drops are covered. |
| F05 | MySQL/TiDB ALTER options/online DDL | 3 | T15 | Bounded ALTER option and INSTANT table premises delivered at `07f4aca5`. |
| F06 | TiDB recovery/flashback | 1 | T16 | Exact TiDB 8.5 recovery boundaries verified and delivered at `0b485f88`. |
| F07 | PostgreSQL schema/collation | 2 | T35 | Qualified schema creation and collation checks delivered at `3eba78f6`. Embedded schema elements and wider provider and execution facts remain open. |
| F08 | PostgreSQL CREATE TABLE | 6 | T36 | Qualified declaration checks delivered at `10354d2b` alongside D2. The exact dimension review lists remaining catalog, parser and execution premises. |
| F09 | PostgreSQL ALTER TABLE | 5 | T37 | D2–D47 bounded ALTER checks delivered; authored and normalized dimensions remain open where catalog or expression facts are incomplete |
| F10 | PostgreSQL DROP/TRUNCATE | 2 | T38 | Qualified target and RESTRICT dependency slice delivered at `b5cf88ce`. Wider boundaries remain in the dimension review. |
| F11 | PostgreSQL indexes/operator families | 4 | T39 | Qualified index and operator-family checks delivered at `47cf3020`. Wider key expressions, predicates and method-specific members remain open. |
| F12 | PostgreSQL views/materialized views | 5 | T40 | Qualified lifecycle, options, refresh and bounded view output and dependency checks delivered through `919f1d17`. Remaining premises below. |
| F13 | PostgreSQL types/operators | 3 | T41 | Qualified declaration checks delivered at `fd6b2792`. The exact dimension review records remaining catalog and execution premises. |
| F14 | PostgreSQL row-security policy | 1 | T42 | Qualified policy target, role and clause checks delivered at `9308fb4c`. General predicate and effective access facts remain open. |
| F15 | PostgreSQL sequences | 2 | T43 | Qualified sequence declarations, options and ownership delivered at `d11b4881`. Session names and runtime issuance remain open. |
| F16 | PostgreSQL foreign objects | 2 | T44 | Qualified wrapper and foreign table checks delivered at `d6cb07d2`. Validator behavior and remote effects remain open. |
| F17 | PostgreSQL text search | 2 | T45 | Qualified configuration and dictionary checks delivered at `fad0c7f4`. Session and custom template facts remain open. |
| F18 | PostgreSQL replication | 3 | T46 | Local publication and subscription premises delivered at `3ca2c47b`. Remote execution effects remain open. |
| F19 | PostgreSQL grants/languages | 3 | T47 | Local privilege and explicit callback checks delivered at `eacc45d4`. Effective authorization remains open. |

## Exact row/dimension queue

Column `Disposition` applies to every listed and normalized dimension unless
the delivered slice explicitly says otherwise. It never means the whole row
is semantically checked. Each family review must replace `unreviewed` with
individual findings, boundaries and concrete evidence; a blanket `done` is
insufficient.

<!-- BEGIN ORIGINAL 63 ROWS -->
| Family | Row ID | Authored dimensions at d8cceea6 | Disposition |
| --- | --- | --- | --- |
| F01 | `mysql.create-procedure` | CREATE PROCEDURE ... BEGIN..END; parameters; SQL SECURITY; DETERMINISTIC; body statements | Qualified occupancy and versioned guard verified. Parameter and body evidence is bound to T27/T28. |
| F01 | `mysql.create-function` | CREATE FUNCTION ... RETURNS ... BEGIN..END; DETERMINISTIC; SQL SECURITY | Qualified occupancy and versioned guard verified. The original RETURN and loadable-function boundaries remain. |
| F01 | `mysql.alter-procedure-function` | ALTER PROCEDURE; ALTER FUNCTION; SQL SECURITY; COMMENT | Visible exact identities, SECURITY and native COMMENT controls verified. Catalog visibility gaps remain. |
| F01 | `mysql.drop-procedure-function` | DROP PROCEDURE; DROP FUNCTION; IF EXISTS | Qualified identity, kind and guarded absence verified. Catalog misses and dependencies require additional facts. |
| F01 | `mysql.stored-body-analysis` | static body SQL; DECLARE/local binding; control flow; PREPARE/EXECUTE dynamic SQL | Existing static body and binding coverage verified with explicit dynamic and unsupported-member boundaries. |
| F02 | `mysql.create-trigger` | BEFORE/AFTER INSERT/UPDATE/DELETE; FOR EACH ROW; FOLLOWS/PRECEDES; OLD/NEW bindings | Visible conflicts and ordering groups verified. Guarded CREATE retains a parser boundary on MySQL 8.x. |
| F02 | `mysql.drop-trigger` | DROP TRIGGER; IF EXISTS | Exact visible identity and guarded absence verified with version-specific case controls. |
| F02 | `mysql.create-event` | ON SCHEDULE AT/EVERY; ON COMPLETION; ENABLE/DISABLE; DO body | Visible identity conflicts and native schedule controls verified. Clock and body execution remain outside audit. |
| F02 | `mysql.alter-event` | ALTER EVENT ... ON SCHEDULE/RENAME/ENABLE/DISABLE/DO | Visible destination conflict and same-identity rename verified. Unseen names retain evidence gaps. |
| F02 | `mysql.drop-event` | DROP EVENT; IF EXISTS | Visible identity and guarded absence verified. Scheduler and execution privileges require separate facts. |
| F03 | `mysql.create-table-generated-column` | GENERATED ALWAYS AS (expr); VIRTUAL; STORED; expression DEFAULT (8.0.13+); INVISIBLE column (8.0.23+) | Existing T07 native and catalog evidence binds generated storage, default expressions and invisible columns. |
| F03 | `mysql.alter-table.add-column` | ADD COLUMN; ADD COLUMN FIRST/AFTER; ADD (col, col, ...) | Exact ordinal effects and grouped ADD comparisons verified. Repeated names preserve the input state. |
| F03 | `mysql.alter-table.drop-column` | DROP COLUMN | Generated column source and dependent leaf effects verified. Other dependency facts remain scoped. |
| F03 | `mysql.alter-table.modify-column` | MODIFY COLUMN; type change; NULL/NOT NULL transitions; FIRST/AFTER | Existing native type, NULL and position comparisons bound. Data-dependent behavior remains separate. |
| F03 | `mysql.alter-table.multi-action` | comma-separated action lists | Bounded ordered ADD and MODIFY controls verified. Unmodeled mixed actions retain their existing gaps. |
| F03 | `tidb.alter-table.columns` | ADD/DROP/MODIFY/CHANGE/RENAME COLUMN; FIRST/AFTER; TiDB same-object modification limits | Native same-name, guarded and case-variant repeats verified. Existing position and identity controls remain in force. |
| F04 | `mysql.create-index` | CREATE INDEX; CREATE UNIQUE INDEX; key parts; prefix lengths | Standalone name, key-part and prefix checks verified against native results. Data-dependent uniqueness and index build outcomes remain open. |
| F04 | `mysql.drop-index` | DROP INDEX name ON table | Native missing-index and sole foreign key backing-index refusals verified. Parent-only inbound directory evidence remains open. |
| F04 | `tidb.drop-index` | DROP INDEX name ON table | Native sole and alternate foreign key support, unrelated indices and guarded absence verified. The shared existence rule honors the standalone guard. |
| F05 | `mysql.alter-table.online-ddl` | ALGORITHM=INSTANT/INPLACE/COPY/DEFAULT; LOCK=NONE/SHARED/EXCLUSIVE/DEFAULT | Native versioned modifier and table-premise checks verified. Record and row-version limits still need evidence. |
| F05 | `mysql.alter-table.options` | ORDER BY; CONVERT TO CHARACTER SET; DEFAULT CHARACTER SET; ROW_FORMAT; AUTO_INCREMENT=; FORCE; ENABLE/DISABLE KEYS; COMMENT; ENGINE | Existing and added canonical controls verify authored option effects. Runtime work depends on engine state. |
| F05 | `tidb.alter-table.online-options` | ALGORITHM= (capability assertion, not selection); LOCK=; multiple actions per ALTER | Native modifiers and tested multi-action effects verified. Other combinations require their own evidence. |
| F06 | `tidb.flashback-recover` | FLASHBACK TABLE; FLASHBACK DATABASE; FLASHBACK CLUSTER; RECOVER TABLE | Native fixtures verify table/database and job-based effects. Valid cluster forms parse and stay explicitly unsupported. Bare syntax still fails parse. |
| F07 | `postgresql.create-schema` | CREATE SCHEMA; IF NOT EXISTS / AUTHORIZATION; schema elements | Exact namespace and role checks delivered. Embedded schema elements and execution authorization remain open. |
| F07 | `postgresql.alter-collation` | REFRESH VERSION; RENAME/OWNER/SET SCHEMA | Qualified target checks and bounded provider-version presence checks delivered. Other provider and session facts remain open. |
| F08 | `postgresql.create-table` | basic columns; IF NOT EXISTS; column defaults; CHECK constraint; PRIMARY KEY / UNIQUE; FOREIGN KEY / REFERENCES; EXCLUDE constraint; table constraint clause; WITH storage parameters; USING access method; ON COMMIT / TABLESPACE; OF composite type; arrays and composite columns; generation-declaration | D2 and qualified direct column, key, foreign-key, expression, storage, typed-table and guard checks delivered. Complex expressions, EXCLUDE and wider resolution remain open. |
| F08 | `postgresql.create-table-constraints` | PRIMARY KEY variants; UNIQUE NULLS NOT DISTINCT; UNIQUE USING INDEX; EXCLUDE; NOT NULL named constraint; REFERENCES actions | Direct primary and unique declarations, USING INDEX refusal, native NULLS flags and bounded REFERENCES actions verified. Additional constraints and actions remain open. |
| F08 | `postgresql.create-table-generated-identity` | GENERATED ALWAYS AS IDENTITY; BY DEFAULT AS IDENTITY; sequence options; GENERATED ALWAYS AS expression STORED; generation-declaration | D2 and bounded generated expression refusals verified. General volatility and expression coercion remain open. |
| F08 | `postgresql.create-table-partitioning` | PARTITION BY RANGE; PARTITION BY LIST; PARTITION BY HASH; PARTITION OF; PARTITION OF DEFAULT; INHERITS | Qualified INHERITS parent checks verified. Partition declarations retain their explicit unsupported boundaries. |
| F08 | `postgresql.create-table-temporary` | TEMP/TEMPORARY; GLOBAL TEMPORARY compat noise; UNLOGGED; ON COMMIT actions | Qualified temporary persistence and ON COMMIT premises verified. Session-derived behavior remains open. |
| F08 | `postgresql.create-table-like` | LIKE basic; INCLUDING options | Qualified LIKE source and destination checks verified. Copied option effects remain evidence gaps. |
| F09 | `postgresql.alter-table.columns` | ADD COLUMN; ADD COLUMN IF NOT EXISTS; DROP COLUMN; DROP COLUMN IF EXISTS CASCADE; RENAME COLUMN; ALTER COLUMN TYPE; ALTER COLUMN TYPE USING; multi-action; add-column-generation-declaration | partial:D2; D6 bounded DROP/guard delivered; D7 bounded system/ordering evidence delivered; D8 exact ADD/RENAME target and guard evidence delivered; D14 TYPE/USING source checks delivered; D18 exact destination, D27 direct DROP dependency and D30 bounded TYPE destination prerequisites and D33 direct column traversal checks delivered. D34 direct TYPE dependencies, D35 inherited TYPE eligibility and D36 ADD traversal and D43 TYPE ONLY, D44 recursive RENAME, D45 extra TYPE catalog dependencies and D47 recursive TYPE inheritance checks delivered; wider conversion and ordered outcomes remain open |
| F09 | `postgresql.alter-table.column-attributes` | SET/DROP NOT NULL; SET/DROP DEFAULT; SET GENERATED / DROP EXPRESSION; ADD/DROP IDENTITY; identity options; RESTART; SET STATISTICS; SET STORAGE / COMPRESSION; SET n_distinct / RESET | partial:D5 generation and D6 statistics/options delivered with accepted gaps; D7 bounded storage/compression/system delivered; D20 replica identity key check and D35 inherited/ONLY DROP EXPRESSION and D39 partition NOT NULL traversal and D40 SET DEFAULT reference checks delivered; broader data, expression and relation scopes remain open |
| F09 | `postgresql.alter-table.constraints` | ADD PRIMARY KEY; ADD UNIQUE; ADD FOREIGN KEY; ADD CHECK; ADD CHECK NOT VALID; DROP CONSTRAINT; DROP CONSTRAINT CASCADE; VALIDATE CONSTRAINT; ALTER CONSTRAINT DEFERRABLE; ALTER CONSTRAINT NOT ENFORCED; NOT NULL NOT VALID; ALTER CONSTRAINT INHERIT/NO INHERIT | partial:D3 PK/FK migration risk and UNIQUE reuse correction; D4 bounded targets delivered; D19 exact CHECK name checks delivered; D21 USING INDEX prerequisites delivered; D22 FK target checks delivered; D23 referenced key checks delivered; D24 direct FK dependencies and D25–D26 CHECK inheritance, D31 bounded default operator classes and D32 ON DELETE SET column checks, D37 scalar FK equality and D38 CHECK validation traversal, D41 default operator class resolution and D42 bounded FK domain/enum/array comparisons and D46 CHECK subquery checks delivered; remaining dimensions open Qualified CHECK references, whole-row fallback evidence and bounded numeric constants delivered through `c5f74b71`. The other constraint decisions and explicit parser boundaries remain in the F09 review. Direct ALTER CONSTRAINT deferrability refusal for a proved partition-derived foreign key delivered through `82ba7f00`. Local and root controls retain their earlier verdicts. Direct DROP CONSTRAINT refusal for a proved inherited partition foreign key delivered through `1414066d`. The existing CHECK and guarded-absence checks remain covered. |
| F09 | `postgresql.alter-table.options` | SET/RESET storage parameters; SET TABLESPACE; ALL IN TABLESPACE SET TABLESPACE; SET ACCESS METHOD; SET LOGGED/UNLOGGED; REPLICA IDENTITY; SET WITHOUT OIDS dead form | partial:D9 bounded attachment delivered; D10 bounded heap declarations delivered; D11 single-table tablespace delivered; D12 ordinary-table methods delivered; D13 persistence delivered; D17 bulk tablespace prerequisites and D29 SET WITHOUT OIDS target evidence delivered; other dimensions open |
| F09 | `postgresql.alter-table.rename-owner` | RENAME TO; SET SCHEMA; OWNER TO | partial:D15 primary destination checks delivered; D16 owner checks and D28 attached SET SCHEMA names delivered; wider dependent moves and permissions remain open |
| F10 | `postgresql.drop-table` | DROP TABLE; IF EXISTS / multi-target | Qualified targets, guarded absence and direct RESTRICT references checked. Wider dependency classes, relation shapes and session binding remain open. Direct external row-type and array column dependencies delivered through `ce8a8478`. Same-set targets and CASCADE effects have native controls. |
| F10 | `postgresql.truncate-table` | TRUNCATE basic; RESTART/CONTINUE IDENTITY; ONLY / CASCADE | Qualified ordering, `ONLY` and RESTRICT references checked. Native fixtures verify bounded identity and CASCADE outcomes. Remaining facts are recorded below. |
| F11 | `postgresql.create-index` | CREATE INDEX basic; UNIQUE; CONCURRENTLY; USING method; expression index; partial WHERE; INCLUDE / NULLS DISTINCT; opclass/parameters; WITH/TABLESPACE | Qualified INCLUDE and NULLS checks add native evidence. General key and method eligibility remain open. |
| F11 | `postgresql.alter-index` | RENAME TO; SET TABLESPACE; SET/RESET parameters; ATTACH PARTITION; IF EXISTS | Rename, tablespace, bounded storage options and exact attachment checked. Complex predicates and expression matching remain open. |
| F11 | `postgresql.create-operator-family` | CREATE OPERATOR FAMILY; AS members | Qualified creation checks the access method, namespace and name slot. AS members retains its PostgreSQL 17 parser refusal. |
| F11 | `postgresql.alter-operator-family` | ADD/DROP members; RENAME/OWNER/SET SCHEMA | Qualified identity, changes and bounded B-tree search members checked. Other member kinds and access methods remain open. |
| F12 | `postgresql.create-view` | CREATE VIEW; OR REPLACE / TEMP; column list; WITH options; CHECK OPTION; RECURSIVE | Options, qualified targets and source-free literal outputs have delivered checks. Broader query and session semantics remain open. |
| F12 | `postgresql.alter-view` | RENAME/SET SCHEMA/OWNER; SET/RESET options; ALTER COLUMN DEFAULT; IF EXISTS | Qualified RENAME, SET SCHEMA, OWNER, SET/RESET options and column defaults delivered in the documented scope. Wider definition and execution facts remain open. |
| F12 | `postgresql.drop-view` | DROP VIEW; IF EXISTS CASCADE | Ordered qualified targets, guarded absence and direct rewrite and row-type column dependencies delivered. Broader cascades and session cases remain open. Generated row-type array column dependencies delivered through `00911608` for qualified single and multi-target scopes. |
| F12 | `postgresql.alter-materialized-view` | RENAME/SET SCHEMA/OWNER; SET/RESET options; DEPENDS ON EXTENSION | Qualified rename, schema-move, owner, selected storage options and installed extension checks delivered. Other options and drop-time effects remain open. |
| F12 | `postgresql.refresh-materialized-view` | REFRESH basic; CONCURRENTLY; WITH NO DATA | Target and concurrent refresh prerequisites delivered. Runtime query, privilege and lock outcomes remain open. |
| F13 | `postgresql.create-type` | ENUM; composite; RANGE; base type; shell type | Qualified enum, composite, range, shell and bounded base declarations verified. Additional I/O, subtype and privilege checks remain open. |
| F13 | `postgresql.alter-type` | ADD VALUE; RENAME VALUE; RENAME/OWNER/SET SCHEMA; composite attributes; SET properties | Exact enum labels, type changes, composite attributes and bounded properties verified. Dependencies and support functions remain open. |
| F13 | `postgresql.alter-operator` | OWNER/SET SCHEMA; SET RESTRICT/JOIN | Exact qualified owner and schema changes and bounded RESTRICT/JOIN checks verified. Session names and other operator changes remain open. |
| F14 | `postgresql.alter-policy` | USING/WITH CHECK; TO role / RENAME | Exact relation and policy targeting, role order, rename occupancy and bounded clauses verified. Combined TO and RENAME is a parser refusal. |
| F15 | `postgresql.create-sequence` | CREATE SEQUENCE; options; OWNED BY / TEMP / IF NOT EXISTS | Exact creation target, guarded absence, options, persistence and OWNED BY verified. |
| F15 | `postgresql.alter-sequence` | RESTART; options; OWNED BY / RENAME / SET SCHEMA; IF EXISTS | Exact definition options, restart bounds, ownership, rename and schema changes verified. Narrowed bounds can still require current value evidence. |
| F16 | `postgresql.alter-foreign-data-wrapper` | HANDLER/VALIDATOR changes; OPTIONS; OWNER/RENAME | Exact routines, option order, target roles and rename occupancy verified. |
| F16 | `postgresql.alter-foreign-table` | OPTIONS; ADD/DROP COLUMN; RENAME/OWNER/SET SCHEMA | Exact targets, columns, options and namespace actions verified. Wider dependencies and validator behavior remain open. |
| F17 | `postgresql.alter-text-search-configuration` | ADD/DROP/ALTER MAPPING; RENAME/OWNER/SET SCHEMA | Built-in token mappings, exact dictionary references and namespace/owner actions verified. |
| F17 | `postgresql.alter-text-search-dictionary` | option changes; RENAME/OWNER/SET SCHEMA | Simple template options and namespace/owner actions verified. Other templates and file validation remain open. |
| F18 | `postgresql.alter-publication` | ADD/DROP/SET TABLE; SET TABLES IN SCHEMA; SET publish options; OWNER/RENAME | Exact local selectors, bounded options and owner/rename verified. Recursive targets and row expressions retain gaps. |
| F18 | `postgresql.alter-subscription` | ENABLE/DISABLE; CONNECTION/PUBLICATION/SET; REFRESH PUBLICATION; SKIP; OWNER/RENAME | AST ENABLE/DISABLE, exact local catalog checks and SKIP syntax verified. Publisher and origin progress facts remain open. |
| F18 | `postgresql.drop-subscription` | DROP SUBSCRIPTION; IF EXISTS; WITH drop_slot | Exact slotless DROP and absence guard verified. WITH drop_slot retains parser error and remote slot effects remain unverified. |
| F19 | `postgresql.grant` | ON TABLE; ALL TABLES IN SCHEMA; columns; WITH GRANT OPTION; sequences/functions/database/schema/types; role membership; tablespace/large objects; GRANTED BY | Exact local object, grantee, grantor and column premises verified. Untested kinds, membership and ACL chains remain open. |
| F19 | `postgresql.revoke` | REVOKE privileges; GRANT OPTION FOR; CASCADE/RESTRICT; role membership; object kinds | Exact local targets, columns and behavior declarations verified. Role membership and transitive effects retain boundaries. |
| F19 | `postgresql.create-language` | CREATE LANGUAGE; OR REPLACE TRUSTED; HANDLER form | Handler signatures and replacement premises verified. Parameterless forms retain extension semantics. |
<!-- END ORIGINAL 63 ROWS -->

## F09 exact dimension review

These five F09 rows have 40 authored labels and 40 normalized records at `d8cceea6`. The label sets match in each row. The normalized column records list `add-column-generation-declaration` first. Each entry gives the covered premise or reproduced boundary and the facts required for further work.

### postgresql.alter-table.columns

| Dimension | Covered premise or reproduced boundary | Remaining premise and evidence needed |
| --- | --- | --- |
| ADD COLUMN | D8 exact ordinary-table column identity and duplicate/system-name checks | D36 at `441e6b34` checks parent and partition traversal. Descendant merges, expression evaluation and effects after earlier ALTERs need separate facts. |
| ADD COLUMN IF NOT EXISTS | D8 actual authored guard, user duplicate no-op, system-name conflict and guarded advice suppression | D36 at `441e6b34` distinguishes a direct partition-child refusal and the existing-user-column guard on an inheritance parent. Stale facts still yield a gap. |
| DROP COLUMN | D6 exact existing target and D7 observed system-member refusal | D27 at `b6d73f04` checks direct view, materialized-view, stored generated-column and inbound FK blockers. D33 checks inherited columns, a direct partition key and partition-parent ONLY. Wider dependency closure needs separate evidence. |
| DROP COLUMN IF EXISTS CASCADE | D6 proved user-member absence suppression and D7 system guard refusal | D33 checks inherited DROP even with the guard and CASCADE. Propagation, dependency closure and locks need separate evidence. |
| RENAME COLUMN | D8 exact source, destination and same-name conflict; independent rename forbid stays | D33 checks direct inherited sources and ONLY with attached children. D44 adds exact descendant collisions and outside inherited definitions. Other descendant shapes and dependency effects need separate facts. |
| ALTER COLUMN TYPE | D14 source identity and D18 exact scoped destination identity | D30 at `36ef07e4` checks scoped literal modifiers, collation and scalar assignment (delivered). D34–D35 at `441e6b34` check direct view and generated-column dependencies plus inherited sources and direct partition keys. D43 adds a check for ONLY with attached children after exact destination and scalar assignment. D45 adds more direct dependent classes. D47 checks outside descendant inheritance after scalar assignment. Broader type identity and row conversion require separate facts. |
| ALTER COLUMN TYPE USING | D14 generated-source restriction and D18 exact destination with USING | D34–D35 at `441e6b34` apply the same bounded dependency and source checks. USING result conversion, constraints, row values and rewrite need separate facts. |
| multi-action | D6–D8 scope and freshness gaps describe why a previously loaded catalog may not apply | No general PostgreSQL execution projection. Any future supported batch relation must follow actual processing phases and preserve unknowns. |
| add-column-generation-declaration | D2 bounded identity and generated declaration validity in ADD | D36 at `441e6b34` checks identity ADD on an ordinary inheritance parent. Expression legality, wider table kinds and data migration remain separate. |

### postgresql.alter-table.column-attributes

| Dimension | Covered premise or reproduced boundary | Remaining premise and evidence needed |
| --- | --- | --- |
| SET/DROP NOT NULL | D5 checks the ordinary target and identity or primary-key guard. D20 distinguishes an attached replica identity key from INCLUDE, unattached unique and FULL. | D39 at `683d84ac` checks partition-parent NOT NULL premises and ONLY traversal. User-data validity and other inheritance effects need separate evidence. |
| SET/DROP DEFAULT | D5 target and conflict with identity or generation, D7 system scope | D40 checks raw column references and subqueries after the existing target checks. The [literal assignment decision](../decisions/2026-11-21-postgresql-set-default-expression-references.md#direct-boolean-and-int4-literal-assignment) adds the bounded bool/int4 cross-type refusal. The [parameter decision](../decisions/2026-11-21-postgresql-set-default-expression-references.md#positional-parameters-in-set-default) covers the whole expression or a direct COALESCE argument. Wider expression trees, general type assignment, function resolution, dependencies and nonordinary relations need separate evidence. |
| SET GENERATED / DROP EXPRESSION | D5 bounded existing generation/identity checks, D7 system controls | D35 at `441e6b34` checks inherited DROP EXPRESSION and attached-child ONLY. Expression evaluation, wider relation kinds and dependent results remain open. |
| ADD/DROP IDENTITY | D5 type/default/nullability preconditions and guard semantics, D7 scope | Runtime identity values, sequence effects outside the verified dependency, and wider table kinds need separate evidence. |
| identity options | D5 scalar/range/duplicate and stable, dependency-bound sequence definition | Unsupported option forms and runtime state need narrower AST or catalog evidence before a rule can decide. |
| RESTART | D5 warns and distinguishes explicit RESTART from START/default changes | Runtime counter/issuance, session and sequence visibility are outside this observation. |
| SET STATISTICS | D6 value and target blocker plus native clamp warning | Wider table kinds and statistical effectiveness need separate premises. |
| SET STORAGE / COMPRESSION | D7 exact ordinary/system source, type strategy and observed build capabilities | Additional build anchors, wider table kinds, actual existing value storage and rewrite remain open. |
| SET n_distinct / RESET | D6 keys/ranges/duplicates and stable numeric parsing, including overflow/underflow gaps | Unknown/opaque keys, precision edges and other relation kinds require independent evidence. |

### postgresql.alter-table.constraints

| Dimension | Covered premise or reproduced boundary | Remaining premise and evidence needed |
| --- | --- | --- |
| ADD PRIMARY KEY | D3 covers index build risk. D21 checks exact USING INDEX identity, ownership and catalog eligibility, including PRIMARY KEY with NULLS NOT DISTINCT. D48 verifies key and INCLUDE columns for a qualified ordinary table, including the refusal for a second primary key. D49 checks partition-key compatibility on a qualified partitioned root. | D31 checks installed class defaults for exact type. D41 resolves binary-compatible and polymorphic defaults. Existing rows, locks and execution outcomes need separate evidence. |
| ADD UNIQUE | D3 separates ordinary build risk from USING INDEX. D21 checks catalog eligibility and distinguishes same-name attachment from an occupied rename destination. D48 verifies key and INCLUDE columns for a qualified ordinary table, including permitted repetitions in INCLUDE. D49 checks qualified partitioned roots, including missing partition columns, expression slots and bounded equality or collation mismatch. | D31 checks installed class defaults for exact type. D41 resolves binary-compatible and polymorphic defaults. Execution prerequisites require separate facts. |
| ADD FOREIGN KEY | D3 covers initial-validation risk. D22 covers exact table and column identity. D23 adds referenced key eligibility, omitted primary-key selection, duplicate referenced columns and arity. | D32 checks ON DELETE SET members. D37 at `683d84ac` checks scalar equality and distinguishes assignment-only casts from implicit casts. D42 checks nested domain bases and the native enum and array equality premises. Remaining polymorphic and custom conversion definitions require independent evidence. Row validity, runtime actions and privileges require separate facts. |
| ADD CHECK | D19 approved exact named collision on a fresh ordinary table. Native same expression with new/unnamed CHECK succeeds; purely inherited constraint can merge. D50 verifies direct invalid Boolean text and bounded int4 coercion premises. D51 verifies exact column binding in simple null tests. | D46 checks subqueries before name collisions. Remaining predicate validity, data and inheritance require separate evidence. |
| ADD CHECK NOT VALID | D19 approved exact collision. Existing follow-up distinguishes migration validation intent. Native can register over an old violation while invalid predicates still fail. D50 applies the same literal check to this form. D51 applies the same exact null-test check to this form. | D46 checks subqueries even under NOT VALID. Other expression legality, row checks and batch outcome remain separate. |
| DROP CONSTRAINT | D4 binds exact target and guard behavior. D24 checks direct RESTRICT foreign-key dependencies on the supporting index in the ordinary-table scope. | D25 and D26 at `7af6f84a` cover direct CHECK inheritance and partition-parent ONLY. Wider dependency closure and foreign-table inheritance need separate evidence. |
| DROP CONSTRAINT CASCADE | D4 binds target, and native CASCADE removes a dependent FK while retaining the table and rows. | D25 and D26 at `7af6f84a` preserve direct inheritance and ONLY refusals under CASCADE. Dependency closure and lock or privilege requirements need separate catalog evidence. |
| VALIDATE CONSTRAINT | D4 exact CHECK/FK kind and validated-state no-op. Native row validation can still refuse `23514` or `23503`. | D38 at `683d84ac` distinguishes ordinary and partitioned ONLY refusal, recursive validation, NO INHERIT and already validated checks. Row-data validity remains a separate evidence requirement. |
| ALTER CONSTRAINT DEFERRABLE | D4 exact FK target and known kind; non-FK fails its kind premise | Wider FK dependencies, table shape and execution ordering remain open. |
| ALTER CONSTRAINT NOT ENFORCED | PG17 parser error and #149 upgrade guidance | Requires published PG18 binding and approved version promotion. |
| NOT NULL NOT VALID | PG17 parser error and #149 upgrade guidance | Requires published PG18 binding and approved version promotion. |
| ALTER CONSTRAINT INHERIT/NO INHERIT | Both spelled alternatives retain PG17 parser errors and #149 upgrade guidance | Requires published PG18 binding and approved version promotion. |

### postgresql.alter-table.options

| Dimension | Covered premise or reproduced boundary | Remaining premise and evidence needed |
| --- | --- | --- |
| SET/RESET storage parameters | D10 bounded heap option declarations and verified native ranges | Additional access methods, relation kinds, version/namespace options and operational outcomes need independent evidence. |
| SET TABLESPACE | D11 exact target existence/global restriction and existing observed same space | Physical move, space, privileges, partition/index/TOAST effects require new facts. |
| ALL IN TABLESPACE SET TABLESPACE | D17 exact endpoint, ordered-role and pg_global premise, same-space context | `OWNED BY` member resolution, lock availability under NOWAIT, space and physical movement are separate. |
| SET ACCESS METHOD | D12 ordinary exact method identity/support and selected same method | Moving physical storage, extension method behavior, other relation kinds and dependencies need separate evidence. |
| SET LOGGED/UNLOGGED | D13 ordinary persistence, direct publication and FK preconditions | Wider relation kinds, dependency closure, WAL/durability and execution outcomes need separate support. |
| REPLICA IDENTITY | D9 exact attached index candidate for USING INDEX, independent mode warnings | Downstream replication usability, invalid-build behavior and coverage of wider kinds stay separate. |
| SET WITHOUT OIDS dead form | PG17 compatibility no-op proved on ordinary/partitioned tables; guards and wrong kinds verified independently | D29 at `36ef07e4` adds exact target-kind and guarded absence evidence (delivered). Removed WITH OIDS remains a parser error. |

### postgresql.alter-table.rename-owner

| Dimension | Covered premise or reproduced boundary | Remaining premise and evidence needed |
| --- | --- | --- |
| RENAME TO | D15 primary relation and row-type naming, including movable autogenerated array exception | Other dependent effects and authorization need separate evidence. |
| SET SCHEMA | D15 direct destination schema/primary relation/row-type and same-schema; native constraints and owned sequence/index collisions remain distinct | D28 at `36ef07e4` checks exact attached index, owned sequence and array type collisions (delivered). Wider relation scope and permission evidence remain open. |
| OWNER TO | D16 exact literal roles, PUBLIC and observed current/session alias semantics | Actual permissions, role membership/grants, partition or other object kinds and dependent ownership remain distinct. |


## F10 exact dimension review

| Row and authored dimension | PostgreSQL 17 evidence | Remaining premise |
| --- | --- | --- |
| `postgresql.drop-table` / DROP TABLE | T38 `pg-destructive-drop-direct-reference-fk`, `-self` and `-cascade` verify exact persistent ordinary-table RESTRICT dependencies and the independent native CASCADE effect. | The rule checks live direct foreign-key and rewrite edges. Other dependency classes, inheritance/partition closure and permissions require separate facts. |
| `postgresql.drop-table` / IF EXISTS / multi-target | T38 `pg-destructive-drop-guarded`, `-drop-repeat`, `-drop-later-missing` and `-drop-direct-reference-all-members` verify guard, duplicate, exact target order and references inside the selected set. | Fresh schema-local binding is required. Search-path, session temporary names, catalog-qualified selection and earlier DDL require their own evidence. |
| `postgresql.truncate-table` / TRUNCATE basic | T38 `pg-destructive-truncate-tree-recursive-fk`, `-partition-root` and `-partition-leaf` verify attached descendant expansion and inherited FK parent traversal. | Foreign tables need FDW eligibility, and temporary or unknown members need session evidence. |
| `postgresql.truncate-table` / RESTART/CONTINUE IDENTITY | T38 `pg-destructive-truncate-restart`, `-truncate-continue` and `-truncate-tree-recursive-restart` verify decoded intent and the actual native owned-sequence outcomes. | Sequence-owner permissions and runtime issuance state are independent. The catalog audit does not predict the next value or change it. |
| `postgresql.truncate-table` / ONLY / CASCADE | T38 `pg-destructive-truncate-tree-only-parent`, `-only-root`, `-partition-repeated-covered` and `-partition-repeated-cascade` verify ordered ONLY membership and bounded native CASCADE outcomes. | Wider CASCADE effects, locks and trigger outcomes require their own evidence. |

The rebased preview at `df33755e` passed 62 cases with 1,761 assertions. The base's 17 case IDs and native controls remain. Forty-seven selectors bind the five PostgreSQL 17 subactions. Both parent rows move from generic notice to mixed coverage. The preview uses existing blocker IDs and treats observed PostgreSQL guarded absence as a no-op. A DROP/TRUNCATE forbid policy remains independently configurable.

The implementation uses three decision records for the exact target binding,
direct DROP RESTRICT dependencies, and extended TRUNCATE target order and
inherited references. The bounded slice shipped at `b5cf88ce15f8ba8a369ef11e553d370d30308d39`. All 50 canonical tasks passed with
3,937 cases and 77,123 assertions. T38 passed 62 cases and 1,761 assertions.
The validator, independent review, isolated cleanup and merged-root gates
passed. The inventory now classifies the two F10 parents as mixed coverage.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44844312112/job/136660156079),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44844312112/job/136660156600),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44844312112/job/136660156544),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44844312471/job/136660156573),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136660256807) passed at the delivered SHA.

- [Destructive table targets](../decisions/2026-11-21-postgresql-destructive-table-targets.md)
- [DROP TABLE RESTRICT dependencies](../decisions/2026-11-21-postgresql-drop-table-restrict-dependencies.md)
- [TRUNCATE inherited references](../decisions/2026-11-21-postgresql-truncate-inherited-references.md)

## F12 exact dimension review

The five rows below contain the same 18 authored and normalized dimension names as `d8cceea6`. The CREATE option, DROP target and REFRESH findings in this review shipped with fixed-SHA evidence. Scope outside those checks remains open.

| Inventory dimension | Evidence and present scope | Remaining premise |
| --- | --- | --- |
| `postgresql.create-view` / CREATE VIEW | `pg-create-view-output-*` checks qualified targets of source-free literal queries, including a proved missing schema, a relation collision and an observed empty output. | Query analysis, type namespace collisions, permissions and broader source facts require separate evidence. |
| `postgresql.create-view` / OR REPLACE / TEMP | `pg-create-view-output-*` checks the existing name, type, modifier and collation prefix and permits an appended output column. TEMP in a proved persistent schema blocks. | The `explicit-temp-alias` control pins the gap for the `pg_temp` session alias. Other query shapes and implicit temporary sources require facts. |
| `postgresql.create-view` / column list | `pg-create-view-output-*` verifies alias order, excess aliases, duplicate exact names, and valid partial alias lists for source-free literal outputs. | Expanded stars and other query-derived outputs still need analysis. |
| `postgresql.create-view` / WITH options | `pg-create-view-options-*` distinguishes recognized Boolean and CHECK OPTION values from invalid values, unknown names and duplicates. The offline control reports a missing observed version. | Effective privileges, barrier behavior and query execution require separate evidence. |
| `postgresql.create-view` / CHECK OPTION | The `pg-create-view-options-not-updatable-check-option` and `-literal-check-clause` controls verify the PostgreSQL 17 refusal for the option and clause forms on source-free literal queries. | LOCAL and CASCADED execution and the automatic updatability of other queries require separate evidence. |
| `postgresql.create-view` / RECURSIVE | `create-view-recursive` parses the recursive form. The AST and binding can express the equivalent recursive query. | Recursive self-binding and query semantics need separate evidence. |
| `postgresql.alter-view` / RENAME/SET SCHEMA/OWNER | T40 `pg-view-namespace-*` checks exact qualified names, row and array type conflicts, missing schemas, same-name rename and same-schema moves. `pg-view-owner-*` covers literal and session roles, including missing roles and PUBLIC. | Unqualified sources still need fresh search-path evidence. Membership, authorization and locks need separate evidence. |
| `postgresql.alter-view` / SET/RESET options | `pg-alter-view-options-*` checks exact target kind before SET/RESET declarations, including invalid SET values, RESET with a value, repetitive or unknown RESET names and ignored namespaces. | CHECK OPTION needs evidence of updatability. Multi-action changes still require ordered-state evidence. |
| `postgresql.alter-view` / ALTER COLUMN DEFAULT | `pg-view-column-default-*` binds the exact qualified view and user column for SET or DROP. It checks source precedence and reports column references and subqueries in a default. Native SET and DROP verify the stored catalog default. | Coercion, function resolution and later writes require independent evidence. |
| `postgresql.alter-view` / IF EXISTS | The qualified `pg-view-namespace-*`, `pg-view-owner-*` and `pg-alter-view-options-*` controls skip proved absence under the guard. A present object of the wrong kind still blocks. | Column default controls also verify guarded absence and wrong-kind precedence. Unqualified source resolution requires fresh session facts. |
| `postgresql.drop-view` / DROP VIEW | T40 `pg-drop-view-set-*` covers ordered qualified targets, duplicates, a dependency within the deletion set and an external rewrite or row-type column dependency. | Other dependency classes and execution authorization require evidence. |
| `postgresql.drop-view` / IF EXISTS CASCADE | T40 covers guarded absence in a list, wrong-kind precedence and native CASCADE removal of the tested external views or column. | Wider CASCADE effects and session-local resolution require independent evidence. |
| `postgresql.alter-materialized-view` / RENAME/SET SCHEMA/OWNER | T40 `pg-view-namespace-*` checks exact source and destination identities. Native and audit controls show the array conflict precedes the attached-index conflict. `pg-view-owner-*` covers present and absent destination roles. | Membership, authorization and locks need separate evidence. |
| `postgresql.alter-materialized-view` / SET/RESET options | `pg-matview-storage-options-*` checks qualified targets, integer fillfactor, selected heap and TOAST options, namespaces and RESET values. Native controls observe stored options and their removal. | Quoted fillfactor, other parameters and custom access methods need distinct evidence. |
| `postgresql.alter-materialized-view` / DEPENDS ON EXTENSION | `pg-matview-extension-*` checks the exact source kind and installed extension for add and remove. Native controls verify repeated add and absent-edge remove. | Drop-time effects, unqualified source selection and authorization remain separate. |
| `postgresql.refresh-materialized-view` / REFRESH basic | `pg-refresh-target-missing`, `-wrong-kind` and `-nonconcurrent-empty` exercise exact target binding and refusal. | Query execution, privileges and lock outcomes remain separate. |
| `postgresql.refresh-materialized-view` / CONCURRENTLY | `pg-refresh-target-*` verifies population, conflict precedence, partial/expression/invalid indexes, a usable unique index and INCLUDE. | A usable index proves the catalog prerequisite. Refresh execution and results require independent facts. |
| `postgresql.refresh-materialized-view` / WITH NO DATA | The native `-nonconcurrent-empty` control observes an unpopulated view. The two conflict controls prove the distinct order of population and flag refusals. | Downstream demand and concurrent sessions require separate evidence. |

At preview base `b5cf88ce`, T40 passed 68 cases and 1,482 assertions. All 33 original cases and their order remain. Four rename cases now expect the rename notice alone, with their SQL and native controls preserved. The coverage catalog was regenerated for the action-specific notices. The 42 additions to inventory evidence bind seven PostgreSQL 17 cells. The WITH options, DROP VIEW and CONCURRENTLY subactions change to mixed. The 301-row distribution stays as on the F10 base. The three decision records describe the default blockers and runtime boundaries.

- [CREATE VIEW option validation](../decisions/2026-11-21-postgresql-create-view-options.md)
- [Materialized view refresh prerequisites](../decisions/2026-11-21-postgresql-materialized-view-refresh-prerequisites.md)
- [DROP VIEW targets and direct dependencies](../decisions/2026-11-21-postgresql-drop-view-targets.md)
- [T40 cases](../../testdata/ddl-golden/T40.json)

The F12 slice shipped at `da3efecc1bc7c0c3158b521b0090da4c295c2818`. All 50 canonical tasks passed with
3,972 cases and 78,266 assertions. T40 passed 68 cases with 1,482 assertions.
The validator, read-only review, isolated cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44849730989/job/136677973201),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44849730989/job/136677972856),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44849730989/job/136677973151),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44849731024/job/136677972934),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136678070236) passed at the delivered SHA.

### F12 view namespace, owner and option changes

`ddl.pg.alter_view.namespace.valid` checks exact PostgreSQL 17 source,
guard and catalog destination premises for ordinary and materialized view
RENAME and SET SCHEMA. Source kind and guarded absence precede destination
checks. Destination relation, row type and associated array identities can
block. For a materialized view, an attached index name can also block a
schema move. The checks keep catalog-dependent names private.

`ddl.pg.alter_view.owner.valid` checks qualified OWNER TO. A proved absent
source under IF EXISTS is silent. The audit checks source kind before the
role fact, and distinguishes absent literal roles, PUBLIC and the role seen
in the audit session. Role membership, privileges and locks require separate
evidence.

The SET/RESET check binds an exact ordinary view before assessing options.
It refuses RESET values and preserves an updatability evidence gap for
CHECK OPTION. Materialized storage options and multi-action statements
retain their independent boundaries.

The combined preview against `da3efecc` passed 109 T40 cases with 3,013
assertions. The earlier 68 cases, authored SQL and native controls remain. The
existing SET options case now verifies the modeled adapter action.
Forty-four references bind four exact PostgreSQL 17 inventory cells.
Delivered at `d4cf7b6caa8798c9af60cfc82f914482c6f1a5ba`. All 50 canonical tasks passed with 4,013 cases and
79,797 assertions. T40 passed 109 cases and 3,013 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44854641890/job/136693858397),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44854641890/job/136693858213),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44854641890/job/136693858406),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44854641840/job/136693858413),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136693935697) passed at the delivered SHA.

- [View namespace targets](../decisions/2026-11-21-postgresql-view-namespace-targets.md)
- [View owner targets](../decisions/2026-11-21-postgresql-view-owner-targets.md)
- [ALTER VIEW options](../decisions/2026-11-21-postgresql-alter-view-options.md)

### F12 view defaults, materialized storage and extension dependencies

The ordinary-view default check binds the qualified view and exact user
column. IF EXISTS skips proved relation absence. The audit rejects raw
column references and subqueries in a default after checking the target.
Type coercion, function resolution and future writes require separate facts.

The materialized-view storage check binds the qualified target and observes
its heap access method and attached TOAST relation. It checks RESET values,
integer fillfactor, selected heap and TOAST parameters and option namespace.
Other parameter and access-method semantics still require evidence.

The extension dependency check binds the qualified materialized view and
installed extension. The native add, repeated add, remove and absent-edge
controls verify the automatic dependency behavior independently of audit.
Drop-time effects and execution permissions require separate evidence.

The joint preview passed 147 T40 cases and 4,224 assertions. The
41 new references bind three exact PostgreSQL 17 cells. Existing authored
SQL and native controls remain. Three baseline CLI cases now check modeled
adapter actions. Delivered at `0062e29e298b649e8efc7795476b795e3a5925ac`. All 50 canonical tasks passed with 4,051 cases and
81,008 assertions. T40 passed 147 cases and 4,224 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44860056068/job/136711142098),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44860056068/job/136711142192),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44860056068/job/136711142220),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44860056069/job/136711141684),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136711220493) passed at the delivered SHA.

- [Ordinary-view column defaults](../decisions/2026-11-21-postgresql-view-column-default.md)
- [Materialized-view storage options](../decisions/2026-11-21-postgresql-materialized-view-storage-options.md)
- [Materialized-view extension dependencies](../decisions/2026-11-21-postgresql-materialized-view-extension.md)

### F12 view output and ordered DROP VIEW targets

The CREATE VIEW rule checks exact qualified targets and ordered outputs for
bounded source-free literal queries. Replacement preserves the existing
column prefix, including names, types, modifiers and collations. The option
rule now rejects CHECK OPTION on these queries.

The DROP VIEW rule binds an ordered qualified list and catches missing or
wrong-kind targets before RESTRICT dependency checks. It recognizes
rewrite dependencies within the selected list and reports external direct
rewrite and row-type column dependencies. Unqualified name resolution,
recursive query semantics and broader CASCADE effects require separate facts.

T40 preview passed 179 cases and 5,354 assertions. Thirty-four
selectors bind six exact PostgreSQL 17 cells. The prior 147 case IDs and
authored DDL remain. One expectation now checks the CHECK OPTION blocker.
Delivered at `919f1d17c11f86bd4eea7a58fc6c53f504565488`. All 50 canonical tasks passed with 4,083 cases and
82,138 assertions. T40 passed 179 cases and 5,354
assertions. The validator, read-only review, owned resource cleanup and
merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44865129618/job/136726995751),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44865129618/job/136726995694),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44865129618/job/136726995564),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44865129574/job/136726995103),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136727062716) passed at the delivered SHA.

- [CREATE VIEW output](../decisions/2026-11-21-postgresql-create-view-output.md)
- [Ordered DROP VIEW](../decisions/2026-11-21-postgresql-drop-view-target-set.md)

### F09 direct primary and unique constraint columns (D48)

`ddl.pg.alter.constraint_columns.valid` checks a single direct ADD PRIMARY
KEY or ADD UNIQUE on a qualified, bound PostgreSQL 17 ordinary table.
It rejects duplicate key members, missing or system columns, and a second
primary key after checking the selected columns. Native evidence confirms
that repeated INCLUDE members and key overlap are accepted. Quoted commas
within a column identifier stay part of that identifier.

The rebased T37 preview passed 576 cases and 17,015 assertions.
Fifteen selectors bind the exact ADD PRIMARY KEY and ADD UNIQUE cells.
Partition key requirements, row validation, locks and ordered ALTER
semantics require separate evidence.

Delivered at `a83a171084d5311ff8feaab48546b6ef246ddff1`. All 50 canonical tasks passed with 4,099 cases and
82,646 assertions. T37 passed with 576 cases and 17,015 assertions. The
validator, read-only review, owned resource cleanup and merged-root gates
passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44869972937/job/136742085319),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44869972937/job/136742085101),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44869972937/job/136742085253),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44869972905/job/136742084765),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136742168384) passed at the delivered SHA.

- [Constraint column checks](../decisions/2026-11-21-postgresql-primary-unique-constraint-columns.md)

### F09 partition keys for primary and unique constraints (D49)

The constraint-column rule now uses exact catalog facts on a qualified
PostgreSQL 17 partitioned root. It checks authored key and INCLUDE members
first. Partition columns must be in the key section, with matching collation
and proved equality semantics. An expression partition key blocks. Fallback
operator classes, subtrees and row validation require separate evidence.

T37 preview passed 589 cases and 17,444 assertions. Fourteen D49
selectors bind ADD PRIMARY KEY and ADD UNIQUE. Two existing D48 expectations
now verify the observed partition-key outcomes.
Delivered at `ddeec62e45a0e9ca7d4eb083d42e32f2425c891e`. All 50 canonical tasks passed with 4,112 cases and
83,075 assertions. T37 passed 589 cases and 17,444 assertions.
The validator, read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44873675390/job/136753543346),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44873675390/job/136753543122),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44873675390/job/136753543368),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44873675529/job/136753543593),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136753629641) passed at the delivered SHA.

- [Partition constraint keys](../decisions/2026-11-21-postgresql-partition-constraint-keys.md)

## F07 exact dimension review

The two rows retain the five authored dimensions at the PostgreSQL 17 anchor. The joint candidate binds 37 selectors across four cells. Its fixed delivery receipt will identify the commit and complete validation chain.

| Inventory dimension | Evidence and present scope | Remaining premise |
| --- | --- | --- |
| `postgresql.create-schema` / CREATE SCHEMA | `pg-create-schema-target-*` distinguishes an occupied exact namespace from a fresh one and checks the `pg_` prefix under the observed setting. Native controls preserve quoted case and distinguish reserved lowercase from ordinary quoted uppercase. | Database CREATE privilege, lock and execution outcomes need separate evidence. |
| `postgresql.create-schema` / IF NOT EXISTS / AUTHORIZATION | `pg-create-schema-target-*` verifies explicit and implicit schema names, missing or PUBLIC owner, and owner resolution before a guard can skip creation. PostgreSQL 17.7 native probes also resolve `CURRENT_USER` and `SESSION_USER`. | Session-role authorization and extension membership are separate premises. Ordered audit withholds schema state for an authored owner when execution remains uncertain. |
| `postgresql.create-schema` / schema elements | T35 `create-schema-schema-elements` preserves the existing adapter refusal for `CREATE SCHEMA app CREATE TABLE t (id int)`. The PostgreSQL 17.7 native control creates the embedded table in a rollback transaction. | The adapter would need modeled child statements, PostgreSQL's creation ordering and exact name resolution under the command's temporary search path. |
| `postgresql.alter-collation` / REFRESH VERSION | `pg-collation-refresh-*` checks exact source identity in the database encoding, rejects refreshing the database default via ALTER COLLATION, and compares version presence for tested libc and built-in C/POSIX cases. | Other provider locales and unqualified source selection need evidence. Updating a recorded version does not establish that dependent indexes were rebuilt. |
| `postgresql.alter-collation` / RENAME/OWNER/SET SCHEMA | `pg-collation-change-*` checks qualified source and destination slots, missing schemas, the same-schema no-op and the TOAST restriction. The owner controls verify resolved session roles and the refusal for a missing role before source lookup. | Temporary namespace aliases, unqualified source selection, execution privileges and other provider encodings need separate evidence. |

- [Original T35 and paired controls](../../testdata/ddl-golden/T35.json)
- [Schema creation targets](../decisions/2026-11-21-postgresql-create-schema-target.md)
- [Collation refresh targets](../decisions/2026-11-21-postgresql-collation-refresh-target.md)
- [Collation namespace and owner checks](../decisions/2026-11-21-postgresql-collation-change-target.md)

The joint T35 preview passed 77 cases and 1,608 assertions. Delivered at `3eba78f63e4815863e0752ffa231e43a5d87ba9f`. All 50 canonical tasks passed with 4,146 cases and
84,219 assertions. T35 passed 77 cases and 1,608 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
The read-only review recorded two additional absent historical Docker Desktop
containers. The fixed run used the separate validation daemon and its
before and after container lists were empty. The cause of the Desktop
absences remains unverified.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44880641963/job/136774959968),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44880641963/job/136774959695),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44880641963/job/136774959924),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44880641893/job/136774959596),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136775035919) passed at the delivered SHA.

### F09 CHECK literal predicates (D50)

The existing ADD CHECK rule rejects invalid untyped Boolean strings and
checks the current implicit or assignment cast catalog for int4 predicates.
The rule keeps cast presence, unresolved facts and complex expressions
outside the proved refusal. Both ADD CHECK and ADD CHECK NOT VALID retain
the established name, guard and subquery checks.

The rebased T37 preview passed 600 cases and 17,779 assertions. Twelve
selectors bind the two CHECK inventory cells. Delivered at `0548f90779d56d9cce8b1c91002c81bf26305665`. All 50 canonical tasks passed with 4,157 cases and
84,554 assertions. T37 passed 600 cases and 17,779 assertions.
The validator, read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44885164434/job/136788784012),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44885164434/job/136788783915),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44885164434/job/136788784066),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44885164520/job/136788783932),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136788853581) passed at the delivered SHA.

- [CHECK literal predicates](../decisions/2026-11-21-postgresql-check-literal.md)

### F09 CHECK null-test column binding (D51)

The ADD CHECK rule binds exact one- and two-part column identifiers in
simple null tests against a fresh PostgreSQL 17 table. A proved absent
column or unsupported system attribute blocks. The target's whole-row
name and `tableoid` remain valid. Wider expression resolution remains
separate.

T37 preview passed 613 cases and 18,182 assertions. Fourteen selectors
bind ADD CHECK and ADD CHECK NOT VALID. Delivered at `6333f67ff92711f012b344a1a4da944636eb0026`. All 50 canonical tasks passed with 4,170 cases and
84,957 assertions. T37 passed 613 cases and 18,182 assertions.
The validator, read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44889066925/job/136800688614),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44889066925/job/136800688767),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44889066925/job/136800688769),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44889067016/job/136800689078),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136800751025) passed at the delivered SHA.

- [CHECK column null tests](../decisions/2026-11-21-postgresql-check-null-test.md)

## F11 exact dimension review

The four rows below retain all 18 authored dimensions. In the joint preview, 111 selectors cover nine exact PostgreSQL 17 cells. The preview evidence remains separate from the fixed delivery receipt. Baseline T39 parser controls, migration advice and native fixtures are listed with the specific premises they exercise.

| Inventory dimension | Evidence and present scope | Remaining premise |
| --- | --- | --- |
| `postgresql.create-index` / CREATE INDEX basic | T39 `create-index-create-index-basic` verifies normalization, the secondary naming warning and the non-concurrent build warning. The native `create index btree` step creates an index on the named column. | General target kind, name collisions, and all key forms still need checks outside the qualified INCLUDE slice. |
| `postgresql.create-index` / UNIQUE | `create-index-unique` verifies unique-kind extraction and its migration warning, and the native partial unique fixture verifies PostgreSQL creates the index. The NULLS controls verify the stored native flag. | Existing duplicate data, NULL behavior during writes and partition-key eligibility for standalone CREATE INDEX require separate evidence. |
| `postgresql.create-index` / CONCURRENTLY | `create-index-concurrently` and the native non-transactional control establish the modeled flag and distinguish it from the non-concurrent warning. | Transaction blocks, partitioned-root and other object-specific concurrent eligibility need independent checks. |
| `postgresql.create-index` / USING method | `create-index-using-method` records the authored method. Native B-tree, GIN and GiST fixtures exercise distinct installed methods. The qualified INCLUDE check rejects tested combinations with hash and GIN. | General method and type compatibility, including extension-defined methods, remains open. |
| `postgresql.create-index` / expression index | `create-index-expression-index` and the native `lower` witness exercise expression keys. The rule retains a count and the plain-key comparisons avoid treating them as plain members. | Expression legality and function properties need separately modeled facts. |
| `postgresql.create-index` / partial WHERE | The `create-index-partial-where` CLI case and native partial-unique index witness verify predicate presence. | Predicate type, allowed references and equivalence are still open. |
| `postgresql.create-index` / INCLUDE / NULLS DISTINCT | `pg-index-include-*` checks a qualified target, names, observed system columns, method eligibility, and stores the exact NULLS fact privately. Native fixtures preserve repeated INCLUDE slots and overlap with keys. | Wider methods, partitioned targets and uniqueness effects on existing or future data need separate evidence. |
| `postgresql.create-index` / opclass/parameters | The `create-index-opclass-parameters` baseline audits the authored form with naming and migration rules. | Exact opclass membership, defaults, parameters and input type need more than the current migration notice. |
| `postgresql.create-index` / WITH/TABLESPACE | `create-index-with-tablespace` preserves the accepted spelling, and the native `create index with params` step exercises an integer fillfactor. | The new ALTER destination and option checks are specific to ALTER. CREATE needs separate creation, method and tablespace premises. |
| `postgresql.alter-index` / RENAME TO | `pg-index-destination-*` checks exact source absence, composite rejection, a clear destination and collision with a table or the source's current name. The ordinary table rename control pins the row-type gap. | Row-type and companion constraint namespaces, unqualified source selection and authorization stay open. |
| `postgresql.alter-index` / SET TABLESPACE | `pg-index-destination-*` checks ordinary and partitioned index targets, missing space, same space and inadmissible `pg_global`. No index is copied during audit. | Execution permissions, data movement and locks need independent evidence. |
| `postgresql.alter-index` / SET/RESET parameters | `pg-index-storage-options-*` verifies target kind, absence guard, integer fillfactor, B-tree Boolean syntax, duplicates, and RESET values. | Quoted integer forms, additional parameters, other access methods and multiple actions require facts. |
| `postgresql.alter-index` / ATTACH PARTITION | `pg-index-attach-*` verifies exact parent and child, direct partition membership, occupied attachments and plain index definitions and constraint premises. Native controls exercise reordered attributes. | Expression or predicate equivalence, detach-pending visibility and broader validation stay open. |
| `postgresql.alter-index` / IF EXISTS | `pg-index-destination-guarded-*`, the wrong-kind tablespace guard and guarded options checks pin the absence short circuit. `pg-index-attach-guarded-absent-parent` records `42601` for the native grammar restriction. | The guard does not establish unqualified namespace selection or authorization. |
| `postgresql.create-operator-family` / CREATE OPERATOR FAMILY | `pg-operator-family-target-*` binds exact namespace, family name and index method. Native controls show that another method may reuse the name. | Unqualified creation namespace and owner privileges remain separate. |
| `postgresql.create-operator-family` / AS members | `create-operator-family-as-members` preserves the parser failure. The PostgreSQL 17.7 native probe reports `42601`. | PostgreSQL's grammar places members under ALTER OPERATOR FAMILY. This remains a syntax boundary for the retained form. |
| `postgresql.alter-operator-family` / ADD/DROP members | `pg-operator-family-member-*` checks one qualified B-tree search operator. It verifies strategy bounds, Boolean result, exact member occupancy and the protected drop slot. | Other methods, ordering operators, support functions, multi-member actions and broader dependency and privilege checks stay open. |
| `postgresql.alter-operator-family` / RENAME/OWNER/SET SCHEMA | `pg-operator-family-change-*` checks qualified source and method, occupied namespaces for that method, owner-first refusal, same-schema no-op and TOAST restriction. | Session alias resolution and access privileges still require evidence. |

The inventory keeps 301 rows and retains the original four F11 parent classifications. No cell is promoted to a full semantic-support claim from a generic warning or parser refusal.

The joint T39 preview passed 162 cases and 4,005 assertions. Delivered at `47cf3020b24fd6fddcf7e9420ee9250b6b3e683c`. All 50 canonical tasks passed with 4,276 cases and
88,372 assertions. T39 passed 162 cases and 4,005 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44893435461/job/136813955853),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44893435461/job/136813955806),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44893435461/job/136813955584),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44893435427/job/136813955470),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136814022125) passed at the delivered SHA.

## F08 exact dimension review

The six PostgreSQL 17 inventory rows retain all 37 authored dimensions. Existing
T36 controls distinguish parser normalization, governance and unsupported forms.
New cases separately record catalog decisions, native outcomes and guard order.

| Inventory dimension | Evidence and present scope | Remaining premise |
| --- | --- | --- |
| `postgresql.create-table` / basic columns | T36 `create-table-basic-columns` pins ordinary extraction. The column rule checks duplicate exact names and reserved system attribute names. The native controls preserve case-distinct quoted names and verify guard precedence. | General type resolution, type modifiers and full default binding need additional facts. |
| `postgresql.create-table` / IF NOT EXISTS | T36 `pg-create-keys-guard-before-key-validation`, `pg-create-commit-guard-skips-on-commit`, `pg-create-typed-guard-skips-missing-type` and `pg-create-storage-guard-skips-method-option-and-tablespace` show when native creation skips the rest of the declaration. | Generic creation guard behavior and session-dependent namespaces need separate evidence. |
| `postgresql.create-table` / column defaults | T36 `pg-create-expression-default-references-sibling-column` and `pg-create-expression-default-contains-subquery` check bounded refusals. Literal array and composite-field defaults execute correctly. | Function overload and type coercion, aggregate and window restrictions remain open. |
| `postgresql.create-table` / CHECK constraint | T36 `pg-create-expression-check-contains-subquery` checks refusal. `pg-create-expression-check-plain-boolean` is a valid control. | The invalid string, integer and missing column controls demonstrate remaining CHECK coercion and binding. |
| `postgresql.create-table` / PRIMARY KEY / UNIQUE | T36 `pg-create-keys-two-primary-key-declarations`, `pg-create-keys-duplicate-key-member`, `pg-create-keys-missing-included-member`, quoted members and INCLUDE controls check direct declarations. | Inherited, typed, LIKE, partition and opclass-dependent keys require their own evidence. |
| `postgresql.create-table` / FOREIGN KEY / REFERENCES | T36 checks external targets, exact local and remote members, referenced key eligibility and arity. Native controls distinguish immediate unique keys from partial indexes. | Self-reference and creation-time type equality use evidence gaps. Partitioning, scoped permissions and execution effects remain open. |
| `postgresql.create-table` / EXCLUDE constraint | T36 `create-table-exclude-constraint` pins explicit unsupported and native PostgreSQL can create the supported fixture shape. | Exclusion elements and strategy/operator compatibility need a normalized contract. |
| `postgresql.create-table` / table constraint clause | T36 `create-table-table-constraint-clause` pins named key extraction. Direct key controls cover selected semantics. | General name collisions, constraint options and cross-constraint ordering remain open. |
| `postgresql.create-table` / WITH storage parameters | T36 `pg-create-storage-valid-heap-fillfactor`, bound refusal, duplicate and Boolean controls verify exact built-in parameters. | Quoted numeric forms, TOAST, other parameters and custom method execution remain open. |
| `postgresql.create-table` / USING access method | T36 verifies explicit heap, wrong kind `55000`, absent method and exact quoted name. | Implicit method selection, custom method handler behavior and partition eligibility remain open. |
| `postgresql.create-table` / ON COMMIT / TABLESPACE | T36 checks target/guard precedence for explicit actions and table spaces. Absent and global spaces refuse. | Space permissions, implicit tablespace and partition inheritance remain open. |
| `postgresql.create-table` / OF composite type | T36 checks exact standalone composite identity and its live option names. Type eligibility, shell status, guards, dropped fields and exact spelling have native controls. | Session namespace, type USAGE and typed defaults or constraints require additional facts. `OF d.c[]` is a native `42601` and a DeltaScope parser diagnostic. |
| `postgresql.create-table` / arrays and composite columns | T36 verifies exact qualified composite types and their array types. Missing array targets, shell types and pseudo-types have refusal controls. Literal composite and array defaults have execution witnesses. | General unqualified type resolution, inherited column types and arbitrary composite expression legality remain open. |
| `postgresql.create-table` / generation-declaration | D2 T36 generation-declaration controls check mixed declaration forms, scalar identity type and sequence parameter domains. | General expression types, volatility and runtime sequence state remain open. |
| `postgresql.create-table-constraints` / PRIMARY KEY variants | T36 checks one primary declaration, key member validity, INCLUDE repeats and key overlap against `pg_index` and ordered attributes. | Additional constraint options and inherited/partitioned cases remain open. |
| `postgresql.create-table-constraints` / UNIQUE NULLS NOT DISTINCT | Both explicit NULL forms are extracted and have positive native `indnullsnotdistinct` and catalog witnesses. | Data-dependent uniqueness and unsupported partition behavior need separate evidence. |
| `postgresql.create-table-constraints` / UNIQUE USING INDEX | `pg-create-keys-using-index-from-create` proves the CREATE form parses and PostgreSQL refuses with `0A000`. | ALTER attachment retains its separate rules and T37 evidence. |
| `postgresql.create-table-constraints` / EXCLUDE | T36 `create-table-constraints-exclude` preserves explicit unsupported. | Method, operator and constraint attribute premises remain open. |
| `postgresql.create-table-constraints` / NOT NULL named constraint | T36 keeps the PostgreSQL 17 parser diagnostic for `CONSTRAINT nn NOT NULL id`. | PostgreSQL 18 syntax awaits an approved parser/anchor change where applicable. |
| `postgresql.create-table-constraints` / REFERENCES actions | T36 checks a valid `ON DELETE SET NULL` column list and refuses a column outside the referencing key with native `42P10`. | Other referential actions, deferrability and type equality need their own evidence. |
| `postgresql.create-table-generated-identity` / GENERATED ALWAYS AS IDENTITY | D2 and `generation-identity-controls` cover the declared mode on an integer column. | Future values and runtime sequence state remain open. |
| `postgresql.create-table-generated-identity` / BY DEFAULT AS IDENTITY | D2 and `generation-identity-controls` cover the declared mode. | Write-time override and issuance remain outside declaration proof. |
| `postgresql.create-table-generated-identity` / sequence options | D2 T36 verifies int64 preservation, negative increment, bounds, invalid forms and explicit CYCLE advice. | Sequence schema/name/ownership and execution depend on their own facts. |
| `postgresql.create-table-generated-identity` / GENERATED ALWAYS AS expression STORED | T36 checks subqueries and direct references to generated columns, including self-reference. A valid generation expression executes with the expected result. | Nested generated references, expression coercion and function volatility remain open. |
| `postgresql.create-table-generated-identity` / generation-declaration | D2 native positive and refusal cases verify declaration restrictions. | Broader execution eligibility remains open. |
| `postgresql.create-table-partitioning` / PARTITION BY RANGE | T36 `create-table-partitioning-partition-by-range` pins explicit unsupported. | CREATE partition key and strategy facts need a normalized semantic model. |
| `postgresql.create-table-partitioning` / PARTITION BY LIST | T36 `create-table-partitioning-partition-by-list` pins explicit unsupported. | Key and partition bounds need strategy-specific evidence. |
| `postgresql.create-table-partitioning` / PARTITION BY HASH | T36 `create-table-partitioning-partition-by-hash` pins explicit unsupported. | Method, key and modulus/remainder checks need their own facts. |
| `postgresql.create-table-partitioning` / PARTITION OF | T36 `create-table-partitioning-partition-of` pins explicit unsupported. | Parent identity, key correspondence and overlap require catalog and bound comparison evidence. |
| `postgresql.create-table-partitioning` / PARTITION OF DEFAULT | T36 `create-table-partitioning-partition-of-default` pins explicit unsupported. | Existing default partition, parent key and overlap require their own facts. |
| `postgresql.create-table-partitioning` / INHERITS | T36 checks ordered parent lookup, duplicate parents, views, partitioned roots and partition children. Successful controls verify `pg_inherits` and the merged column order. | The audit keeps a gap for inherited column merging and session-dependent parent names. |
| `postgresql.create-table-temporary` / TEMP/TEMPORARY | T36 verifies native temporary namespace rules for exact explicit targets. | Session temporary schema identity and privilege remain open. |
| `postgresql.create-table-temporary` / GLOBAL TEMPORARY compat noise | `pg-create-commit-global-temp-spelling` preserves the native deprecation notice and local temporary behavior. | No session-wide proof is claimed. |
| `postgresql.create-table-temporary` / UNLOGGED | T36 `pg-create-commit-unlogged-on-commit-delete-rows` verifies a permanent-storage refusal, and the temporary alias yields a gap. | Crash/recovery behavior and session-derived persistence remain outside declaration proof. |
| `postgresql.create-table-temporary` / ON COMMIT actions | T36 same-session native controls verify PRESERVE ROWS, DELETE ROWS and DROP after commit. | Transaction execution and temporary aliases require session evidence. |
| `postgresql.create-table-like` / LIKE basic | T36 checks exact ordered source kinds, destination guard and duplicate copied column names. | Source resolution via `search_path`, temporary overlays and execution permissions still require evidence. |
| `postgresql.create-table-like` / INCLUDING options | T36 preserves ordered AST sources and effective INCLUDE/EXCLUDE option bits. | Individual copied DEFAULT, generation, identity, index and storage definitions require additional facts. |

The joint T36 preview passed 214 cases and 5,903 assertions. Delivered at `10354d2bf167f0741a6eeb079e72cb981b4d15a8`. All 50 canonical tasks passed with 4,431 cases and
93,615 assertions. T36 passed 214 cases and 5,903 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44897967671/job/136827571766),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44897967671/job/136827571796),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44897967671/job/136827571522),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44897967497/job/136827571236),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136827630993) passed at the delivered SHA.

## F13 exact dimension review

All twelve original dimensions retain their own evidence. Qualified catalog
checks in T41 cover the premises below. The parent rows remain mixed.

| Inventory dimension | Evidence and present scope | Remaining premise |
| --- | --- | --- |
| `postgresql.create-type` / ENUM | `pg-enum-duplicate-enum-labels`, `pg-enum-duplicate-empty-labels`, the UTF-8 length boundary and shell and automatic-array fixtures check exact creation and every label. | Other encodings, implicit namespace selection and execution authorization. |
| `postgresql.create-type` / composite | `pg-composite-exact-composite-creation`, `pg-composite-duplicate-attributes`, and the shell, array and occupancy controls check exact target and attribute names. | Attribute types, collation and dependent objects. |
| `postgresql.create-type` / RANGE | `pg-range-integer-range`, subtype refusal, repeated option and collation controls check the exact destination and qualified subtype. `pg-range-empty-parameters-parser-boundary` verifies native `42601`. | Operator-class selection, collation identity, multirange name conflicts, support functions and execution privileges. |
| `postgresql.create-type` / base type | `pg-base-type-full-base-type-without-shell` and `pg-base-type-full-base-type-missing-input` verify the shell and required I/O declarations. The signature-mismatch case is a native refusal and an audit gap. | I/O function resolution, representation semantics and privileges. |
| `postgresql.create-type` / shell type | `pg-base-type-new-shell-type`, `pg-base-type-repeated-shell-type` and the automatic-array witness verify name eligibility. | Session namespaces and the privilege to create a shell. |
| `postgresql.alter-type` / ADD VALUE | T41 enum cases cover exact target kind, empty labels, duplicate guards before neighbor lookup and qualified placement. | Visibility and use of new labels during a transaction, implicit namespace selection and other encodings. |
| `postgresql.alter-type` / RENAME VALUE | T41 enum cases verify exact source and destination labels, equality, length and old-label lookup. Existing ADD VALUE advice applies only to ADD. | Other encodings, earlier unproved catalog mutations and permissions. |
| `postgresql.alter-type` / RENAME/OWNER/SET SCHEMA | T41 type-namespace cases cover type and relation names, the associated array, same-name versus same-schema behavior, owner precedence and shell ownership. | Broader dependency moves, ownership and namespace privileges and session namespaces. |
| `postgresql.alter-type` / composite attributes | T41 composite cases cover exact single ADD, DROP, ALTER and RENAME names, guarded absence and standalone relation kind. | Multi-action state, dropped attribute placeholders, dependent objects and type coercion. |
| `postgresql.alter-type` / SET properties | T41 base type cases verify fixed versus variable length, current storage, forbidden properties and kind. | Function resolution, type representation safety and effects on dependent types. |
| `postgresql.alter-operator` / OWNER/SET SCHEMA | T41 operator-change cases cover exact unary and binary signatures, role precedence and occupied destination. | Implicit operand naming, session namespaces and permissions. |
| `postgresql.alter-operator` / SET RESTRICT/JOIN | T41 estimator cases cover `NONE`, repeated option order, qualified signatures, result eligibility, valid built-ins and missing-function refusal. | Other SET options, unqualified function selection and execution privileges. |

The joint T41 preview passed 221 cases and 6,241 assertions. Delivered at `fd6b27926267b96138773fc37e21a8d56346bdcb`. All 50 canonical tasks passed with 4,572 cases and
99,020 assertions. T41 passed 221 cases and 6,241 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44902948993/job/136842505359),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44902948993/job/136842505579),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44902948993/job/136842505631),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44902948983/job/136842505275),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136842593183) passed at the delivered SHA.

## F14 exact dimension review

| Inventory dimension | Evidence and present scope | Remaining premise |
| --- | --- | --- |
| `postgresql.alter-policy` / USING/WITH CHECK | T42 verifies exact table and policy binding, SELECT and INSERT clause eligibility, quoted columns and null tests. The native controls demonstrate predicate parsing before missing policy diagnosis and preserve observed changes to each clause. | General expression types and function resolution, policy ownership and effective row access require separate evidence. |
| `postgresql.alter-policy` / TO role / RENAME | T42 verifies role resolution before relation lookup, PUBLIC skipping later roles, table-local destination occupancy, rename to the same name and destination-before-source precedence. The combined TO and RENAME form still returns native SQLSTATE `42601` and a parser diagnostic. | Session-dependent relation lookup and execution privileges require separate evidence. |

The joint T42 preview passed 65 cases and 1,261 assertions. Delivered at `9308fb4c9a1812618bd8a724f20d7dfca0814976`. All 50 canonical tasks passed with 4,592 cases and
99,812 assertions. T42 passed 65 cases and 1,261 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44906994323/job/136854599770),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44906994323/job/136854599769),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44906994323/job/136854599620),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44906994256/job/136854599427),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136854666281) passed at the delivered SHA.

## F15 exact dimension review

| Inventory dimension | Evidence and present scope | Remaining premise |
| --- | --- | --- |
| `postgresql.create-sequence` / CREATE SEQUENCE | T43 checks the qualified catalog target and confirms a smallint sequence definition without calling `nextval`. | Session namespace selection and execution privileges. |
| `postgresql.create-sequence` / options | T43 checks type, bounds, START, nonzero INCREMENT, positive CACHE, duplicate options and default descending bounds. Existing CYCLE advice is preserved. | Unqualified AS type resolution and runtime issuance. |
| `postgresql.create-sequence` / OWNED BY / TEMP / IF NOT EXISTS | T43 verifies owner kind, same owner and schema, exact column names, temporary namespace refusal and guard precedence. Valid VIEW ownership has a dependency witness. | Session-owned temporary schemas and name resolution through `search_path`. |
| `postgresql.alter-sequence` / RESTART | T43 verifies explicit restart bounds and that a bare restart uses START. | The audit reads stable definitions without reading or advancing current issuance. |
| `postgresql.alter-sequence` / options | T43 checks exact relation kind and altered bounds. It leaves an evidence gap when bounds narrow without a replacement restart. | Concurrent issuance and later batch effects. |
| `postgresql.alter-sequence` / OWNED BY / RENAME / SET SCHEMA | T43 distinguishes automatic and identity ownership, verifies their rename dependencies and checks exact relation occupancy. A free sequence can stay in its schema, while an owned sequence fails before destination resolution. | Execution privileges and wider dependency changes. |
| `postgresql.alter-sequence` / IF EXISTS | T43 verifies absent targets short-circuit ALTER, RENAME and SET SCHEMA. A relation of the wrong kind still fails. | Unqualified and session-dependent relation identities. |

The joint T43 preview passed 86 cases and 2,951 assertions. Delivered at `d11b4881b2d6b8740327c24e4ee5f037b495e6c7`. All 50 canonical tasks passed with 4,642 cases and
102,392 assertions. T43 passed 86 cases and 2,951 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44913894106/job/136875255978),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44913894106/job/136875255966),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44913894106/job/136875255774),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44913894095/job/136875255649),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136875330065) passed at the delivered SHA.

## F16 exact dimension review

| Inventory dimension | Evidence and present scope | Remaining premise |
| --- | --- | --- |
| `postgresql.alter-foreign-data-wrapper` / HANDLER/VALIDATOR changes | T44 checks qualified routine signatures and the handler result type. Native cases verify NO HANDLER, NO VALIDATOR, missing handlers and incompatible handler returns. | Unqualified function resolution, caller authorization, and behavior of handler or validator code. |
| `postgresql.alter-foreign-data-wrapper` / OPTIONS | T44 checks exact ordered option names. ADD requires absence. SET and DROP require presence. DROP then ADD and case-distinct names have native catalog witnesses. | Option values and validator results need separate evidence. |
| `postgresql.alter-foreign-data-wrapper` / OWNER/RENAME | T44 verifies role lookup before wrapper resolution, the target role's superuser requirement, destination occupancy and same-name rename refusal. | Caller privileges and session-dependent authorization. |
| `postgresql.alter-foreign-table` / OPTIONS | T44 checks exact foreign table kind and reuses the ordered option checks. | Validator results and the behavior of external systems. |
| `postgresql.alter-foreign-table` / ADD/DROP COLUMN | T44 verifies exact column names, guarded absence or occupancy, invalid system-column DROP and rename precedence. | Column type resolution, dependent and inherited changes, and multi-action state. |
| `postgresql.alter-foreign-table` / RENAME/OWNER/SET SCHEMA | T44 verifies source relation kind, destination occupancy, same-schema moves and role lookup after relation resolution. | Caller privileges and additional dependency changes. |

The joint T44 preview passed 89 cases and 2,825 assertions. Delivered at `d6cb07d2f93b81fe8dd26a7529cc864ba01e6057`. All 50 canonical tasks passed with 4,683 cases and
104,716 assertions. T44 passed 89 cases and 2,825 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44919263042/job/136891517804),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44919263042/job/136891517872),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44919263042/job/136891517612),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44919262993/job/136891517708),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136891611844) passed at the delivered SHA.

## F17 exact dimension review

| Row and authored cell | Verified PostgreSQL 17 premise | Remaining evidence |
| --- | --- | --- |
| `postgresql.alter-text-search-configuration` / `ADD/DROP/ALTER MAPPING` | Exact qualified configuration, built-in parser token aliases, ordered dictionary targets, ADD occupancy, ALTER insertion and replacement, and guarded DROP. T45 covers missing token before dictionary lookup and duplicate token names. | An unqualified dictionary or configuration needs effective session resolution. A custom parser needs a trusted token directory. |
| `postgresql.alter-text-search-configuration` / `RENAME/OWNER/SET SCHEMA` | Exact source, destination collision, and role resolution. T45 compares same-name RENAME and occupied SET SCHEMA with native errors. | Ownership, role membership and namespace privileges depend on the executing session. |
| `postgresql.alter-text-search-dictionary` / `option changes` | Exact dictionary and built-in simple template. T45 checks unsupported names, final ACCEPT Boolean value and absent option removal. | Stop-word-file availability requires evidence from the PostgreSQL server filesystem. Other templates require validator-specific evidence. |
| `postgresql.alter-text-search-dictionary` / `RENAME/OWNER/SET SCHEMA` | Exact source, destination and owner role. T45 tests conflicts and verifies the same owner transactionally. | Execution permissions and unqualified lookup depend on session facts. |

The two rows retain `mixed_coverage`. The 26 PostgreSQL 17 references include
24 live controls, one offline evidence gap and one corpus reference. Catalog
option values, opaque identities and unauthored dictionary names stay private.
See [decision record](../decisions/2026-11-21-postgresql-text-search-changes.md)
and [T45](../../testdata/ddl-golden/T45.json).

The joint T45 preview passed 59 cases and 1,435 assertions. Delivered at `fad0c7f4da39de4eed392b7f0cd8da332b9467e1`. All 50 canonical tasks passed with 4,708 cases and
105,801 assertions. T45 passed 59 cases and 1,435 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44937661085/job/136947737758),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44937661085/job/136947737609),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44937661085/job/136947738008),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44937661276/job/136947738124),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136947815184) passed at the delivered SHA.

## F18 exact dimension review

| Row and authored cell | Verified PostgreSQL 17 premise | Remaining evidence |
| --- | --- | --- |
| `postgresql.alter-publication` / `ADD/DROP/SET TABLE` | Exact publication source, qualified direct relation eligibility, presence and membership. T46 verifies successful changes, duplicate simple selectors, ONLY, FOR ALL TABLES and DROP row-filter and column-list refusals. | Implicit names, ordinary inheritance expansion, expressions and column-list validation require additional catalog or semantic facts. |
| `postgresql.alter-publication` / `SET TABLES IN SCHEMA` | Exact named schemas, ordered table/schema checks, set replacement, and schema versus existing column-list conflict. | `CURRENT_SCHEMA` and temporary schema aliases require the effective session. |
| `postgresql.alter-publication` / `SET publish options` | Ordered options, action list and Boolean syntax, and existing partition root row-filter or column-list conflict. | Runtime publisher behavior requires publisher evidence. |
| `postgresql.alter-publication` / `OWNER/RENAME` | Exact owner role lookup and same-name or occupied rename. | Actor ownership and role privileges require executing-session evidence. |
| `postgresql.alter-subscription` / `ENABLE/DISABLE` | AST Boolean including a quoted name containing DISABLE. ENABLE requires a slot name. | Worker and publisher state require runtime evidence. |
| `postgresql.alter-subscription` / `CONNECTION/PUBLICATION/SET` | Local identity, ordered publication-list changes, bounded options, slot preconditions and refresh policy. | Connection validation, remote publication availability and slot alteration require publisher or libpq evidence. |
| `postgresql.alter-subscription` / `REFRESH PUBLICATION` | Disabled subscription refusal and observed two-phase premise. | Remote refresh, transaction context and live worker state require separate facts. |
| `postgresql.alter-subscription` / `SKIP` | Reset and bounded LSN syntax. | Valid nonzero LSN requires replication origin progress. |
| `postgresql.alter-subscription` / `OWNER/RENAME` | Current-database identity, exact role lookup and rename occupancy. | Grant and role privileges require executing-session evidence. |
| `postgresql.drop-subscription` / `DROP SUBSCRIPTION` | Exact current-database identity and slotless local DROP. A slot is an explicit evidence gap with a native transaction refusal control. | Publisher-side slot effects and transaction context remain open. |
| `postgresql.drop-subscription` / `IF EXISTS` | Guard distinguishes a known absent name from an existing slotless subscription. | Remote work for an existing subscription with a slot needs publisher evidence. |
| `postgresql.drop-subscription` / `WITH drop_slot` | PostgreSQL 17.7 and the bound parser reject WITH with `42601`. | Revisit only if a future PostgreSQL grammar actually adds the form. |

All three parent rows retain `mixed_coverage`. The 69 references bind the 12
PostgreSQL 17 cells. Findings and gaps exclude notification contents, unreported
catalog publication names and slot identifiers. See [decision record](../decisions/2026-11-21-postgresql-replication-changes.md)
and [T46](../../testdata/ddl-golden/T46.json).

The joint T46 preview passed 96 cases and 3,981 assertions. Delivered at `3ca2c47b448883c268266e9a530c3c8897a6a060`. All 50 canonical tasks passed with 4,775 cases and
109,484 assertions. T46 passed 96 cases and 3,981 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44946815679/job/136975824213),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44946815679/job/136975824269),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44946815679/job/136975824020),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44946815677/job/136975824000),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136975898675) passed at the delivered SHA.

## F19 exact dimension review

| Row and authored cell | Verified PostgreSQL 17 premise | Remaining evidence |
| --- | --- | --- |
| `postgresql.grant` / `ON TABLE` | Ordered exact qualified targets, target kind, allowed privileges and grantee existence. T47 contrasts missing second targets and valid ordinary tables, views and sequence privileges. | Unqualified names and effective grant authority require executing-session evidence. |
| `postgresql.grant` / `ALL TABLES IN SCHEMA` | The AST and exact schema existence are checked. T47 shows the original table and view receiving the privilege in the native transaction. | The catalog rule reports a gap for member expansion and effective grants. |
| `postgresql.grant` / `columns` | Exact ordered column names, duplicates, and per-target availability. T47 verifies quoted comma names, system columns and a column missing from the second target. | Per-column grant chains require ACL and actor facts. |
| `postgresql.grant` / `WITH GRANT OPTION` | The parser preserves the modifier. T47 contrasts an ordinary role grant and the native PUBLIC refusal. | PUBLIC option effects retain an evidence gap for the privileges surviving the grantor check. |
| `postgresql.grant` / `sequences/functions/database/schema/types` | Sequence identity and kind, schema identity and privilege classes, and database identity in the provider. T47 retains the function and database native steps. | Routine and type object lookup remain unsupported, and live database-specific prerequisites need their own control. |
| `postgresql.grant` / `role membership` | Canonical T47 records a native ADMIN OPTION change and a missing role refusal. | Membership audit remains explicitly unsupported and needs a dedicated role declaration and identity model. |
| `postgresql.grant` / `tablespace/large objects` | Exact named tablespace identity is normalized. T47 retains native tablespace steps and compares an unknown large-object OID. | Large-object identity remains unsupported, and actor ACL facts remain open. |
| `postgresql.grant` / `GRANTED BY` | Exact grantor existence and the native current-user requirement. T47 distinguishes missing role and a different existing grantor. | The effective executing role still comes from the connected session. |
| `postgresql.revoke` / `REVOKE privileges` | Exact targets, role identities, column availability and authored privilege compatibility share the grant checks. | Existing privilege provenance and effective effects require ACL facts. |
| `postgresql.revoke` / `GRANT OPTION FOR` | The modifier is preserved. The bounded column declaration and a native CASCADE control are retained. | Effective grant-option changes depend on ACL provenance. |
| `postgresql.revoke` / `CASCADE/RESTRICT` | The parser preserves behavior and the existing warning remains. T47 retains the native test and scoped column control. | Transitive grant effects need complete grant-chain evidence. |
| `postgresql.revoke` / `role membership` | Native `REVOKE ADMIN OPTION FOR ... RESTRICT` changes the owned membership tuple. | Audit retains the explicit unsupported marker. |
| `postgresql.revoke` / `object kinds` | Named schema and supported relation classes share exact identity and privilege checks. | Unsupported kinds and effective privileges retain their own boundary. |
| `postgresql.create-language` / `CREATE LANGUAGE` | PostgreSQL and the parser lower the parameterless form to extension creation. T47 contrasts installed and unavailable extensions. | Extension installation, availability and scripts belong to the extension path. |
| `postgresql.create-language` / `OR REPLACE TRUSTED` | Parameterless form becomes `CREATE EXTENSION IF NOT EXISTS`. Explicit HANDLER replacement preserves the language OID and owner. | The parameterless TRUSTED keyword carries no execution premise. |
| `postgresql.create-language` / `HANDLER form` | Qualified handler, INLINE and VALIDATOR signatures, return type, callback order, local database and language-name conflict. | Unqualified callback lookup, actor privileges and callback execution require separate evidence. |

The three rows retain `mixed_coverage`. See [T47](../../testdata/ddl-golden/T47.json),
[privilege declarations](../decisions/2026-11-21-postgresql-object-privilege-declarations.md)
and [language declarations](../decisions/2026-11-21-postgresql-language-declarations.md).

The joint T47 preview passed 125 cases and 3,324 assertions. Delivered at `eacc45d46940f6080c045fe0a4ba0e64b19d0195`. All 50 canonical tasks passed with 4,826 cases and
111,990 assertions. T47 passed 125 cases and 3,324 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44954200488/job/136998584812),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44954200488/job/136998584674),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44954200488/job/136998584830),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44954200431/job/136998584537),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/136998660126) passed at the delivered SHA.

## F06 exact dimension review

| Original cell | TiDB 8.5 evidence | Audit boundary |
| --- | --- | --- |
| `FLASHBACK TABLE` | Isolated TiKV fixture refuses an occupied destination with `1050` and restores the renamed table with its witness row when the destination is free. | `flashback_table` unsupported. Execution needs the source DDL history and snapshot, GC and destination facts. |
| `FLASHBACK DATABASE` | Isolated TiKV fixture restores the dropped database and its table's witness row. The canonical unistore fixture refuses when its GC safe point is missing. | `flashback_database` unsupported. Recovery requires the history and snapshot for the database and its contained objects. |
| `FLASHBACK CLUSTER` | TiDB and the bound parser reject bare syntax. The two documented recovery point forms parse and generate `flashback_cluster` unsupported. | Live cluster restoration requires topology, safe point, DDL timing and execution evidence. The tests stop at the audit boundary for valid cluster requests. |
| `RECOVER TABLE` | TiKV restores by name and explicit DROP job ID. It refuses a missing history entry or a CREATE job ID. | `recover_table` unsupported. A job lookup and retrievable snapshot are required. |

The parent row retains `mixed_coverage`. Six canonical T16 cases supplement
its historical probes. [Native fixture and evidence](../../testdata/ddl-inventory/evidence/tidb.flashback-recover.native.md)
· [Decision record](../decisions/2026-11-21-tidb-recovery-boundaries.md).

The joint T16 preview passed 66 cases and 974 assertions. Delivered at `0b485f886099f60b3b7a5023b521bf616a8aab75`. All 50 canonical tasks passed with 4,832 cases and
112,120 assertions. T16 passed 66 cases and 974 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44960903269/job/137019256983),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44960903269/job/137019256855),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44960903269/job/137019256927),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44960903245/job/137019256707),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137019318257) passed at the delivered SHA.

## F05 exact dimension review

| Row and authored cell | Verified scope and remaining evidence |
| --- | --- |
| `mysql.alter-table.online-ddl` / `ALGORITHM=INSTANT/INPLACE/COPY/DEFAULT` | T15 contrasts MySQL 5.7's unavailable INSTANT with supported 8.x additions and the observed MyISAM, InnoDB COMPRESSED and FULLTEXT refusals. INPLACE and COPY/DEFAULT controls preserve existing warning behavior. Record-size and row-version limits need additional facts. |
| `mysql.alter-table.online-ddl` / `LOCK=NONE/SHARED/EXCLUSIVE/DEFAULT` | T15 verifies bounded legal pairs and COPY+NONE and INSTANT+explicit lock refusals. Actual lock acquisition and duration depend on concurrent activity. |
| `mysql.alter-table.options` / `ORDER BY` and `FORCE` | T15 pins the MySQL 8.0 warning. MySQL 5.7.44 and 8.4.10 native controls and CLI observations preserve the fixture schema and row. Storage engine and operation details determine physical work. |
| `mysql.alter-table.options` / `CONVERT TO CHARACTER SET` | T15 pins the MySQL 8.0 warning. Native 5.7.44 and 8.4.10 controls verify the character column’s recorded `utf8mb4` and `utf8mb4_bin` and preserve the row. Conversion success for business values remains data-dependent. |
| `mysql.alter-table.options` / `DEFAULT CHARACTER SET` | Native T15 checks the changed table default and unchanged character set of existing columns across three MySQL versions. |
| `mysql.alter-table.options` / `ROW_FORMAT` | Existing catalog comparison raises a warning for a changed format. Native T15 checks DYNAMIC to COMPACT. |
| `mysql.alter-table.options` / `AUTO_INCREMENT=` | Existing comparison warns on a shrink request. Native T15 observes the requested 4 clamped to 8 with a fixture row of 7. MySQL 8.x state verification reads uncached statistics. |
| `mysql.alter-table.options` / `ENABLE/DISABLE KEYS` | T15 retains the lifecycle notice and observes InnoDB’s 1031 note with unchanged index state. MySQL 5.7.44, 8.0.46 and 8.4.10 MyISAM controls verify the nonunique index’s disabled and enabled states, with the primary key, unique key and rows preserved. Lock duration and physical work depend on execution conditions. |
| `mysql.alter-table.options` / `COMMENT` and `ENGINE` | Existing metadata comparisons report a changed option. Native T15 observes the changed comment and the MyISAM engine. |
| `tidb.alter-table.online-options` / ALGORITHM and LOCK | T15 compares accepted COPY, INPLACE, DEFAULT and LOCK requests with the INSTANT index refusal. These clauses do not select a MySQL algorithm. |
| `tidb.alter-table.online-options` / multiple actions | T15 verifies successful supported combinations, refusal without partial updates for the tested INSTANT index combination, and the last authored algorithm. Other action combinations require their own checks. |

The MySQL and TiDB modifier parent rows move to `mixed_coverage`. The MySQL table-option row remains `mixed_coverage`. [T15](../../testdata/ddl-golden/T15.json) · [Decision record](../decisions/2026-11-21-mysql-instant-add-table-prerequisites.md)

The joint T15 preview passed 94 cases and 1,778 assertions. Delivered at `07f4aca5b779009468fc16dc403ec1c926d87d56`. All 50 canonical tasks passed with 4,884 cases and
113,418 assertions. T15 passed 94 cases and 1,778 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44966064784/job/137035257871),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44966064784/job/137035257676),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44966064784/job/137035257938),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44966064804/job/137035257889),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137035314703) passed at the delivered SHA.

## F01 exact dimension review

| Row and authored cell | Verified evidence and remaining premise |
| --- | --- |
| `mysql.create-procedure` / CREATE PROCEDURE ... BEGIN..END | T27 retains compound-body controls. The new qualified identity cases compare a duplicate, guarded duplicate and missing schema against the native server. A catalog miss remains a gap. |
| `mysql.create-procedure` / parameters | T27 verifies modeled names and parameter shape. Default charset and conversion behavior require additional declaration facts. |
| `mysql.create-procedure` / SQL SECURITY | D1 and the three native T27 catalog cases cover authored and default security modes. |
| `mysql.create-procedure` / DETERMINISTIC | The parser preserves explicit true and false and keeps omission absent. MySQL 5.7.44, 8.0.46 and 8.4.10 record `YES`, `NO` and `NO` for the three declarations. Runtime determinism remains outside static audit. |
| `mysql.create-procedure` / body statements | T27 compares bounded static SQL and malformed declaration members. Runtime statements and unmodeled members retain their boundaries. |
| `mysql.create-function` / CREATE FUNCTION ... RETURNS ... BEGIN..END | T27 verifies qualified stored function identity separately from procedure and loadable-function identity. RETURN retains its explicit body boundary. |
| `mysql.create-function` / DETERMINISTIC | The qualified function control verifies the catalog declaration. Binary logging and runtime determinism need separate facts. |
| `mysql.create-function` / SQL SECURITY | Existing D1 warnings and T27 native catalog evidence remain in force. |
| `mysql.alter-procedure-function` / ALTER PROCEDURE and ALTER FUNCTION | Qualified identity includes the kind. T27 compares visible targets, missing targets and wrong-kind targets. Missing visibility cannot prove absence. |
| `mysql.alter-procedure-function` / SQL SECURITY and COMMENT | Existing security warnings retain their authoring semantics. Native controls verify the updated comment and INVOKER state. The audit records the comment declaration without reading its payload. |
| `mysql.drop-procedure-function` / DROP PROCEDURE and DROP FUNCTION | Native controls verify target-kind distinction and shared names. Audit checks a visible qualified identity and leaves an unobserved target as an evidence gap. |
| `mysql.drop-procedure-function` / IF EXISTS | T27 verifies the absent guarded no-op and case behavior at each anchor. |
| `mysql.stored-body-analysis` / static body SQL, DECLARE/local binding, control flow | Existing T27/T28 cases cover bounded static SQL, declarations and modeled branch analysis. Calls and runtime values require separate evidence. |
| `mysql.stored-body-analysis` / PREPARE/EXECUTE dynamic SQL | T28 compares statically known literals, expression sources and the dynamic boundary. No body is invoked to establish coverage. |

The DROP parent row moves to `mixed_coverage`. Existing SQL SECURITY findings and unknown-body boundaries are retained. [Decision record](../decisions/2026-11-21-mysql-routine-targets.md) · [T27](../../testdata/ddl-golden/T27.json) · [T28](../../testdata/ddl-golden/T28.json)

Isolated preview T27 passed 99 cases and 2,286 assertions. Delivered at `a2f73b0ce445031ac59d1e8840127b5a78246887`. All 50 canonical tasks passed with 4,932 cases and
115,077 assertions. T27 passed 99 cases and 2,286 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44971740396/job/137052832407),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44971740396/job/137052832198),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44971740396/job/137052832395),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44971740266/job/137052831864),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137052905453) passed at the delivered SHA.

## F02 exact dimension review

| Row and authored cell | Verified evidence and remaining premise |
| --- | --- |
| `mysql.create-trigger` / BEFORE/AFTER INSERT/UPDATE/DELETE, FOR EACH ROW | The parser retains timing, event, target and row form. T29 adds native duplicate, missing-table and legal sibling controls. The guarded CREATE form is native on the two MySQL 8.x anchors and retains a parser boundary in DeltaScope. |
| `mysql.create-trigger` / FOLLOWS/PRECEDES | T29 compares exact sibling grouping, successful placement, wrong table, wrong timing and missing sibling. Unobserved sibling evidence stays a gap. |
| `mysql.create-trigger` / OLD/NEW bindings | Existing T29 verifies OLD on INSERT and NEW on DELETE boundaries and retains valid pseudo-column references. Runtime values and body effects require separate evidence. |
| `mysql.drop-trigger` / DROP TRIGGER and IF EXISTS | T29 distinguishes an absent target from guarded absence and records case-variant behavior for each MySQL version. The provider supplies positive visibility evidence only. |
| `mysql.create-event` / AT/EVERY, ON COMPLETION, ENABLE/DISABLE and DO | T30 preserves bounded declarations and body analysis, checks duplicate identity, observes one-time preserve versus invalid interval, and leaves the fixture disabled. Evaluation of scheduler expressions requires additional facts. |
| `mysql.alter-event` / ON SCHEDULE/RENAME/ENABLE/DISABLE/DO | Existing schedule, status and body controls remain. Added native cases show missing, occupied, same-name, case-only and successful rename. Catalog absence stays a gap. |
| `mysql.drop-event` / DROP EVENT and IF EXISTS | T30 verifies absent rejection and guarded absence. Executing-session privileges and scheduler effects remain separate. |

The DROP rows move to `mixed_coverage`. See [decision record](../decisions/2026-11-21-mysql-trigger-event-identities.md), [T29](../../testdata/ddl-golden/T29.json), and [T30](../../testdata/ddl-golden/T30.json).

Isolated preview T29 passed 57 cases and 1,298 assertions, T30 passed 61 cases and 1,416 assertions. Delivered at `d49b805c2fd64382a5fd4df9f4aae8238ea9f0e9`. All 50 canonical tasks passed with 4,995 cases and
117,200 assertions. T29 passed 57 cases and 1,298 assertions, T30 passed 61 cases and 1,416 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44977093432/job/137069308262),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44977093432/job/137069308357),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44977093432/job/137069308049),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44977093269/job/137069308110),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137069370087) passed at the delivered SHA.

## F04 exact dimension review

| Row and authored dimension | Verified evidence and remaining premise |
| --- | --- |
| `mysql.create-index` / `CREATE INDEX` | T09 compares successful standalone creation, duplicate names and missing columns across MySQL 5.7.44, 8.0.46 and 8.4.10. The existing existence rules supply the findings. |
| `mysql.create-index` / `CREATE UNIQUE INDEX` | T09 checks successful creation of a unique prefix member and its native catalog state. Data-dependent uniqueness and build behavior need separate evidence. |
| `mysql.create-index` / `key parts` | T09 checks the exact explicit column target. T10 retains expression, visibility and member-kind evidence. |
| `mysql.create-index` / `prefix lengths` | The existing configurable `prefix_length` feature detects 1089. Native full-width prefixes produce a NULL `SUB_PART` and shorter prefixes retain their length. |
| `mysql.drop-index` / `DROP INDEX name ON table` | T09 compares 1091, 1553 and success with an alternate supporting index. The new warning requires a positive dependency from ordered pre-state. A parent-only observation leaves inbound dependencies unproved. |
| `tidb.drop-index` / `DROP INDEX name ON table` | T09 compares sole and alternate FK support, an unrelated index, and guarded absence. `DROP INDEX IF EXISTS` no longer triggers the missing-index blocker. |

The three parent rows move to `mixed_coverage`. The warning uses the same dependency classifier as ordered state and keeps the underlying constraint identities private. [T09](../../testdata/ddl-golden/T09.json) · [Decision record](../decisions/2026-11-21-mysql-tidb-drop-index-dependencies.md)

Isolated preview T09 passed 157 cases and 4,440 assertions. Delivered at `3b0cd01b0bd0d6f0b482aa485f31b1d5b9521865`. All 50 canonical tasks passed with 5,027 cases and
118,186 assertions. T09 passed 157 cases and 4,440 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44983193844/job/137087988059),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44983193844/job/137087988329),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44983193844/job/137087988418),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44983193879/job/137087988257),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137088054325) passed at the delivered SHA.

## F03 exact dimension review

| Row and authored dimension | Verified evidence and remaining premise |
| --- | --- |
| `mysql.create-table-generated-column` / GENERATED ALWAYS AS (expr), VIRTUAL, STORED | T07 verifies bounded expression references, both storage kinds, and stored and virtual dependencies across the MySQL anchors. Arbitrary expression evaluation needs separate evidence. |
| `mysql.create-table-generated-column` / expression DEFAULT (8.0.13+) and INVISIBLE column (8.0.23+) | T07 compares versioned acceptance and native catalog effects. The declaration rule does not evaluate default expressions on business data. |
| `mysql.alter-table.add-column` / ADD COLUMN and ADD COLUMN FIRST/AFTER | T05 verifies first-path and ordered additions. T08 compares missing and existing AFTER targets. |
| `mysql.alter-table.add-column` / ADD (col, col, ...) | T05 compares valid grouped actions and repeated names. Native MySQL rejects a repeat with 1060. The ordered state keeps the original shape. |
| `mysql.alter-table.drop-column` / DROP COLUMN | T07 compares a generated leaf with a still-referenced base column. Broader dependencies require additional catalog facts. |
| `mysql.alter-table.modify-column` / MODIFY COLUMN, type change, NULL/NOT NULL transitions, FIRST/AFTER | T05 and T08 bind source type, nullability and position. Native outcome can depend on values and SQL mode. |
| `mysql.alter-table.multi-action` / comma-separated action lists | T05 and T08 compare supported sequences and atomically refused actions. A declaration with insufficient facts keeps its existing evidence gap. |
| `tidb.alter-table.columns` / ADD/DROP/MODIFY/CHANGE/RENAME COLUMN and FIRST/AFTER | T05, T07 and T08 compare the existing transitions. TiDB checks the tested AFTER references against the original table. |
| `tidb.alter-table.columns` / TiDB same-object modification limits | T05 reports repeated ADD declarations, including case variants and guarded forms. T08 verifies refusal for repeated MODIFY and DROP followed by ADD on the same identity. Other action combinations need their own evidence. |

The six parent rows move to `mixed_coverage`. The existing blocker now covers repeated names in ADD-only lists. [Decision record](../decisions/2026-11-21-mysql-tidb-repeated-add-column.md) · [T05](../../testdata/ddl-golden/T05.json) · [T07](../../testdata/ddl-golden/T07.json) · [T08](../../testdata/ddl-golden/T08.json)

Isolated preview T05 passed 268 cases and 5,512 assertions. Delivered at `cec48651b9ec8cb47d985c215e6ce9eea94da90e`. All 50 canonical tasks passed with 5,048 cases and
118,683 assertions. T05 passed 268 cases and 5,512 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44989397565/job/137106924695),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44989397565/job/137106924721),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44989397565/job/137106924560),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44989397560/job/137106924556),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137106999998) passed at the delivered SHA.

## F09 qualified CHECK and numeric constants (D52–D53)

`ADD CHECK` and `ADD CHECK NOT VALID` on a fresh qualified ordinary table
now check simple column and whole-row null tests with exact schema, relation
and observed current-database qualifiers. PostgreSQL can resolve a missing
qualified field through a whole-row function or cast. Audit blocks the missing
column only when neither catalog name exists. A possible fallback keeps a gap.

The direct numeric controls distinguish `integer`, `bigint` and `numeric`,
including the signed boundary, radix syntax, digit separators, and bounded
decimal and exponent syntax. The catalog observation binds the exact source
type. Explicit-only Boolean casts still refuse. Assignment and implicit casts
retain a gap for the result. General expressions, inheritance and data effects
retain their earlier boundaries.

[T37](../../testdata/ddl-golden/T37.json) · [Qualified references](../decisions/2026-11-21-postgresql-check-qualified-reference.md) · [Numeric constants](../decisions/2026-11-21-postgresql-check-numeric-constants.md)

Isolated preview T37 passed 650 cases and 19,523 assertions. Delivered at `c5f74b7156f64ee97d0a6b7a126c84a8571fa11b`. All 50 canonical tasks passed with 5,085 cases and
120,024 assertions. T37 passed 650 cases and 19,523 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44996146949/job/137127506556),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/44996146949/job/137127506418),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/44996146949/job/137127506605),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44996146997/job/137127506254),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137127582785) passed at the delivered SHA.

## F10 DROP TABLE row-type dependencies

T38 compares external columns whose type is a selected table’s composite row or its generated array, and verifies that RESTRICT refuses while a dependent outside the selection survives. Selecting the dependent table in the same DROP succeeds in either order. CASCADE removes the dependent column while the table and its fixture row remain. The controls preserve the distinction between a missing DROP target, an ignored absent target under IF EXISTS, and a deleted column that carries no live dependency.

The existing finding now includes these direct normal dependency edges. Partition and inheritance targets, general dependency closure and execution permissions require further facts. [T38](../../testdata/ddl-golden/T38.json) · [Decision record](../decisions/2026-11-21-postgresql-drop-table-row-type.md)

Isolated preview T38 passed 70 cases and 2,046 assertions. Delivered at `ce8a8478ef1431dad5fa1caf1dd9b9f912d16e53`. All 50 canonical tasks passed with 5,093 cases and
120,309 assertions. T38 passed 70 cases and 2,046 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45006663519/job/137159660978),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45006663519/job/137159660590),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45006663519/job/137159660841),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45006663515/job/137159660683),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137159762532) passed at the delivered SHA.

## F12 DROP VIEW row-type array dependencies

T40 compares qualified single and multi-target DROP VIEW with ordinary columns that directly use the generated array type of a selected view. A live dependent column outside the DROP set blocks RESTRICT. A selected dependent view is excluded from the external dependency check. IF EXISTS preserves the absent-target guard and a later missing target without a guard still fails first.

The PostgreSQL 17 RECURSIVE cell also links the existing T40 parse and audit
control. Recursive self-binding and query execution remain open.

The CASCADE case verifies that PostgreSQL removes the dependent column and keeps the consumer table and row. The audit preserves the existing reason code and leaves general dependency closure and execution prerequisites open. [T40](../../testdata/ddl-golden/T40.json) · [Decision record](../decisions/2026-11-21-postgresql-drop-view-row-type-arrays.md)

Isolated preview T40 passed 187 cases and 5,650 assertions. Delivered at `00911608e4ec93087207538d78fabc391bad3578`. All 50 canonical tasks passed with 5,101 cases and
120,605 assertions. T40 passed 187 cases and 5,650 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45011808279/job/137175555381),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45011808279/job/137175555435),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45011808279/job/137175555249),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45011808292/job/137175555724),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137175629084) passed at the delivered SHA.

## F09 deferrability on partition-derived foreign keys (D54)

For a single ALTER CONSTRAINT on an exact qualified PostgreSQL 17 target, the existing blocker now checks a positively observed partition-derived foreign key. Native controls reproduce the refusal with changed flags, identical flags and ONLY. The same fixture verifies the local foreign key, the partition root and an unrelated ordinary foreign key. The parent relation and linkage stay private.

Native ONLY on the root in this fixture still propagates to the derived constraint. The audit checks the directly addressed constraint. Wider propagation, trigger effects and execution prerequisites require separate evidence. [T37](../../testdata/ddl-golden/T37.json) · [Decision](../decisions/2026-11-22-postgresql-derived-fk-deferrability.md)

Isolated D54 T37 passed 622 cases and 18,503 assertions. Delivered at `82ba7f008568d158422a19c485c05e53df43e96f`. All 50 canonical tasks passed with 5,110 cases and
120,926 assertions. T37 passed 659 cases and 19,844 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45017033078/job/137191440009),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45017033078/job/137191439715),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45017033078/job/137191440011),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45017032927/job/137191439588),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137191501388) passed at the delivered SHA.

## F08 exact T36 evidence bindings

Nineteen selectors bind twelve PostgreSQL 17 subactions across four CREATE TABLE rows. The identity cells point to exact positive or refusal cases with native catalog witnesses. The EXCLUDE and partition cases preserve their explicit unsupported outcomes, and the PostgreSQL 17 table-level named NOT NULL form retains its parser boundary. The selectors reuse the T36 receipt at `10354d2b`, which passed 214 cases and 5,903 assertions. Row and cell classifications retain their earlier values. [T36](../../testdata/ddl-golden/T36.json)

Delivered at `fa051151f86e13ea91d86bf02c07e917abc8fd4f`. All 50 canonical tasks passed with 5,110 cases and
120,926 assertions. T36 passed 214 cases and 5,903 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45022063452/job/137206951336),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45022063452/job/137206951111),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45022063452/job/137206951258),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45022063375/job/137206951204),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137207015444) passed at the delivered SHA.

## F07 exact T35 evidence bindings

The existing `create-schema-schema-elements` case now binds directly to the PostgreSQL 17 nested schema elements cell. The case preserves the adapter unsupported outcome and the prior native comparison remains in the F07 review. The inventory and cell classifications retain their values. [T35](../../testdata/ddl-golden/T35.json)

Delivered at `8308e30f339010f021e5a2104da5f2d3c4487946`. All 50 canonical tasks passed with 5,110 cases and
120,926 assertions. T35 passed 77 cases and 1,608 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45027551733/job/137223774759),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45027551733/job/137223774684),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45027551733/job/137223774806),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45027551723/job/137223774747),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137223871615) passed at the delivered SHA.

## F11 exact T39 evidence bindings

Eight CREATE INDEX cells now point directly to their existing PostgreSQL 17 normalization and audit controls. The retained `AS members` form links the exact parser refusal. Index eligibility, method behavior and expression semantics retain their earlier boundaries. The inventory and cell classifications retain their values. [T39](../../testdata/ddl-golden/T39.json)

Delivered at `98c056e5cc82133abc4dfaaa682a143f23ea2cd8`. All 50 canonical tasks passed with 5,110 cases and
120,926 assertions. T39 passed 162 cases and 4,005 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45033067511/job/137240722940),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45033067511/job/137240722866),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45033067511/job/137240722648),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45033067563/job/137240722365),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137240831279) passed at the delivered SHA.

## F09 direct DROP CONSTRAINT on a partition-inherited FK (D55)

The existing target rule now refuses a direct DROP of an exact qualified partition foreign key when the PostgreSQL 17 catalog proves a positive inheritance count. The native controls distinguish the derived key from local, root and unrelated keys. `ONLY` and `IF EXISTS … CASCADE` preserve the direct refusal. A missing constraint guarded by `IF EXISTS` succeeds.

The catalog query and output contract preserve the earlier CHECK check. The finding exposes the authored target and kind. General cascade effects and execution prerequisites need their own evidence.

The inventory also links the existing T37 PostgreSQL 17 parser refusals for ALTER CONSTRAINT NOT ENFORCED, table-level NOT NULL NOT VALID, and ALTER CONSTRAINT INHERIT. The parser-upgrade census separately records the NO INHERIT refusal. These entries retain their PostgreSQL 17 classification and #149 follow-up. [T37](../../testdata/ddl-golden/T37.json) · [Parser-upgrade census](../decisions/2026-06-02-v0.250.0-parser-upgrade-candidate-evidence-pack.md) · [Decision](../decisions/2026-11-22-postgresql-drop-inherited-foreign-key.md)

Isolated D55 T37 passed 621 cases and 18,463 assertions. Delivered at `1414066dfebf219f574f186f56fab55d4baa9897`. All 50 canonical tasks passed with 5,118 cases and
121,207 assertions. T37 passed 667 cases and 20,125 assertions. The validator,
read-only review, owned resource cleanup and merged-root gates passed.
[Go tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45041021582/job/137265268755),
[PostgreSQL tests](https://github.com/Fanduzi/DeltaScope/actions/runs/45041021582/job/137265268689),
[Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45041021582/job/137265268474),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45041021524/job/137265268673),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/137265404042) passed at the delivered SHA.

## F09 direct bool/int4 SET DEFAULT assignment

For `postgresql.alter-table.column-attributes` → `SET/DROP DEFAULT` →
`postgres17`, the existing default-on rule adds two refusals: direct `bool → int4`
and `int4 → bool` literals in single-action `SET DEFAULT` against an observed
qualified ordinary column. PostgreSQL 17.7 refused both with `42804`; same-type,
accepted unknown-text and explicit-cast controls stayed silent. The finding
adds `default_literal_type_mismatch`, not catalog IDs or literal contents.
Other coercions, dependencies, relation kinds and ordered action effects remain
open. The inventory adds seven evidence links without changing classifications
or denominators. See the [decision](../decisions/2026-11-21-postgresql-set-default-expression-references.md#direct-boolean-and-int4-literal-assignment)
and the [T37 controls](../../testdata/ddl-golden/T37.json).

`milestone/pg-alter-default-assignment` fast-forwarded from
`0101f8c09a42680d8da0bc687b8f7eaaf32046ef` to
`7d76a83789650be50b8c63bf345f97b33e47593c`; the same range was pushed to
`origin/main`. The implementation is `9a6d4389`; the follow-up adds SDK, HTTP
and MCP assertions. The final candidate passed build, unit and lint gates
with and without PostgreSQL tags, vet, tidy, formatting, diff, inventory,
corpus, coverage catalog and decision-record checks. In the isolated worktree,
`make ddl-golden TASK=all ARTIFACT_DIR="$receipt/full"` ran 50 tasks, 5,124 cases
and 121,403 assertions with no failures or skips. The command
`python3 scripts/ddl_golden.py validate --artifact "$receipt/full/all/artifact.json"`
validated the aggregate. T37
passed 673 cases and 20,321 assertions. Read-only review confirmed the SHA,
CLI hash, scope and evidence bindings. The merged-tree checks passed with a
fresh lint cache after the shared cache reported paths from a removed preview.
The protected checkout was unchanged; isolated resources were cleaned up and
its VM stopped. Receipts: `/private/tmp/ds-r2-opclass.CUubqR/f09-default-assignment-7d76a837/`.

At that implementation SHA, [Go](https://github.com/Fanduzi/DeltaScope/actions/runs/45873004650/job/140092946220),
[PostgreSQL](https://github.com/Fanduzi/DeltaScope/actions/runs/45873004650/job/140092946260),
[lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45873004650/job/140092945804),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45873004655/job/140092945521),
and [Pages](https://github.com/Fanduzi/DeltaScope/runs/140093067017) succeeded.
This documentation records those results, not a new Golden run. F09 remains open.

## F09 positional parameters in SET DEFAULT

Delivered at `84b077743da4ce5179c2e2890faf603357775ba1` from base
`20b18009e2bca29b489a981cb5249cbfd07e6809` by fast-forward on
`milestone/pg-alter-default-assignment`, then pushed to `origin/main`.
For `postgresql.alter-table.column-attributes` → `SET/DROP DEFAULT` →
`postgres17`, the existing rule adds `default_parameter_inapplicable` for
a bare positional parameter or a direct COALESCE argument in a single-action
SET DEFAULT on a verified qualified ordinary table. PostgreSQL 17.7 refused
both with `42P02`. Quoted text remains valid; existing generated-column and
subquery refusals keep their precedence. Findings omit the parameter number
and expression. Other expression shapes and relation kinds remain open.
See the [decision](../decisions/2026-11-21-postgresql-set-default-expression-references.md#positional-parameters-in-set-default)
and the [T37 controls](../../testdata/ddl-golden/T37.json). Six evidence links
were added without changing inventory classifications or denominators.

Build, test and lint (default and PostgreSQL-tagged), vet, tidy, formatting,
diff, inventory, corpus, coverage catalog and decision gates passed. In the
isolated checkout, `make ddl-golden TASK=T37 ARTIFACT_DIR="$receipt/targeted"`
passed 678 cases and 20,487 assertions.
`make ddl-golden TASK=all ARTIFACT_DIR="$receipt/full"` passed all 50 tasks,
5,129 cases and 121,569 assertions, with no failures or skips.
`python3 scripts/ddl_golden.py validate --artifact "$receipt/full/all/artifact.json"`
validated the aggregate. Read-only review matched every artifact to the
candidate and CLI hash. Merged-tree gates passed before pushing; the protected
checkout was unchanged and the isolated VM stopped after fixture cleanup.
Receipts: `/private/tmp/ds-r2-opclass.CUubqR/f09-default-parameter-84b07774/`.

[Go, PostgreSQL and lint](https://github.com/Fanduzi/DeltaScope/actions/runs/45914973560),
[TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/45914973597), and
[Pages](https://github.com/Fanduzi/DeltaScope/runs/140231981711) succeeded at
that implementation SHA. This entry records that run; it is not fresh Golden
evidence for the documentation commit. F09 remains open.

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

- #149 checked 2026-11-18 against official tags/releases and `go list`: v7 has
  no published versions and `@latest` has no matching version. Existing v6.2.5
  (2026-09-30) still binds PG17, not the awaited PG18 Go binding; project stays
  v6.2.2. The five PG18 candidates remain parser_error and guidance tests passed.
  No new binding was found, so no comment/closure/upgrade occurred. Future
  publication still requires separately approved upgrade/promotion verification.
- PR #112 reviewed at `0fa922d0` (all three commits): recommend modifying then
  retaining the thin router, restoring explicit L3 retirement/non-gating and
  reconciling ignored local guidance; old-head lint failure is not CI green.
  [Recommendation](https://github.com/Fanduzi/DeltaScope/pull/112#issuecomment-6977093461)
  was submitted; PR remains OPEN, without merge/closure/history rewriting.
- PR #150 reviewed at `99cffb17`: recommend preserving the redacted historical
  T20 report, not treating its 2f01/b33 evidence as current-HEAD acceptance or
  closure of resource/data-loss/privacy uncertainties. Public artifact hashes,
  35/497 and 8-executed/12-reused identities were checked, not private forensics.
  [Recommendation](https://github.com/Fanduzi/DeltaScope/pull/150#issuecomment-6977362746)
  was submitted; PR remains OPEN and unchanged, without merge/closure.
- Release/tag decisions are outside this program's push authorization.

## Ledger verification at creation

The 63 row IDs and 19 family memberships are mechanically compared with the
original family table. Every authored dimension is compared, in order, with
its exact inventory row at fixed baseline `d8cceea6`; the normalized record
names and per-row union count are checked separately. Local Markdown links
and existing D1/D2 proof refs are checked. The inventory, matrix, rules, policy,
corpus and golden manifests are unchanged by creating this ledger.
