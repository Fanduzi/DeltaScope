# DDL-ALTER-INSTANCE-EXT — local tracking record (T26-R2 ruling 三/D)

Local tracking only. **No issue created** (not authorized). This record
is a responsibility entry, not a coverage claim — nothing below is
implemented or natively verified.

## 1. Origin

R1 scope review found `mysql.alter-instance`'s inventory `subactions`
list is incomplete relative to the official `ALTER INSTANCE` grammar.
Reviewer ruling (T26-R2): surplus subactions belong to an independent
sibling task, **removed from T26 implementation proposal and the
planned native matrix**, kept here with official entries, version bases,
unfinished status, and a responsibility entry — never registered as
"covered".

## 2. Ownership read-back (existing two subactions)

`testdata/ddl-inventory/inventory.yaml` row `mysql.alter-instance`
carries `owner: T26` and `subactions`:
- `ROTATE INNODB MASTER KEY (5.7.11+/8.0)` — **stays T26-owned**
- `RELOAD TLS (8.0+)` — **stays T26-owned**

Per ruling these two are read back as T26-owned; not reopened, not
transferred. TiDB `ALTER INSTANCE` forms remain a separate vendor
boundary (TiDB's own statement semantics, feature `alter_instance`
unaudited boundary in current code).

## 3. Surplus registry — DDL-ALTER-INSTANCE-EXT

| # | official form | version gate | privilege | state today | disposition |
|---|---|---|---|---|---|
| E1 | `ALTER INSTANCE {ENABLE\|DISABLE} INNODB REDO_LOG` | 8.0.21+ | documented server privileges | parser_error (no grammar claim) | sibling task candidate |
| E2 | `ALTER INSTANCE ROTATE BINLOG MASTER KEY` | 8.0+ (needs `binlog_encryption` + keyring) | `BINLOG_ENCRYPTION_ADMIN`/`SUPER` | parser_error | sibling task candidate |
| E3 | `ALTER INSTANCE RELOAD KEYRING` | 8.0.24+ (keyring **component** only) | `ENCRYPTION_KEY_ADMIN` | parser_error | sibling task candidate |
| E4 | `RELOAD TLS FOR CHANNEL {mysql_main\|mysql_admin}` modifier | 8.0.21+ | `CONNECTION_ADMIN` | grammar variant of inventoried subaction — registered as extension surface | sibling task candidate |
| E5 | `RELOAD TLS ... NO ROLLBACK ON ERROR` modifier | 8.0+ | `CONNECTION_ADMIN` | same | sibling task candidate |

## 4. Proposed assignment path (suggestion only, no issue)

- Formal T-number/issue assignment: propose next free milestone task
  under #79 after reviewer confirms; sibling of T26, dependent on the
  same ALTER-INSTANCE recognition seam T26 builds for its two
  inventoried subactions.
- Deferred dependency: keyring capability is an instance-premise class
  (component installed? plugin? none) — the sibling task inherits the
  metadata-fact scheme from `r2-metadata-facts.md` rather than
  duplicating it.
- Explicit non-coverage: listing here does not mark these forms as
  covered, does not enter the T26 native case matrix, and does not
  alter the `mysql.alter-instance` catalog entry.
