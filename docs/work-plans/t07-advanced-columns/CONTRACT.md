# T07 advanced columns — implementation contract

Status: FROZEN for T07 implementation. Independently authored SQL, role,
profile, version, result and native-schema constants are in
`scripts/t07_contract.py`; `testdata/ddl-golden/T07.json` is its materialized
manifest and `roles.json` its role index. Internal read-only design review and
lead source/native-oracle review preceded this freeze; neither is external
acceptance. Same-contract implementation/test/collector bugs may be repaired
without reducing the frozen responsibilities or denominator.
Owner: #86 under #79. Task base: 7a65fe162695ce1248d6edfb825b04e6b81b48b8.
T06 closeout: 1997dac1ae5ec5fe6abefdb7f9868e1a15999a47.
Sources: [SOURCES.md](SOURCES.md). The user authorizes internal freezing without
another approval round, not changes to the parent result/status contract.

## 1. Scope and semantic seam

Implement one shared application/SDK path for CREATE/ALTER generated columns,
expression defaults and column visibility. Existing parser/spec/provider,
registry/EvidenceReporter, ordered state and result aggregation remain the
seams. Do not duplicate semantics in transports, execute submitted SQL, add an
expression evaluator, change permissions, or undertake T08/T09/T10 governance.
T06's 496 cases and original 728 validator checks remain intact; T07 adds its
own independent denominator and sensitivity checks.

The first input is exactly:

```sql
CREATE TABLE t (
  a INT,
  b INT GENERATED ALWAYS AS (a + 1) STORED
);
ALTER TABLE t DROP COLUMN a;
```

With only the dependency rule enabled, the supported-target oracle is two
retained DDL statements, first complete/zero findings, second complete/exactly
one blocker for a being referenced by b, aggregate reject, nil audit error,
CLI fail-on blocker exit 1. Its opposite control adds ordinary c and drops c:
no dependency finding, the remaining a/b facts stay available to a subsequent
ordinary CREATE INDEX on b. A generic notice/drop-forbid/existence finding does
not satisfy either oracle.

## 2. Fixed internal facts (no new public result fields)

Keep all existing Column fields and meanings, including Type and A17 rendering,
Length/Unsigned, HasDefault/DefaultValue/DefaultIsNull/current-time flags,
GeneratedWhen/identity facts on PostgreSQL and unextracted unrelated options.
Add these internal fields to spec.Column with `json:"-"`:

- `Generated *GeneratedColumn`: nil = no generated attribute; non-nil carries
  `Storage` (`virtual`, `stored`, or `unknown`) and `Expression ExpressionFacts`.
  AST Stored=false means effective VIRTUAL, including the omitted-storage form;
  do not claim whether the optional storage keyword was explicitly written.
- `DefaultExpression *ExpressionFacts`: nil = no expression-default fact;
  non-nil represents expression syntax/semantics, not a literal string that
  merely resembles SQL. A non-nil fact with Known=false is unknown, not empty.
- `Visibility string`: `visible`, `invisible`, or empty (withheld/not yet
  resolved). `VisibilityExplicit bool` is declaration presence, never inferred
  from catalog normalization. CREATE omission resolves to visible; a full
  MODIFY/CHANGE definition with omission resolves to visible on the tested
  MySQL forms, as proved by native V05/V06. SET/DROP DEFAULT preserves visibility.

`ColumnReference` has Schema/Table/Name strings. `ExpressionFacts` has
`References []ColumnReference`, `Known bool`, `Functions []string`,
`Features []string` and `Fingerprint string`. All are internal. References are
AST column references, never literal contents; qualifier identity is retained.
Functions and feature names are normalized structure, not raw SQL. Fingerprint
is SHA-256 of parser-restored expression structure with lowercase identifiers/
keywords and valid string escaping, after unwrapping outer parentheses only.
It is for bounded identity comparisons, never evaluation or diagnostics.
Unknown/missing fingerprint cannot prove expression equality.

Known=true plus an empty reference list means known no local column reference.
Known=false remains unknown even if a partial list was collected. Foreign/
unresolved qualifiers do not bind to same-spelled local names. Preserve
reference/declaration order and deduplicate identical reference identities.
Use the existing locked AST visitor; cap new expression traversal at 4096 nodes
and dependency traversal at 1024 columns/64 edges per path. A limit hit is
explicit unknown/incomplete, not an empty complete graph. No configurable
pipeline or generalized expression execution engine is introduced.

