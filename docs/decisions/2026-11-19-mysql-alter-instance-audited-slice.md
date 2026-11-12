# MySQL ALTER INSTANCE audited slice

## Status

Submitted — pending reviewer ruling
(T26-ALTER-INSTANCE-PUBLICATION-01, frozen contract D1..D10; Phase 0
semantic evidence and contract accepted — Phase 0 R2 archive
`dceb35ae…` released implementation).

## Decision

The MySQL `ALTER INSTANCE` family audits exactly three authored forms
— `RELOAD TLS`, `RELOAD TLS NO ROLLBACK ON ERROR`, and
`ROTATE INNODB MASTER KEY` — each binding a typed
`spec.AlterInstanceDeclaration` and leaving the named boundary into
the audited slice pending the enrich-time per-form version gate.
Everything else keeps its parser-error or generic `alter_instance`
boundary.

`NO ROLLBACK ON ERROR` is a variant member on the reload feature —
the same `alter_instance.reload_tls` identity — never a separate
feature ID (frozen D1/D7). Its independent failure semantics (a
suppressed error returning success with possibly partial effect)
means a return code under it is even less informative than a plain
rc 0 — the adjudication consumes neither.

Applicability is per form (`MySQLAlterInstanceAuditedDomain`): RELOAD
TLS audits on MySQL 8.0+ (probed 1064 on 5.7.44 — the grammar does
not exist there), ROTATE INNODB MASTER KEY audits from MySQL 5.7.11
(probed rc 0 on 5.7.44 / 8.0.46 / 8.4.10 including the component
keyring). A proven-too-old or unresolvable version gates the
statement to `boundary=unaudited` with `gate=version`.

Under TiDB the RELOAD forms keep `boundary=unaudited` — TiDB
genuinely parses and executes them (probed rc 0 on v8.5.0) but this
slice does not own TiDB semantics — while ROTATE keeps
`boundary=vendor` as MySQL-only grammar TiDB rejects at parse
(probed 1064).

## Adjudication

`spec.AdjudicateAlterInstance` → `Metadata.AlterInstance` is the one
truth the rule projection and the batch own-apply both consume. The
arm table is deliberately small: `blocked` when an earlier batch
statement already ran an ALTER INSTANCE apply (the instance surface
the premise would read may have changed), otherwise `unproven` — the
TLS-context / keyring instance premise has no supplier this round
(frozen D4/D6) and a successful return code never discharges it.
`Owed` carries `alter_instance.batch_effects` on blocked; on unproven
it carries `alter_instance.privilege` while the statement's own
projection is not consumable plus the form's instance premise
(`alter_instance.tls_context` or `alter_instance.keyring`).

## Privilege premise

`alterInstancePrivilegePremise` is the family's own projection over
the shared request-level observed-account grant evidence (frozen
D3): a proven `SUPER` / `ALL PRIVILEGES` witness satisfies every
form; an observed form-specific dynamic grant satisfies its own form
(`CONNECTION_ADMIN` for RELOAD TLS, `ENCRYPTION_KEY_ADMIN` for ROTATE
on a proven MySQL 8.0+ domain — the dynamic vocabulary does not
exist on 5.7 where SUPER is the native requirement). A missing
observation or grant is unproven — never a proven denial — and an
earlier authored grant/role statement suppresses a satisfied
premise. The shared loader fires under the same narrow gate as the
SRS family: conditional mode selected, MySQL dialect, and a
final-qualified audited statement in the batch.

## Post-state

Nothing is minted (frozen D6): there is no generic instance-state
channel this round — no TLS-reload or key-rotation post-state is
published. The own apply only marks the instance surface mutated so
a later same-batch statement binds blocked.

## Findings, gaps, privacy

Three rules: `ddl.mysql.alter_instance.reload_tls` and
`ddl.mysql.alter_instance.rotate_master_key` (notice) are the audited
markers and evidence-gap carriers — owed facts publish under
`unknown_instance_state` (or `unproven_privilege` when privilege is
the only debt); `ddl.mysql.alter_instance.privilege` (notice) fires
only on a consumable premise that reached the outcome arm with
`{form, conditional_audit}` metadata. Finding metadata carries
`{form, outcome, no_rollback_on_error?, conditional_audit?}` — never
certificate material, key text, keyring contents, account names, or
grant text (frozen D10).

## Deferred scope

- No TLS-context / keyring provider seam — the instance premise
  stays owed until one exists.
- No post-state proof channel — a successful rc 0 never proves
  reload or rotation.
- TiDB RELOAD TLS / NO ROLLBACK semantics are unaudited (a separate
  TiDB work order would own them).
- Every other ALTER INSTANCE form (ENABLE/DISABLE KEYRING, FOR
  CHANNEL, REDO_LOG, binlog forms) stays outside the slice.
- No generic ALTER INSTANCE or instance-lifecycle framework.

## Verification

- `go test ./internal/application/audit` — nine dedicated
  `TestT26AlterInstance*` cases plus the full package.
- `go test ./...` — full regression including SERVER, SRS, T25,
  and shared parser suites.
- Corpus fixture `testdata/sql-corpus/mysql/ddl/findings/
  mysql_alter_instance_rules.sql` covers the two audited markers.
- Native probes (Phase 0 archive): rc 0 / 1064 / 1227 / 3185 /
  3888 / 2026 evidence across MySQL 5.7.44, 8.0.46, 8.4.10, and
  TiDB v8.5.0.
