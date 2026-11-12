# 80-5 / 80-6 — Name identity, collation, length `[NAT]`

## Schema (x_schema_collation.txt, s73_schema_collation.txt)

- 8.0.46: `Server_name char(64) utf8mb3_general_ci` PK; `Host
  char(255) ascii_general_ci`; `Port int`.
- 8.4.10: same `char(64)` Server_name (ci family).
- 5.7.44: `char(64) utf8_general_ci`; `Port int(4)`.

## Case-fold (h5_l/h5_u/h5_m, s7_case)

`MixedCase` created; `alter server mixedcase`, `alter server
MIXEDCASE`, `alter server MixedCase` all succeed and hit the SAME
row — the stored canonical spelling `MixedCase` never changes
(Username cycles mc_lower→mc_upper→mc_exact/mcl_57 while
`Server_name` stays `MixedCase`). `WHERE Server_name='mixedcase'`
and `='MIXEDCASE'` both return `MixedCase` — collation-folded
matching on the PRIMARY KEY, not binary compare.

## Length boundary

- 64-char name: CREATE accepted, ALTER accepted, row addressable.
- 65-char CREATE (`nao_h65_...abcde`): **silently truncated** —
  catalog holds `nao_h65_...abcd` (64). No error, no warning shown
  in setup output. Confirmed on 8.0 and 8.4.
- 65-char ALTER on the same spelling: **`ERROR 1477`** — but the
  error's echoed name is itself truncated to 64
  (`Data source error:  nao_h65_...abcd`). The lookup fails on the
  authored 65-char spelling even though a 64-char prefix row
  exists — the display truncation does not mean the lookup used
  the truncated key; the authored spelling misses the row.

## Product consequence

- `NameSpellingRuneCount <= 64` audited-boundary is consistent:
  inside it, identity is provable (utf8mb3_general_ci folded PK);
  a 65+ spelling lands in 1477-territory anyway — the boundary
  excludes a case that could never succeed.
- Case-fold equivalence is collation-provable at catalog level for
  the observer's own reads — but the product's conservative
  same-identity relations (differently-cased spellings = unproven
  relation) remain valid, because the *authored* spelling is what
  the write path matches; nothing here forces a relaxation.
