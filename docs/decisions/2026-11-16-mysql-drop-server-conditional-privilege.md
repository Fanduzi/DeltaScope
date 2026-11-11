# Decision: MySQL DROP SERVER conditional privilege premise + shared adjudication + OrderedAbsent (T26-SERVER-DROP-EXECUTION-01)

Date: 2026-11-16
Status: Accepted (implementation candidate; reviewer ruling pending)
Related milestone/version: issue #105 T26 under parent issue #79
Related commits: T26-SERVER-DROP-EXECUTION-01 implementation commit (this change set)
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
CURRENT_USER()` bound on one dedicated new session — gated behind an
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
  that order on one dedicated `*sql.Conn`, preserves account and grant
  rows verbatim internally, and propagates query/scan/iteration
  errors bounded (`server drop privilege observation failed` with
  `Unwrap` preserved) — a failed read never degrades to
  unknown-but-successful evidence.
- Shared exact-token collector `spec.CollectMySQLGlobalPrivileges`:
  the T24 resource-group bundle and this loader parse `GRANT ... ON
  *.* ...` through one mechanism — whole tokens only, so substring
  lookalikes never bind.
- The `ServerDropPrivilegeWitness` requires all three domains:
  account bound, version in the supported domain, and exact global
  `SUPER` or `ALL PRIVILEGES`. `ALL PRIVILEGES` counts only under the
  MySQL 8.0.x series (5.7's `ALL` does not imply `SUPER`).
- Sentence-level scope projection: the request-level observation is
  immutable; an authored `GRANT`/`REVOKE` or the `set_role`/
  `grant_role`/`revoke_role`/`grant_proxy` boundary features end the
  applicable range, so later statements keep the privilege gap even
  under a positive request witness.
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
a dedicated session — it never claims the future executor, a pooled
session's effective privileges, or that a new execution session
exists. Deriving every consumer from one adjudication table is the
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
- Default (no mode) behavior is byte-identical to the pre-existing
  contract: notice + `unknown_server_state` gap with
  `server.existence`/`server.drop_privilege` owed — verified against
  the T26 golden manifest.

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
  must consume the same adjudication/arm vocabulary rather than
  recomputing premises.
- Any new privilege-affecting statement family must register as a
  scope terminator or the observed-account premise silently widens.
- `OrderedAbsent` is channel-internal; serializing it externally is a
  contract break.

## Links

- Commits: T26-SERVER-DROP-EXECUTION-01 candidate (this change)
- Tests: paths listed in Related tests
- Docs: `docs/verification/t26-server-drop-premise-01/`,
  `docs/verification/t26-semantics-01/`
