# 80-11 — Grant-scoped visibility vs write-path eligibility `[NAT]`

Question: can an account holding the ALTER-required privilege but
**without** `SELECT` on `mysql.servers` still ALTER a SERVER?

## Evidence (all three versions)

| case | account | grants | target | account-view read | ALTER result |
|---|---|---|---|---|---|
| c11_a1 8.0 | na_alt | `SUPER ON *.*` only | nao_a1 (exists) | `ERROR 1142` denied — **row invisible** | **succeeded**, wc=0 |
| c11_a2 8.0 | na_alt | SUPER only | nao_miss (absent) | `ERROR 1142` | `ERROR 1477 (HY000)` absent |
| c11_b1 8.0 | na_altsel | SUPER + `SELECT ON mysql.servers` | nao_b1 | row visible | succeeded |
| c11_b2 8.0 | na_altsel | SUPER + SELECT | absent | — | 1477 |
| c11_d1 8.0 | na_roleuser | USAGE + role na_fedrole(SUPER) **SET ROLE NONE** | nao_d1 | 1142 | `ERROR 1227` SUPER needed |
| c11_d2 8.0 | na_roleuser | same account **SET ROLE na_fedrole** | nao_d2 | 1142 | succeeded |
| c11_e1/e2 | root | full | exists / absent | — | success / 1477 |
| 8.4.10 | identical matrix | — | — | identical | identical |
| s71_* 5.7 | na_alt / na_altsel | SUPER ± SELECT | exists | 1142 / visible | both succeed |
| s72_* 5.7 | both | SUPER ± SELECT | absent | — | 1477 both |

mysql80-native/c11_a1.txt, c11_a2.txt, c11_b1.txt, c11_d1.txt,
c11_d2.txt; mysql84-native same ids; mysql57-native/s71_*, s72_*.

## Verdicts

1. **SUPER alone suffices** — `mysql.servers` SELECT is not part of
   the write path. An account that cannot observe the object at all
   can still mutate it (`alt_sent_1` verified in the root post-state).
2. **Existence checking is visibility-independent** — the
   catalog-blind SUPER account receives the TRUE absent error 1477,
   not a visibility-masked variant. The write path reads the real
   catalog regardless of the caller's SELECT domain.
3. **Visibility ≠ eligibility, in both directions**: a denied read
   does not imply ALTER fails; a successful ALTER does not imply the
   executor can observe the row afterward (na_alt's post-read still
   gets 1142 while the row now holds its own write).
4. **Privilege state is session-role-bound** — same account, same
   grants, `SET ROLE NONE` → 1227 vs `SET ROLE na_fedrole` → success.
   `CURRENT_ROLE()` observed NONE vs `na_fedrole@%` respectively.
5. **Contrast with SRS dependencies** — SRS DROP enforcement was
   visibility-scoped (invisible deps did not block). SERVER ALTER's
   existence check is NOT visibility-scoped. Different subsystems,
   different rules — the work order's warning was correct.

## Product-path consequence

The provider's bounded existence read (`select server_name … where
server_name = ?`) executes under the OBSERVER account. This evidence
proves the product's conservative stance is not merely safe but
*correct-necessary*: a zero-row/denied observer read cannot prove
absence because the write path provably operates on rows invisible
to that same read domain. `OrderedAbsent` refusals remain sound
(they rest on proven in-channel absence, not catalog reads).
