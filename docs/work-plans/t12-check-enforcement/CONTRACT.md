# T12 CHECK constraint and enforcement-state analysis — implementation contract

Status: FROZEN for T12 implementation. `testdata/ddl-golden/T12.json` is the
materialized golden manifest. Internal freeze is authorized by the task order;
it is not external acceptance. Same-contract implementation/test/collector
bugs may be repaired without reducing the frozen responsibilities or
denominator.
Owner: #91 under #79. Task base: T11 accepted head
`186710db69088d59eae82726bbc6ad4789f8a353` on `workfree` (T11 product
`27edfea2f41ed699e32cf47d5c76db5cad36f897`).

## 1. Scope and semantic seam

Give CHECK constraints a source-faithful lifecycle: declaration (CREATE and
ALTER ADD), removal (DROP CHECK / DROP CONSTRAINT), enforcement flips
(ALTER CHECK|CONSTRAINT [NOT] ENFORCED), and the product/version/runtime
boundaries where the same syntax is accepted but never executes. The shared
seam already exists: `spec.Constraint` carries a normalized `Type: "check"`
member produced by CREATE table constraints and by `alter.Constraint` on
ALTER `add_constraint` actions; `alter_check`/`drop_check` are already
normalized actions. What is missing today (real gaps this contract closes):

- `spec.Constraint` carries no enforcement fact and no expression facts:
  `ALTER CHECK x ENFORCED` and `ALTER CHECK x NOT ENFORCED` normalize to the
  same payload (`action=alter_check`, `Type=constraint_0`, no direction —
  verified by extraction probe).
- `ConstraintsUnknown` is set whenever a real CHECK exists in the catalog —
  the provider treats every CHECK as unmodeled.
- `alter_check` has no ordered-state transition and no audited coverage:
  the statement is recognized but silently unjudged.
- `add_constraint` CHECK hits `s.invalidateAll` — an unmodeled tombstone for
  a decidable transition.
- Column-level `CHECK(...)` options degrade to `UnextractedOptions`.
- Naming rules judge only `DDL.Constraints`; ALTER `add_constraint` CHECK
  names are unjudged (`appliesToCreateTableOrAlterCheckConstraint` only
  recognizes `alter.Options["constraint_type"]=="check"`, which the MySQL-
  family extractor never sets).

Out of scope: solving or evaluating arbitrary CHECK expressions, emulating
the database, partitioning, DML, and any contract change to T11's
PK/UNIQUE/FK semantics. Inline column `REFERENCES` stays parse-and-ignore.

### 1.1 Native facts (probed; four anchors)

MySQL 8.0.46 / 8.4.10 (identical on every probed cell):

- `CREATE|ADD ... CHECK` lands and enforces by default; violating INSERT →
  ERROR 3819. `CHECK ... NOT ENFORCED` lands stored with `ENFORCED=NO`.
- `ALTER CHECK|CONSTRAINT x NOT ENFORCED` → rc=0, idempotent,
  `TABLE_CONSTRAINTS.ENFORCED=NO`, violating rows then land.
- `ALTER CHECK|CONSTRAINT x ENFORCED` → rc=0 on clean data (ENFORCED=YES);
  **refuses 3819 when violating rows exist** — a data-conditional flip.
- `ALTER CONSTRAINT` is a working alias on MySQL 8.x (same AST node, same
  rc=0 behavior — both keywords observed).
- `ALTER CHECK missing|non-check-name` → ERROR 3821.
- `DROP CHECK x` and `DROP CONSTRAINT x` on a **present** CHECK both remove
  it (rc=0) — the drop is deterministic once the member is proven, unlike
  the FK/UNIQUE/PK divergent arms T11 models.
- `ADD CONSTRAINT` with a duplicate check name → ERROR 3822.
- Bare `CHECK(...)` auto-names `t_chk_N` (observed `t_chk_1`).
- `DROP COLUMN` on a column a CHECK references **silently drops that check**
  (rc=0, member gone from catalog) — no refusal.
