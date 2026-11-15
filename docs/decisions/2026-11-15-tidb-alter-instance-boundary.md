# TiDB ALTER INSTANCE boundary identity and promotion premise (T32)

Date: 2026-11-15
Issue: #111 (milestone #79)
Status: decided

## Decision

Every `ALTER INSTANCE` spelling under the `tidb` dialect lands an
explicit class — never a silent pass counted as coverage:

- `ALTER INSTANCE RELOAD TLS` and `ALTER INSTANCE RELOAD TLS NO
  ROLLBACK ON ERROR` are real TiDB grammar (proven natively: the T32
  manifest's tidb85 anchor executes RELOAD TLS with rc 0) held at the
  named `alter_instance.reload_tls` unaudited boundary. The boundary
  entry now preserves the action kind (feature id), the instance
  target identity (`metadata.object: "instance"`), the recorded
  `no_rollback_on_error` member, and the owed promotion premise
  `metadata.gap_fact: "alter_instance.version_applicability"`.
- `ALTER INSTANCE ROTATE INNODB MASTER KEY` is MySQL-only grammar and
  stays a `boundary: vendor` entry under the tidb dialect.
- Every other spelling — truncated, extra tokens, or a fabricated
  instance target such as `ALTER INSTANCE 'tidb-1:4000' RELOAD TLS`
  (the vendored TiDB grammar accepts no target operand at all) — stays
  a parser error, not a boundary.

`spec.AlterInstanceFactVersionApplicability` names the owed premise a
future TiDB audit slice must prove before support may be claimed; no
version gate is installed today because no TiDB instance-management
slice exists — the marker records what promotion requires, it does not
pretend the gate already ran.

## Rationale

The T26 MySQL slice's boundary entries carry `object`/`gate` identity
plus named gap facts; the TiDB boundary was a bare `{boundary:
unaudited}` blob that lost the action kind's target and left the
promotion precondition unwritten. Aligning the TiDB metadata shape
keeps the classification honest across dialects without inventing a
TiDB audited domain that does not exist.

## Public contract

- `alter_instance.reload_tls` under `dialect: tidb` emits
  `boundary: unaudited`, `object: instance`,
  `gap_fact: alter_instance.version_applicability`, and
  `no_rollback_on_error: true` only when authored.
- `alter_instance.rotate_innodb_master_key` under `dialect: tidb`
  emits `boundary: vendor`.
- Malformed or fabricated-target spellings remain `parser_error`
  diagnostics.

## Verification

- `make ddl-golden TASK=T32` — 15/15 cases, 133 assertions, including
  the tidb85 native RELOAD TLS execute (rc 0).
- `go test ./internal/application/audit -run TestT32` — boundary
  identity, vendor boundary, and parser-negative tests pass.
- `make ddl-inventory-gate` — `tidb.alter-instance` row evidence,
  status_evidence, and acceptance refs all file-bound.
- `python3 scripts/test_ddl_golden.py` — contract suite green with
  T32 in the canonical 31-task set.

## Update (T59 / #143)

The `alter_instance.version_applicability` promotion premise is
discharged as a pinned boundary: T32's live tidb85 metadata case
(native `ALTER INSTANCE RELOAD TLS` rc=0 plus the unaudited boundary
carrying the action identity and gap fact) is now attached to every
subaction cell of `tidb.alter-instance`. The premise is deferred —
TiDB documents no audited-applicability version for ALTER INSTANCE —
so `explicit_unsupported` stays the honest terminal state until such a
premise exists.
