# MySQL 5.7.44 behavior — evidence status

## Established

- `[DOC]` The r2 official-sources matrix records ALTER SERVER as
  present in 5.7 from the statement family + the 8.0 doc's own 5.7
  notes — but **the 5.7 alter-server page was never fetched** (R2
  flagged this `[PEND-1]`: "5.7 editions redirected; content not
  directly obtained"). Treat 5.7 applicability as manual-unverified.
- `[NAT]` None — no prior capture on 5.7.
- Product domain today is 8.0.x-only; 5.7 is outside every gate —
  it would take `gate:"version"` like 8.4.

## Native gaps `[PEND]` — proposed probes

| # | question | notes |
|---|---|---|
| 57-1 | does 5.7.44 accept `ALTER SERVER … OPTIONS` at all | the r2 matrix asserts yes from family-level docs — needs one live statement |
| 57-2 | error class for absent server | likely the shared ER_FOREIGN_SERVER_DOESNT_EXIST family — verify code/sqlstate |
| 57-3 | `mysql.servers` schema + `Server_name` collation | 5.7's system schema predates the data dictionary — verify column set/collate |
| 57-4 | duplicate/empty/out-of-range options | same questions as 80-2/3/4 |
| 57-5 | SUPER-vs-existence ordering | same as 80-7 |

5.7 evidence stays strictly separated — no fact crosses the version
boundary without a probe on that version.