Shared domain interfaces are `AssessAdvancedColumns(statement Statement)
AdvancedColumnAssessment` and `ColumnDependents(table Table, columns []Column,
name string) ColumnDependencyAssessment`. The former returns Applies, Issues,
Gaps, InvalidTransition and UnresolvedTransition; the latter returns ordered
Dependents plus Known. The shared assessment additionally carries structured
`DependencyEffects []ColumnDependencyEffect` (Column, Action, Dependents, Known)
and separate DependencyGaps. Compatibility Applies/Features/Issues/Gaps do not
absorb dependency-only actions or their evidence. This keeps the two rule
applicability domains distinct while state consumes one assessment. Each dependent carries Column, Kind and Dependency
(direct/transitive). Issues carry Column/Feature/Action/ReasonCode/MinimumVersion;
gaps carry ReasonCode/RequiredFacts. These spec-owned data shapes import no
rule/report/infrastructure packages. Rules map them to existing Finding/
EvidenceGap, and ordered state reuses their transition flags, not verdicts.
A dependency break sets InvalidTransition independently of enabled policies;
version uncertainty alone does not erase a CREATE declaration's conditional
shape. Expose one infrastructure helper
`ParseCatalogExpressionFacts(text string) spec.ExpressionFacts` from parser/tidb;
AST traversal stays private there. The MySQL provider uses that helper rather
than introducing another expression parser.

Deep-copy both new pointers, embedded expression facts and all new slices in
cloneColumn. Test declaration/provider/old-projection immutability, independent
requests, cancellation before publication and the necessary race path.

## 3. Parser adaptation

The locked TiDB parser accepts generated expressions but lacks column
visibility and rejects valid MySQL arithmetic/column-reference DEFAULT
expressions. These are demonstrated by the baseline parser/native records.
Do not upgrade or fork the dependency merely to change representation.

Add a narrow grammar inside the existing parser adapter. Tokenize DDL with
quoted identifiers, escaped string literals, comments and balanced parentheses;
recognize column-option positions in CREATE and ALTER ADD/MODIFY/CHANGE and the
single `ALTER [COLUMN] name SET VISIBLE|INVISIBLE` form. Do not infer support by
regex/substring inspection of a failed parse. Run the supplement before the
existing parse, bind every recovered fact to a successfully parsed table/column
identity, and retain original statement text, counts, positions and siblings.

The concrete infrastructure carrier is an adapter-owned statement wrapper
embedding an underlying ast.StmtNode plus typed column/visibility supplements.
WrapStatements classifies the underlying real node; extraction and legacy
coverage switches also receive that node. Original text stays on the wrapper
and underlying node. Supplements are applied to domain facts before coverage
classification, after exact table/column/action binding. Do not add invented
ColumnOptionType values to the third-party AST. For the visibility setter,
parse the narrow syntax as its own typed adapter form with validated table/
column identifiers (the existing SELECT-column grammar may validate identifiers
statically), not by representing visibility as a fake DEFAULT or integer type.
Non-T07 actions retain their existing grammar, order and unextracted markers.

For parenthesized DEFAULT, parse its expression using the existing TiDB
expression grammar (a SELECT expression, never executed), validate one
expression and no additional statement, and install the real expression AST in
the final column/default action. Temporary same-width parse placeholders must
never escape as facts. Preserve parenthesized expression identity where the
base grammar discards it. Column visibility uses typed adapter-owned facts, not
a fabricated default, comment or index attribute. Supplemental parsing failures
remain bounded parser errors; unsupported semantics are not fabricated from
arbitrary malformed text. Index visibility and non-T07 SQL keep their old path.

MySQL/TiDB ALTER SET/DROP DEFAULT use `set_column_default`/
`drop_column_default` with Column.OldName and a default-only Definition;
TouchesDefault is set. Do not reuse the existing PostgreSQL `set_default`/
`drop_default` action space: its enabled forbid rules would unintentionally
introduce T08 policy blockers. Those rule defaults and their old dialect/action
behavior are unchanged. T07 owns advanced-expression facts and their state;
the T08 inventory row retains general default-policy equivalence/unknown-prior
state governance, with a narrative cross-reference to this narrower T07 seam. Visibility becomes `set_column_visibility` with a
visibility-only Definition, canonical target name and explicit visibility fact.
ADD/MODIFY/CHANGE continue sharing extractColumn. Do not change the A17 private
type renderer or globally lowercase member literals.

