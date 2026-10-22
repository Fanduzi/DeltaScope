# T08 column governance — implementation contract

Status: FROZEN for T08 implementation. The frozen role matrix is in
`roles.json`; `testdata/ddl-golden/T08.json` is its materialized manifest.
Internal read-only design review preceded this freeze; it is not external
acceptance. Same-contract implementation/test/collector bugs may be repaired
without reducing the frozen responsibilities or denominator.
Owner: #87 under #79. Task base: 45463ffbb31f649aa33af0cacb7a556b2959599f
(T07-R2 evidence head); code parent 9538c05905d2cf78895bcdcc8346b251453c3151.
T07 closeout: `docs/work-plans/t07-advanced-columns/CLOSEOUT.md`.
Sources: [SOURCES.md](SOURCES.md). The user authorizes internal freezing without
another approval round, not changes to the parent result/status contract.

## 1. Scope and semantic seam

Implement one shared application/SDK path for ordinary column governance:
ADD/DROP/MODIFY/CHANGE/RENAME column actions, the MySQL/TiDB
`ALTER [COLUMN] c SET|DROP DEFAULT` setters, explicit-attribute-change policy,
metadata-aware existence/compatibility checks, declared column position
(`FIRST`/`AFTER`), bounded multi-action ALTER TABLE, and the ordered-state
successors of all of the above. Existing parser/spec/provider, registry,
EvidenceGap, coverage and ordered-state seams remain; do not duplicate
semantics in transports, execute submitted SQL, predict permissions or
rollback, or undertake T09+ responsibilities (TTL, generated-column governance
beyond T07's frozen facts, general index/foreign-key/CHECK rewriting).

The five #87 acceptances map to contract sections:

- A1 — the ALTER COLUMN SET/DROP DEFAULT silent path and consistent
  explicit-attribute policy semantics with MODIFY/CHANGE: §5.
- A2 — narrowing, type-family, signed/unsigned, NULL, auto_increment,
  position, multi-column and mixed actions; missing prior-value evidence is
  explicitly unverified: §5/§6/§8 roles.
- A3 — metadata-aware existence/compatibility covers present, confirmed
  absent, and missing evidence; offline unknown is never absent: §5/§7/§8.
- A4 — per-action source/target identity and declared position preserved;
  source→target shape updates feed later index/constraint checks: §3/§7.
- A5 — four anchors with post-change catalog oracle; default and custom
  policy plus PostgreSQL regression: §6/§8.

### 1.1 The first input and its equivalent

Primary input on pre-state `golden.t(c INT NULL DEFAULT 0, keep_c INT NULL)`:

```sql
ALTER TABLE t ALTER COLUMN c SET DEFAULT 1;
```

With only `ddl.alter.set_default.explicit_default_change.forbid` enabled at
blocker (and every unrelated rule off), the oracle is: normalized action
`set_column_default` naming column `c`, exactly one blocker finding under that
rule ID, verdict reject, `--fail-on blocker` exit 1, no other finding.

Equivalent representative under the same policy intent:

```sql
ALTER TABLE t MODIFY COLUMN c INT NULL DEFAULT 1;
```

produces exactly one blocker under
`ddl.alter.modify_column.explicit_default_change.forbid`. "Equivalent" means
the same `explicit_default_change` policy intent fires — not identical
metadata across different actions. Opposite controls:
`ALTER TABLE t ALTER COLUMN c DROP DEFAULT` governed by
`ddl.alter.drop_default.explicit_default_change.forbid`, and
`ALTER TABLE t ADD COLUMN other_c INT NULL` under both default rules enabled
produces no finding. Each control runs on an independent fresh pre-state; a
prior statement's success or failure never supplies the next control's
premise.

## 2. Reuse map

### 2.1 Existing and reused as-is

- Action normalization: `add_columns`, `drop_column`, `modify_column`,
  `change_column`, `rename_column`, `set_column_default`,
  `drop_column_default`, `set_column_visibility`, `rename_table`,
  `add_constraint`, `add_index`, `drop_index`, `drop_primary_key`,
  `drop_foreign_key`, `rename_index`, `table_option`, `placement_policy`.
  MySQL/TiDB `ALTER COLUMN SET|DROP DEFAULT` normalize to
  `set_column_default`/`drop_column_default` (T07 boundary: deliberately not
  the PostgreSQL `set_default`/`drop_default` names).
- `Alter.Column.OldName` (source identity), `Alter.Column.Definition`
  (target/full declaration), `Alter.Column.Change` (`TouchesDefault`,
  `TouchesNullability`, `TouchesAutoIncrement`, `DeclaresPrimaryKey`),
  `Alter.Name` (canonical subject), `DDL.Alter` slice order = statement order.
- `forbiddenExplicitAlterColumnChangeRule` — per-action matching with full
  finding metadata (`table`, `action`, `name`, `column_name`, `change_kind`).
- `alterObjectExistenceRule` — generic member existence with EvidenceGaps
  mirroring its skip premise (unknown member set → gap, never fabricated
  absence); `alterPrimaryKeyExistenceRule`, `newColumnTargetExistsRule`.
- `alterColumnCompatibilityRule` for `modify_column`/`change_column` — type
  family change, integer narrowing, string narrowing, unsigned flip,
  nullability tightening (nullable→NOT NULL), AUTO_INCREMENT removal, plus
  `missing_source_column`/`incomplete_source_column` evidence gaps.
- `unknownPriorNullabilityAdvisoryRule` — the notice precedent for authored
  nullability with unknown prior state.
- `tableExistenceRule` (`ddl.table.exists.alter.require`), table/column
  declaration rules on CREATE, PG namespaces `ddl.pg.*` and the PG-scoped
  `ddl.alter.*.forbid` action rules (dialect-allowlisted).
- Provider column facts: `information_schema.COLUMNS` ordered by
  `ORDINAL_POSITION` (slice order IS the position fact), full
  type/sign/length/nullability/default/default-expression/EXTRA/
  visibility/generated facts; `TableSnapshot` explicit `PrimaryKeyUnknown`,
  `IndexesUnknown`, `ConstraintsUnknown`.
- Ordered-state templates: `applyAlterTable` single-action dispatch,
  `prepareModifyColumn`, `prepareColumnIdentity`, `prepareDropColumn`,
  `prepareColumnDefault`, `prepareColumnVisibility`, `modifyPublication`
  tombstone + replace seam, dependent-key computation
  (`modifyDependentKeys`, `modifyIdentityDependentKeys`), rename pair,
  `AssessAdvancedColumns` gate (`InvalidTransition`/`UnresolvedTransition`).
- Coverage: `statementCoverageAspects`, `auditedAlterAction` per dialect,
  `column_position` aspect gap (becomes audited under this contract, §7).
- Golden runner, manifest format (`metadata_cases`, `cli_cases`,
  `error_cases`, `required_case_ids`), `validate_artifact` validator chain.

### 2.2 Existing but not wired (T08 connects, unchanged semantics)

- `ddl.alter.set_default.explicit_default_change.forbid` and
  `ddl.alter.drop_default.explicit_default_change.forbid` are registered,
  default enabled, blocker — but bind to the PostgreSQL action names
  `set_default`/`drop_default` only, so MySQL/TiDB
  `set_column_default`/`drop_column_default` never reach them (the #87 silent
  path). T08 extends their action binding to the MySQL/TiDB names; PG binding
  is unchanged.
- `hasColumnPositionClause` already parses `FIRST|AFTER` presence; the AST
  additionally carries `Position.Tp` (`first`/`after`) and
  `Position.RelativeColumn` — the position target is extracted in §4.
- `set_not_null`/`drop_not_null` are PostgreSQL-only action names; MySQL and
  TiDB express nullability changes only via MODIFY/CHANGE, so the
  `set_not_null`/`drop_not_null` explicit-change rules stay PG-scoped in
  practice and need no wiring.

### 2.3 Genuinely missing (T08 adds, bounded)

- Column-existence checks for `set_column_default`/`drop_column_default`
  (new rule IDs, §5.3).
- Position-target existence check and position-aware post-state (§5.3, §7).
- Bounded multi-action successor (§7.4): today every `len(ddl.Alter) != 1`
  invalidates.
- Effective-default transition in the compatibility rules (§5.4): defaults
  are the one column attribute not yet compared.
- MODIFY/CHANGE omission semantics normalization (§5.5, §7.3): whether an
  omitted DEFAULT/NULL/AUTO_INCREMENT clears, preserves, or resets is fixed
  per product by §6 native evidence.

## 3. Fixed internal facts

- `spec.Alter` gains `Position *spec.AlterPosition`. `AlterPosition` is a
  spec-owned type: `Kind string` (`"first"` or `"after"`) and `After string`
  (normalized relative column name; empty for `first`). `HasColumnPosition`
  remains the presence flag; `Position` non-nil ⇔ `HasColumnPosition` true.
  Public JSON is the normalized shape — the position fact is part of the
  statement result (`json:"position,omitempty"`), carrying kind + normalized
  identifier only.
- No other new public result fields. `TableSnapshot.Columns` order remains
  the position fact; no ordinal field is added.
- `Alter.Column.OldName`/`Definition`/`Change`, `Alter.Name`, `DDL.Alter`
  order and the `Touches*` flags keep their frozen meanings.
- Clone safety: `Position` is a pointer — deep-copied wherever `Alter` is
  copied; request-local like all extraction facts (race-checked path).

## 4. Parser adaptation

`extractAlterSpec`/`extractAlterSpecs` read `specification.Position` and emit
`Alter.Position{Kind, After}` (`RelativeColumn.Name.L`, normalized lowercase
like other identifiers). `HasColumnPosition` keeps driving from the same AST
check — the two facts are always consistent. ADD `(a, b, ...)` multi-column
lists get one position at the spec level; per MySQL grammar a parenthesized
column list carries no position clause, so a multi-column spec with a
position is a parser-visible anomaly: the extractor records
`UnextractedOptions:["column_position"]` on the first emitted alter instead
of attributing a per-column position, yielding incomplete coverage and a
tombstoned successor — never a normalized position fact (native N-cells
confirm the syntax error on all anchors).

No other grammar changes. `ALTER COLUMN SET|DROP DEFAULT`, MODIFY, CHANGE,
RENAME, ADD, DROP extraction is unchanged.

## 5. Rule wiring and policy matrix

### 5.1 Explicit-operation policy (frozen intent)

The `explicit_*_change.forbid` family forbids the *authored attribute
operation*, not the effective value difference. `SET DEFAULT 0` on a column
already `DEFAULT 0` fires — no no-op exemption. `forbid: false` disables the
check; `enabled: false` removes the rule. The rule does not need prior
metadata (except the existing metadata-aware modify-nullability variant,
whose transition gate is unchanged).

Wiring (§2.2): the rule structs take an action list instead of one action:

| Rule ID | Bound actions | change_kind |
|---|---|---|
| `ddl.alter.set_default.explicit_default_change.forbid` | `set_default` (PG), `set_column_default` (MySQL/TiDB) | explicit_default_change |
| `ddl.alter.drop_default.explicit_default_change.forbid` | `drop_default` (PG), `drop_column_default` (MySQL/TiDB) | explicit_default_change |
| `ddl.alter.modify_column.explicit_default_change.forbid` | `modify_column` | explicit_default_change |
| `ddl.alter.change_column.explicit_default_change.forbid` | `change_column` | explicit_default_change |
| `ddl.alter.modify_column.explicit_nullability_change.forbid` | `modify_column` (metadata-aware) | explicit_nullability_change |
| `ddl.alter.change_column.explicit_nullability_change.forbid` | `change_column` | explicit_nullability_change |
| `ddl.alter.modify_column.explicit_auto_increment_change.forbid` | `modify_column` | explicit_auto_increment_change |
| `ddl.alter.change_column.explicit_auto_increment_change.forbid` | `change_column` | explicit_auto_increment_change |
| `ddl.alter.set_not_null.explicit_nullability_change.forbid` | `set_not_null` (PG-only action) | explicit_nullability_change |
| `ddl.alter.drop_not_null.explicit_nullability_change.forbid` | `drop_not_null` (PG-only action) | explicit_nullability_change |

SET DEFAULT ↔ MODIFY/CHANGE equivalence is this mapping: the same
`explicit_default_change` intent under the action-specific rule ID. All
default level blocker, param `forbid` (strict bool, default true). Findings
keep full identity metadata; one finding per matching action (never keyed
by rule_id, so multi-action statements keep per-column results).

### 5.2 Action-level forbid

No MySQL/TiDB action-level forbid is added for the setters —
`ddl.alter.set_default.forbid`/`ddl.alter.drop_default.forbid` remain
PostgreSQL-dialect-allowlisted warning rules. The explicit-change family is
the single governance point; a generic action forbid would double-count the
same intent.

### 5.3 Existence rules (new IDs, shared `alterObjectExistenceRule`)

| Rule ID | Actions | Object | Check | Default |
|---|---|---|---|---|
| `ddl.alter.set_default.exists.require` | `set_column_default` | column | require present | enabled, blocker |
| `ddl.alter.drop_default.exists.require` | `drop_column_default` | column | require present | enabled, blocker |
| `ddl.alter.column_position.exists.require` | `add_columns`, `modify_column`, `change_column` carrying an `after` position | column | require present | enabled, blocker |

They bind only the MySQL/TiDB action names: PG `set_default`/`drop_default`
never match, so PG behavior is unchanged. The position rule selects the
position's relative column name; `first` positions carry no reference and
are skipped. `forbidIfExists=false` findings mirror the existing family:
`column "x" does not exist on table "t"` with `table`/`action`/`name`/
`exists:false` metadata. EvidenceGaps mirror the skip premise exactly —
`target_table.columns` for a withheld column set; a confirmed-absent table
adds no member gap.

### 5.4 Compatibility rules — effective-default transition

`ddl.alter.modify_column.compatibility.require` /
`ddl.alter.change_column.compatibility.require` (default enabled, warning;
`required: true` strict bool) keep every existing comparison and gain the
default attribute, the last uncompared governed attribute:

- Resolve the *effective* target default per §6 product semantics
  (omitted-DEFAULT resolution, dialect-parametrized).
- `source.HasDefault` vs effective-target presence flips → findings
  `column "x" loses its default value` / `column "x" gains a default value`.
  Presence is a provable fact on both sides and stays determined.
- Both sides have a default → the comparison is three-state (amended
  T08-R1): proven-same → silent; proven-different →
  `column "x" changes its default value`; unproven → no transition finding
  and a bounded evidence gap instead. One shared comparison drives both
  `Evaluate` and `EvidenceGaps` — a fact can never be convicted as changed
  in one place and called unknown in the other.
- Finding metadata carries classification only:
  `source_default`/`target_default` ∈ `none|literal|null|current_timestamp|
  expression` — never the literal text, expression, or fingerprint.
- Source facts drive `EvidenceGaps`: `source_column.default` joins
  `incomplete_source_column` when a loaded source column's default identity
  is unproven; `missing_source_column` stays as today. An unknown source
  produces a gap, never a "changed" or "unchanged" claim.

#### 5.4-R1 Comparable default identity (amended)

`Column.DefaultValue` is source-shaped text by contract — declaration side
it carries the rendered literal with its quote layer; provider side it
carries verbatim `COLUMN_DEFAULT` bytes. The two representations are not a
comparable identity, and an empty expression fingerprint is an unknown
fact, never a determined change (both pre-R1 defects).

- `spec.Column.DefaultLiteral *spec.LiteralDefaultFacts` (internal,
  `json:"-"`) is the comparable literal identity: `Kind` ∈
  `{"string","numeric"}` plus `Value`, the decoded literal value bytes.
  nil means the identity is unproven or the default is not literal-class —
  an empty declared string is a proven `""`, never the unknown sentinel.
- Declaration side: the typed AST yields the decoded value — `'x'` → `x`,
  `'''x'''` → `'x'`, `'O''Reilly'` → `O'Reilly`, `'NULL'` → `NULL` —
  with kind `string`; integer literals (including a unary `±`) decode to
  kind `numeric`. Non-string/non-integer literal forms (decimal, bit, hex,
  bool, bytes) and non-literal expressions carry no literal identity —
  bounded scope, not a coercion simulator.
- Provider side: `COLUMN_DEFAULT` stores the decoded literal value bytes
  on all four anchors (`'x'`→`x`, `'''x'''`→`'x'`, `'O''Reilly'`→
  `O'Reilly` — content quotes are data, not a representation layer), so
  the stored bytes are the literal identity verbatim; kind comes from the
  column's type family (integer/decimal/float → `numeric`, otherwise
  `string`). A keyword-looking stored string like `CURRENT_TIMESTAMP` on
  a non-temporal column stays literal — `DefaultIsCurrentTimestamp` is
  asserted only for temporal column families.
