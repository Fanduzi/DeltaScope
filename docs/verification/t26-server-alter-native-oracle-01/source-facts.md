# Source facts — evidence ↔ product path linkage `[SRC]`+`[NAT]`

Work order: no parser/source re-audit required; this file binds the
native evidence to the already-audited path (see the accepted
contract round's source-facts).

| product element | native evidence |
|---|---|
| recognizer claims `ALTER SERVER name OPTIONS(members)`, whole-input | every probe used exactly that shape; accepted on all 3 versions |
| no guard form (`IF EXISTS`) claimed | 80-1: `if exists` → native 1064 — parser boundary correct |
| 7-key OPTIONS vocabulary | all keys natively alterable; unknown keys/empty list stay unclaimed → native 1064 |
| `DuplicateKeys` → owed `server.alter_options` | 80-2: native accepts silently last-wins — owed-fact (unproven) is the honest verdict |
| `RangeState=out-of-range` >9999 | 80-4: PORT 10000 **stores** — documented bound not enforced; unproven (not refusal) correct |
| overflow digits unclaimed | 80-4: 2^64 silently clamps to 0 — parser stricter than native, safe direction |
| char members require quoted literal; `''` admitted | 80-3: `''` stores — real write, not NULL |
| `NameIdent` ident vs string-literal | utf8mb3_general_ci PK matching; string-literal decode variance unchanged |
| `NameSpellingRuneCount <= 64` boundary | 80-6: 64 works; 65 authored → 1477 — boundary excludes an impossible case |
| `server.alter_privilege` SUPER premise | 80-7: 1227 without SUPER — privilege truly required, checked first; role-state bound (80-11 d1/d2) |
| bounded existence read `select server_name…` | 80-11: write path uses TRUE catalog; observer visibility orthogonal — zero-row/denied ≠ absent, confirmed necessary |
| `OrderedAbsent` → terminal refusal | 80-1: absent ALTER natively fails 1477 — product refusal aligns |
| `PublishPresent` presence-preservation only | 80-9: native merge-update writes option cols only — product's bounded claim (presence persists, attributes unclaimed) is exact |
| adjudication A1–A4 arms | no native result contradicts any arm; all `[PEND]`s from the contract round now have NAT evidence |

No product file was touched this round (work order A/M).
