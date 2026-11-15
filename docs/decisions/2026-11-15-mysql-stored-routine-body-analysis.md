# MySQL stored-routine body control flow, bindings, and dynamic-SQL boundary (T28)

Status: accepted
Date: 2026-11-15
Issue: #107 (parent #79, builds on #106/T27)

## Context

T27 graduated routine body members into the shared statement machinery
but modeled only flat members: `IF`/`CASE`/`WHILE`/`LOOP`/`REPEAT`/nested
`BEGIN` compounds, every `DECLARE`, `SET`, and `PREPARE` arrived as opaque
bounded members — or, worse, `SET`/`PREPARE` were silently dropped — so
static risks inside branches and loops were invisible, and no analysis
distinguished a statically provable prepared text from a dynamic one.

## Decision

1. `mysql_routine.go` replaces the flat member split with a recursive
   classifier (`classifyRoutineMember`). `IF`/`CASE` become branch-bearing
   `RoutineCompound`s (`Branches` in source order, `HasElse` recorded);
   `WHILE`/`LOOP`/`REPEAT`/nested `BEGIN` become body-bearing compounds
   (labels accepted); `DECLARE` models variable/condition/handler/cursor —
   handler bodies and `CURSOR FOR` SELECTs stay executable members;
   `SET` records `RoutineAssignment`s with literal proof only — a SET
   with no variable assignments (GLOBAL/NAMES/@@-only) classifies as an
   ordinary statement instead, and partially unmodeled targets surface
   `routine.body.set`; `PREPARE` records literal/`@var`/expression
   sources with the source expression's authored offset; flow verbs
   (`leave`/`iterate`/`signal`/`resignal`/`open`/`fetch`/`close`/`get`)
   stay bounded `routine.body.flow` evidence — modeled, never silently
   consumed. Parameter lists now carry direction/name/type — the
   `create_procedure.option.params`/`create_function.option.params`
   aspects are retired. Construct keywords count only in
   member-initial position (after `;`, a label prefix, or a
   `then`/`else`/`do` connective): mid-member `IF()` calls, `DROP TABLE
   IF EXISTS`, and `REPEAT()` functions never open constructs, and a
   non-initial `case` pairs its own `end` as a case-expression — this
   member-initial discipline runs in both the token-level
   `routineMemberScanner` and the byte-level `statement_boundary.go`
   routine-depth tracker. Analysis resources are bounded:
   `routineBodyMaxDepth` (8) and `routineBodyMaxMembers` (512); overflow
   members and still-unparseable members emit explicit
   `routine.body.depth`/`routine.body.size`/`routine.body.parse`/
   `routine.body.declaration`/`routine.body.return` evidence — never a
   silent pass, panic, or timeout-based skip.
2. `evaluate.go` folds the body through `routineBodyWalker`, an
   env-aware recursive walk. A binding maps a name to a proven literal
   pointer (`nil` = bound-but-dynamic, absent = never bound); parameters
   start dynamic because they are caller-supplied. `IF`/`CASE` evaluate
   every arm on a copied env and merge by unanimity — a fact survives
   only when every possible path agrees, and a missing `ELSE` counts the
   pre-branch env as one path. Loop bodies evaluate once (their findings
   are real may-execute risks) but any binding the body could change
   degrades to dynamic — no execution count is fabricated. Handler and
   cursor members evaluate in an isolated env so conditional execution
   never contaminates the sequential path. `PREPARE` audits only
   statically proven literal text — parsed through the shared
   `Parse`/`Extract`/`EvaluateStatementDetailed` pipeline with findings
   re-anchored at the source expression's authored position (`prepare`
   metadata); dynamic or unresolvable sources and unparseable literals
   stay `routine.body.prepare` bounded evidence with a `source`
   discriminator (`var`/`expr`/`literal_parse`). `EXECUTE` keeps the
   `execute_prepared` dynamic boundary — prepared statements are never
   executed or inferred from runtime values. Every bounded executable
   member — `EXECUTE`, `CALL`, an unparseable or unmodeled member, and
   the rebinding/control-redirect flow verbs — invalidates all proven
   literal bindings afterwards: a member whose effects the tracker
   cannot see may have rebound any variable, so nothing downstream
   audits a stale literal.
3. New rule `ddl.routine.variable_shadowing.warn` (warning) reports a
   local `DECLARE`d variable reusing a parameter name — reads bind to
   the local after declaration.
4. The golden runner gains a `via: stdin` execute-step mode: the mysql
   client splits `-e` text on `;` and never honors `DELIMITER` there, so
   native compound-body fixture creation now rides stdin batch mode
   (DELIMITER works) with the batch recorded verbatim in evidence.

## Public contract

- Unsupported-entry `sql` for routine members is the verbatim member
  slice — never resolved variable content. A runtime marker
  (`T28_RUNTIME_SECRET`) never appears in serialized output, and the
  artifact credential scan already covers the ephemeral run secret.
- Verdict/coverage semantics unchanged: bounded members lower coverage
  to `incomplete`, warnings to `review`, blockers to `reject`;
  `--fail-on none` still exits non-zero on review.
- T27 manifest expectations evolved in place (compound members and
  parameters now audited): `mysql-nested-compound` now rejects with two
  member blockers, `mysql-params-gap` renamed `mysql-params-modeled`
  (pass/complete), `mysql-declare-gap` uses a malformed `DECLARE x`
  member to keep the `routine.body.declaration` boundary pinned.

## Deferred scope

- No SQL interpreter: condition truth values, `OPEN/FETCH/CLOSE` cursor
  effects, `RESIGNAL`/`SIGNAL`, and cross-routine `CALL` dependencies
  stay bounded (`routine.body.call`) rather than simulated.
- `RETURN` members keep the `routine.body.return` boundary (function
  return expressions are not evaluated).
- The ast-fallback `params`/`has_body` unextracted-option paths remain
  as defensive markers for extraction fallbacks the recognizer declines.

## Verification

- `make ddl-golden TASK=T28 ARTIFACT_DIR=/tmp/ddl-golden`: 29 cases /
  350 assertions PASS — golden-path dynamic body review+incomplete with
  `routine.body.prepare`+`execute_prepared`, static safe pass, static
  dangerous reject, branch/loop conservative merges, literal-PREPARE
  audited blocker, shadowing warn, depth/size bounds, parser-negative
  no-leak, TiDB vendor boundary, plus per-anchor native compound-body
  creation receipts on mysql 5.7.44/8.0.46/8.4.10 via stdin batch.
- `make ddl-golden TASK=T27 ARTIFACT_DIR=/tmp/ddl-golden`: 30 cases /
  336 assertions PASS under the evolved contract.
- `go test ./...`: 39 packages ok / 0 FAIL.
- `python3 scripts/test_ddl_golden.py`: 869/869 contract cases PASS.
- `make ddl-inventory-gate`: PASS — `mysql.stored-body-analysis` row +
  create-procedure/create-function cells cite `gate:T28`,
  `routine_t28_test.go`, `T28.json`, and the `routine_body_analysis`
  corpus fixture.
