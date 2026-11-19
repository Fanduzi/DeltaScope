# testdata/ddl-inventory

Machine-checkable official DDL acceptance inventory for the
mysql-tidb-ddl-completion milestone (issue #79). This is the versioned
denominator: what the official documentation actually offers, classified
honestly — not a count of implemented rules.

## Files

| File | Responsibility |
|------|----------------|
| inventory.yaml | The three MySQL CREATE/ALTER routine rows bind `sql-security` subactions to `ddl.routine.security.definer.warn` corpus selectors and exact T27 CLI/anchor-owned catalog cases; CREATE rows remain mixed, ALTER becomes mixed rather than completed because general attributes remain notice-only. The inventory: verified official sources, status vocabulary, owner tasks (real issues), the `required_row_ids` denominator, and one row per statement family/subaction scoped to the versions where it exists |
| required_rows.txt | Locked denominator baseline — one row ID per line, checked in independently of `inventory.yaml`. The gate requires `rows` == `required_row_ids` == this file, so shrinking both YAML fields together still fails |
| matrix.json | Derived view of the inventory (`id`, `product`, `owner`, derived `status`, `parser_status`, `audit_status`, `applicability`, optional `subaction_status`). Regenerate with `python3 scripts/gen_ddl_inventory_matrix.py` after any inventory change; do not hand-edit |
| T06-base-traceability.md | Bounded T06 (#85) traceability table: maps each CREATE TABLE acceptance dimension on `mysql.create-table`/`tidb.create-table` to normalized spec fields, consuming rules, and concrete evidence; records accepted representation limits and other-task boundaries |
| T06-CLOSEOUT.md | Final T06 (#85) technical closeout index (T06技术收口材料齐备): binds every accepted dimension to the traceability rows, the SHA-frozen umbrella evidence, and the owner boundary for deferred dimensions; issue #85 stays open |
| evidence/ | Per-row offline CLI probe captures (`<row-id>.probes.txt`): recorded `deltascope audit` output for the row's `sql_shape` at every declared version (`=== version <v> (target …)` sections). Regenerate by re-running the probe commands recorded in the file header |

## PostgreSQL ordinary-table owner targets

D16 binds exact role existence, special role syntax and observed owner sameness
to the original `OWNER TO` cell. The decision keeps permission and execution
premises separate. Existing classifications and denominators are preserved.

## PostgreSQL ordinary-table rename and schema destinations

D15 binds exact PG17 primary relation, row-type and namespace evidence to the
original `RENAME TO` and `SET SCHEMA` cells. Dependent object movement remains
independent. Row and subaction classifications and denominators are preserved.

## PostgreSQL ALTER COLUMN TYPE source checks

D14 binds exact PG17 source evidence to the original `ALTER COLUMN TYPE` and
`ALTER COLUMN TYPE USING` anchors. Destination types, dependency analysis,
expressions and execution remain independent. Row and subaction classifications
and denominators are preserved.

## PostgreSQL ordinary-table persistence transitions

D13 binds the exact PG17 ordinary-table persistence, publication and foreign-key
evidence to the original `SET LOGGED/UNLOGGED` anchor. Other relation kinds,
permissions, replication behavior and runtime lock outcomes remain separate.
Existing row and subaction classifications and denominators are preserved.

## PostgreSQL ordinary-table access method destinations

D12 binds exact PG17 method presence, kind and same-method evidence to the
original `SET ACCESS METHOD` anchor under `postgresql.alter-table.options`.
Partition-root, handler behavior, permission and locking remain separate.
The existing row and subaction classifications and denominators are preserved.

## PostgreSQL single-table tablespace destinations

D11 attaches exact PG17 destination and same-space evidence to the original
`SET TABLESPACE` cell under `postgresql.alter-table.options`. Bulk movement,
permissions, disk capacity and lock behavior stay independent. All existing
row and subaction classifications and denominators are preserved.

## PostgreSQL heap table storage options

D10 attaches the precise PG17 SET/RESET evidence only to the original
`SET/RESET storage parameters` anchor under `postgresql.alter-table.options`.
The bounded declaration result covers observed ordinary persistent non-typed
heap tables. TOAST, extensions and unproven inputs remain gaps. Existing row
and child classifications and denominators are preserved.

## PostgreSQL replica identity index attachment

D9 attaches exact T37 and corpus evidence to the original `REPLICA IDENTITY`
cell under `postgresql.alter-table.options`. The PG17 checks cover native
attachment prerequisites for the verified table scope. Runtime usability
remains separate, including assignment of an invalid index. The original
row and child classifications and denominators are preserved.

## PostgreSQL column identity and existence guards

D8 binds the original ADD COLUMN, ADD COLUMN IF NOT EXISTS, RENAME COLUMN and
multi-action cells to the exact T37 live, offline and parser-boundary cases.
The RENAME COLUMN child and its PG17 anchor advance from notice-only to mixed.
The parent and all 301 row IDs remain intact. The findings cover identity,
while TYPE/USING and broader table kinds, permission, dependency and ordering
premises remain open. See the
[decision](../../docs/decisions/2026-11-18-postgresql-alter-column-identity.md).

## PostgreSQL column storage/compression and system-target tranche

D7 extends exactly ten original cells in two already-mixed F09 parents:
columns DROP / DROP IF EXISTS CASCADE / multi-action, and column-attributes
SET/DROP NOT NULL, SET/DROP DEFAULT, SET GENERATED / DROP EXPRESSION,
ADD/DROP IDENTITY, SET STATISTICS, SET STORAGE / COMPRESSION and
SET n_distinct / RESET. No cell or parent status is promoted; all 301 rows,
original subaction sets and 19-family/63-row backlog denominators stay intact.
Matrix changes are generated from YAML, not handwritten.

The storage/compression cell binds actual typstorage, default/type/case,
lz4 enum, system-target and no-immediate-rewrite controls to the two new rule
selectors and exact T37 case IDs. Existing D5/D6 cells gain only relevant shared
system-target evidence, not data/expression/permission/dependency certification.
System negative-attnum members are independent of user known-empty; guarded
ctid is not absent/no-op. Prior-DDL/parser/sibling and broader-kind gaps remain
explicit. Role/schema ownership and exact native refusal/physical/data receipts
remain part of canonical evidence.

Initial T37 was 156/156, 2772 assertions at 631575a6 + dirty evidence.
Read-only review rejected 9c's unqualified instance sources; that all run was
interrupted, not passed. The same enum-reader ID now proves owned namesake/
operator controls after system-source qualification, bringing focused coverage
to **156/156, 2774 assertions**, still 52 native steps (25 success/27 refusal),
at 9c + dirty fixed source. The later d445 all passed 50/3488/61808 and validate
but was rejected for new column-catalog comparison shadowing. System-qualified
attribute filtering/type binding, permanent query tests and actual-provider
native red/fixed controls now preserve the same IDs/selectors; enum-reader adds
0:7 and p:x independent controls. Focused corrected coverage is 156/156, 2776
assertions at d445 + dirty source, still 52 native DDL steps. No old aggregate
substitutes for the corrected committed HEAD's full all/validate/read-only review.
See the [decision](../../docs/decisions/2026-11-18-postgresql-column-storage-compression-targets.md).
D7 shipped **b966f73c76352507a8dcf03b01063e0f43255564** after all **50 tasks,
3488/3488 cases, 61810 assertions** and validate; T37 is **156/156, 2776
assertions**, still 52 native steps (25 success/27 refusal). All per-task/CLI
builds use that complete SHA, with nonzero/no-failure/no-skip execution and owned
cleanup rc0/residual[]. Full candidate/merged-root gates and independent review
passed, then FF-only merge/normal push and exact-SHA Go/PG/lint/TLS/Pages CI
succeeded; links are in the decision. Original workspace, protected release
worktree and unrelated services were retained. No inventory status/denominator
is promoted by delivery, and this source evidence is not attributed to later
docs HEADs. Other original F09 dimensions remain open.

## PostgreSQL generation-declaration tranche

`postgresql.create-table`, `postgresql.create-table-generated-identity` and
`postgresql.alter-table.columns` bind exact `generation-declaration` /
`add-column-generation-declaration` cells to T36/T37 mode/conflict/gap controls
and their own PostgreSQL 17.7 catalog/refusal/no-audit-execution metadata cases.
Only these bounded new subactions are audited. Existing broad expression and
attribute cells retain their prior states. The generated/identity parent moves
`generic_notice` to `mixed_coverage`; all three parents remain mixed and no
whole row becomes semantically checked. New rule IDs are attached as literal
selectors in corpus evidence refs. `matrix.json` is regenerated from YAML.

## PostgreSQL ALTER constraint migration-risk tranche

Only `postgresql.alter-table.constraints` changes: ADD PRIMARY KEY and ADD
FOREIGN KEY become audited for the approved authored build/initial-validation
risk dimensions. ADD UNIQUE keeps its audited classification while its reuse
arm stops claiming an index build. Literal subaction selectors, exact T37 CLI
case selectors, PG17.7 owned native catalog/refusal cases and rule-specific
corpus selectors bind each cell. Historical probe captures remain historical;
the isolated new profiles supply the new positive and silent-mode evidence.
The parent remains mixed; no other F09 cell, parser boundary or whole-row
aggregate is promoted. Target legality, index eligibility and constraint-state
resolution remain open. `matrix.json` is generated, with all 301 row counts
unchanged.

## PostgreSQL constraint-target tranche

Only evidence on `postgresql.alter-table.constraints` changes. DROP CONSTRAINT,
DROP CONSTRAINT CASCADE, VALIDATE CONSTRAINT and ALTER CONSTRAINT DEFERRABLE
bind literal subaction selectors, exact new T37 native/CLI cases and matching
rule-specific corpus references. Native missing CASCADE, exact quoted/PK
identity, wrong-kind/missing target and no-op controls are distinct from
prior-DDL/parser gaps and 23514/23503 data refusals. Target proof does not
certify those broader boundaries. All row/cell statuses and all 301-row
aggregate counts are unchanged; the parent remains mixed. `matrix.json` is
regenerated from YAML, not edited by hand.

## PostgreSQL existing-column generation tranche

Only `postgresql.alter-table.column-attributes` gains D5 evidence, bound to its
six original subactions with literal selectors, exact T37 native/CLI cases and
rule-specific corpus refs. SET GENERATED / DROP EXPRESSION and RESTART move
from notice-only to audited for bounded target/guard and authored-risk checks.
SET/DROP NOT NULL, SET/DROP DEFAULT and identity options become **mixed**, not
fully audited: nullable-data, expression and unread current-value premises are
not certified. ADD/DROP IDENTITY keeps audited status with stronger precise
identity/guard/type/default/nullability proof. Historical probes are retained
as history, not substituted for new native evidence.

The parent remains mixed. No other row/subaction, authored subaction set,
original dimensional denominator or 301-row aggregate count changes. Inherited,
partition, unlogged, stale/parser, unmodeled and data/current-value gaps remain
explicit accepted-stage boundaries, not whole-F09 completion. Regenerate
`matrix.json` from YAML; never edit its generated contents.

## PostgreSQL column tuning and guarded-drop tranche

D6 binds seven original subactions in three already-mixed F09 rows. Columns:
DROP COLUMN / guarded CASCADE gain exact fresh PG17 target/guard/native proof;
multi-action becomes **mixed**, not claimed prospective-state support.
Column attributes: SET STATISTICS retains audited bounded declaration/target
proof with DEFAULT/-1/zero/max/native-clamp controls; n_distinct/RESET becomes
**mixed** because quoted/nonfinite/overflow/extension SET values remain gaps.
Unknown and duplicate RESET are native successes, not invalid SET declarations.
Go-rounded-to-zero/subnormal and minimum-normal-boundary Float literals remain
unmodeled PG strtod compatibility gaps, never fabricated zero facts; true zero
and ordinary small normal values remain separate positive controls.
Two options cells bind corrected fillfactor/same-AM/partition advice and file
identity controls only; they do not gain parameter/method eligibility support.

T37 keeps all old 118 objects/profiles/IDs and adds the 18 frozen cases (136,
2186 assertions); 59 independent native actions, attribute/file/data/guard
receipts and owned cleanup distinguish refusals, no-ops and execution. Catalog
NULL represents PG17 default statistics, including authored -1. Earlier DROP
revokes global old directories: fresh controls precede it and later gaps stay
explicit. No sampling/data/expression/permission analysis or sequential engine
is added. All original subaction sets, 301 row statuses and original program
denominators remain unchanged; generate `matrix.json` from YAML. This is not
whole-row/F09 completion. The bounded code slice shipped as fa37a41b after
50 exact-HEAD tasks/3468 cases/61220 assertions, validate and accurate-SHA
Go/PG/lint/TLS/Pages success; the decision record preserves that historical
code attribution separately from later documentation HEADs.

## Row contract

Every row carries: `id` (stable, unique), `product` (`mysql`/`tidb`),
`versions`, `sources`, `family`, `subactions`, `sql_shape`, `prerequisites`,
`status` (derived aggregate — never authored independently),
`parser_status`, `audit_status`, `applicability`, optional
`subaction_status`, `status_evidence`, `targets`, `acceptance`
(`dimensions`, `refs`), `owner`.

The v2 status model separates three axes:

- `parser_status`: `recognized` | `partial` | `unrecognized`
- `audit_status`: `audited` | `notice_only` | `explicit_unsupported` |
  `silent` | `not_applicable` | `mixed`
- `applicability`: one cell per anchor carrying `anchor`,
  `version_range`, `official_presence` (`documented` | `not_documented` |
  `not_assessed`), `product_scope` (`registered` | `version_gated` |
  `vendor_boundary` | `not_applicable` | `unknown`), cell-level
  `parser_status`/`audit_status`, and `evidence` refs — evidence is
  required on every version cell.

`subaction_status` records exist where subactions differ meaningfully;
each carries a stable `subaction_id`, its own parser/audit status, and
complete applicability cells with evidence for every declared version.

`status` is derived from the row's applicability cells by frozen rules
(enforced by the gate): all registered applicable cells audited →
`semantically_checked`; all notice-only → `generic_notice`; all
explicit-unsupported → `explicit_unsupported`; all silent →
`parse_only`; all relevant documented forms parser-unrecognized →
`parser_unsupported`; all relevant product cells vendor-boundary →
`vendor_not_supported`; mixed registered audit states → `mixed_coverage`.
Out-of-domain `version_gated`/`vendor_boundary`/`not_applicable` cells do
not downgrade a completely audited registered domain, and
parser-unrecognized documented forms take precedence over mixed
registered states.

`required_row_ids` lists every row ID and must equal the row set exactly —
deleting a row without updating the denominator fails the gate. The checked-in
`required_rows.txt` baseline holds the same set outside the editable YAML, and
the gate additionally locks per-product minimums (mysql ≥ 63, tidb ≥ 50) in the
test itself — shrinking any pair of files leaves the third behind and fails.

`owner` must be a declared milestone task with a real GitHub issue number
(`owners` map). A gap with no owning task means a new child issue under #79
must be created first — a proposal without an issue is not an owner.

Statuses (mutually exclusive, never substitutable):

- `semantically_checked` — feature-specific check exists with concrete
  evidence (`file:`/`gate:` acceptance ref required)
- `generic_notice` — parses but only generic/`*.notice` findings fire
- `parse_only` — parses silently, zero findings
- `parser_unsupported` — parser_error on a vendor-valid form
- `vendor_not_supported` — product boundary, not a to-do item
- `explicit_unsupported` — statement recognized but the product emits
  explicit unsupported/bounded evidence (not audited, not silent)
- `mixed_coverage` — registered applicable cells carry different audit
  states (per-subaction split)

`acceptance.refs` entries are `file:<repo-path>` (must exist),
`gate:<make-target>`, or `missing:<description>` for honest gaps.

Cell `evidence` follows a stricter contract (R4): every cell must carry
at least one concrete `file:`/`gate:` ref — `missing:` entries are notes,
never evidence. `file:` refs must exist. Probe files under `evidence/`
must be row-scoped (basename `<row-id>.probes.txt`) and must contain a
probe section for the cell's own anchor version, so evidence copied from
the wrong row or the wrong version fails the gate. `gate:` refs must
resolve to a committed golden manifest (`gate:T<NN>` →
`testdata/ddl-golden/T<NN>.json`). The aggregate derivation runs over
the union of row and subaction cells, so a `semantically_checked` parent
cannot mask a silent or explicit_unsupported registered subaction.

R6 — subaction-bound evidence: cells inside a `subaction_status` record
must additionally carry at least one ref bound to that exact subaction,
so one subaction cannot borrow another's evidence and row-level probes
cannot mask missing subaction evidence. Bound forms: an `evidence/`
probe file named `<row-id>.<subaction-slug>.probes.txt` whose header
declares `row_id: <row-id>` and `subaction_id: <exact-subaction-id>`
plus per-version `=== version <v> ` sections, or a
`file:<path>#<subaction-id>` selector whose literal is present in the
referenced file. A subaction cell citing a row-level probe or a probe
declaring a different `subaction_id` fails the gate; a row-level cell
citing a subaction-bound probe likewise fails.

Rows owned by future tasks may record
incomplete implementation — they are assigned, not counted as coverage.

## Gate

`make ddl-inventory-gate` runs `TestDDLInventoryContract` (schema,
vocabularies, per-cell evidence, version/subaction coverage, derived
aggregate status), the denominator-lock negative
`TestDDLInventoryDenominatorLocked`, and the derivation-rules unit test
`TestDDLInventoryAggregateDerivation`
(`internal/application/audit/ddl_inventory_contract_test.go`).

Verified sources: MySQL 5.7 (frozen Oracle mirror — the dev.mysql.com 5.7
URL redirects to 9.7), MySQL 8.0, MySQL 8.4, TiDB release-8.5 docs.
Schema version 2 (parser/audit/applicability/subaction separation),
verification date: 2026-11-13.