- Same literal class on both sides: kind equal + value equal → same; kind
  equal + value different → different **only inside a proven comparable
  domain** (§5.4-R2); kind different → unproven (cross-domain coercion is
  not modeled). Expression class compares
  `DefaultExpression.Fingerprint` only when both sides are `Known` with a
  non-empty fingerprint — equal → same, unequal → different, anything
  else → unproven.
- Unproven never emits a transition finding. The gap vocabulary keeps
  `incomplete_source_column`/`source_column.default` for source-side
  unproven; target-side or cross-domain unproven adds reason
  `unproven_default_identity` with `target_column.default` (plus
  `source_column.default` when the comparison domain itself is shared-
  unknowable). Reason codes and fact names stay machine-readable — never
  values.
- Derived state carries the same fact: CREATE/SET DEFAULT publish the
  declared literal identity, DROP DEFAULT clears it, MODIFY/CHANGE publish
  the full new definition's identity, and `cloneColumn` deep-copies the
  pointer — so a provider-sourced and a declaration-derived pre-state
  judge identically.

#### 5.4-R2 Comparable domain for numeric literals (amended)

`Kind="numeric"` marks the literal's *value domain*, not a proven shared
encoding. Catalog scale/type normalization (`decimal(10,2)` stores `1.00`)
and declaration AST text (`DEFAULT 1` decodes `1`) are different encodings
of possibly-equal values; comparing those bytes directly as identity
convicted a representation difference as a determined change (R2 defect).

