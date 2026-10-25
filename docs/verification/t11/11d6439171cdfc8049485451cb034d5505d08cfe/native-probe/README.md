# T11 native-anchor probe

Native FK facts were observed directly on the live ddl-golden compose
anchors before the oracle was frozen. Raw session logs (verbatim
`docker exec` mysql client output) are preserved beside this file and
also under `docs/work-plans/t11-unified-constraints/`:

- `fk-probe.log` — MySQL 5.7.44 / 8.0.46 / 8.4.10 probes
- `fk-probe-tidb.log` — TiDB v8.5.0 probes

## Facts pinned by these probes (consumed by the T11 contract)

- Named and bare `ALTER ... ADD [CONSTRAINT] FOREIGN KEY` accepted on
  all four anchors; a supporting index is created implicitly.
- Implicit-index naming: authored constraint name when given; first
  local column for a bare add (MySQL); TiDB shows the same named-index
  behavior for named constraints.
- `information_schema.REFERENTIAL_CONSTRAINTS` exposes
  `DELETE_RULE`/`UPDATE_RULE`/`MATCH_OPTION` on all anchors (TiDB
  v8.5.0 included — the meta cases `t11-tidb85-*` exercise the live
  provider join on that column set).
- `DROP FOREIGN KEY` removes the constraint but leaves the supporting
  index; `DROP INDEX` on a still-needed index is refused (1553).
- Duplicate FK name → 1826; missing drop name → 1091; missing member
  column → 1072; unindexed referenced columns → 1215/1822 (MySQL),
  6125 (TiDB).
- TiDB v8.5.0 has `tidb_enable_foreign_key` ON — FKs are enforced, and
  a CLUSTERED primary key is a valid referenced target
  (`t11-tidb85-clustered-parent` golden case).
- TiDB `DROP CONSTRAINT` also drops a named FK — recorded as a T12
  boundary, not modeled in T11.

The catalog-query facts (`KEY_COLUMN_USAGE` join +
`TABLE_CONSTRAINTS` type census) are additionally exercised by every
`metadata_cases` row in the T11 golden on `mysql80` and `tidb85`.