## 4. Catalog source

Extend the existing ordered COLUMNS query by GENERATION_EXPRESSION; preserve
all original selected values and ordering. No second table snapshot read is
introduced. Parse catalog expressions as data through the same infrastructure
expression extractor, never execute/EXPLAIN them. Catalog attributes/effective
definitions are not original declaration spelling or presence.

MySQL8.0/8.4 catalog expression text can contain an SQL-string escape layer
(e.g. backslash-escaped quote delimiters); raw bytes/HEX remain unchanged.
Try the normal expression grammar first. Only a syntactically valid encoded
SQL-string representation with recognized escapes may be decoded through the
same parser and reparsed as one expression. Never globally remove backslashes,
lowercase literal text, evaluate SQL, or turn an undecodable value into known
zero references. Test literal quotes/backslashes and the recorded JSON/literal
catalog forms explicitly.

Read generated storage from complete EXTRA attribute tokens plus generation
text. Generated attribute with absent/unparseable expression produces non-nil
unknown facts, not absence. DEFAULT_GENERATED identifies expression defaults;
temporal CURRENT_TIMESTAMP keeps the old temporal contract. Keep original
HasDefault and stored DefaultValue bytes. SQL NULL never proves original
DEFAULT omission; an empty value never automatically means no DEFAULT.
EXTRA INVISIBLE proves effective invisibility; no marker on an otherwise
complete catalog row means effective visible, not explicitly declared visible.

Tests must update the same-root controlled driver row shape (old eight values
plus generation expression), explicitly labeling ordinary fixtures as empty
generation text. Never add a production fallback that accepts a truncated
catalog response. Query/scan/Rows.Err/provider/cancellation errors propagate.
Prove complete ordered sets and one LoadTableSnapshot per identity with real
provider-source AuditSQL and query ledgers.

## 5. Two stable rules and their exact contracts

### `ddl.alter.column.dependency.require`

Default enabled, level blocker; params `required: true` (strict bool).
AppliesTo: MySQL/TiDB ALTER DROP COLUMN and identity-changing CHANGE/RENAME
COLUMN; not ordinary MODIFY with the same identity. PostgreSQL is unchanged.
The rule reports known generated/default-expression dependents, direct or
transitive. It does not blanket-forbid DROP or claim indexed-column optimization.

One finding per affected dependent column, in declaration order. Exact metadata
keys: `table`, `column`, `dependent_column`, `dependency_kind` (`generated` or
`expression_default`), `dependency` (`direct` or `transitive`), `action`.
Message: `cannot <drop|rename> column "<column>": <generated|expression-default>
column "<dependent>" depends on it` (one logical line; no expression text).
`required=false` is loaded/inapplicable; disabled is absent. Pure policy state
does not control fact propagation.

Unknown target/withheld columns -> unknown_table_state with
`target_table.columns`; unknown expression dependency -> incomplete_source_column
with `source_column.expression_dependencies`. Confirmed absence or missing
source member stays owned by existing existence checks, not a substitute
risk finding. Known blockers and independent gaps coexist. A multi-action
statement that could also repair/remove dependents is not falsely called a
known break: emit `incomplete_alter_dependency_effects` with
`alter.column_dependency_effects` and use the old conservative state boundary.
No source declaration is relabeled metadata-unknown to stand in for that net
effect uncertainty. Multiple per-action blockers are not emitted for a
multi-action form whose net dependency effect is not resolved.

### `ddl.column.advanced.compatibility.require`

Default enabled, level blocker; params `required: true`, optional `features`
list restricted to `generated`, `expression_default`, `visibility` (default all).
Reject invalid param types/values; no temporary or per-transport rule IDs.
AppliesTo: declared advanced CREATE/ALTER facts and the source-aware generated
transitions needed by the fixed roles. Ordinary unchanged columns/defaults
are negative controls, not a version-dependent blanket statement check.

Exact finding metadata keys: `table`, `column`, `feature`, `action`,
`reason_code`, `product`, `target_version`, `minimum_version` (empty when not a
minimum-version failure). Message: `column "<column>" advanced definition is
not supported for the target (<reason_code>)`. Level is configurable normally.
No expression, literal, canonical text, fingerprint or raw banner is exposed.