- Only domains with an established cross-source canonical encoding compare
  directly: `string` values are decoded bytes (canonical by definition),
  and `numeric` values compare only when **both** sides carry canonical
  integer text (`0` or `-?[1-9][0-9]*` — no leading zeros, plus signs,
  scale, or whitespace). Identical recorded bytes short-circuit to same
  before the domain check — equal representations need no encoding
  reasoning (`1.00` vs `1.00` stays same, pinned by rule-level cell
  `numeric identical scale silent`).
- A numeric pair outside the canonical-integer domain is unproven —
  `unproven_default_identity` over both `*_column.default` facts — even
  though each side's own literal fact is recorded. Known facts whose
  pairing cannot be proven comparable are a comparison gap, never a
  fabricated missing-fact or a determined change.
- Deferred: DECIMAL/FLOAT/YEAR-adjacent normalization across encodings
  (scale, scientific notation, sql_mode effects). R1's string and integer
  positives are unchanged; presence flips, class switches, and
  expression-fingerprint semantics are unchanged.

### 5.5 Explicit-operation vs effective-change, separated

- Explicit-op rules answer "did the user write this attribute operation".
- Compatibility rules answer "does the effective attribute change".
- MODIFY/CHANGE omitting DEFAULT is not an authored default operation — the
  explicit-op rule does not fire for it; whether it is an *effective* change
  is decided by §6 product semantics and then governed by §5.4. The same
  statement may carry an explicit-op blocker for one attribute and a
  compatibility warning or evidence gap for another; a gap never erases a
  decided policy result.
