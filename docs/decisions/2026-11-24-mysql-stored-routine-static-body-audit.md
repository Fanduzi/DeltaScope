# MySQL stored-routine static body audit (T27)

Status: accepted
Date: 2026-11-24
Issue: #106 (parent #79)

## Context

The milestone rule coverage counts only known rules. `CREATE PROCEDURE`
and `CREATE FUNCTION` bodies carry real DML (`DELETE`/`UPDATE`/`INSERT`)
that rule percentages never saw: the shared parser extracted only a
`has_body` marker, parameter lists recorded `create_procedure.option.params`
gaps, and `CREATE FUNCTION`, `ALTER PROCEDURE/FUNCTION`, `DROP FUNCTION`
were outright parser errors. Official MySQL client dumps also wrap routine
definitions in `DELIMITER` directives whose bodies contain `;` — the
statement splitter used to cut at every unquoted `;`, so a routine body
could never survive as one top-level statement.

## Decision

1. `statement_boundary.go` arms routine-body depth once a chunk's authored
   prefix proves a stored-routine CREATE (`CREATE … PROCEDURE|FUNCTION`
   outside parentheses). Block keywords (`begin`/`if`/`while`/`loop`/
   `repeat`/`case`) raise `routineDepth` and `end` lowers it;
   `endSuppressed` keeps an `END IF`-style closer from re-arming. Member
   `;` inside the body never splits the top-level chunk, and every
   completed statement resets all routine state so depth never leaks into
   a following routine. Client `DELIMITER <d>` directive lines are blanked
   in place — byte/line offsets preserved — and the active delimiter
   (`;`, `$$`, `//`, or any client spelling) terminates chunks. `$` is
   deliberately not a word byte so `END$$` scans as `END` + delimiter.
2. `mysql_routine.go` claims the whole lifecycle on MySQL: `CREATE
   [DEFINER=] PROCEDURE`, `CREATE [DEFINER=] FUNCTION ... RETURNS`,
   `ALTER PROCEDURE/FUNCTION` characteristic-only forms, `DROP
   PROCEDURE/FUNCTION`. It fabricates a `routineStatement` carrier whose
   `spec.RoutineDeclaration` records kind, qualified name, `Definer`
   shape, `ParamsPresent`, `ReturnsType`, modeled `Characteristics`, and
   the bounded `Body` (`Compound` flag + ordered members).
3. Body members are re-tokenized verbatim and re-enter `Parser.Parse` on
   their own slice; positions translate back into the parent statement's
   line/column grid. Each member lands as `spec.RoutineBodyStatement`
   embedding a full `spec.Statement` — DML rules (`dml.where.require` is
   the golden-path oracle) fire on member statements exactly as on
   top-level SQL, with findings rebound to the parent statement index.
4. Everything not statically bounded stays explicit evidence, never a
   guess: compound members (`routine.body.compound_member`), `DECLARE`
   (`routine.body.declaration`), `RETURN` (`routine.body.return`), member
   extraction failures (`routine.body.extract`), dynamic `EXECUTE`
   (`execute_prepared`), and parameter lists (`<op>.option.params`).
   `PREPARE`/`DEALLOCATE` stay exempt session statements — marking the
   declaration would be a false gap while the `EXECUTE` site already
   carries the dynamic-execution boundary.
5. Routine definitions are audit-only: the extractor never executes body
   SQL, never applies members to ordered schema state, and `DDL.Routine`
   is `json:"-"` so member SQL/positions never duplicate into serialized
   output or collide with the statement's own SQL identity.
6. TiDB keeps all six operations behind `boundary:vendor` unsupported
   evidence — a parser-accepted shape is still a vendor boundary, not a
   support claim, and bodies are never folded under TiDB.
7. Six lifecycle notices (`ddl.create/alter/drop_procedure.notice`,
   `ddl.create/alter/drop_function.notice`) ship enabled by default so
   routine operations stay visible even where bodies audit clean.
8. The golden runner's unconditional `unsupported.sql == raw_sql`
   identity check is relaxed exactly once: entries carrying
   `routine`/`routine_member` metadata may record the verbatim member
   slice, which must still appear inside the owning statement's raw SQL.
   Fabricated text and unmarked entries still fail the check.

## Public contract

- `unsupported[].metadata` gains `routine` / `routine_member` for member
  evidence; `feature` IDs listed in decision 4 are stable.
- Findings on body members carry `metadata.routine` /
  `metadata.routine_member` and a location inside the parent statement.
- `DELIMITER` directives are client framing: they never appear in
  `raw_sql`, `normalized_sql`, or any serialized field.
- New rule IDs default-enabled: `ddl.alter_procedure.notice`,
  `ddl.alter_function.notice`, `ddl.create_function.notice`,
  `ddl.drop_function.notice` (plus the existing create/drop procedure
  notices); all appear in `configs/deltascope.example.yaml`.
- Parser-upgrade candidacy no longer lists routine prefixes; remaining
  candidates are tablespace/resource-group forms.

## Deferred scope

DECLARE scoping, handler conditions, cursors, control-flow semantics,
call-graph effects, and SQL SECURITY evaluation stay unaudited — owner
T28 (`mysql.stored-body-analysis` inventory row keeps `mixed_coverage`
until that work lands). MySQL `CREATE FUNCTION` with a `BEGIN…END` body
audits members but `RETURN` stays an explicit boundary because return
semantics are not modeled.

## Verification

- `go test ./internal/application/audit -run T27` — body-blocker golden
  path, clean-WHERE control, string/comment semicolons, three delimiter
  spellings, nested-compound boundaries, state-reset regression, TiDB
  vendor boundary.
- `make ddl-golden TASK=T27` — 30 cases / 338 assertions on real
  `mysql:5.7.44`, `mysql:8.0.46`, `mysql:8.4.10`, `pingcap/tidb:v8.5.0`
  anchors; metadata cases prove native `CREATE PROCEDURE` acceptance on
  each MySQL anchor and that the audit itself creates nothing
  (`post_verify` ROUTINES count 0 before the driver executes one).
- `python3 scripts/test_ddl_golden.py` — 869 contract cases including
  the new routine-member sql-identity positives/negatives.
- Inventory rows re-derived: `mysql.create-procedure` /
  `mysql.create-function` → `mixed_coverage`, `mysql.alter/drop-
  procedure-function` → `generic_notice` (was `parser_unsupported`),
  `mysql.stored-body-analysis` → `mixed_coverage`, TiDB rows stay
  `vendor_not_supported` with T27 evidence attached.
