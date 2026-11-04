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
  modeled and are called out as evidence gaps where relevant.

## Verification

- Unit/ordered-state: `batch_state_t20_test.go` covers bind/unbind,
  missing/unvouched/proven policy, schema/partition/range binds,
  create/alter/drop/replace, bound-drop refusal, rename rekeying,
  mixed-ALTER atomicity, MySQL vendor refusal, and no-aliasing.
- Native TiDB 8.5 anchor: real `CREATE/ALTER/DROP POLICY`, binds at
  table/schema/partition/range scope, 8239/8241 refusals, `= DEFAULT`
  unbind, `ALTER RANGE dist` 1105 — executed during development and
  encoded as `T20` Golden metadata cases.
- Golden: `testdata/ddl-golden/T20.json` — 23 required case IDs across
  four anchors; native receipts are evidence, not topology claims.
