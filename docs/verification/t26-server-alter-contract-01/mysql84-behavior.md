# MySQL 8.4.10 behavior — evidence status

## Established

- `[DOC]` 8.4 manual mirrors the 8.0 grammar and semantics (the
  r2 official-sources matrix records ALTER SERVER present in 8.4,
  same page family under `/8.4/en/`).
- `[NAT]` None specific to ALTER — prior rounds' SERVER evidence was
  captured on 8.0.x only.
- Version-applicability is already settled elsewhere: the audited
  domain registered today is **MySQL 8.0.x only** — 8.4 sits outside
  `MySQLServerDropPrivilegeDomain` and takes the `gate:"version"`
  boundary. Any future widening needs its own order.

## Native gaps `[PEND]` — proposed probes

Same matrix as `mysql80-behavior.md` (84-1 … 84-11) — the semantic
questions are identical; the point of running them on 8.4 is to
detect divergence in error classes, `mysql.servers` schema, and
privilege ordering before any domain-widening claim could ever be
made. No 8.4-specific behavior is asserted this round.
