# 80-7 — Privilege vs existence check ordering `[NAT]`

Discriminating design: compare the error on an **existing** vs
**missing** target under an account that lacks SUPER.

## Evidence

| case | account | can read catalog? | target | result |
|---|---|---|---|---|
| d7_c1 8.0 | na_usage (USAGE only) | no (1142) | nao_r1 exists | `ERROR 1227` SUPER needed |
| d7_c2 8.0 | na_usage | no | nao_missd absent | `ERROR 1227` — **not 1477** |
| d7_s1 8.0 | na_selonly (SELECT on servers, no SUPER) | yes | nao_r2 exists | `ERROR 1227` |
| d7_s2 8.0 | na_selonly | yes | nao_misss absent | `ERROR 1227` — **not 1477** |
| 8.4.10 | all four cells | — | — | identical |
| s75_* 5.7 | na_usage / na_selonly | no / yes | exists / absent | 1227 all four |

mysql80-native/d7_*.txt; mysql84-native/d7_*.txt;
mysql57-native/s75_*.txt.

## Verdict

**Privilege check precedes existence check** on all three versions:
the missing-object error 1477 never leaks through a privilege
failure — `na_selonly` can even see that the target row does not
exist yet still receives 1227, not 1477.

Ordering is therefore:

```
parse → privilege (SUPER; else 1227) → existence (else 1477) → apply
```

This matches the product's adjudication shape — `server.alter_privilege`
is an independent owed-fact; a proven-absent refusal never needs to
infer the privilege path, and an unprivileged attempt can never
manufacture an absence oracle.
