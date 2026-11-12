# 80-9 — Merge vs replace: per-field verdict `[NAT]`

Seed `nao_merge` with seven sentinel option columns, then
`ALTER SERVER nao_merge OPTIONS (USER 'merge_new')` as root; compare
the full row before/after (mysql80/84-native/f9_mg.txt,
mysql57-native/s79_merge.txt).

| column | pre | post (8.0/8.4/5.7) | verdict |
|---|---|---|---|
| Server_name | nao_merge | nao_merge | unchanged |
| Host | m_host | m_host | **retained** |
| Db | m_db | m_db | **retained** |
| Username | m_user | merge_new | updated — targeted column |
| Password | m_pass | m_pass | **retained** |
| Port | 3361 | 3361 | **retained** |
| Socket | m_sock | m_sock | **retained** |
| Wrapper | mysql | mysql | unchanged |
| Owner | m_owner | m_owner | **retained** |

## Verdicts

- ALTER is a **true partial merge**: only the declared option
  columns are written; every undeclared column keeps its stored
  value. No NULL-clearing, no default rewriting, no name change,
  no row recreation (`servers_total` constant).
- This is the native basis of the accepted presence-preservation
  contract: the write set is option columns of an existing row —
  never row creation/deletion.
- Product boundary stays honest: DeltaScope reads only
  `server_name`, so it proves presence-persistence — it does NOT
  claim the option values landed, even though this probe shows
  they did in this controlled case. Sentinel-only columns were
  read; no real credential material entered evidence.
