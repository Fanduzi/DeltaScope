# Unresolved items + stop-condition assessment

## Resolved this round (contract round [PEND] → [NAT])

80-1 absent error (1477/HY000, 3 versions), 80-2 dup last-wins
silent, 80-3 empty-string stored `''`, 80-4 PORT bounds (10000
stores; `-1`/`'3306'` → 1064; 2^64 → silent clamp to 0), 80-5 ci
collation + case-fold, 80-6 64/65-char boundary (CREATE truncates,
ALTER→1477), 80-7 privilege-before-existence (all 3 versions),
80-9 true merge-update, 80-11 write-path visibility-independent —
the decisive divergence from SRS dependency semantics,
57-1..5 supplement (all consistent; roles absent → n/a on 5.7).

## Still open

1. **80-6 lookup key shape** — the 65-char ALTER error truncates the
   displayed name to 64 but fails; whether the lookup used the
   authored 65-char spelling or a truncated key that missed for a
   different reason is not fully distinguished. Immaterial: the
   product boundary already excludes >64-rune names.
2. **Binlog replication surface** — ALTER SERVER is documented as
   not written to the binlog; not probed (no replica rig). The
   product never modeled it; no contract impact.
3. **FEDERATED engine availability** — these containers may lack
   the FEDERATED storage engine; `mysql.servers` admin ops worked
   regardless. Whether FEDERATED presence changes ALTER outcomes
   is unprobed — out of declared scope (no real FEDERATED links
   authorized).
4. **`partial_revokes` observer path** — enabled observers' zero-row
   under partial revokes remains a possibility on real installs;
   the container defaults had `partial_revokes=0`. The zero-row ≠
   absent rule already covers it.
5. **charset/SQL-mode string-literal names** — unchanged from the
   contract round; `NameIdent=false` boundary stands.

## Stop conditions (work order M) — none triggered

- native vs R5 conflict: none — merge-update confirms the theorem
- parser grammar change: not needed (recognizer strictness is safe
  — the only divergence is parser *rejecting* forms native accepts,
  e.g. overflow PORT, which is a coverage choice not a false claim)
- generic lifecycle / privilege contract / FEDERATED/TiDB /
  CREATE-DROP-OrderedAbsence / T25 / SRS / presence-preservation
  implementation: all untouched

## Environment teardown

Disposable containers `t26-ai-*`/`t26-go-*`/`t26-*-fwd*` remain for
the reviewer's possible re-verification; all probe state is confined
to `nao_*`/`MixedCase` server rows and `na_*` accounts — no real
credential or host material exists anywhere in evidence.
