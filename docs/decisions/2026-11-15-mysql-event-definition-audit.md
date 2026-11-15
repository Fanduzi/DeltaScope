# MySQL event definition, schedule, and static body audit (T30)

Date: 2026-11-15
Issue: #109 (milestone #79)
Status: decided

## Decision

The MySQL event family is claimed by a strict whole-input recognizer
(`internal/infrastructure/parser/tidb/mysql_event.go`) that fabricates a
typed `eventStatement` carrying `spec.EventDeclaration`:

- `CREATE [DEFINER=...] EVENT [IF NOT EXISTS] <name> ON SCHEDULE
  <AT <expr>|EVERY <interval>> [STARTS <expr>] [ENDS <expr>]
  [ON COMPLETION [NOT] PRESERVE] [ENABLE|DISABLE|DISABLE ON SLAVE]
  [COMMENT 'x'] DO <body>`
- `ALTER [DEFINER=...] EVENT <name>` with any authored subset of those
  clauses plus `RENAME TO` — a bare `ALTER EVENT <name>` with no clause
  is not real syntax and is never claimed
- `DROP EVENT [IF EXISTS] <name>`

Clause authorship is strict: clauses must appear in grammar order
(schedule → completion → rename → status → comment → body), each at
most once, `RENAME TO` is ALTER-only, `DEFINER` requires a payload, and
`COMMENT` requires a string — out-of-order, duplicated, or payload-free
forms decline to the base parser rather than fabricating a declaration.
`ALTER EVENT ... DO <body>` with no other clause claims a body
replacement; a bare `ALTER EVENT <name>` with no clause claims nothing.

`spec.DDLOperation` gains `create_event`, `alter_event`, `drop_event`,
distinct from the existing PostgreSQL `*_event_trigger` operations.
`DDL.Event` holds the declaration (`json:"-"`) so clock expressions,
body text, and account data never serialize.

### Schedule and completion attributes are authored facts

`EventSchedule` captures the verbatim authored text of `AT`, `EVERY`,
`STARTS`, and `ENDS`, plus its kind (`at`/`every`) and `Repeating()`.
`OnCompletion` distinguishes authored `PRESERVE`/`NOT PRESERVE` from an
absent clause. The audit models schedule shape; it never evaluates
clock values, waits for the scheduler, or predicts runtime outcomes.

### Enabled-state semantics are conservative

`EventDeclaration.EffectiveEnabled()` encodes: CREATE defaults to
enabled; ALTER changes state only when authored (`StatusSet`);
`DISABLE ON SLAVE` still executes on the primary — only a full
`DISABLE` stops the schedule.

`ddl.event.enabled_execution.warn` fires whenever a statement provably
leaves an event scheduled to execute. An `ALTER EVENT` that authors no
status clause cannot prove the enabled premise: instead of guessing, it
owes the `missing_event_state` evidence gap requiring
`event.enabled_state`, lowering coverage to `unverified` — never a
fabricated warning.

### DO bodies reuse the shared machinery without executing

Single-statement and `BEGIN…END` bodies flow through the T28 routine
member machinery. Member-scoped bounded features retag into the
`event.body.*` namespace and findings carry `event`/`event_member`
metadata at positions translated back into the original SQL. A
`DISABLED` event still audits its body — disable controls scheduling,
never analysis. Defining or enabling an event is always inert: the
audit never executes user SQL, never starts the scheduler, and event
operations stay in the prospective-state inert whitelist.

`DEFINER` keeps only its clause shape (`current_user`/`named`); the
account payload is never stored, serialized, or echoed into evidence.

### TiDB stays an explicit vendor boundary

TiDB does not implement events. All three forms claim the shape, then
emit `vendor_boundary` unsupported entries (`boundary: vendor`,
`object: event`); bodies are never audited for TiDB.

## Rationale

Rule coverage percentages previously hid that valid event syntax never
reached analysis. Modeling declarations plus the shared body walker
distinguishes policy violations (body findings), evidence gaps
(`missing_event_state`), unsupported analysis (`event.body.*` member
bounds), parser failures (malformed/truncated forms stay parser
errors), and successful audits — while scheduled-execution risk is a
first-class warning, not a runtime prediction.

## Public contract

- New rules: `ddl.create_event.notice`, `ddl.alter_event.notice`,
  `ddl.drop_event.notice`, `ddl.event.enabled_execution.warn`
  (registered for MySQL only, enabled by default).
- New evidence-gap reason `missing_event_state` requiring
  `event.enabled_state` on status-free `ALTER EVENT`.
- Body findings/bounds carry `event`/`event_member` metadata and
  `event.body.*` feature names; `routine.body.*` names never leak into
  event evidence.
- Coverage census: MySQL event forms move from parser-error to
  finding-covered (MySQL parser-error feasibility cases drop to 0);
  TiDB event forms move from parser-error to vendor boundary.

## Verification

- `make ddl-golden TASK=T30 ARTIFACT_DIR=/tmp/ddl-golden-t30` — 27
  cases / 278 assertions on mysql57/80/84 native receipts (audit
  creates no event; a native DISABLED compound-body event persists with
  `STATUS='DISABLED'`) plus tidb85 vendor boundaries.
- `internal/application/audit/event_t30_test.go` — 11 tests covering
  the golden path, safe WHERE bodies, enable/at/every warnings,
  DISABLE ON SLAVE, missing-state gaps, compound members, drop
  lifecycle, DEFINER shape, and TiDB boundaries.
- `go test ./...` green; `python3 scripts/test_ddl_golden.py` 871
  contract cases; `make ddl-inventory-gate` green with regenerated
  matrix.

## Links

- Parser: `internal/infrastructure/parser/tidb/mysql_event.go`
- Model: `internal/domain/spec/event.go`
- Rules: `internal/domain/rule/ddl/mysql_event_rules.go`,
  `internal/domain/rule/ddl/mysql_tidb_lifecycle_rules.go`
- Walker scope: `internal/application/audit/evaluate_routine_body.go`
- Golden manifest: `testdata/ddl-golden/T30.json`
- Corpus: `testdata/sql-corpus/mysql/ddl/findings/event_lifecycle_analysis.*`
