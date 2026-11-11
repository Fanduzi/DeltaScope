# Decision: MySQL CREATE SERVER first-slice execution — shared adjudication + OrderedPresent (T26-SERVER-CREATE-EXECUTION-01)

Date: 2026-11-16
Status: implemented on this branch; independent reviewer verdict pending
Related milestone/version: issue #105 T26 under parent issue #79
Related commits: DROP product `9dad37b8` (R2 PASS, tree `eb544128`),
contract acceptance `112beb08` (R4 PASS), contract archive `e54e1e6e`
Related docs: `docs/verification/t26-server-create-contract-01/`
(authoritative contract), `docs/verification/t26-server-create-execution-01/`
(execution evidence), `docs/decisions/2026-11-12-mysql-create-server-contract.md`,
`docs/decisions/2026-11-16-mysql-drop-server-conditional-privilege.md`

## Context

The accepted contract (R4 PASS, `112beb08`) defined the audited
identifier-form `CREATE SERVER` first slice: parser facts, two-stage
qualification, an independent privilege premise, one shared
adjudication, and `OrderedPresent` — the post-state the DROP contract
never had. This decision records how the product implemented that
contract and which boundaries it kept.

## Decision

- Audited slice, gated in two stages. The coverage boundary admits
  only identifier-form names within the 64-rune bound plus
  identifier-form `mysql` wrappers (ASCII fold). A create that passes
  shape then faces the G2 version gate in `enrichStatementsWithMetadata`:
  an unknown or out-of-domain resolved version marks it unaudited with
  `gate: "version"` metadata *before* `statementsNeedMySQLServerPrivilege`
  can trigger the family loader — so a shape- or version-failed create
  never runs the privilege observation. Conflicting version inputs
  (malformed pin, product mismatch, pin/observed mismatch) exit earlier
  as typed errors and never reach the gate.
- New parser facts, still privacy-bounded. `WrapperIdent` records the
  wrapper's token category while it is visible; `ServerOption.RangeState`
  records `in-range`/`out-of-range`/`unproven` over the documented
  0-9999 `PORT` bound. No raw OPTIONS literal is ever stored. uint64
  overflow stays a parser rejection — it never manufactures an
  `unproven` member. Unknown keys, empty lists, malformed members, and
  trailing tokens keep their established parser-rejection boundaries.
- Independent premise. `serverCreatePrivilegePremise()` projects the
  shared request witness per statement (unproven / consumable /
  suppressed-by-scope). A satisfied `server.drop_privilege` never
  discharges `server.create_privilege` — each statement's owed set is
  its own. The once-per-request loader, the same
  `MySQLServerDropPrivilegeObservation`, and the same
  `observed_account_new_session` mode serve both operations; no new
  hook, mode, or request parameter exists.
- One adjudication. `spec.AdjudicateServerCreate(state, privilege,
  optionsClean, contaminated)` is the only truth source, attached as
  `Metadata.ServerCreate` after binding. First-match-wins arms: A1
  barrier (whole-batch contamination or the channel's
  `server.batch_effects` unread reason) owes exactly
  `{server.batch_effects}`; A2 terminal refusal on proven presence
  (OrderedPresent or a proven catalog row) owes nothing and never
  reads privilege/options; A3 proven create (OrderedAbsent +
  consumable + clean) publishes OrderedPresent; A4 unproven owes the
  exact per-premise set — a clean zero-row catalog read still owes
  `server.existence`, never an inferred absence.
- `OrderedPresent` is a second ordered answer. It publishes only from
  a proven create in the same batch, is mutually exclusive with
  `OrderedAbsent`, carries the conditional marker (every publish rests
  on a consumable conditional premise), is consumed before any
  resolver call after the taint/barrier early exits, is cleared by
  every later unknown effect exactly like `absent`, and is never
  synthesized from a provider read. DROP consumes it as proven
  presence (the same `state.Read && state.Exists` alternative as a
  catalog row — never fabricating `Read`/`Exists`/`Status`).
- Rule surface. `ddl.mysql.server.create.notice` (default: enabled,
  notice) projects the adjudication: `conditional_create`,
  `refused_present` (escalated to blocker), `blocked_batch_effects`,
  `unverified`. The `conditional_audit` marker projects the
  adjudication's exported `ConditionalAudit` fact verbatim — present
  only when the conditional scope actually fed the conclusion (the
  statement's own consumable premise in a proven/unproven arm, or a
  consumed conditionally-published ordered source). It is never
  inferred from privilege availability alone: a barrier owes only
  `server.batch_effects` and carries no marker however consumable the
  privilege was, and a catalog-present refusal carries no marker even
  with a consumable premise, because the conditional scope fed
  neither conclusion. A conditional `OrderedPresent` consumed as the
  presence answer does mark the refusal it fed. Missing documented
  USER/HOST/DATABASE members emit a second advisory-only finding
  (`notice: connection_members`) with fixed generic text — never a
  refusal, never an owed fact, asserting no native warning, and
  enumerating no option keys. No authored name, wrapper, OPTIONS
  content, account, or grant text reaches any finding, gap, or
  diagnostic.
- Contamination semantics. Whole-batch contamination reaches the arm
  table as a direct input (never projected through `UnreadReason`):
  contaminated statements adjudicate barrier, ordered entries are
  never rewritten on that path, and the global guard blocks all later
  ordered consumption and publication.

## Deferred scope

Literal-name auditing, literal/non-mysql wrappers, complete
string-literal decode, `serverRelDistinct` expansion, ALTER SERVER,
native SERVER DDL / FEDERATED / remote connectivity validation, any
new privilege-role or witness-domain enhancement, and every TiDB
semantic stay out of scope — exactly as the accepted contract bounded
them.

## Verification

- `go test` on all touched packages: parser facts (WrapperIdent,
  RangeState boundary values, overflow rejection, member boundaries),
  spec truth table (all four arms, owed-set formulas, conditional
  provenance, OptionsClean/ConnectionMembersComplete), application
  chain (G2 gate before loader, independent premise, every arm's
  public shape, OrderedPresent produce/consume/invalidate, DROP
  consuming present, contamination, objectCache sharing),
  rule-level arm projections and leak checks, pkg-level hook firing.
- T26 DDL golden (raw artifact counts): 31 cases — 23 `cli_audit`
  PASS + 2 TiDB DB fixture cells PASS + 6 MySQL DB cells FAIL = 25
  PASS / 6 FAIL; 273 assertions = 267 true / 6 false. The six fails
  are fixture-environment boundaries (no reachable MySQL): recorded
  FAIL, never relabeled product failures; native CREATE SERVER
  semantics stay unverified. All 23 CLI cases run provider-less
  without a target pin (`version_evidence` null) — they are
  boundary-regression evidence, not in-domain CREATE proof; the two
  create cases among them pin `gate: "version"` under that request.
  Six builds are registered in the evidence appendix — precommit
  workspace build, the golden artifact's own recorded build (source
  state unestablished), the clean R1 candidate build, the R2 workspace
  build used for the historical CLI run (source relationship
  unestablished), the clean R2 candidate build, and the R3 build
  measured to carry the same SHA-256 as the R2 build under the
  recorded build conditions — with no identity inferred across the
  unestablished entries.
- Full unit suite green: 35 packages, zero failures.