- `RENAME COLUMN` and renaming `CHANGE COLUMN` on a column a CHECK
  references **refuse** ERROR 3959 ("Check constraint 'x' uses column
  'y', hence column cannot be dropped or renamed") — provably unchanged;
  a same-name CHANGE retype succeeds (rc=0, check stays bound).
- Catalog: enforcement state lives in
  `INFORMATION_SCHEMA.TABLE_CONSTRAINTS.ENFORCED` (YES/NO);
  `CHECK_CONSTRAINTS.CHECK_CLAUSE` holds the expression text (loaded for
  column-reference extraction only — never propagated raw).

MySQL 5.7.44:

- `CREATE|ADD ... CHECK` parses rc=0 but is **silently dropped**: SHOW
  CREATE shows no constraint, violating INSERT lands — accepted-but-ignored,
  never enforced, never stored.
- `ALTER CHECK`, `DROP CHECK`, `DROP CONSTRAINT` → ERROR 1064 syntax
  refusal.
- Version boundary (official docs, not probeable below the anchors): CHECK
  enforcement landed in 8.0.16, `ALTER|DROP CHECK` syntax in 8.0.19 — both
  anchors are ≥8.0.46; sub-anchor versions inherit the documented
  boundary only.

TiDB v8.5.0 (`tidb_enable_check_constraint`):

- OFF: every CHECK form parses rc=0 but nothing is stored — CREATE/ADD
  silently drops, ALTER/DROP no-op rc=0, violating INSERT lands.
- ON: CHECKs store and enforce (INSERT → 3819); ALTER CHECK works both
  directions rc=0; `ALTER CONSTRAINT` alias works; missing/non-check names
  → ERROR 3940; `DROP CHECK`/`DROP CONSTRAINT` on a present check both
  remove it; `DROP COLUMN` silently drops referencing checks.
- Same-name binding cannot collide: TiDB's check name space is separate —
  `ALTER CHECK uk_w ...` where `uk_w` is a unique key → 3940.
- `RENAME COLUMN` on a referenced column → 3959 refusal (same as MySQL);
  `CHANGE COLUMN` on a referenced column → rc=0, the rename lands and the
  referencing check is **silently dropped** (SHOW CREATE shows the
  renamed column, no constraint) — a real TiDB/MySQL divergence.
- Catalog: members live only in `TIDB_CHECK_CONSTRAINTS` (carries
  TABLE_NAME + CHECK_CLAUSE; verified on the anchor). `TABLE_CONSTRAINTS`
  stores **no** CHECK rows on TiDB, and no catalog column exposes the
  enforcement bit — the `/*!80016 NOT ENFORCED */` marker inside SHOW
  CREATE output is the only surface, so provider-loaded TiDB members keep
  `Enforced` unvouched.

### 1.2 Normalized fact contract

`spec.Constraint` gains:

- `Enforced string` — `"enforced"` | `"not_enforced"` | `""` (unknown or
  unproven). Parser: `ast.Constraint.Enforced` bool (grammar default true —
  undeclared == ENFORCED keyword at AST level, same native outcome) maps to
  `"enforced"`/`"not_enforced"`. Provider: `TABLE_CONSTRAINTS.ENFORCED`
  YES→`"enforced"`, NO→`"not_enforced"`.
- `Expression spec.ExpressionFacts` — the existing evaluation-free fact
  record (References, Functions, Features, TopForm, Fingerprint, Known).
  `Expression.References` supplies dependent-column facts as bounded
  `ColumnReference`s. Raw expression text is never stored, never emitted.
- `InColumn bool` — distinguishes a column-option CHECK from a
  table-constraint CHECK (both land as catalog CHECK members).

`spec.Alter.Options["check_enforced"]` = `"true"` | `"false"` carries the
ALTER CHECK direction (`ast.AlterTableAlterCheck` payload `Constraint.Enforced`).

`spec.InstanceFacts` gains `TiDBEnableCheckConstraint bool` +
`TiDBEnableCheckConstraintKnown bool` (mirrors `RestrictNonStandardKey`).

Provider (`internal/infrastructure/metadata/mysql`):

- `loadCheckConstraints` appends `Type: "check"` members to
  `snapshot.Constraints` along two probed catalog shapes:
  - MySQL shape: `TABLE_CONSTRAINTS` (name, `ENFORCED`) joined to
    `CHECK_CONSTRAINTS` (`CHECK_CLAUSE` → `ParseCatalogExpressionFacts`;
    the clause text itself is never retained) on catalog+schema+name —
    `CHECK_CONSTRAINTS` carries no TABLE_NAME column on MySQL.
  - TiDB shape: `TIDB_CHECK_CONSTRAINTS` (name, TABLE_NAME,
    `CHECK_CLAUSE`) — no enforcement column exists, so TiDB members stay
    `Enforced`-unvouched.
  Clause parse failure → member keeps `Expression.Known=false`, not a
  dropped member. The unmodeled-type counter excludes `'CHECK'` once
  modeled.
- On TiDB flag OFF the catalog genuinely stores no CHECKs — an empty set is
  a proven empty set.
- `LoadInstanceFacts` adds `tidb_enable_check_constraint` to the variable
  read (TiDB-only row; absent on MySQL → unknown, never guessed).

### 1.3 Rule contract (new IDs frozen here before implementation)

| Rule ID | Default level | Fires when |
|---|---|---|
| `ddl.constraint.check.not_enforced.notice` | notice | A CHECK is declared `NOT ENFORCED` (CREATE/ADD) or flipped `NOT ENFORCED` (ALTER CHECK|CONSTRAINT) on an arm where the state is real (MySQL ≥8, TiDB flag-ON). Never fires on the ENFORCED direction, ignored arms, or ordinary column changes; a provably missing/non-check name means the flip provably refused, so no enforcement change is claimed. Metadata: `table`, `constraint`, `enforced`=`not_enforced`. |
| `ddl.alter.alter_check.exists.require` | blocker | `ALTER CHECK|CONSTRAINT x` where x binds no proven CHECK member: missing name (3821/3940 class) or a proven non-check name — refused-statement finding. Unproven member/name/flag → gap, never a fabricated finding. |
| `ddl.alter.drop_check.exists.require` | blocker | `DROP CHECK|CONSTRAINT x` where x binds no removable member in any vouched collection (check set, FK set, index set, PK object) — refused under both keywords. Unvouched collections → gap. |
| `ddl.alter.alter_check.version.require` | blocker | `ALTER CHECK|CONSTRAINT` or `DROP CHECK|CONSTRAINT` on a MySQL arm below the 8.0.19 syntax boundary (5.7 anchor: 1064 refusal) — refused-syntax finding. Unobserved version → gap, never a fabricated refusal. TiDB arms answer through the flag instead, so this rule is MySQL-only. |
| `ddl.constraint.check.ignored.notice` | notice | A CHECK declaration targets an arm that provably parses-but-ignores it: MySQL <8.0.16-boundary (5.7 anchor), TiDB flag observed OFF. Unobserved flag/version → gap on the state consumers, not this finding. |

Existing rules reused unchanged in semantics: `ddl.constraint.check.name.*`
(prefix/suffix/contains — now judging ALTER `add_constraint` CHECK names via
`alter.Constraint.Type=="check"` as well as `alter.Options`), plus the
`ddl.alter.add_constraint.notice` generic.

### 1.4 Ordered-state contract

- `applyAddCheck`: declared CHECK lands per dialect/facts —
  MySQL ≥8 boundary → member (authored name or `NameUnknown` auto-name,
  declared enforced state, Columns from expression local references);
  MySQL <boundary → ignored, member never lands; TiDB flag ON → lands,
  OFF → ignored, unobserved → undecidable (`ConstraintsUnknown`). Duplicate
  name vs proven member → refused (3822), no post-state.
- `applyAlterCheck` (new): sole `alter_check` action, proven check member —
  NOT ENFORCED → deterministic post-state `enforced=not_enforced` (data-
  independent, idempotent); ENFORCED → data-conditional (3819 possible) —
  member survives but its `Enforced` unvouches. Missing/non-check name →
  refused, state keeps. MySQL <8.0.19-boundary / 5.7 → 1064, provably
  unchanged. TiDB ON → same semantics (3940 class); OFF → no-op;
  unobserved flag → member `Enforced` unvouched.
- `applyDropCheck`: adds the CHECK-member arm — a proven CHECK member drops
  deterministically under **both** keywords on MySQL 8.x and TiDB flag-ON
  (both probed on the still-present member): clean member removal, set
  stays vouched. TiDB flag-OFF/unobserved → `markConstraintsUnknown`
  (bounded). All T11 FK/UNIQUE/PK/secondary arms unchanged.
- `applyDropColumn`: a dropped column silently removes CHECK members whose
  `Columns` contain it (probed both anchors) — those members are filtered
  from the vouched set; unrelated members keep.
- `checkColumnIdentityOutcome` (RENAME/CHANGE COLUMN): a CHECK member
  proven to reference the renamed column makes the statement **refuse**
  natively — `checkIdentityRefused` publishes the exact pre-state, not a
  tombstone. Probed arms: MySQL 8.0.46/8.4.10 → ERROR 3959 for both
  RENAME COLUMN and renaming CHANGE COLUMN; TiDB flag-ON → 3959 for
  RENAME COLUMN but CHANGE COLUMN lands the rename while **silently
  dropping** the referencing member (`checkIdentityCascade`). Same-name
  retype rebinds nothing (proceeds). Unvouched members or undecidable
  arms (TiDB flag unobserved/contradictory) → unsafe, publication stays
  withheld. Check references are never rewritten by the rename path.
- `derivedCreateShape`: CHECK members land only on enforcing arms (MySQL
  ≥boundary, TiDB flag-ON); ignored arms publish no member.
- Column-level CHECK declarations ride the same landing path via
  `InColumn` members.

### 1.5 Coverage / evidence-gap contract

- `alter_check` enters the audited-action sets (MySQL and TiDB) — the
  silent path closes.
- Unproven name/member/version/flag → attributable
  `target_table.constraints` (or member-scoped) evidence gap,
  `CoverageUnverified`, `VerdictReview` — never a fabricated pass/fail.
- Incomplete earlier statement must not mask a later statement's gap
  (predecessor-coverage independence stays per T11).

### 1.6 No-leak contract

`CHECK_CLAUSE` and expression literals never appear in findings, gaps,
diagnostics, logs, or serialized metadata. Only `ExpressionFacts`
(references, function names, feature markers, top-form, fingerprint) and
`Enforced`/`Name`/`Columns` may surface. `Expression` serializes in
`spec.Constraint` for rule consumption; the raw clause is dropped at the
provider boundary.

## 2. Golden Path (first slice)

- Fixture: real `t(id INT, CONSTRAINT chk_id CHECK (id > 0))` on MySQL
  8.0.46 / 8.4.10; version + `TABLE_CONSTRAINTS.ENFORCED=YES` proven.
- Policy: `ddl.constraint.check.not_enforced.notice` isolated at blocker.
- Input: `ALTER TABLE t ALTER CHECK chk_id NOT ENFORCED;`
- Oracle: one finding — `ddl.constraint.check.not_enforced.notice`,
  blocker, `table=t`, `constraint=chk_id`, `enforced=not_enforced`, source
  location bound, coverage complete, verdict reject, exit 1. Catalog
  confirms `ENFORCED=NO`.
- Counter-cases in the same manifest: `ENFORCED` direction → 0 findings;
  ordinary `ADD COLUMN` → 0 findings; MySQL 5.7 declaration → ignored-
  notice reveal, never an enforcement claim; TiDB flag arms → respective
  real semantics.

## 3. Manifest cells (T12.json)

- `ddl_steps`: fixture DDL per anchor (5.7 fixture has no stored check —
  lands nothing).
- `cli_cases` (offline, no-DB): not-enforced flip finding; enforced
  direction clean; column change clean; add-declared `NOT ENFORCED`;
  naming across CREATE multi-check and ALTER add; drop/alter without
  provider → gap arms; TiDB flag-unobserved arm.
- `metadata_cases` (real anchors): mysql80/mysql84 golden paths + catalog
  verify; mysql57 ignored reveal; tidb85 flag-ON golden path; tidb85
  flag-OFF ignored reveal; `ALTER CHECK missing` exists finding;
  `DROP CHECK` on present member clean removal.
- `syntax_negative`: malformed CHECK syntax cells per anchor.
- Denominator: frozen at 25 cases / 280 assertions on the first complete
  collection (8 anchor db cells, 9 offline CLI cells, 8 metadata cells);
  the runner-broken iterations before the TiDB catalog shape was probed
  are recorded in the evidence bundle, not counted as contract changes.

## 3a. Prior-oracle updates justified by T12 support

Authorized oracle changes (old value → new value → reason); all other
prior expectations stay frozen.

| Cell | Old oracle | New oracle | Reason |
|------|-----------|------------|--------|
| `T11.meta.t11-mysql80-dropfk-check-bearing` (`ALTER TABLE t11ck DROP FOREIGN KEY ghost_fk` against a table carrying `CONSTRAINT ck_v CHECK (v > 0)`) | exit 0 / verdict review / 0 findings / coverage `unverified` / one `unknown_table_state` gap requiring `target_table.constraints` | exit 1 / verdict reject / 1 finding (`ddl.alter.drop_foreign_key.exists.require` blocker, `exists=false`, `name=ghost_fk`, `table=t11ck`) / coverage `complete` / 0 gaps | T11 could not represent CHECK members, so a check-bearing constraint collection was unvouched and the FK exists rule fell back to a bounded gap. T12 loads CHECK members from `TABLE_CONSTRAINTS`/`TIDB_CHECK_CONSTRAINTS`, making the same collection fully vouched: `ghost_fk` is a proven-missing FK name — the native-refusal blocker, not a guess. |
| T03-era `unsupported` cells (`coverage_t03_test.go`, `corpus_testdata_test.go`, `alter_table_constraints.expected.yaml`, `index_global_and_check` pairs) | `alter_table.add_constraint.check`, `alter_table.drop_check`, `create_table.column.check` recorded bounded `unsupported` evidence with coverage `incomplete` | declarations/actions audited; unsupported evidence removed (in-column CHECK becomes a modeled member, ADD/DROP/ALTER CHECK audited by the T12 rules) | T12 promotes these forms to audited status (§1.5); `create_table.column.check` is now extracted as an `InColumn` member. |
| `T12.cli.t12-cli-create-notenforced-offline-gap` and `T12.cli.t12-cli-add-check-declared-offline-gap` (explicit `NOT ENFORCED` on an offline/unobserved-version MySQL target) | `evidence_gaps` 2 — `check.not_enforced.notice` + `check.ignored.notice` `missing_target_version` | `evidence_gaps` 3 — plus `ddl.alter.alter_check.version.require` `missing_target_version` on the same statement | R1 made the authored enforcement clause a first-class fact: on an unobserved version the clause can be a 5.7-style 1064 refusal or a stored declaration, so the version gate honestly requires `target.version`. The older oracle predates clause awareness; the added gap is stricter evidence, not a downgraded finding. |

## 4. Boundaries not promoted

- `ALTER CHECK ... ENFORCED` post-state is data-conditional — the model
  unvouches the member's `Enforced` rather than claiming either value.
- `RENAME COLUMN` on a check-referenced column is probed refused (3959)
  on MySQL 8.0.46/8.4.10 and TiDB flag-ON; TiDB flag-ON CHANGE COLUMN
  silently drops the referencing member instead. The TiDB flag-OFF arm
  cannot hold members, and TiDB flag-unobserved stays undecidable —
  unprobed refusal-vs-drop arms never publish a claimed outcome.
- TiDB flag toggled between provider load and audit: bounded by the
  observed-fact mechanism (unobserved → undecidable).
- MySQL <8.0.46 sub-versions: documented 8.0.16/8.0.19 boundary only —
  anchors prove ≥8.0.46 behavior, not every patch.
- `MATCH`/expression evaluation: stays out of scope.

## 5. R1 remediation (reviewer groups A–E)

Independent-acceptance round 1 closed five product defects located by the
reviewer. Every behavior below is probed on native anchors before it was
encoded; probe logs live in `probes/r1/` and the public-path red baseline in
`docs/verification/t12r1/red/baseline_red.log` (13 `--- FAIL` markers on the
pre-remediation product, all green after).

### 5.1 Spelling is a first-class fact (A)

`ast.Constraint` normalizes the authored keyword away, so the extractor
recovers it from the already-split clause text with a comment/quote-aware
scanner (`clauseCheckFacts`): `alter.Options["check_spelling"]` is
`spec.CheckSpellingCheck` or `spec.CheckSpellingConstraint`; an
unrecoverable spelling stays absent and downgrades every spelling-gated
answer to unproven rather than guessing. `Constraint.EnforcedExplicit`
records only an authored `[NOT] ENFORCED` clause — never inferred from a
default `Enforced` value — for CREATE elements and ALTER payloads alike.

### 5.2 Native facts added by R1 probes

| Anchor | Probed fact |
|--------|-------------|
| MySQL 8.0.17 (new in-band anchor, linux/amd64) | `ALTER CHECK` / `DROP CHECK` execute; `ALTER CONSTRAINT` / `DROP CONSTRAINT` refuse 1064 — the generic alias floor is 8.0.19, not 8.0.16. |
| MySQL 5.7.44 | Bare CHECK declarations parse and are silently dropped; any authored `ENFORCED`/`NOT ENFORCED` clause refuses 1064 (table-level, `CONSTRAINT`-named, ADD CONSTRAINT, and inline column forms); every ALTER/DROP CHECK spelling refuses 1064. |
| MySQL 8.0.46/8.4.10 | Secondary indexes are outside the constraint namespace: `DROP CHECK ix` refuses 3821, `DROP CONSTRAINT ix` 3940. Generic `DROP CONSTRAINT` deterministically removes a uniquely-bound check/unique/PK/FK member. A cross-type same-name pair under the generic keyword refuses 3939; under `CHECK` spelling it binds only the check member. Generic `ALTER CONSTRAINT` on a non-check member refuses 3941. Check and FK names share one namespace (1826 on collision). |
| TiDB v8.5.0 flag ON | The generic keyword binds the CHECK member first — no 3939-equivalent exists. An in-column duplicate CHECK name is silently ignored while the column lands (vs MySQL's atomic 3822). Generic DROP resolution differs from MySQL and is modeled only where probed. |
| MySQL 8.0.46/8.4.10 | `DROP COLUMN` silently cascades only a CHECK whose proven reference set is exactly the dropped column; multi-column references refuse 3959. `ADD CHECK` on a missing referenced column refuses 1054 atomically. A duplicate in-column CHECK name refuses 3822 atomically — the new column never lands. Multi-action statements are atomic: any refusing action voids the whole statement. |

### 5.3 Shared target classifier (B)

`spec.CheckDropTarget` / `spec.CheckAlterTarget`
(`internal/domain/spec/check_target.go`) is the single binding classifier
consumed by rule findings, evidence gaps, and ordered state — the three can
no longer diverge. Outcomes: `CheckTargetProven`,
`CheckTargetMissing`, `CheckTargetWrongKind`,
`CheckTargetAmbiguous`, `CheckTargetUnproven`. Rules:

- `CHECK` spelling consults only the check namespace.
- Generic `CONSTRAINT` spelling counts check+unique+PK+FK members; exactly
  one binds, more than one of different kinds is the probed 3939 ambiguity.
- Secondary indexes never count, but an index member of unvouched kind may
  conceal a unique — generic binding then answers Unproven, not Missing.
- An unvouched namespace makes binding Unproven.
- MySQL-only ambiguity: TiDB's probed generic binding is preserved per
  dialect inside the classifier.

### 5.4 Ordered-state corrections (A, C, D)

- `applyAlterCheck`/`applyDropCheck` gate on the authored spelling through
  `spec.CheckAlterSyntaxOutcome` (CHECK ≥8.0.16, generic ≥8.0.19) and bind
  through the shared classifier; proven refusals publish unchanged,
  deterministic removals drop only the bound member (subject to the FK
  sole-coverer dependency premise refined in §5.7 — a successful binding
  does not by itself prove the drop lands), unproven arms unvouch
  the consulted namespaces.
- `applyCreateTable`, `inColumnChecksOutcome`: a MySQL ignored-arm member
  carrying an authored enforcement clause is a proven 1064 refusal —
  `spec.CheckExplicitClauseRefused` — so the table/column never lands
  (TiDB's ignored arm accepts and ignores, so it is not refused there).
- `dropColumnCascadedChecks` removes only members whose modeled reference
  set is exactly `{dropped column}`; `dropColumnLocalVerdict` turns a
  multi-column or unvouched reference into the proven-refusal/unsafe arms.
- `applyAddCheck` refuses on a proven-missing referenced column before
  publication; `inColumnChecksOutcome` decides land/refused/undecidable
  atomically with the column (TiDB-ON's probed silent-ignore lands the
  column while skipping the colliding member).
- Multi-action statements carrying `alter.Constraints` are unprovable and
  invalidate rather than publishing a partial successor.

### 5.5 Rule-surface corrections (A, B, E)

- `ddl.alter.alter_check.version.require` / `drop_check` version findings
  split the floor by authored spelling; unproven spelling or version yields
  evidence gaps, and a disabled version policy can no longer let the state
  layer publish a refused post-state.
- `ddl.alter.drop_check.exists.require` and `alter_check.exists.require`
  consume the shared classifier — secondary-index names are proven
  refusals (3821/3940), cross-type same-name generic targets are the
  proven 3939 ambiguity, and missing names stay blockers.
- `ddl.constraint.check.ignored.notice` suppresses members carrying an
  authored enforcement clause only where the clause is a proven refusal —
  MySQL's ignored arm (1064, handled by the version rule). TiDB's flag-OFF
  arm accepts and ignores authored clauses, so its ignored notice stands
  (§5.7).
- `ddl.constraint.check.name.*` governance now reads `alter.Constraints`
  (inline ALTER CHECK members) in both applicability and name selection,
  deduplicating names already carried on `ddl.Constraints`/`alter.Constraint`.

### 5.6 Boundaries kept unproven

- Unrecoverable spelling inside the 8.0.16–8.0.18 band → Unproven.
- Index members of unknown kind under generic binding → Unproven.
- TiDB generic-drop resolution beyond the probed cells → Unproven.
- MySQL patches other than the probed anchors (5.7.44, 8.0.17, 8.0.46,
  8.4.10) are not individually vouched.

### 5.7 T12-R2 corrections (reviewer groups A–D)

**A — attribution, not keyword presence.** The extractor proves an
authored `[NOT] ENFORCED` clause only in binding position — the first bare
word after a `CHECK(...)` close paren inside the element/clause text that
was already correlated to its AST spec. `ENFORCED` as a column or
expression identifier never sets `EnforcedExplicit` (probed: 5.7 accepts
`CHECK(enforced > 0)` as ignored, refuses only the authored clause 1064).
Element and clause splitting skip comments and quoted text, so `/* , */`
and `/* ( */` can no longer fake boundaries or bodies. When the AST count
and the clause text cannot be aligned the statement marks the affected
members `Constraint.EnforcedUnproven` instead of claiming a proven
omission — the version rule emits an evidence gap and no copy of a
sibling action's spelling or clause fact is reused.

**A4 — TiDB OFF keeps the ignored notice.** Suppression of
`ddl.constraint.check.ignored.notice` on authored enforcement clauses is
gated through `spec.CheckExplicitClauseRefused`, which is MySQL-only;
TiDB's flag-OFF arm accepts and ignores authored clauses (probed), so the
ignored finding/review verdict is restored there.

**B — binding success does not imply landing.** Generic `DROP
CONSTRAINT` bound to the primary object or a unique member now runs the
same FK sole-coverer dependency premise the dedicated drops use
(probed 1553 on 8.0.46 for `DROP CONSTRAINT \`primary\``, `DROP PRIMARY
KEY`, and a sole-covering `DROP CONSTRAINT ux_p`; a replacement coverer
drops clean). A same-name pair of check member plus unknown-kind index
member keeps the generic binding unproven AND unvouches the check
member's existence afterward — either world (secondary → check removed,
unique → 3939 refusal) is native, so coverage reports the gap rather than
a complete/pass. A name that could bind the primary object honors a
standalone `PrimaryKeyUnknown` fact as Unproven instead of Missing.

**C — column-scoped CHECKs.** TiDB never stores an in-column CHECK
member (probed flag-ON: fresh name lands the column but no member —
`DROP CHECK` then answers the proven 3940 absence; the R1
duplicate-name silent-ignore is the same storage rule, not a
name-collision special case), so `applyInColumnChecks` skips member
publication for TiDB entirely. MySQL in-column CHECKs may reference only
their own column — a cross-column reference is the probed 3813
whole-statement refusal, and `NOT ENFORCED` does not relax it.

**D — multi-action identity.** `prepareMultiAction` runs a
statement-level refusal scan before any action publishes: a
CHECK-referenced RENAME/CHANGE evaluated through
`checkColumnIdentityOutcome` against the pre-state (probed 3959 atomic on
8.0.46 — sibling ADD never lands), or a proven syntax refusal on any
action, preserves the exact pre-state. TiDB flag-ON `CHANGE COLUMN`
cascades every provably-referencing CHECK member — the single-column
limit applies to DROP COLUMN only; the multi-column cascade is now
probed, not extrapolated.