- The catalog cannot recover whether the original DDL wrote `DEFAULT NULL`,
  `DEFAULT 'NULL'`, or omitted DEFAULT — the T06-A4 source boundary is
  retained; comparisons consume `HasDefault`/`DefaultIsNull`/`DefaultValue`/
  expression facts exactly as the provider supplies them.

### 5.6 Privacy

Finding/gap/explanation/diagnostic payloads carry action, table, column
identity, existence/classification flags and fixed reason vocabulary only —
never default literals, expressions, fingerprints, raw SQL, provider error
payloads or credentials. `raw_sql` echo keeps its existing contract.

## 6. Product and version responsibilities

Anchors: MySQL 5.7.44, MySQL 8.0.46, MySQL 8.4.10, TiDB 8.5.0 (frozen
images/digests from the existing golden compose set).

Native-confirmed semantics (all §8.2 cells executed 2026-10-21 against the
frozen images; raw matrix preserved in the evidence set):

- `ALTER COLUMN c SET|DROP DEFAULT`: supported on all four anchors; a
  missing target column is rejected (1054) on all four.
- `MODIFY|CHANGE [COLUMN] ... [FIRST|AFTER col]`: all four.
- `RENAME COLUMN`: MySQL 8.0+, TiDB (supported since 5.x); MySQL 5.7 rejects
  (parser accepts — the existing `rename_column.version.require` gate).
