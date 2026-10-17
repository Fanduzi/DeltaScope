# Decision: T06-A14 ordinary CREATE TABLE table-option allowlist proof

Date: 2026-10-17
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A14 proof commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a14_test.go`, `internal/domain/rule/ddl/table_option_t06a14_test.go`, `internal/application/audit/batch_state_t06a14_test.go`, `pkg/deltascope/audit_table_option_t06a14_test.go`, `internal/interfaces/http/audit_table_option_t06a14_test.go`, `scripts/test_ddl_golden.py` T06-A14 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `testdata/ddl-inventory/inventory.yaml`, T06-A14 baseline `/tmp/ds-t06-a14-baseline/`

## Context

The T06-A14 baseline (offline, collected on evidence HEAD `a2ea0b6c`)
proved the three table-option allowlist rules
(`ddl.table.engine.allowlist`, `ddl.table.charset.allowlist`,
`ddl.table.row_format.allowlist`) already implement the frozen semantics
on real parser output: a 36-run offline matrix (six roles × three
families × mysql/tidb) plus a real-AST probe showed the distinct option
payloads (engine/charset as verbatim `StrValue`, row_format as a numeric
enum normalized to uppercase names), the omission contract (undeclared
options stay absent from `DDL.Options`), the `containsFold` case-folded
membership, the CREATE-only and PostgreSQL-skip applicability, and the
three distinct rule_summary states. No product red existed — what was
missing was committed proof: the `require_explicit` switch matrix, the
complete finding identity (including `actual=""` on omission), and the
anchored catalog facts that distinguish declaration-level evidence from
database-resolved values had no dedicated evidence.

## Decision

- **`require_explicit=false` exempts omission only.** The same
  disallowed declaration (`ENGINE=MyISAM`, `DEFAULT CHARSET=latin1`,
  `ROW_FORMAT=COMPACT`) rejects under both the strict and optional
  isolated profiles; only the omitted declaration passes under optional.
  `omitted-optional` stays `loaded=1`/`applicable=1` — a different
  mechanism from A13's `forbid=false` (`applicable=0`) and from the
  all-off control (absent `rule_summary`). All six roles are pinned per
  family.
- **Declaration layer and catalog layer are separate evidence.**
  `DDL.Options` records what the SQL declared — verbatim text for
  engine/charset, the normalized enum name for row_format — and an
  absent key means "not declared", never an observed server default
  (`actual=""`). Anchored roles then record the resolved catalog fact
  through fixed identity-bound queries (`golden:t:<value>` scalars), so
  a policy `reject` on `ENGINE=MyISAM` still replays on the driver and
  records `TABLES.ENGINE=MyISAM` on MySQL: the verdict never rewrites
  the declaration, and the catalog never falsifies the verdict.
- **TiDB's catalog display is compatibility representation, not
  physical storage.** On the TiDB anchor both `ENGINE=InnoDB` and
  `ENGINE=MyISAM` record `TABLES.ENGINE=InnoDB`, and both ROW_FORMAT
  declarations record `ROW_FORMAT=Compact`: TiDB parses but ignores
  those options. The frozen oracle records that display verbatim —
  equal outputs across roles remain bound per (sql, profile, verdict,
  anchor) — and nothing is claimed about physical engines, row layouts,
  or performance.
- **Table charset is observed through the real collation
  association.** `information_schema.TABLES.TABLE_COLLATION` joins
  `COLLATION_CHARACTER_SET_APPLICABILITY` to `CHARACTER_SET_NAME`; the
  oracle requires exactly one mapped row per role
  (`golden:t:utf8mb4` / `golden:t:latin1`). An INT column carries no
  charset, a column-level `CHARACTER SET` never fabricates
  `Options["charset"]`, and no default-collation name is hardcoded.
- **Family oracles stay scoped.** The engine family never reuses the
  shared "all InnoDB" structure helper — MyISAM is a legal denied role —
  and never freezes ROW_FORMAT alongside. The row_format family pins
  `ENGINE=InnoDB` as the fixture's default-engine premise, not as an
  extra declared input.
- **No schema changes.** `spec.DDL.Options` gains no
  declaration-provenance field; `rule_summary` gains no field; rules,
  coverage, PostgreSQL gating, and shipped defaults (charset allowlist
  `utf8`+`utf8mb4`, warning levels, `require_explicit=true`) are
  unchanged. The six `t06-a14-*-isolated` profiles are test
  configuration only.

## Public contract

- Frozen case ids: `t06-a14-{mysql|tidb}-<family>-{allowed-strict|denied-strict|omitted-strict|omitted-optional|denied-optional|denied-off}` (offline, family ∈ engine/charset/row-format) and `t06-a14-{mysql57|mysql80|mysql84|tidb85}-<family>-{allowed-strict|denied-strict}` (anchored), raising T06 to 472 required cases.
- Messages pinned verbatim: `table engine must be one of [InnoDB]`, `table charset must be one of [utf8mb4]`, `table row format must be one of [DYNAMIC]` — blocker, statement index 0 at 1:1, metadata `{table:t, option:<key>, actual:<declared|"">, allowed:[<value>]}`.
- Isolated profiles: `t06-a14-<family>-strict-isolated` (`values=[<value>]`, `require_explicit=true`) and `t06-a14-<family>-optional-isolated` (`require_explicit=false`); `all-rules-disabled` is reused for the off control.
- Anchored oracles: `golden:t:InnoDB|MyISAM` (engine, MySQL vs TiDB display), `golden:t:InnoDB:Dynamic|Compact` (row_format, MySQL vs TiDB display), `golden:t:utf8mb4|latin1` (charset association on all four anchors) plus the shared one-table/one-nullable-INT-column/no-PK/no-secondary-index fixture shape.

## Deferred / out of scope

Explicitly out of this slice: the remaining table-option universe
(`KEY_BLOCK_SIZE`, `STATS_*`, `AUTO_INCREMENT` runtime, compression,
checksum), `ALTER TABLE` option changes and `CONVERT TO`, partitioning,
`CREATE TABLE ... LIKE`/`AS`, physical storage/performance/row-layout
claims, the full charset/collation alias and ordering-behavior matrices,
omitted-option server defaults (no `actual=""` promotion), and TiDB's
internal storage model.

## Verification evidence

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A14 -v` — extraction, branch matrix, parameter boundaries, audit-role, and post-state groups green.
- `go test -count=1` on `pkg/deltascope` and `internal/interfaces/http` — SDK/HTTP representatives green.
- `make ddl-golden-validator-test` — contract cases green, including the T06-A14 mutation group (missed/forged findings per role, omitted-strict actual laundered to a server default, swapped option keys, widened allowed lists, dropped metadata keys, duplicated findings, denied-optional pass laundering, omitted-optional 1/0 and all-off fabricated summaries, non-integer counters, strict/optional/off record and profile swaps, cross-family borrowing, weakened profiles, forged MySQL/TiDB catalog identities, missing/multirow charset associations, skipped or failed driver replay, cross-anchor re-keying, offline demotion, duplicates, and denominator shrinkage).
- `make ddl-golden TASK=T06` — 472/472 on the four frozen anchors; per-case driver replay, catalog oracles, and cleanup records preserved in the artifact.
