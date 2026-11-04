# Decision: TiDB 8.5 placement-policy lifecycle, binding state, and the metadata/topology split (MySQL vendor boundary)

Date: 2026-11-12 — milestone decision-record convention: decision dates
are assigned per task slot and run ahead of commit dates; neighboring
records follow the same convention.
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T20 under #99)
Related commits: this task's commits on the milestone line after `57c207a8`
Related tests: `internal/application/audit` T20 suite
(`batch_state_t20_test.go`), parser disposition updates
(`stmt_disposition_test.go`), coverage pins
(`coverage_t03_test.go`), census privacy pins
(`mysql_tidb_ddl_normalized_silent_metadata_census_test.go`), corpus
fixtures under `testdata/sql-corpus/`, and
`make ddl-golden TASK=T20 ARTIFACT_DIR=...` against
`testdata/ddl-golden/T20.json`
Related docs: module README updates in the same change
(`internal/domain/spec`, `internal/application/audit`,
`internal/domain/rule/ddl`, `internal/infrastructure/parser/tidb`,
`internal/infrastructure/metadata/mysql`), `testdata/ddl-inventory`,
coverage catalog `docs/reference/ddl-coverage.*`

## Context

Issue #99 owns the TiDB placement-policy surface: `CREATE | ALTER | DROP
PLACEMENT POLICY`, placement binds on databases, tables, partitions, and
`ALTER RANGE`, the `= DEFAULT` unbind form, and the new
`ddl.tidb.placement.warn` finding. TiDB 8.5 native probing established
four facts that shape the design:

- A named-policy bind refuses natively when the policy is absent
  (ERROR 8239); binding is a catalog fact (`*_PLACEMENT_POLICY_NAME`
  columns in `SCHEMATA`, `TABLES`, `PARTITIONS`) that is provable from
  `information_schema` — while `SHOW PLACEMENT` reporting `PENDING`
  demonstrates that catalog binding never proves topology deployment.
- `= DEFAULT` is an unconditional unbind at every level; it does not
  require the policy to exist.
- `DROP PLACEMENT POLICY` refuses while any object is bound
  (ERROR 8241).
- `ALTER RANGE` accepts only `global`/`meta` on v8.5; other range names
  fail parse/semantic validation (1105). The legacy
  `/*T![placement]*/` direct-options form is a 1064 parse error on
  8.5 — the comment is lexed as real tokens, not silently ignored.

MySQL has no placement feature: every placement statement or option
under MySQL is a vendor boundary that preserves exact pre-state,
including `= DEFAULT` (an unbind is still a vendor feature).
`/*T![feature]*/` comments are a different question — MySQL parses them
as ordinary comments and ignores them entirely, so under the MySQL
dialect they can carry no placement semantics at all (T20-R2).

## Decision

1. **Policy registry is ordered state, seeded from metadata.** The
   provider gains `LoadPlacementPolicies`; a nil catalog means
   unvouched (unreadable), a non-nil map — including empty — is a
   proven catalog. `PlacementPolicyFact` carries proven `BoundTargets`
   so a bound-policy drop refuses (native 8241 parity) and an
   unvouched drop stays unproven rather than guessing absence.
2. **Binding is a tri-state outcome, not a flag.**
   `PlacementBindOutcome` returns refused (non-TiDB dialect, or named
   policy proven absent), applied (DEFAULT unbind unconditionally; named
   policy proven present), or unproven (policy catalog cannot answer).
   Proven refusal preserves exact pre-state; unproven withholds only
   the affected fact. A placement spec may only be the sole
   schema-changing action of a multi-spec ALTER — probed 8200 on
   v8.5.0 ("Unsupported multi schema change for alter table
   placement"): any structural/option sibling or a second placement
   spec refuses the whole statement atomically; only ALGORITHM/LOCK
   modifiers combine (probed rc=0) (T20-R1 correction).
3. **Inbound references are maintained, not recomputed.** Bind,
   unbind, drop-table/drop-schema, and rename paths update
   `BoundTargets` with exact identity matching (`schema:foo` must not
   collide with `schema:foobar`; renames rekey `table:` identities).
   Policy ALTER *replaces* the option definition — probed on
   v8.5.0: after `ALTER PLACEMENT POLICY p FOLLOWERS=4`,
   `SHOW CREATE` emits only `FOLLOWERS=4`; unstated prior members do
   not survive. Policy identity and bound references are untouched
   (T20-R1 correction).
4. **Topology is explicitly out of scope.** A successful binding audit
   states that the catalog would record the binding — it never claims
   placement has deployed. `SHOW PLACEMENT`/`SCHEDULE_STATE` data is
   documented as a separate evidence layer; where it cannot be proven,
   the finding and decision record say so instead of implying success.
5. **Bounded option modeling with a privacy contract.** Placement
   options are stored as bounded keys (`primary_region`, `regions`,
   `follower_count`, `voter_count`, `learner_count`, `schedule`,
   `constraints`, `leader_constraints`, `follower_constraints`,
   `voter_constraints`, `learner_constraints`, `survival_preferences`)
   with normalized display values; raw constraint bodies (e.g. region
   name lists) are not copied into findings or metadata projections.
   Census tests pin this no-leak contract.
6. **`ALTER RANGE` is bounded to `{global, meta}`.** Other known names
   refuse; missing range information is unproven. MySQL `ALTER RANGE`
   is a vendor boundary.
7. **Rule `ddl.tidb.placement.warn`** is a warning-level statement rule
   that fires once when a placement lifecycle or binding action is
   proven applied; refused, unproven, and unbind outcomes stay silent
   (unproven yields evidence gaps instead). The rule never mutates
   ordered state; rule enablement does not change transitions.
8. **MySQL ignores `/*T!` feature comments; the dialect entry blanks
   them (T20-R2).** `blankTiDBFeatureComments` rewrites `/*T! ... */`
   regions to same-width whitespace before the shared TiDB parser sees
   a chunk, using a real lexer pass (strings, quoted identifiers, and
   other comments are never scanned). Positions, `RawSQL`, and
   statement boundaries keep the original text; the AST simply never
   contains the comment's tokens. Probed on mysql:8.0.46 —
   create/alter/direct-option comment forms all land rc=0 with no
   binding, `ALTER TABLE t` with only a comment is a legal no-op, and
   the bare `PLACEMENT POLICY` clause stays a 1064 vendor boundary. The
   blanking covers every `/*T!` feature uniformly because MySQL treats
   them all identically; it does not touch `/*!` executable comments
   or `/*+` hints.
9. **`ALTER DATABASE` applies non-placement options before the binding
   step (T20-R2).** v8.5.0 `AlterSchema` runs
   `ModifySchemaCharsetAndCollate` then `ModifySchemaDefaultPlacement`:
   a proven-absent policy fails 8239 yet the declared charset still
   lands — in either SQL option order — and a prior binding survives
   the failed rebind. `schemaPostAttrs` therefore returns the placement
   outcome separately: ALTER publishes the proven non-placement attrs
   and keeps the prior binding on a refused bind, invalidates only the
   `placement_policy` attr and the inbound refs on an unproven bind;
   CREATE stays atomic (8239 → no schema row). The earlier comma-form
   probe (1064) was a grammar boundary, not the multi-option grammar —
   whitespace-separated `DatabaseOptionList` is the legal form.

### T20-R3 — catalog premise rule and the topology evidence gap

Two contract gaps surfaced when the real PD/TiKV fixture ran and the
acceptance review re-read #99 against the shipped profile:

10. **`ddl.tidb.placement.exists.require` owns the catalog premise
    (blocker).** The warn rule's job is binding risk on *applied*
    outcomes, so disabling it — or simply never firing it — left a
    proven-absent policy reference silently passing (`pass/0/0`). The
    new rule adjudicates the existence premise independently of the
    risk warning: a proven-absent named reference refuses as a blocker
    (8239 parity), a proven-bound policy drop refuses (8241 parity),
    and a bare `CREATE POLICY` on a proven-existing name refuses.
    `OR REPLACE`/`IF NOT EXISTS`/`IF EXISTS` disarmed forms and
    `= DEFAULT` unbinds carry no premise and stay silent; an
    unvouched catalog or bound-target set yields that rule's own
    `placement_policy.catalog`/`placement_policy.bound_targets` gaps.
    Ordered state already refused these paths — the change surfaces
    the refusal in public findings instead of letting the internal
    state alone imply it.
11. **Unknown topology is a structured gap, not a suggestion.** #99
    requires 拓扑未知给gap而非猜测可部署; previously an applied bind
    produced only the binding-risk warning and `coverage=complete`,
    which implied deployability the product never verified. The warn
    rule now emits a `placement_policy.topology` evidence gap for
    topology-relevant applied outcomes — binds and policy ALTERs that
    reassign live placement — while `Metadata.PlacementTopology` is
    nil. `spec.PlacementTopologyFacts` (label universe + observed
    `SCHEDULING_STATE`, `json:"-"`) is defined as the receiving slot;
    no provider populates it yet, so the gap fires honestly on every
    applied bind — including against a real cluster whose topology is
    provably satisfiable. CREATE/DROP policy, DEFAULT unbinds, and
    refused/unproven outcomes carry no topology premise and never
    stack the gap.
12. **Premise attribution is exact, and OR REPLACE rewrites are
    topology-relevant (T20-R3 review corrections).** Both placement
    rules report only the facts actually missing: an unvouched catalog
    owes `[catalog, bound_targets]` on a drop (existence decides
    whether the bound set is even consultable), while a
    proven-existing policy with an unread bound set owes
    `bound_targets` alone. `CREATE OR REPLACE` on a proven-existing
    policy carries the topology gap — it rewrites the definition every
    bound object deploys, the same live reassignment as ALTER POLICY.
    Review also surfaced a laundering bug in the ordered-state seed:
    a provider fact whose `BoundTargets` was nil (unread) seeded
    `refsKnown=true`, laundering "bound set unvouched" into "provably
    unbound" — `refsKnown` is now only set when the provider read is
    present.
13. **Real-cluster evidence bounds the deferral.** A disposable
    1×PD + 3×TiKV + 1×TiDB v8.5.0 fixture (labels `region=t20rev`,
    `zone=z1..z3`, `host=h1..h3`, STRICT mode) proved the native
    spectrum this contract separates: satisfiable bind → `SCHEDULED`;
    missing policy → 8239 with the prior binding preserved;
    label-unmatchable policy → 8243 refused at bind time; a
    replica-count policy (FOLLOWERS=5 vs 3 stores) → bind accepted and
    persistently `PENDING`. Receipts live under
    `docs/verification/t20-tidb-placement-lifecycle/` beside the
    compose manifest; they bound the semantics the rules model but do
    not claim the product evaluates topology.

## Consequences

- TiDB placement statements are promoted from the unaudited gap list to
  audited coverage; MySQL retains its vendor-boundary coverage rows.
- `TableSnapshot`/`SchemaSnapshot` gain `PlacementPolicy` (and
  `PartitionPlacements` member map) projection fields. Subpartitioned
  catalogs leave leaf member placement unobserved rather than
  fabricating a directory.
- Deferred/unmodeled: `PLACEMENT POLICY` option *values* do not drive
  state beyond bounded display facts; `SHOW PLACEMENT`,
  `SHOW PLACEMENT FOR`, label schedules, and topology/PD facts are not
  modeled. Applied binds carry the structured
  `placement_policy.topology` gap until a provider wires
  `PlacementTopologyFacts`; missing-policy and bound-drop refusals are
  public blocker findings, not silent state.
- Golden oracle corrections (T20-R3, case IDs preserved):
  `tidb85-bind-missing-refused`, `tidb85-drop-bound-refused`, and
  `tidb85-alterdb-partial` now expect `reject`/blocker/exit 1; the four
  applied-bind cases expect `unverified` + the topology gap; the four
  catalog-gap CLI cases expect two per-rule `placement_policy.catalog`
  gaps.

## Verification

- Unit/ordered-state: `batch_state_t20_test.go` covers bind/unbind,
  missing/unvouched/proven policy, schema/partition/range binds,
  create/alter/drop/replace, bound-drop refusal, rename rekeying,
  mixed-ALTER atomicity, MySQL vendor refusal, and no-aliasing.
- Native TiDB 8.5 anchor: real `CREATE/ALTER/DROP POLICY`, binds at
  table/schema/partition/range scope, 8239/8241 refusals, `= DEFAULT`
  unbind, `ALTER RANGE dist` 1105 — executed during development and
  encoded as `T20` Golden metadata cases.
- Golden: `testdata/ddl-golden/T20.json` — 25 required case IDs across
  four anchors; native receipts are evidence, not topology claims.
- T20-R3 public-path matrix: `batch_state_t20r3_test.go` — topology gap
  on applied binds/policy ALTERs, per-rule catalog-gap ownership,
  exists.require blocker on proven refusals, disarmed-premise silence,
  and fail-loud provider errors.
- Corpus: `testdata/sql-corpus/tidb/ddl/findings/placement_policy_missing.*`
  covers `ddl.tidb.placement.exists.require` on the supported-rule
  inventory.