- Full-redefinition (CONFIRMED, identical on all four anchors):
  MODIFY/CHANGE restate the complete column; omitted attributes reset —
  `MODIFY c INT` on `c INT NULL DEFAULT 0` yields `IS_NULLABLE=YES,
  COLUMN_DEFAULT NULL`; `MODIFY c INT NULL` still clears the default;
  `MODIFY u INT` on `u INT UNSIGNED NOT NULL DEFAULT 0` drops `unsigned`,
  resets nullability to YES and clears the default; `CHANGE c c2 INT` clears
  identically. **Omitted DEFAULT is removed, never preserved — on TiDB too.**
- AUTO_INCREMENT removal via MODIFY (version split, CONFIRMED): all three
  MySQL anchors silently drop `EXTRA=auto_increment` (`MODIFY pk INT` →
  rc=0, EXTRA empty, PK member stays NOT NULL); **TiDB 8.5 rejects** the
  same statement — `ERROR 8200 ... can't remove auto_increment without
  @@tidb_enable_auto_increment_in_generated`. The product must not map a
  MySQL error class onto TiDB or vice versa.
- Multi-action (CONFIRMED dialect split): TiDB validates every action's
  premises against the pre-state — `ADD c1, ADD c2 AFTER c1` fails (1054,
  `c1` not in pre-state), `DROP b, ADD b` fails (1060 duplicate), touching
  the same column twice fails (8200 "Unsupported operate same column").
  MySQL applies left-to-right — `ADD c1, ADD c2 AFTER c1` and `DROP b, ADD
  b` succeed (re-added column lands at the end); `MODIFY a INT, MODIFY a
  BIGINT` still fails on MySQL too (1054). Disjoint multi-actions
  (`ADD m1, ADD m2`; `ADD a, ALTER COLUMN c SET DEFAULT 2`; `MODIFY c ...,
  MODIFY keep_c ...`; `ADD x, DROP b`) succeed on all four.
- Atomicity: a multi-action ALTER TABLE is one statement — a failed premise
  means no successor; never publish a prefix of applied actions.
- Position (CONFIRMED): `FIRST` lands at ordinal 1; `AFTER known` lands at
  index+1; `AFTER missing` fails (1054) on all four; `MODIFY ... FIRST` and
  `CHANGE a a2 ... AFTER c` move/rename on all four; parenthesized
  `ADD COLUMN (m,n) FIRST` is a syntax error (1064) on all four — the
  grammar carries no position on the multi-add form.