Version/fact handling uses existing VersionIdentity and EvidenceReporter:
missing_target_version -> target.version; out-of-range ->
target_version_out_of_validated_range / target.version.validated_range;
missing source generation/identity/type -> incomplete_source_column with the
precise fixed source_column fact names in the role oracle. Missing versions do
not suppress dependency/name checks. Disabled compatibility checks do not hide
independent tool/vendor capability boundaries.

## 6. Fixed product/version responsibilities

- MySQL generated columns: >=5.7.6 (constructed before/at boundary controls,
  actual 5.7.44/8.0.46/8.4.10 anchors). Omitted storage is VIRTUAL.
- MySQL expression DEFAULT syntax: >=8.0.13, distinct from literal/SQL NULL and
  the existing bare temporal current-time default forms.
- MySQL explicit VISIBLE/INVISIBLE and SET visibility: >=8.0.23. Keep all
  structural columns, including invisible columns, in later explicit lookups.
- TiDB 8.5.0: CREATE generated VIRTUAL/STORED, ADD VIRTUAL, same-expression
  stored CHANGE rename and ordinary controls are independently observed.
  ADD STORED/stored-expression changes/ordinary-to-STORED are known negatives;
  VIRTUAL/STORED conversion is negative on all anchors.
- TiDB stored-to-ordinary: the exact 8.5.0 anchor succeeds and clears generation
  despite contradictory release-8.5 documentation. Pin this observed patch
  only; do not extrapolate this conversion to every 8.5 patch. Other unverified
  patches yield feature-specific `target_version_out_of_validated_range` with
  `target.version.stored_to_ordinary`; global validated-series identity is not
  rewritten. This transition is unresolved for publication, not a known database
  rejection. Other checks and known independent dependencies remain usable.
- TiDB expression defaults: not MySQL parity. CREATE JSON_OBJECT is a positive;
  arithmetic/local-column CREATE defaults are known negatives. ALTER UUID on
  VARCHAR and DROP a CREATE-defined JSON expression default are separate
  positive controls; ALTER SET JSON expression is a known negative (1101).
- TiDB explicit column visibility is a vendor boundary on the validated
  product scope, not a parser bug or simulated visibility. Both explicit
  VISIBLE and INVISIBLE have native 1064 proof. No tested raw visibility
  attribute was accepted-but-ignored; do not invent that outcome.

Generated disallowed functions/constructs and invalid local reference/order/
cycle premises are distinct bounded reasons. Default expressions may use
nondeterministic built-ins without evaluating them. The selected TiDB function
forms must obey its documented restricted support, including CREATE/ALTER
and type differences. Unknown function semantics or unreliably bound
expressions remain explicit tool analysis boundaries, not a false complete
zero-reference fact. The existing JSON-generated corpus form also recognizes
JSON_UNQUOTE/JSON_EXTRACT, UNHEX/SHA2 and CAST as deterministic structure;
references come from their arguments, not the literal JSON path or a hash
value. This is fact extraction, not a cryptographic/runtime evaluator. The
finite role oracle defines the promises, not arbitrary
SQL-string or function-version Cartesian coverage. On a version with known
visibility support, `no_visible_columns` is the bounded reason for CREATE or
SET visibility leaving no visible column. Version/vendor negatives take
precedence over inventing that structural conclusion on unsupported syntax.
Recognized forbidden constructs/nondeterministic generated functions use
`expression_construct_forbidden`/`generated_function_forbidden`; unknown
functions produce the fixed unaudited aspect
`<create_table|alter_table>.<column/action>.expression_dependencies` with
metadata `aspect: column, reason: unknown_function`, never an echoed function
argument or fake known-empty reference list. The CREATE feature is exactly
`create_table.column.expression_dependencies` as frozen in the CLI oracle.

## 7. Ordered state and ownership

Reuse the existing pre-state -> conditional publication loop. CREATE/ADD carry
new facts; one known-template MODIFY/CHANGE preserves/replaces advanced facts
within the accepted ordinary type-class boundary. Do not restore a tombstone
or reread stale provider state. Related ordinary CREATE INDEX sees generated/
invisible columns without becoming an index optimizer.

DROP a known referenced source and known invalid/version/vendor operations
must not publish a false success shape, independently of rule switches. An
unrelated c removal and a leaf-generated-column removal preserve remaining
known columns/dependencies when all other existing template premises hold.
Do not leave every table containing a generated column permanently unknown.
Unknown dependency facts, multi-action interactions and unrecomputable loaded
constraints keep the accepted conservative publication boundary.

