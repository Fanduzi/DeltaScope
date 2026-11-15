# DeltaScope v0.512.0 Release Notes

Release date: 2026-11-16

## Summary

v0.512.0 lands the MySQL/TiDB official DDL semantic-acceptance milestone (#79, T01–T32). Every officially inventoried DDL statement for MySQL 5.7.44, MySQL 8.0.46, MySQL 8.4.10, and TiDB v8.5.0 now lands an explicit class — policy violation, evidence gap, unsupported analysis, parser failure, or audited result — instead of passing silently on parser recognition alone.

The acceptance oracle is executable and versioned: 31 canonical golden manifests run real-database cases against digest-locked anchors (mysql:5.7.44, mysql:8.0.46, mysql:8.4.10 linux/amd64·arm64, pingcap/tidb:v8.5.0 arm64) under `TASK=all`, and the aggregate validator fails closed on denominator shrinkage, missing artifacts, skipped cases, or mixed git HEADs. The final milestone run recorded 2565 cases and 50875 assertions, all passing at `8816924f`.

DeltaScope remains static analysis: it does not execute submitted SQL, return query results, or decide authorization.

## New Audit Slices

- **Stored-program bodies (T27–T30)**: static SQL inside MySQL procedures, functions, triggers, and events is member-scanned into typed statements. Control-flow compounds (IF/CASE/loops) keep member-initial semantics, bounded executable members invalidate proven-literal bindings, `OLD.<col>`/`NEW.<col>` trigger references bind to event semantics with column-level dependency records, and event schedules (AT/EVERY/STARTS/ENDS/ON COMPLETION/DISABLE ON SLAVE) extract as typed members. Member-level bounds keep sibling statements audited instead of swallowing the whole body.
- **TiDB families (T19–T23S)**: TTL lifecycle with native refusal evidence and composite-ALTER refusal handling; placement-policy lifecycle, binds, and `ALTER RANGE`; sequences; resource groups (CREATE/ALTER/DROP audited, `SET RESOURCE GROUP` and BACKGROUND members bounded); physical layout, AUTO_ID_CACHE, and cache tables with a TiKV/TiFlash-backed lane.
- **MySQL low-frequency DDL (T24–T26)**: resource groups with resolved-version gates (8.0.3+) and live deployment evidence (`@@server_uuid`, nproc, CAP_SYS_NICE) bound per run; InnoDB tablespace lifecycle with occupancy-aware drops and rename adjudication; SERVER, spatial reference systems, and ALTER INSTANCE with privilege-premise adjudication and per-form version gates.
- **Prospective ordered schema state**: migration batches evaluate in authored order with proven-absence channels, contamination barriers, and lazy whitelists.

## Boundary and Version Semantics

- Recognized-but-unsupported statements carry named `boundary` markers (`unaudited`, `vendor`) with the owed promotion premise — e.g. TiDB `ALTER INSTANCE RELOAD TLS` keeps `alter_instance.reload_tls` unaudited with `object:instance` and `gap_fact:alter_instance.version_applicability` (T32). MySQL-only `ROTATE INNODB MASTER KEY` stays a TiDB vendor boundary.
- Version gates apply per form: RELOAD TLS MySQL 8.0+, ROTATE 5.7.11+, SRS 8.0.4+, trigger FOLLOWS/PRECEDES 5.7.2+, resource groups 8.0.3+ — a missing or out-of-series version is unproven, never a pass.
- New rules include `ddl.event.enabled_execution.warn` (ENABLED default / DISABLE ON SLAVE evidence gaps), `ddl.trigger.ordering.version` (pre-5.7.2 ordering rejection), and lifecycle notices for triggers and events.

## Infrastructure

- `make ddl-golden TASK=all` runs the canonical manifest set sequentially with per-task compose lifecycles; `reaggregate` rebuilds the aggregate from persisted artifacts and refuses missing or mixed-HEAD evidence; `required_all_tasks` in `anchors-baseline.json` locks the 31-task denominator.
- `deployment_evidence` setup steps probe the live anchor for server UUID, version, vCPUs, and capability bits before audit, so committed evidence stays provenance-only.
- The DDL inventory (`testdata/ddl-inventory/inventory.yaml`) carries file-bound evidence per row, and the capability matrix / bilingual coverage docs track the audited-vs-boundary split.

## Non-Goals

- Not SQL execution, authorization, or a SQL syntax/grammar coverage claim.
- Schema-level `ALTER DATABASE ... PLACEMENT POLICY` remains an explicit unaudited boundary; table-level placement is audited.
- TiDB `ALTER INSTANCE` stays unaudited until a slice proves version applicability; MySQL ALTER INSTANCE is the audited dialect.
- MCP still has no Query Access tool.

## Rule Catalog Facts

| Metric | Count |
|--------|------:|
| Total rules | **531** |
| blocker | 162 |
| warning | 183 |
| notice | 186 |

The catalog now spans 517 DDL and 14 DML entries across common (301), PostgreSQL (191), MySQL (13), and TiDB (36) dialect scopes. The total includes three default-disabled `dml.impact.*` opt-in rules. It is not the Default Policy count and not Loaded.

## Corpus and Catalog Facts

- Supported rule-and-dialect fixture coverage: **790/790**, **100.0%**, **356** YAML fixture files. This is not SQL syntax or grammar coverage.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **410** entries (mysql 63, tidb 57, postgresql 290, parser_upgrade_candidate 6).

## Evidence

- `TASK=all` aggregate at `8816924f`: 31 tasks, 2565 cases, 50875 assertions, all pass; `validate --task all` artifact valid.
- `make test`: 39 packages clean; golden contract suite 890/890; inventory gate file-bound; PostgreSQL gates and npm launcher tests green.
- Decision records under `docs/decisions/` cover every boundary promotion and deferred premise (T19–T32).