- PK member: `MODIFY pk INT NULL` and `CHANGE pk pk2 INT NULL` fail
  `ERROR 1171 (All parts of a PRIMARY KEY must be NOT NULL)` on all four.
- Default-kind facts (CONFIRMED): `d1 INT DEFAULT NULL` vs `d2 INT` (no
  DEFAULT clause) are **catalog-indistinguishable** — both
  `COLUMN_DEFAULT IS NULL` (the A4 source boundary holds natively);
  `DEFAULT 'NULL'` stores the literal (HEX `4E554C4C`); `DEFAULT ''` stores
  the empty string (HEX empty, `IS NULL`=0); `SET DEFAULT NULL` on a NOT
  NULL column fails `ERROR 1067` on all four; `SET DEFAULT 'x'` on a column
  with no prior default succeeds.
- `ALTER COLUMN` setters cannot target a missing column (1054, confirmed),
  ADD/CHANGE destination collisions fail, `AFTER` a missing column fails —
  all confirmed per anchor in §8.2.
- Type rendering: `int(11)` on 5.7 vs `int` on 8.0/8.4/TiDB — an
  oracle-normalization fact (T06 fidelity), not a T08 semantic.
- No AUTO_RANDOM, allocator, next-ID, data-conversion or
  existing-data-safety claims anywhere. Empty-table ALTER success is never
  evidence of data safety.

## 7. Ordered state and ownership

### 7.1 Single-action templates (existing + position)

- `add_columns`, `modify_column`, `change_column`, `rename_column`,
  `drop_column`, `set_column_default`, `drop_column_default`,
  `set_column_visibility` keep their frozen single-action publications —
  with position now modeled rather than aspect-gapped:
  - `FIRST` → the affected column occupies index 0 of the post-state column
    order (add: insert; modify/change: move).
  - `AFTER x` → the affected column occupies index(`x`)+1; `x` must resolve
    in the (per-dialect) validation set, else the premise fails and the
    affected identities tombstone — a known-illegal shape is never
    published.
  - No position → existing end-append / in-place semantics unchanged.
  - `Position` on actions that cannot legally carry it (e.g.
    `drop_column`, setters) remains a coverage-level anomaly — parser does
    not produce it for those actions.
- The `column_position` aspect gap is removed once the aspect is audited;
  `fullyAuditedStatement` then admits positioned templates. `HasColumnPosition`
  false-positives (no position) do not exist — the flag and `Position` are
  one fact.

### 7.2 MODIFY/CHANGE effective-default resolution in post-state

`prepareModifyColumn`/`prepareColumnIdentity` resolve the published column's
default facts through the same spec-owned normalization §5.4 uses — one
seam, consumed by findings and state. Whatever §6 native evidence decides
for omitted-DEFAULT/NULL/AUTO_INCREMENT is what the successor publishes;
independent of which policies are enabled. A transition whose effective
outcome is not provable keeps the tombstone.

### 7.3 Identity and dependents

Rename/change migrate `PrimaryKey.Columns`, primary-key constraint columns
and ordinary index column refs (existing `rewriteIdentityColumnRefs`);
foreign keys and unmodelled members keep their refuse-or-tombstone
contracts. `modifyIdentityDependentKeys`/`modifyDependentKeys` semantics are
unchanged; a multi-action publication collects dependents across all
touching identities.

### 7.4 Bounded multi-action successor

For `len(ddl.Alter) > 1` on `DDLOperationAlterTable` with full coverage:

- Dialect-parametrized premise resolution:
  - MySQL: actions apply left-to-right; each action's premises validate
    against the state accumulated from preceding actions.
  - TiDB: every action's premises validate against the pre-state only; an
    action referencing a sibling's product or touching a sibling's object is
    a known failure.
- Publishable only when every action is in the provable template set:
  `add_columns` (each on a distinct name), `modify_column`, `change_column`,
  `rename_column` (distinct old names, destinations collision-free),
  `set_column_default`/`drop_column_default`/`set_column_visibility` (each
  on an existing column no sibling adds/drops/renames), `drop_column` (on an
  existing column no sibling re-adds or references). Any action outside the
  set, a duplicate/conflicting object touch, an unresolvable premise, or an
  unprovable net effect → the whole bound identity set tombstones — no
  prefix publication.
- The net successor applies the actions in declared order to a cloned
  shape (insert/move/remove columns per §7.1, default/visibility flips per
  the setter publications, identity refs per §7.3), once.
- Findings stay per-action (§5.1); policy findings and the state outcome are
  independent — a policy reject does not publish a successor, and a clean
  audit still publishes the modeled net effect.