Default-only and visibility-only setters clone the existing column, modify only
that attribute and keep unrelated identity/type/generated/default/visibility
facts as appropriate. Full MODIFY/CHANGE omission follows the effective
native-defined attribute semantics, not an invented inheritance. The old
ordinary-only literal-default conservative DROP behavior in T06-A4 is not
silently widened to obtain a green T07 result. Pure team-policy rejection
never suppresses a syntactically valid conditional transition; known broken
structural premises do. All new reference/pointer ownership is request-local.

## 8. Evidence/oracle and test roles

The committed `roles.json`, T07 manifest and independent Python oracle fix
207 required Golden identities: 8 database baseline, 2 baseline CLI, 34 offline
CLI, and 163 metadata cases (38 native roles × four anchors plus 11 provider
sources). Ordered identity digest is
`d723ac9553bf4e6ccc6ba1a73be2e4ba036903bb77a32c72f6b49878502f1d25`.
Every exact SQL, profile, dialect/target version, per-statement
finding/gap/coverage/position and native setup/step/structure/teardown contract
is authored there before production work. This is distinct from the 152
pre-contract observation bindings and T06's immutable 496 identities.
Provider cases deliberately cover generated dependencies on all anchors,
expression-dependent columns on MySQL8.0/8.4, independent JSON defaults on
TiDB, invisible explicit-index lookup on MySQL8.0/8.4 and visibility negatives
on MySQL5.7/TiDB. Separate declaration/derived cases cover function setters and
DROP expression defaults; the provider role is not a full function Cartesian
matrix.

The native-schema oracle was checked against retained observations at all 152
bindings without another database run; the first review discrepancy and its
correction are retained, including MySQL5.7 G17 error1054 and MySQL8 catalog
escaped delimiters. These are native representation/error facts, not copied
baseline product expectations. Baseline product results only identify missing
behavior; they are never copied into future expectations.

Each family includes dangerous/legal/counterexample, required=false/disabled,
known supported/known unsupported/missing version, declaration/provider/derived
representatives, CREATE/ALTER differences and stable identities. Real parser/
AuditSQL first-path red must compile on the baseline and fail semantic
assertions, not missing symbols/fixtures/config or connectivity. Generated
first path precedes default/visibility implementation, but completion requires
all families. Tests cross the shared Audit/SDK seam and real provider; direct
extractor tests diagnose source facts, not replace end-to-end proof.

Additional fixed controlled/shared cases are software tests, not extra Golden
business identities:

- C01: TiDB target8.5.1, `CREATE TABLE t(a INT,b INT AS(a+1) STORED); ALTER
  TABLE t MODIFY COLUMN b INT;`, compatibility-only profile: zero findings,
  one statement1 feature-specific patch gap described above; ordinary
  post-shape is not published. 8.5.0 same input is the native-positive control.
- C02: both dialects at their fixed modern targets, `CREATE TABLE t(a INT,b
  INT AS(a+1) STORED); ALTER TABLE t DROP COLUMN b,DROP COLUMN a;`, dependency
  profile: no claimed known-break finding; statement1 gap
  incomplete_alter_dependency_effects / alter.column_dependency_effects;
  later table projection unknown under the old multi-action boundary.
- C03: G02 plus `CREATE INDEX idx_b ON t(b);`: exactly a,b remain in order,
  b still depends on a, no index-missing finding, one provider read at most.
- C04: G11 plus `ALTER TABLE t DROP COLUMN a;`: exactly one statement2
  dependency blocker, column=a/dependent_column=b2; old b cannot reappear.
- C05: G14 plus `ALTER TABLE t DROP COLUMN a;`: supported anchors retain
  ordinary b with no generated fact or dependency finding; no stale source
  expression survives the conversion.
- C06: G16 plus `ALTER TABLE t DROP COLUMN a;`: unchanged b dependency yields
  exactly one statement2 blocker after the unrelated c type change.
- C07: supported MySQL CREATE invisible hidden_c, SET literal DEFAULT,
  SET expression DEFAULT, DROP DEFAULT: every projection retains hidden_c
  invisibility; setters do not alter its name/type/generated facts. Full
  MODIFY/CHANGE omission instead follows V05/V06 reset.
- C08: dependency-only controlled provider has generated b with Known=false:
  DROP a emits no fabricated blocker/pass, precisely one incomplete_source_column
  / source_column.expression_dependencies gap. Known=true with empty references
  is the distinct no-dependency control. Nil/empty/duplicate/reordered source
  collections cannot hide a required member.
