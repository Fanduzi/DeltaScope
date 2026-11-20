# PostgreSQL 17 recursive column rename

Status Accepted

F09 D44 extends the existing RENAME COLUMN target rules. A fresh qualified PostgreSQL 17 ordinary or partitioned relation can have attached descendants. For a recursive rename, the provider checks exact source and destination attributes through those descendants. It compares each source attribute's inheritance count with the number of its parents in the selected tree. A name collision or an outside inherited definition can block the rename. The finding identifies the authored relation and column while keeping descendant identities private. An incomplete tree, stale relation, or missing recursive source retains evidence gaps. For ONLY, the native attached-child refusal precedes source lookup, even when the authored source column is absent.

The native and CLI comparison covers eight PostgreSQL 17.7 cases. Four operations succeed. The other four return `42701`, `42P16`, or `42703`. The separate rollback witness compares catalog and file identities, data and an independent guard sequence. Permissions, locks and longer-lived effects require their own evidence.

- `/tmp/ds-pg-f09-rename-descendants-cli-24389415/evidence.json`
- [PostgreSQL 17.7 recursive attribute rename](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)

The joint preview passed all 560 T37 cases with 16,507 assertions.