### 7.5 Chains that must hold end-to-end

With a controlled provider (known pre-state, ledger on LoadTableSnapshot):

1. `SET DEFAULT` → subsequent `MODIFY` reads the new default facts.
2. `DROP DEFAULT` → subsequent `MODIFY` reads cleared defaults.
3. `ADD COLUMN` → `MODIFY`/`RENAME` → `CREATE INDEX` on the resulting name.
4. `CHANGE`/`RENAME` → old name absent, new name present for existence
   rules and the next statement.
5. Positioned add/modify → the next statement's checks read the reordered
   `Columns`.
6. `DROP COLUMN` → old absent, other members intact.
7. Tombstoned/polluted pre-state stays unknown for successors; a never
   re-read provider snapshot cannot wash an unknown back to known.
8. `MODIFY` omitted-DEFAULT post-state matches the §6 anchor answer.

Each chain asserts provider call counts, pre-state identity and the exact
post-state; the old DDL, provider objects and prior snapshots stay
immutable.

### 7.6 Error and boundary channels

- Provider failure/context cancellation propagate as errors — never a
  metadata gap or empty snapshot.
- `UnresolvedTransition`/`InvalidTransition` (T07) tombstone as before;
  T08 adds no path around the assessment gate.
- Unaffected schemas/entries are never invalidated; tombstone scope is the
  bound identities plus already-loaded dependents only.
- `clearModifyAffectedStats` reuse for stats cleanup — no recomputed
  cardinality, allocator, or row counts; unknown values are never written 0.

## 8. Evidence, oracle and frozen roles

### 8.1 Role classes

`roles.json` freezes the denominator and per-role binding. Classes:

- `cli_cases` — offline/controlled-metadata CLI + shared Audit seam roles:
  first-path blocker, MODIFY equivalent, DROP/opposite controls, disabled
  and `forbid:false` controls, default-governance literals (number, string,
  `NULL`, `'NULL'`, `''`), MODIFY/CHANGE explicit and omitted-default cells,
  nullability cells (explicit NULL/NOT NULL/omitted, both directions, PK
  member, prior-unknown advisory), type/sign/auto_increment cells, position
  cells, multi-action cells, state-chain cells (§7.5), parser-negative and
  unsupported controls. Expected verdict/findings/evidence-gaps/state bound
  per role.
- `metadata_cases` — live-anchor roles: the same governance matrix against
  the four fixtures with real provider pre-state, setup/cleanup steps and
  post-change catalog assertions (§8.2 oracle columns).
- `error_cases` — parser failure, startup/config failure (distinct from
  product semantic failure), cancellation.
- `required_case_ids` — the frozen denominator; removing a case or
  loosening an expectation is a contract violation.

### 8.2 Bounded native cells (recorded before production code lands)

Each native cell: drop-if-exists setup + absence check → product audit does
NOT execute DDL → independent driver executes the fixed ALTER → on success
query `information_schema.COLUMNS` (full set in ordinal order: type+flags,
`IS_NULLABLE`, `COLUMN_DEFAULT` raw+HEX, `EXTRA`, `COLUMN_KEY`) and table
options; on failure record the real error number/source and verify no
success shape; cleanup drops only what this run created; final COUNT of
created objects = 0.

- N-omit: `MODIFY c INT` (omit NULL and DEFAULT) and `MODIFY c INT NULL` /
  `MODIFY c INT NOT NULL` variants on `c INT NULL DEFAULT 0` → effective
  `IS_NULLABLE`/`COLUMN_DEFAULT`, all four anchors.
- N-omit-ai: `MODIFY c INT` on `c INT AUTO_INCREMENT` → `EXTRA`, all anchors.
- N-setter: `ALTER COLUMN c SET DEFAULT 1`, `SET DEFAULT NULL`,
  `SET DEFAULT 'NULL'`, `SET DEFAULT ''`, `DROP DEFAULT` → `COLUMN_DEFAULT`
  raw+HEX, all anchors.
- N-pos: `ADD a INT FIRST`, `ADD b INT AFTER keep_c`, `ADD x INT AFTER
  missing`, `MODIFY keep_c INT FIRST`, `CHANGE keep_c keep_c INT AFTER a` →
  `ORDINAL_POSITION` sets and error classes, all anchors.
- N-multi: `ADD a INT, ADD b INT`; `ADD a INT, ALTER COLUMN c SET DEFAULT
  1`; `MODIFY c INT NULL, MODIFY keep_c INT NOT NULL`; `ADD c1 INT, ADD c2
  INT AFTER c1` (the documented MySQL/TiDB split); `MODIFY c INT, MODIFY c
  BIGINT` (same object twice); `DROP keep_c, ADD keep_c INT` → per-anchor
  accept/error + resulting column set/order.
