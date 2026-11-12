# Proposed contract — A1–A12 re-adjudication with native evidence

Labels: `[DOC]` manual, `[SRC]` source-verified, `[NAT]` this round's
probes, `[PEND]` still unproven.

| clause | adjudication | evidence |
|---|---|---|
| A1 syntax/version | `ALTER SERVER name OPTIONS(members)` accepted on 8.0.46, 8.4.10, 5.7.44 `[NAT]`; no IF EXISTS `[NAT 1064]`; product domain 8.0.x remains the registered audited domain — 8.4/5.7 native success does NOT widen it | FROZEN + NAT |
| A2 OPTIONS vocab | seven keys all alterable `[NAT]`; unknown key → native 1064 `[NAT]` (recognizer's unclaim→base-diagnostic matches) | FROZEN + NAT |
| A3 dup/unknown | dup accepted last-wins silently `[NAT]` → owed `server.alter_options` correct (unclean = unproven, not refused); unknown key → 1064 `[NAT]` | FROZEN + NAT |
| A4 NULL/empty | `''` accepted and stored as `''` (clears column) `[NAT]`; `NULL` not a literal → unclaimed `[SRC]` | FROZEN + NAT |
| A5 absent object | 1477/HY000 ER_FOREIGN_SERVER_DOESNT_EXIST `[NAT]`; catalog zero-row still never proves absence (80-11 proves the write path sees rows the observer can't) | FROZEN + NAT |
| A6 privilege/order | SUPER required `[NAT 1227]`; **privilege check precedes existence check** `[NAT]`; session-role-state-bound `[NAT]`; shared observed-account premise remains the right conditional | FROZEN + NAT |
| A7 identity/collation | `Server_name char(64)` ci collation (utf8mb3_general_ci 8.x, utf8_general_ci 5.7) `[NAT]`; case variants hit same row, canonical spelling preserved `[NAT]`; 65-char CREATE truncates, 65-char ALTER → 1477 `[NAT]` | FROZEN + NAT |
| A8 post-state | **merge-update proven `[NAT]`**: single-field ALTER touches only its column. Product still reads only `server_name` — proves presence-persistence, deliberately not attribute truth | FROZEN + NAT |
| A9 observability | one bounded read; zero-row ≠ absent now has affirmative native support (write path is visibility-independent) | FROZEN + NAT |
| A10 batch invalidation | presence-preserving apply unchanged; no native result contradicts the arm structure | FROZEN |
| A11 privacy | option literals never stored/echoed `[SRC]`; probes used sentinels; full-row reads confined to root truth | FROZEN + NAT |
| A12 TiDB/FEDERATED | unchanged; no widening | FROZEN |

## Contract deltas proposed

**NONE.** Every native answer landed inside the accepted R5
semantics — the frozen contract did not need loosening anywhere,
and the previously-deferred `[PEND]` items now carry `[NAT]`
evidence consistent with it. Two clauses deserve explicit notation:

- A6 ordering: privilege-before-existence is now `[NAT]` — the
  adjudication's independent owed-facts remain correct because a
  non-privileged caller cannot reach the existence oracle anyway.
- A2/A3 nuance: "documented bound" ≠ "enforced bound" (PORT 10000
  stores; overflow clamps silently). The product's dirt→unproven
  mapping already encodes this honestly — a documented-contract
  violation that still lands natively is exactly "unproven", not
  "failed".

## R5 conflict check (work order M)

No conflict with accepted presence-preservation semantics: native
merge-update is precisely the theorem's foundation — option columns
written, row never created/deleted. No stop condition triggered.
