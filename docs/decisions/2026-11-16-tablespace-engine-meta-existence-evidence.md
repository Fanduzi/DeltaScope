# Tablespace ENGINE-alter metadata existence evidence (T61 / #146)

## Decision

T25 gains a metadata-connected case pinning what the graduated
`ALTER TABLESPACE <name> ENGINE [=] <engine>` form actually resolves
through the existing `INNODB_TABLESPACES` provider seam — and what it
deliberately still owes. No provider change was made.

## Live-verified outcome (mysql80 anchor)

- `ALTER TABLESPACE t25fx_eng ENGINE=INNODB` on a proven-present
  tablespace emits only `ddl.mysql.tablespace.alter.warn` — existence,
  kind (`general`), and occupancy all bind through the seam; statement
  coverage `complete`.
- `ALTER TABLESPACE t25fx_ghost ENGINE=INNODB` on an absent target emits
  `ddl.mysql.tablespace.alter.warn` **plus** the
  `unknown_tablespace_state` gap (`tablespace.existence` owed);
  statement coverage `unverified`, verdict `review`.

## Rationale — absence is deliberately unproven

`loadMySQLTablespaceObject` returns the snapshot unvouched on
`sql.ErrNoRows`, matching the provider's conservative catalog contract:
a zero-row answer never fabricates absence (the same precedent used by
`loadSequenceObject`, where the catalog filters by privilege). Flipping
that to `MetadataStatusNotFound` would let an invisible-but-real
tablespace be ruled absent and silently promoted — a false-negative
blocker class the provider layer refuses to emit. The
`standalone_absent` applicability arm therefore stays unreachable under
metadata-connected audits; `unknown_tablespace_state` on absent names
remains the honest terminal evidence state, not a gap to close.

## Public contract

- No rule or provider code changed; this is an evidence pin over the
  existing seam.
- Present-target audits are now proven complete for the ENGINE-led form;
  absent-target audits keep their explicit evidence gap.

## Verification

- `testdata/ddl-golden/T25.json` `mysql80-engine-alter-existence`
  metadata case — 52/52 cases, 784 assertions, live mysql80 anchor,
  artifact `/tmp/ddl-golden-depth/T25/artifact.json` @ `84be045c`.
- `go test ./internal/application/audit` — no regression.

## Links

- Issue: #146 (child of #144)
- Graduation record: `docs/decisions/2026-11-15-mysql-alter-tablespace-engine-graduation.md`