- N-pk: `MODIFY pk INT NULL` / `MODIFY pk INT` on a PK-member column →
  accept/coerce/error per anchor.
- N-default-kind: `SET DEFAULT` literal vs `'NULL'` vs `NULL` → whether
  `COLUMN_DEFAULT` distinguishes authored NULL-string from SQL NULL
  (documents the retained A4 boundary precisely).

Expected outcomes are bound per cell from §6 docs; a doc-silent cell records
the observed anchor result and the contract mechanism consumes it.

**Executed** (126 cells × 4 anchors + 3 extra kind cells; raw JSON
`native-matrix.json`/`native-extra.json` in the evidence set): every §6
bullet above is confirmed natively per anchor — full-redefinition clears
omitted DEFAULT/NULL/unsigned on all four; TiDB rejects AI removal (8200)
and pre-state-unsatisfied multi-action premises (1054/1060/8200) where
MySQL succeeds; same-column multi-op fails everywhere (1054 vs 8200);
PK-null MODIFY fails 1171 everywhere; `SET DEFAULT NULL` on NOT NULL fails
1067 everywhere; parenthesized `ADD COLUMN (…)` + position is a syntax
error everywhere; authored-NULL-string vs SQL-NULL stays
catalog-distinguishable only by `COLUMN_DEFAULT IS NULL` + HEX (the A4
boundary). All fixtures dropped; residual object COUNT = 0.

### 8.3 Validator requirements

`ddl-golden-validator` (existing strict checks) plus T08 checks rejecting:
setter downgraded to no-op; SET/DROP borrowing MODIFY/CHANGE results or vice
versa; lost/wrong/duplicated action targets or positions; finding keyed by
rule_id overwriting a sibling column's result; profile-swapped expectations;
true/false, loaded/unloaded, unknown/absent conflation; declared-known
forged as metadata-missing; unknown prior forged as unchanged; post-state
missing a column, wrong column order, resurrected old name; audit-without-
driver or native failure masquerading as policy reject; anchor cross-binding;
case/check removal or shrunken denominator; literals/expressions leaking
into diagnostics.

### 8.4 Cross-surface roles

Shared Audit carries the matrix once. CLI covers JSON output, normalized
position shape, `--fail-on` exit codes, offline mode (no accidental connect
flags), startup failure vs product semantic failure. SDK/HTTP/MCP each take
the representatives the parent contract requires: pass, policy reject,
missing-prior review/unverified, version-gated negative, real parser
failure, T07 expression eval gap, privacy, cancellation — no
adapter-specific semantics.

### 8.5 Regressions held

T07 generated/visibility/fold boundary (207), T06 ordinary declarations
(496), T05 state chains, T02–T04, PG default rules/actions/provider — all
regression, none silently upgraded; §9 lists the pre-declared upgrades.

## 9. Old expectations and upgrades

- `EXPECTED-UPGRADES.md` lists every frozen expectation this task promotes:
  exact case, statement, rule_id, old value, new value, reason. In scope:
  T05/T06 manifest cells where `column_position` coverage or the
  set/drop-default wiring changes an expectation, plus any T05 multi-action
  cells the bounded template now models. Out of scope: no SQL changes to
  avoid problems, no negative deletion, no finding downgrade, no
  denominator shrink.
- Known candidate upgrades at freeze time: positioned ALTER cells in T05
  golden that previously expected `column_position` aspect gaps now produce
  audited outcomes; T05 multi-action cells that tombstoned may gain
  modeled successors. Each gets a row only after the new oracle is bound.

## 10. Final code identity and delivery

- One milestone branch, focused commits, code and evidence separate.
- Final SHA → one full gate run (§8.3 + repo gates) → evidence archive at
  `docs/verification/t08/<final_code_sha>/` → fast-forward `origin/workfree`.
- Binary identity (`go version -m`, SHA-256, build command) recorded for
  working CLI, golden CLI and probes separately; deterministic-build check.
- Environment failures are preserved, never washed; credentials only via
  env vars, never in artifacts.
- No merge/release/PR lifecycle claim; issues stay open; stop ready for
  human review.

## 11. Role matrix

See `roles.json` for the machine-readable frozen matrix. Summary:

- ~40 offline/controlled `cli_cases` + ~70 `metadata_cases` across the four
  anchors + ~6 `error_cases` — the exact denominator is fixed in
  `roles.json.role_count` and `required_case_ids` before implementation.
- Every role binds: id, SQL, pre-state, dialect/anchor, policy profile,
  expected verdict/findings (rule_id + metadata assertions), coverage,
  evidence gaps, post-state or catalog oracle.
- Independent oracle: case expectations are authored against §5/§6/§7
  semantics and the §8.2 native records — never derived by reading the
  implementation's own output.
