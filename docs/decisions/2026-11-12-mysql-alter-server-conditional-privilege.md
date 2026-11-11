# Decision: MySQL ALTER SERVER first-slice execution — presence-preserving shared adjudication (T26-SERVER-ALTER-EXECUTION-01)

Date: 2026-11-12
Status: implemented on this branch; independent reviewer verdict pending
Related milestone/version: issue #105 T26 under parent issue #79
Related commits: contract acceptance `93945c01` (R5 ACCEPTED, tree
`8df83639`), CREATE product `2afe8ecb` + docs-only child `28152dfd`
(`08266403` survives only as pre-amend history), DROP product
`9dad37b8`
Related docs: `docs/verification/t26-server-alter-contract-01/`
(authoritative contract), `docs/verification/t26-server-alter-execution-01/`
(execution evidence), `docs/decisions/2026-11-16-mysql-create-server-conditional-privilege.md`,
`docs/decisions/2026-11-16-mysql-drop-server-conditional-privilege.md`

## Context

The accepted contract (R5, `93945c01`) defined the audited MySQL
identifier-form `ALTER SERVER server_name OPTIONS (...)` first slice.
ALTER updates option fields on the named `mysql.servers` row — a
partial update/merge that the presence-only channel provably cannot
turn into a create or delete — so the slice's central semantic is
*presence preservation*: no arm ever mints a new barrier, clears an
ordered answer, or records a `serverWrite`, while proven presence is
republished so a later same-identity statement still consumes it.

## Decision

- Audited slice narrows the coverage boundary one more step.
  `mysqlServerAuditedForm` admits identifier-form names within the
  64-rune bound; ALTER carries no wrapper clause, no `IF EXISTS`, and
  no `IF NOT EXISTS`. Literal names, overlong forms, and TiDB keep
  their established boundaries. Option cleanliness stays an
  adjudication premise, never a shape gate.
- G2 version gate extends to ALTER before the family loader. An
  unknown or out-of-domain resolved version marks the shape-passing
  alter unaudited with `gate:"version"` — the keep-boundary-bind order
  still issues its one catalog read only under the full premises:
  clean channel (no taint/contamination), no ordered answer on the
  identity, resolver cache miss, identifier-form name, and a provider
  that actually executes — while the privilege loader never runs.
  `statementsNeedMySQLServerPrivilege`
  widens to the alter kind, still under the same mode + dialect gate.
- Independent premise, same witness. `serverAlterPrivilegePremise()`
  projects the request-level observation per statement
  (unproven/consumable/suppressed-by-scope) with the positional
  GRANT/REVOKE terminator — a satisfied `server.drop_privilege` or
  `server.create_privilege` never discharges `server.alter_privilege`.
- One adjudication, four arms. `spec.AdjudicateServerAlter(state,
  privilege, optionsClean, contaminated)` is the single truth source
  attached as `Metadata.ServerAlter`: A1 barrier (contamination,
  channel taint, or relation-scan `batch_effects`) owes exactly
  `{server.batch_effects}`; A2 terminal refusal on OrderedAbsent owes
  nothing; A3 proven alter (proven presence + consumable + clean)
  owes nothing; A4 unproven owes the exact per-premise formula.
  `PublishPresent` is true wherever presence stands proven (A3, and
  A4 whose presence premise is discharged) — unknown existence
  publishes nothing, mints no entry, and records no barrier.
- Two conditionality axes stay separate. The published
  `OrderedPresentConditional` equals `ConditionalSource` — the
  consumed ordered flag or the catalog's unconditional false — never
  `ConditionalAudit`. A catalog-sourced republish under a consumable
  own premise therefore lands `flag=false` while the statement's own
  audit stays marked; only a consumed conditional ordered state
  republishes `flag=true`. Conditional flags are never laundered
  unconditional, and unconditional flags are never laundered
  conditional.
- Ordered apply is presence-preserving. A1 is a uniform no-op on all
  three shadowing sources — the barrier it reports is pre-existing
  state, never a minted entry; A2 leaves the consumed absent standing;
  A3/A4 republish where applicable. Audited ALTER never appends a
  `serverWrite`. Boundary ALTER forms keep their existing write-record
  and taint semantics unchanged.
- Public projection. `ddl.mysql.server.alter.notice` derives findings
  and owed facts verbatim from the attached adjudication: outcomes
  `conditional_alter`/`refused_absent`/`blocked_batch_effects`/
  `unverified`, refusal escalated to blocker, `conditional_audit`
  projected from `ConditionalAudit` only. The decided P2
  `connection_members` advisory — "ALTER SERVER specifies a partial
  options update; stored and resulting connection member sets are
  unverified." — emits on the A3/A4 arms only, carrying no outcome,
  marker, or gap. Public SQL stays the fixed `ALTER SERVER <redacted>`
  skeleton; no name, option key/value, account, or grant text reaches
  any emitted shape.

## Consequences

- Deferred, unchanged: SRS, `ALTER INSTANCE`, T27, TiDB audited ALTER,
  identity-relation widening, stored-options reads, remote
  connectivity, FEDERATED viability, new public API/CLI flags, opaque
  revision markers, native error classes, duplicate/range oracles.
- Accepted sibling semantics stay frozen: the accepted CREATE chain
  (product `2afe8ecb` + docs-only child `28152dfd`) and DROP behavior,
  including the DROP arm's own barrier-entry minting, is untouched.
- The shared request witness, observation seam, and mode vocabulary
  serve all three operations; no new request field exists.
- R3 added two nil-defaulted internal fields on `batchState`
  (`afterBindObserve`, `afterApplyObserve`) fired between bind and
  own-apply inside `enrichStatements` — a test-observation seam so the
  AM-6.6/10.2 three-timepoint metadata witnesses read real internal
  state instead of approximating from outside. They are nil in
  production and change no behavior; the contract's public surface is
  unchanged.
