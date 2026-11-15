# DML acceptance denominator and golden closeout (T50–T56)

Date: 2026-11-15
Issue: #133 (parent), #134–#140 (tasks)
Status: decided

## Decision

DML became a first-class acceptance domain with its own official
denominator — separate from the DDL inventory — spanning three product
scopes (mysql 5.7/8.0/8.4, tidb 8.5, postgresql 17) and 44 inventory
rows (`T50`, commit `7ac193a3`).

Acceptance evidence is produced by four golden manifests, one per
dialect surface, all under the enable-all policy profile:

- **T51 — MySQL mutation verbs.** INSERT (VALUES/SET/SELECT/ON
  DUPLICATE KEY UPDATE/IGNORE/priority+DELAYED/PARTITION), REPLACE
  (VALUES/SET/SELECT), UPDATE and DELETE (single and multi-table,
  ORDER BY+LIMIT, LOW_PRIORITY/QUICK/IGNORE modifiers), and DO. Flat
  scope: native execution chains with post-state verifies on
  mysql57/mysql80/mysql84/tidb85 plus pinned CLI audits on the mysql
  and tidb dialects and a live metadata exists/impact case.
- **T52 — MySQL query/utility verbs.** SELECT family (joins,
  subqueries, locking reads, DISTINCT, parenthesized, ORDER/LIMIT),
  UNION, and the 8.0+ forms (TABLE, VALUES, WITH incl. RECURSIVE,
  EXCEPT, INTERSECT, FOR SHARE/NOWAIT/SKIP LOCKED). Audit boundaries
  pinned for SELECT INTO var/outfile, HANDLER, CALL, LOAD DATA/XML
  and IMPORT TABLE; native pinned-refusal steps prove CALL, LOAD DATA,
  LOAD XML and IMPORT TABLE reach the server and fail on missing
  objects rather than being rejected client-side.
- **T53 — TiDB-native DML.** New `tidb` manifest scope (tidb85 anchor
  + tidb CLI dialect). IMPORT INTO is pinned as an explicit
  unsupported audit boundary and as a native refusal against a missing
  file; mutation and query verbs executed natively on tidb85.
- **T54 — PostgreSQL DML.** `postgresql` scope on postgres17. INSERT
  (VALUES/DEFAULT/SELECT/ON CONFLICT/RETURNING), UPDATE (FROM join,
  RETURNING) and DELETE (USING, RETURNING) are audited with real
  findings; SELECT/TABLE/VALUES/WITH, MERGE and COPY stay the audited
  product's explicit unsupported boundary while psql executes every
  one of them on the live anchor — including a real MERGE and
  SELECT INTO — so the boundary is proven vendor-capable but
  auditor-declined, never silent.

## Classification outcomes (T55)

Every DML row keeps its measured cell classification, now backed by
golden rather than probe-only evidence:

- `semantically_checked` — mutation verbs whose rule surface
  (where.require, join.on.require, insert.select.forbid,
  on_duplicate.forbid, replace.forbid, limit/order_by forbids,
  exists.require, impact estimates) fired under enable-all.
- `parse_only` — SELECT-family and set-operation forms the auditor
  parses but does not semantically audit (mysql/tidb SELECT, UNION,
  EXCEPT, INTERSECT, TABLE, VALUES, WITH).
- `explicit_unsupported` — vendor-real statements the auditor
  declines (CALL, DO, LOAD DATA, IMPORT INTO, PG SELECT family, MERGE,
  COPY).
- `parser_unsupported` — grammar the vendored parsers do not
  recognize (HANDLER, LOAD XML, IMPORT TABLE); pinned as parser-error
  diagnostics, not silent gaps.

`required_all_tasks` grew to 49 (T51–T54 added); the canonical
denominator lock in `anchors-baseline.json` is unchanged in mechanism.

## Aggregate (T56)

Single-HEAD `TASK=all` run over all 49 canonical tasks:

- HEAD: `ecca2f24` (T55 inventory commit)
- Tasks: 49 canonical (T21-arm64 and other platform-variant lanes
  excluded); T31 not in the canonical set.
- Cases: 3345 executed, 3345 passed, 59,054 assertions.
- `ddl_golden.py validate` on the aggregate artifact: all task
  artifacts share one HEAD, no zero-case tasks, no skipped cases.

## Deferred scope

- DML corpus files under `testdata/sql-corpus/` remain sparse; the
  golden manifests now carry the executable evidence instead. Corpus
  expansion is tracked separately if wanted.
- PG metadata-connected DML audits show no `dml.table.exists.require`
  finding on missing tables (postgres meta path has no table-existence
  provider for DML) — recorded honestly in T54's metadata case.
- HANDLER / LOAD XML / IMPORT TABLE parser gaps are not among the six
  named parser-upgrade candidates; they stay parser_unsupported.
