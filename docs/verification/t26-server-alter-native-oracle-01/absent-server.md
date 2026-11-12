# 80-1 — Absent SERVER + IF EXISTS grammar `[NAT]`

## Evidence

| case | version | account | statement | result |
|---|---|---|---|---|
| e1_abs | 8.0.46 | root | `alter server nao_e1miss options (USER 'probe')` | `ERROR 1477 (HY000)` "The foreign server name you are trying to reference does not exist. Data source error:  nao_e1miss" — wc 0, no catalog mutation (servers_total 26→26) |
| e1_abs | 8.4.10 | root | same | identical 1477 |
| s72_* | 5.7.44 | na_alt/na_altsel | same shape | identical 1477 |
| e1_ifx | 8.0.46 | root | `alter server if exists nao_e1if options (USER 'x')` | `ERROR 1064 (42000)` syntax error near 'if exists' |
| e1_ifx | 8.4.10 | root | same | identical 1064 |
| s7_ifexists | 5.7.44 | root | same | identical 1064 |

## Verdicts

- Native absent-object error is **1477 / HY000**
  (`ER_FOREIGN_SERVER_DOESNT_EXIST`), identical text on all three
  versions: "The foreign server name you are trying to reference
  does not exist. Data source error:  <name>". Note the doubled
  space before the name — the error echoes the (possibly
  truncated, see identity-collation.md) spelling verbatim.
- No warnings attached; no catalog mutation.
- **No IF EXISTS / IF NOT EXISTS form exists** for ALTER SERVER —
  1064 parse error on all three versions. The parser's refusal to
  claim a guard form is natively correct; DROP SERVER's IF EXISTS
  does not generalize.
- Combined with 80-7: privilege fails first, so a non-SUPER caller
  cannot even learn 1477 — the absence oracle requires privilege
  (or in-channel ordered-absent proof, which is what the product
  uses).