- C09: mutate every nested new pointer/slice in a derived clone and reread:
  original parser DDL, provider facts, previous/next projections and a second
  request remain byte/field equivalent to their complete expected collections.
  Count actual provider calls (one per table identity), cancellation and race.
- C10: default-policy MySQL/TiDB literal/default-expression setter roles must
  not load a new PG default-forbid behavior merely because the action was
  modeled. Existing PostgreSQL action/policy controls retain their old result.
- C11: scoped same-spelled external column references and strings are not local
  dependencies; quoted real local references are. Unknown qualified identity
  is never silently known-empty. Resource limits yield bounded unknown facts,
  not complete pass, and no private marker is copied to added channels.

Privacy uses artificial literal markers and separately asserts retained
raw_sql/normalized_sql and original unsupported sql echo. Only those existing
input-echo paths are exempt; new findings/gaps/diagnostics/explanations/log/error
channels must not contain markers. Do not delete public raw SQL to pass.

The validator uses independent frozen constants through full validate_artifact,
not artifact.expected or manifest/artifact agreement alone. It rejects missing/
duplicate/cross-statement/cross-column/dependent findings, switched profiles/
versions/anchors, known->unknown and unknown->empty laundering, skipped native
success/rejection or intermediate facts, missing/duplicate structure members,
double-deleted required checks, strict integer/JSON type laundering and a
shrunk denominator. The valid control passes; adversarial mutations fail with
assertions, not exceptions. Original T02–T06 contracts are not weakened.

## 9. Old expectations and non-goals

Before upgrading an old expectation, record exact statement/rule_id/field,
old/new value and T07 ownership in EXPECTED-UPGRADES. Expected source changes
include removal of generated-as-unextracted boundaries, valid expression/
visibility grammar promotion, exact new dependency/version findings and
necessary catalog/driver row-shape additions. Do not change unrelated SQL,
profiles, rules, finding levels or negatives to bypass failures. Generated
columns cannot require an illegal ordinary DEFAULT merely because they lack
HasDefault; any existing default-presence rule adjustment is generated-only.

Do not change A17 Type rendering/ordinary sentinels, DEFAULT NULL/comment
identity, A15 rough formulas, PostgreSQL behavior, generic column governance,
index optimization, partition/constraint owner scope, runtime values, arbitrary
SQL mode matrices, evaluation, lock prediction, rollback or authorization.

## 10. Final code identity and delivery

Finalize facts/rules/grammar/tests/role oracle/manifest/validator/docs/L2/L3/ADR
and expected upgrades, then commit one final complete code SHA. No subsequent
source/test/project-document amendment just for titles or headers. Update
warehouse-free task progress at /tmp/ds-t07-proof/PROGRESS.md; final phase
records are outside the source tree.

At that SHA execute T07 complete focused/parser/provider/transport/isolation/
race checks, A17 and affected T05/T06 regressions, GOFLAGS=-count=1 make test,
pg-unit-test-gates, sql-corpus-gates, ddl-inventory-gate,
ddl-coverage-catalog-test, docs-example-gates, decision-record-gate,
ddl-golden-validator-test, gofmt and actual task-range/L2/L3 checks.
Run actual T07 and T02/T03/T04/T05/T06 Golden + binary-enabled validate serially,
each with its own external artifact root, raw commands/rc and actual final HEAD.
Native probes and Golden cannot compete for fixture names. Only owned
self-created empty tables/containers/networks are cleaned; pre-existing
resources stay untouched. No repeated native run solely for archive layout.

Retain separate work CLI, each actual Golden CLI and probe build command,
path/hash/actual go version -m; archive builds record origin/source hashes and
absence of VCS fields honestly. Evidence-only commit goes under
`docs/verification/t07/<full-final-code-sha>/`, with complete raw cases/policies/
catalog/structure, red/green, privacy/errors/sensitivity, source/build identity,
T06 closeout and T07 acceptance mapping. No executable binary, credential, DSN,
.mcp.json contents or large HTML is committed. Preserve failed originals.

Normal ff push to origin/workfree only after review/gates; verify live refs,
parent chain and key remote raw originals. No main push, PR/merge/tag/release,
issue comments/checkbox/closure or T08 work. Stop at ready for human review,
not a fabricated external Reviewer or hosted-CI conclusion.
