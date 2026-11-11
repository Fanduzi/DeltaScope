# Decision: MySQL DROP SERVER conditional privilege premise + shared adjudication + OrderedAbsent (T26-SERVER-DROP-EXECUTION-01)

Date: 2026-11-16
Status: Implementation candidate — reviewer ruling pending. The
APPROVED design contracts this record implements are listed under
Related docs; this ADR itself records the candidate design, not an
accepted contract.
Related milestone/version: issue #105 T26 under parent issue #79
Related commits: T26-SERVER-DROP-EXECUTION-01 implementation candidate
commit `8b52fd55` (this change set)
Related tests: `internal/domain/spec/server_drop_premise_test.go`, `internal/application/audit/server_drop_premise_test.go`, `internal/infrastructure/metadata/mysql/provider_server_drop_privilege_test.go`, `pkg/deltascope/audit_t26_premise_test.go`, `testdata/ddl-golden/T26.json` (offline cells unchanged)
Related docs: `docs/verification/t26-server-drop-premise-01/` (accepted premise contract, parent `cfd34782`), `docs/verification/t26-semantics-01/` (accepted `b8a67e7b`)

## Context

T26-SERVER-DROP-01 shipped the audited MySQL identifier-form
`DROP SERVER` slice with `server.drop_privilege` permanently owed —
SUPER/grant evidence was deliberately unwired — and no `OrderedAbsent`
representation: an audited drop could only leave the unproven barrier.
The approved premise contract
(`docs/verification/t26-server-drop-premise-01`, R2 PASS at parent
`cfd34782`) fixed what evidence may count: a conditional,
positive-only observation — `CURRENT_USER()` plus `SHOW GRANTS FOR
CURRENT_USER()` read on one database session — gated behind an
explicit caller opt-in mode, scoped per statement, and consumed by one
shared adjudication so findings, evidence gaps, and ordered post-state
can never diverge.

## Decision

- Closed privilege-mode vocabulary: `Request.PrivilegeAuditMode`
  accepts only `observed_account_new_session`; any other value is
  `ErrInvalidPrivilegeMode`. Empty keeps the legacy default where
  `server.drop_privilege` stays owed.
- Provider seam `LoadMySQLServerDropPrivilege(ctx, dialect)`: MySQL
  only, runs `CURRENT_USER()` then `SHOW GRANTS FOR CURRENT_USER()` in
  that order on one `*sql.Conn` obtained via `DB.Conn` — one database
  session guaranteed by `database/sql`, which MAY return a pooled
  connection; it is not a dedicated new physical session. The
  observation binds the observed account's grants at read time on the
  observed connection only. Query, scan, rowset, and close failures
  propagate wrapped (cause preserved via `Unwrap`); the application
  layer owns the fixed public error boundary
  (`server drop privilege observation failed`) — a failed read never
  degrades to unknown-but-successful evidence.
- Shared exact-token collector `spec.CollectMySQLGlobalPrivileges`:
  the T24 resource-group bundle and this loader parse `GRANT ... ON
  *.* ...` through one mechanism — whole tokens only, so substring
  lookalikes never bind.
- The `ServerDropPrivilegeWitness` requires all three domains:
  account bound, version in the supported domain, and exact global
  `SUPER` or `ALL PRIVILEGES`. `ALL PRIVILEGES` counts only under the
  registered MySQL 8.0.x proof domain; versions outside that domain —
  including 5.7 — are unproven inputs, not negative proofs.
- Sentence-level scope projection: the request-level observation is
  immutable; an authored `GRANT`/`REVOKE` or the `set_role`/
  `set_default_role`/`grant_role`/`revoke_role`/`grant_proxy`
  boundary features end the applicable range (positional, never
  retroactive), so later statements keep the privilege gap even under
  a positive request witness. The terminator only suppresses later
  consumability — it never claims the statement revoked a direct
  grant.
- One `spec.ServerDropAdjudication` per audited statement drives the
  evidence-gap set, finding level/message/explanation, `outcome`
  metadata (`conditional_drop`, `conditional_absent`,
  `refused_absent`, `guarded_absent`, `blocked_batch_effects`,
  `unverified`), and the ordered channel post-state. Terminal refusal
  is a blocker and owes no gap; a consumable witness discharges only
  `server.drop_privilege` — it never clears barriers, proves absence,
  or widens identity.
