# MySQL trigger definition audit and OLD/NEW binding (T29)

Status: accepted
Date: 2026-11-24
Issue: #108 (parent #79, builds on #106/T27 and #107/T28)

## Context

The locked TiDB grammar cannot parse `CREATE|DROP TRIGGER`, so every
trigger form degraded to a parser error and trigger bodies — real DML
with static risks — were invisible to policy. Triggers also carry
semantics routines lack: a bound target table, an event, timing, an
optional ordering clause, and `OLD`/`NEW` pseudo-columns whose legality
depends on the event.

## Decision

1. `mysql_trigger.go` claims `CREATE [DEFINER=...] TRIGGER name
   BEFORE|AFTER INSERT|UPDATE|DELETE ON table FOR EACH ROW
   [FOLLOWS|PRECEDES name] body` and `DROP TRIGGER [IF EXISTS] name`
   whole-input, mirroring the routine recognizer. The FOR EACH ROW
   body — single statement or BEGIN…END compound — reuses the T28
   member machinery unchanged (`splitRoutineBodyMembers`, compound
   classification, resource bounds); member evidence retags into the
   `trigger.body.*` namespace so features name the owning object
   family. TiDB keeps the vendor boundary on both operations.
2. `TriggerDeclaration` models timing, event, bound target, DEFINER
   shape, `FOLLOWS`/`PRECEDES` order, and `UsesOld`/`UsesNew` lexical
   facts: only the qualified `OLD.<col>`/`NEW.<col>` shape counts —
   bare names stay ordinary identifiers. `OldNewValid()` binds legality
   to the declared event: INSERT reads NEW only, DELETE reads OLD only,
   UPDATE reads both. Illegal use surfaces `trigger.body.old_new`
   bounded evidence; legal use never fabricates a missing-table
   reference.
3. The audit walker evaluates trigger bodies under a `trigger` scope —
   identical semantics to routine bodies (no execution, no prospective
   state mutation, bounded-member env invalidation) — minting
   `trigger.body.*` features and `trigger`/`trigger_member` metadata.
   The byte-level statement splitter treats `trigger` like the routine
   heads so member semicolons never split the CREATE.
4. Lifecycle notices `ddl.create_trigger.notice` and
   `ddl.drop_trigger.notice` follow the routine convention; both
   operations join the ordered-state inert set (a trigger definition
   never mutates table state at audit time).

## Public contract

- `create_trigger`/`drop_trigger` are normalized `spec.DDLOperation`
   values under both MySQL-family dialects; TiDB emits the vendor
   boundary with `boundary: vendor` metadata.
- Body-member unsupported entries keep the verbatim member slice;
   `trigger.body.*` features share the `routine.body.*` vocabulary
   with the object family renamed.
- Trigger definitions are inert for ordered schema state — creating
   one never implies executing its body.

## Deferred scope

- OLD/NEW resolution binds to the trigger's *event semantics*, not to
  target-column metadata — column existence and type checks need
  catalog metadata and stay downstream.
- `FOR EACH ROW` is required by the MySQL grammar; non-row forms stay
  unclaimed parser errors.
- No runtime semantics: the audit never triggers INSERT/UPDATE/DELETE
  and never infers WHEN-clause truth (there is no WHEN clause in
  MySQL's row-level trigger grammar).

## Verification

- `go test ./internal/application/audit -run TestT29`: 8/8 PASS —
  body DELETE blocker with `trigger` metadata, compound-member audit,
  legal/illegal OLD/NEW legs, lifecycle notice, FOLLOWS body audit,
  and both TiDB vendor-boundary legs.
- `go test ./...`: all packages ok after contract evolution
  (parser-error census drops the trigger family, cross-dialect census
  moves both forms to finding-covered/vendor-boundary, catalog and
  corpus fixtures regenerated).
- Corpus fixture `trigger_body_analysis` pins two notices, one body
  blocker, and the `trigger.body.old_new` boundary.
- Golden manifest, native anchor fixtures, and inventory evidence
  land in the T29 close-out (gates pending at commit time).
