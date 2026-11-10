# T26-R2 output redaction contract (design only — no implementation)

> **R3 addendum (T26-R3, implementation stage).** Implemented as
> designed, with the concrete shape now frozen: `spec.Statement.PublicSQL()`
> / `PublicNormalizedSQL()` route every serialized SQL field through the
> declaration's fixed-shape renderer — `CREATE SERVER <name> FOREIGN DATA
> WRAPPER <wrapper> OPTIONS (<redacted>)`, `ALTER SERVER <name> OPTIONS
> (<redacted>)`, `DROP SERVER [IF EXISTS ]<name>`,
> `CREATE [OR REPLACE ]SPATIAL REFERENCE SYSTEM [IF NOT EXISTS ]<srid>
> [<attributes redacted>]`, `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS ]
> <srid>`. `evaluate.go` applies it at all four assembly sites
> (unsupported-item SQL, gap SQL, and both statement-result fields) —
> including the non-empty `item.SQL` path, which can never outrank the
> redaction for sensitive families. Diagnostics never embed SQL;
> findings carry no raw-SQL field; markdown/SARIF/gitlab renderers read
> the already-redacted result fields. Internal `RawSQL` stays verbatim.

Reviewer ruling applied: no echo exemption for `raw_sql`,
`normalized_sql`, or `unsupported[].sql` on T26 sensitive statements.
Design below separates **internal input** from **external
representation**; positions always map to the original input.

## 1. Internal input vs external representation

- **Internal**: `spec.Statement.RawSQL` remains the authentic input for
  semantic analysis — recognizers, position mapping, finding locations
  all consume the original. Never mutate input to achieve redaction
  (mutating first = auditing a different statement).
- **External**: a per-family flag on the declaration —
  `[proposed] spec.Statement.RedactedOutput bool` (or a kind-derived
  classifier) — marks statements whose serialized surfaces must carry
  the redacted rendering instead of the verbatim text.
- **Redacted rendering**: fixed-shape placeholder preserving statement
  boundary + kind, e.g. `CREATE SERVER <redacted>` for SERVER; for SRS
  where the header is non-sensitive, `[proposed]` render the statement
  with only the DEFINITION literal replaced:
  `CREATE SPATIAL REFERENCE SYSTEM 4120 NAME 'Greek' DEFINITION <redacted>`.
  The placeholder carries **no length or content hash** of the secret
  (length itself can leak).
- **Position integrity**: `Index`, `Line`, `Column` are computed on the
  original input at parse time (parse.go `statementChunk`) — positions
  continue to locate the original statement in the caller's input. The
  redacted field carries a documented notice: "positions refer to input
  offsets, not to this rendering."

## 2. Complete exit-point inventory (audited paths today)

| # | exit | today | proposed contract |
|---|---|---|---|
| E1 | `statements[].raw_sql` | verbatim (`report/result.go:88`, assembled `evaluate.go:41,98`) | redacted rendering when flagged; field stays `string`, may hold placeholder |
| E2 | `statements[].normalized_sql` | verbatim (`result.go:89`, `evaluate.go:42,99`) | same |
| E3 | `unsupported[].sql` | verbatim backfill `evaluate.go:33,74` | **must not rely on empty-`item.SQL`**: set `item.SQL` to the redacted rendering explicitly at the point the entry is created (the generic backfill keeps verbatim semantics for unflagged statements) — this is the ruling's named trap |
| E4 | `findings[].*` | rule text only today | findings must never interpolate option values/definition text — add contract note on affected rules |
| E5 | `diagnostics[]` | fixed strings + keyword codes only; **no sql field** (`statement.go:87-97` verified) | keep as-is — clean by construction |
| E6 | `evidence_gaps[].*` | `rule_id`,`reason_code`,`required_facts` only (T04 contract: never SQL text) | keep as-is |
| E7 | stdout/stderr | fixed reason lines only (probes confirm) | keep as-is |
| E8 | markdown renderer | echoes `NormalizedSQL`/`RawSQL` (`render.go:44-50`) | consumes the same redacted fields — no separate path |
| E9 | error wraps | `fmt.Errorf("parse sql: ...")` wraps parser messages — parser messages may include input fragments | **flagged**: parser error text is vendor-produced and may quote input around the failure point; contract = diagnostics keep fixed reason (already true), and the wrapped error is an internal `error` — verify it never reaches serialized output; stderr shows only the fixed reason line today |
| E10 | `unsupported[].metadata` | free-form map | rule: never place option values/definition text in metadata |

## 3. Field contract

| field | keep? | type | redacted value | consumer impact |
|---|---|---|---|---|
| `raw_sql` | keep | string | redacted rendering (placeholder-bearing) | consumers see placeholder — documented output-contract change for flagged families only |
| `normalized_sql` | keep | string | same | same |
| `unsupported[].sql` | keep | string | same | same |
| positions | unchanged | int | — | none |
| `statement.Kind` | unchanged | enum | — | none |

Generic-fallback safety: the flag lives on `spec.Statement`; every
assembly point (E1–E3) selects rendering by flag, so clearing a field
doesn't re-expose input — the *source* `RawSQL` is simply never chosen.
Minimal change surface = the flag + one render helper + the three
assembly sites + recognizers setting the flag.

## 4. Path-by-path behavior

| path | today | proposed |
|---|---|---|
| recognized SERVER/SRS | n/a (no recognizer yet) | flag set at declaration; all echo fields redacted |
| named boundary / unsupported | verbatim via E3 | redacted rendering set explicitly at entry creation |
| malformed SQL (parser_error) | no sql in diagnostic — clean | unchanged; stderr/error wraps internal only |
| mixed multi-statement | sibling `raw_sql` verbatim | only flagged siblings redacted; unflagged keep verbatim (no over-redaction) |
| no complete AST | diagnostic has no SQL (verified E5/E9) | unchanged; rule: error text never reaches serialized surfaces |

## 5. Test design (paired path+leak assertions per ruling 六.5)

Each case asserts **path first, then absence**:
1. statement reached expected branch (recognized / boundary / parser_error);
2. `Index/Line/Column/Kind` correct, `coverage` expected;
3. then: marker strings (`golden-secret`, marker host/user/db, marker
   DEFINITION payload) absent from **every serialized field** of the
   whole result JSON + stderr capture — grep-style scan over the artifact.
Parser-early-exit results count as their own class and never substitute
for success-path proof.

## 6. Change surface — T26-bounded

- New: `spec.Statement.RedactedOutput` (or equivalent), one render
  helper, flag set by new T26 recognizers, explicit SQL at three
  assembly sites for flagged statements.
- Compatibility: unflagged statements byte-identical output; flagged
  families didn't exist before → no consumer regression beyond the
  documented contract change.
- Other credential-carrying families (e.g. `ALTER USER ... IDENTIFIED
  BY`, `CREATE USER`): **out of T26 scope** — registered as a separate
  tracking item `DDL-CREDENTIAL-ECHO` with the same contract template,
  no project-wide refactor piggybacked.
