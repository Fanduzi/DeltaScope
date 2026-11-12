# 80-2 / 80-3 / 80-4 — OPTIONS edge semantics `[NAT]`

## G1 duplicate keys (g2_dup, all versions)

`OPTIONS (USER 'first_val', USER 'second_val')` → **accepted**,
`warning_count 0`, stored `second_val` — **last-wins, silently**.
8.4 and 5.7 identical (`second_val`/`second_57`).

Product mapping: the recognizer records `DuplicateKeys` → owed
`server.alter_options` → unproven arm. Native behavior (silent
last-wins) means dirt does NOT mean failed — the owed-fact
treatment (unclean → unproven, never refusal) is exactly right:
the ALTER landed, but which author intent won is genuinely
unclear, so "unproven" is the honest verdict.

## G2 empty strings (g3_u/g3_h/g3_p)

| statement | result | stored |
|---|---|---|
| `USER ''` | accepted, wc 0 | Username `''` (was g3_seed) |
| `HOST ''` | accepted, wc 0 | Host `''` (was g3_h) |
| `PASSWORD ''` | accepted, wc 0 | Password `''` (was g3_old — **cleared**) |

Empty string is a real write — it overwrites to `''`, not NULL.
Columns are `NOT NULL DEFAULT ''`; `''` is a legitimate stored
value, not an absence marker. 8.4/5.7 identical.

## G3 PORT boundaries (g4_*)

| input | 8.0.46 | 8.4.10 | 5.7.44 |
|---|---|---|---|
| `PORT 0` | stored 0, wc 0 | same | — |
| `PORT 1` | stored 1 | same | — |
| `PORT 9999` | stored 9999 | same | — |
| `PORT 10000` | **stored 10000** | same | stored 10000 |
| `PORT -1` | `ERROR 1064` syntax | same | — |
| `PORT '3306'` | `ERROR 1064` syntax | same | — |
| `PORT 18446744073709551616` (2^64) | **accepted, stored 0**, wc 0 | same | — |

- The documented 0–9999 bound is **not enforced** — 10000 stores.
  The parser's `RangeState=out-of-range` (>9999) marks documented-
  contract dirt; the ALTER still lands natively → owed-fact
  (unproven) is correct, refusal would be wrong.
- Unsigned grammar is real: `-1` and quoted `'3306'` → 1064,
  matching the parser's digit-literal rule.
- **Overflow silently clamps**: 2^64 → stored `0`, no warning.
  The recognizer refuses overflow digits (unclaims → base
  diagnostics) while native accepts-and-clamps — the parser is
  *stricter*, a safe direction (no false clean claim).
- `Port` column is `int` (5.7 shows `int(4)`).
