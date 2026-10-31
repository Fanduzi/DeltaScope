# T17 evidence bundle — database/schema lifecycle @adc1f732

This bundle closes T17/#96 under parent #79: MySQL/TiDB
`CREATE|ALTER|DROP DATABASE|SCHEMA` lifecycle is now modeled as ordered
prospective schema state, with provider resolution, six new governance
rules, coverage consumption of schema default attributes, and a 47-case
golden suite over the four pinned anchors. T16 evidence bundles under
`docs/verification/t16-lifecycle-temporary-conditional/` are preserved
untouched.

## Commit covered

| Commit | Content |
|---|---|
| `adc1f732` | `feat(ddl): T17 database/schema lifecycle — ordered schema state and six rules (#96)` — 42 files: `spec.DDL.AlterDefaultSchema` + `Metadata.SchemaTarget` (+`MetadataSchemaTarget()` accessor); `auditmeta`/`mysql.Provider` `ResolveObject` seam resolving `SCHEMATA` (probed `DEFAULT_ENCRYPTION` and TiDB placement columns, errors propagated not downgraded); `batchState.schemaEntries` tri-state with charset/collation/encryption/placement attributes, convergent `IF [NOT] EXISTS` arms, DROP cascade over tracked contained tables, create-table gating on proven-absent schema; six rules (`exists.forbid`/`alter.exists.require`/`drop.exists.require` with `schema.existence` gaps, `denylist.forbid`, `system.modify.forbid`, `option.encryption.applicability` with `target.version` gap); coverage consumption of charset/collate/encryption while placement stays extracted-but-unaudited; T17.json manifest (47 cases / 4 anchors: 28 cli_metadata + 11 cli_audit + 4 db_ddl + 4 db_syntax_negative), mysql+tidb corpus pairs, inventory closeout, CONTRACT + ADR + probe archive, module READMEs, capability-matrix section, example YAML. |

## Golden line at this head

`T17_MYSQL_PASSWORD=root python3 ./scripts/ddl_golden.py run --task T17
--artifact-dir /tmp/ddl-golden-t17-final` → **47/47 cases, 721 assertions,
PASS**, `head_sha == cli.build.head_sha == adc1f732`, required == executed
(47 manifest ids == 47 executed), `compose_down_rc=0`, zero residual
containers (`golden/T17.artifact.json`, `golden/T17-run.log`).

Anchors: `mysql:5.7.44` (linux/amd64), `mysql:8.0.46`, `mysql:8.4.10`,
`pingcap/tidb:v8.5.0` — per-case digests inside the artifact; TiDB server
reports `8.0.11-TiDB-v8.5.0`.

Case mix (47): 4 `db_ddl` lifecycle cells (one per anchor — fixture
schema create/alter/drop + contained-table cascade verified natively
with `SCHEMATA`/`TABLES` pre/post checks and real engine stderr receipts
— e.g. MySQL `ERROR 1008`, `ERROR 1007`), 4 `db_syntax_negative` cells,
28 `cli_metadata` cases (`SchemaTarget` projection, existence gaps,
denylist/system policies, ENCRYPTION applicability, READ ONLY /
placement unsupported boundaries, name-less ALTER binding to the
trusted request schema), and 11 `cli_audit` cells covering offline
unvouched tri-state behavior.

## Gates collected

See `gates/INDEX.md`: `go test ./... -count=1`, `go vet ./...`, race on
`./internal/...`, sql-corpus, inventory, coverage-catalog, golden
validator contract (758 cases), parser-error contract,
unsupported-diagnostics, decision-record, docs-example, both PostgreSQL
gate packs, three-level-doc (worktree + committed range
`8784346d..adc1f732`), and two lint baselines — all green except the two
lint runs whose findings are 100% pre-existing baseline (zero new items;
one T17-file style finding was fixed before commit).

## Native probe archive

`docs/work-plans/t17-database-schema-lifecycle/probes/p17_schema_matrix.log`
(re-runnable via `probe17_schema_lifecycle.sh`; README indexes the
sections) records on all four anchors: charset/collation lifecycle +
table inheritance, SCHEMA synonyms, missing/duplicate/conditional forms,
drop+recreate, contained-object cascade, orphan-table operations
(1051/1049), ENCRYPTION per-version applicability (5.7 1064 / 8.x lands /
TiDB accepted-but-ignored with warning), READ ONLY ALTER-only boundary,
UPGRADE DATA DIRECTORY NAME refusal, placement policy 8239, system-schema
leniency matrix (5.7 drops `mysql` rc0 — fixture rebuilt; 8.x 3552/1044;
TiDB 1105/1044), and name-less ALTER binding (1046 without a default
schema).

## Deferred / boundary (by contract, not oversight)

- TiDB placement binding stays an extracted-but-unaudited aspect: policy
  existence is unvouchable today, so the statement reports
  `unsupported_statement` + incomplete coverage rather than a fabricated
  pass (native 8239 probed for a missing policy).
- `ALTER DATABASE ... READ ONLY` and `UPGRADE DATA DIRECTORY NAME`
  remain explicit unsupported entries.
- No catalogue enumeration on DROP: the cascade tombstones only tracked
  identities — contained names/counts are never listed or leaked.