- `ServerState.OrderedAbsent` (`json:"-"`, never serialized): set only
  by publish arms (proven drop or convergent `IF EXISTS` absent),
  always implies `Exists=false`, is consumed ahead of any provider or
  cache access, is invalidated by later unknown writes, and a literal
  unprovable-name write taints the channel so all absent states stay
  suppressed. A zero-row catalog read still binds unknown — never
  absent.

## Rationale

Reviewer rulings required the conditional premise to be honest about
what it proves: `observed_account_new_session` vouches only that the
observed account held `SUPER`/`ALL PRIVILEGES` at observation time on
the observed connection. Three distinct things stay separate: the
observed connection (the session `DB.Conn` returned, possibly pooled),
the declared future new execution session (the conditional scope's
declared model), and any actual future execution (never claimed —
offline audit never executes). Deriving every consumer from one
adjudication table is the
accepted mechanism for keeping findings, gaps, coverage, and ordered
state on a single truth — the same shape T24/T25 already use.

## Public Contract

- `pkg/deltascope`: `PrivilegeAuditMode`,
  `PrivilegeAuditModeObservedAccountNewSession`,
  `MySQLServerDropPrivilegeObservation`,
  `MySQLServerDropPrivilegeProvider`, `ErrInvalidPrivilegeMode`.
- Public output never carries the authored server name, observed
  account, or grant text; conditional findings mark
  `conditional_audit=observed_account_new_session` and an `outcome`
  token from the fixed vocabulary.
- Default (no mode) compatibility is verified field-by-field, not
  byte-identical: verdict, evidence-gap set (`unknown_server_state`
  with `server.existence`/`server.drop_privilege` owed), statement
  and aggregate coverage, and boundary-statement behavior all match
  the pre-existing contract on the T26 golden manifest. New surface
  versus the old candidate: findings now carry the `outcome` token
  and conditional-scope metadata, and guarded/refusal explanations
  were narrowed in R1-D.

## Deferred / Out Of Scope

- No `CREATE SERVER`/`ALTER SERVER` lifecycle semantics, SRS
  adjudication, or literal-name DROP audit; their ordered effects stay
  unaudited writes/taint.
- No negative-privilege claims: grant visibility is positive-only, so
  an unproven premise is a gap, never a proven-denial.
- No live/native MySQL execution; provider behavior is verified
  through stub-driver tests only.
- No push, merge, release, or issue close.

## Verification Evidence

- `go test -count=1` on `internal/domain/spec`,
  `internal/application/audit`, `internal/infrastructure/metadata/mysql`,
  `internal/domain/rule/ddl`, `pkg/deltascope` — all pass; earlier
  full-repo `go test ./...` passed on the same tree.
- `ddl_golden.py` runner path (`make_policies` + `execute_cli_case`)
  over `testdata/ddl-golden/T26.json`: `mysql-drop-server`,
  `mysql-drop-server-if-exists`, `mysql-server-mixed-batch` — all
  `status: pass` with every assertion `ok`.
- Stub-driver provider tests cover verbatim preservation, same-conn
  query order, dialect gate, unbound account, empty grants, and
  query/scan/iteration error propagation.

## Consequences

- Future SERVER lifecycle work (CREATE/ALTER audit, SRS semantics)
  re-derives its own premises and vocabulary — nothing here obligates
  those slices to this DROP-specific adjudication shape. The shared
  mechanism worth reusing is the single-truth-table pattern, not the
  DROP arm vocabulary.
- Any new privilege-affecting statement family must register as a
  scope terminator or the observed-account premise silently widens.
- `OrderedAbsent` is channel-internal; serializing it externally is a
  contract break.

## Links

- Commits: T26-SERVER-DROP-EXECUTION-01 candidate (this change)
- Tests: paths listed in Related tests
- Docs: `docs/verification/t26-server-drop-premise-01/`,
  `docs/verification/t26-semantics-01/`
