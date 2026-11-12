# MySQL 8.0.46 behavior — evidence status

`[DOC]` = 8.0 manual (fetched in prior rounds); `[NAT]` = native
evidence previously captured in an accepted archive; `[PEND]` = no
native evidence — proposed probe only (work order C forbids new
probes this round).

## Established `[DOC]`

- Grammar: `ALTER SERVER server_name OPTIONS (option [, option] ...)`
  — present since 5.x; 8.0 page fetched for R5.
- Semantics: "adjusting any of the options permitted in CREATE
  SERVER … the corresponding fields in `mysql.servers` are updated
  accordingly" — **partial merge update**, not row replacement.
- Requires `SUPER`. Causes implicit commit; not written to binlog
  (documented-but-unmodeled: no txn/binlog surface in this model).
- `server_name` ≤64 chars, case-insensitive, may be quoted; longer
  spellings are documented as silently truncated.
- OPTIONS vocabulary = CREATE's seven keys; PORT numeric 0–9999.

## Native gaps `[PEND]` — proposed probes (NOT run)

| # | question | proposed probe |
|---|---|---|
| 80-1 | absent-server error class/code | `ALTER SERVER no_such OPTIONS (USER 'u')` → capture error code + sqlstate + rc |
| 80-2 | duplicate key semantics | `OPTIONS (USER 'a', USER 'b')` → error? last-wins? capture `mysql.servers` row after |
| 80-3 | empty-string values | `OPTIONS (USER '', PASSWORD '')` → accepted? stored value? |
| 80-4 | `PORT` bounds | `PORT 0`, `PORT 9999`, `PORT 10000`, `PORT -1`, `PORT '3306'` → which refuse/warn |
| 80-5 | name case-fold | `CREATE SERVER 'MixedCase'` then `ALTER SERVER mixedcase OPTIONS (...)` → which row changes; `Server_name` collation capture |
| 80-6 | >64-char name | authored 65+ → truncated identity vs error |
| 80-7 | privilege-vs-existence order | USAGE-only user `ALTER` on absent name → 1227-vs-1476 ordering (does SUPER check precede the row check?) |
| 80-8 | unknown key | `OPTIONS (FOO 'x')` → exact error code (1064?) |
| 80-9 | post-state column set | `SELECT * FROM mysql.servers` before/after single-key ALTER → prove merge (other columns unchanged) |
| 80-10 | warnings | `SHOW WARNINGS` same-session after accepted ALTER (any notes?) |
| 80-11 | grant-scoped `mysql.servers` visibility | restricted user existence read vs ALTER outcome (does the dep read's visibility domain match the write check?) — mirrors the SRS D3 finding |
