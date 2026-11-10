# T26-R1 baseline: current implementation, real offline results, leak surface

Evidence-only. Probe binary: `CGO_ENABLED=0 go build -o /tmp/t26probe/deltascope
./cmd/deltascope` on HEAD `4896137f` — sha256
`b68c4dbb23e0452c269f0ecd6698a1d0f36578cbfb59b4f64f25eeb061ef2f11`,
go1.26.1 darwin/arm64. Inputs/outputs archived verbatim in `probes/`.

## 1. Source chain (current code, all paths verified)

| stage | location | what happens to SERVER/SRS today |
|---|---|---|
| input/split | `internal/application/audit/parse.go` | statements split with positions; `RawSQL` retained verbatim (`:140 RawSQL: rawSQL`) |
| dialect parser | `internal/infrastructure/parser/tidb/parser.go` | TiDB-grammar parse; CREATE/ALTER/DROP SERVER and all SRS forms **fail** → `parser_error` diagnostic, 0 statements |
| MySQL supplements | `mysql_tablespace.go`, `mysql_resource_group.go`, `supplement.go` | strict whole-input recognizers for tablespace/resource-group families only — nothing claims SERVER/SRS/ALTER INSTANCE |
| unhandled boundary | `coverage_boundary.go` `unhandledStatementFeature` | `*ast.AlterInstanceStmt` → feature `"alter_instance"` (line ~239); TiDB grammar parses `ALTER INSTANCE RELOAD TLS` |
| diagnostics | `internal/application/audit/diagnostics.go` | `newParserErrorDiagnosticWithGuidance` — fixed-text reason, keyword-only `GuidanceCode`/`EvidenceRef`, **no SQL content stored** |
| unsupported echo | `internal/application/audit/evaluate.go:33-34` | `if item.SQL == "" { item.SQL = statement.RawSQL }` — `unsupported[].sql` backfills the **full raw statement**; same backfill for gaps (:73-74) |
| spec | `internal/domain/spec/statement.go` | `Statement.RawSQL`/`NormalizedSQL` echo input; `UnsupportedDetail.SQL` field exists (`:65-70`); no Servers/SRS metadata fields |
| provider | `internal/infrastructure/metadata/mysql/provider.go` | `LoadInstanceFacts`, `LoadTableSnapshot` only — **no** `mysql.servers`, `ST_SPATIAL_REFERENCE_SYSTEMS`, or `ST_GEOMETRY_COLUMNS` readers |
| output | `internal/domain/report/result.go:88,119` | `raw_sql`, `unsupported[]` serialize verbatim |

Focused checks per work order:
- SERVER is **not** degraded to a generic unrecognizable statement — it is
  a clean `parser_error` (feature unknown), which is honest but loses the
  family entirely (silent-miss risk the issue targets).
- SRS is not reduced to a name or raw string because nothing parses it;
  the risk materializes only if a future recognizer retains text loosely.
- Parser-success-but-adapter-miss is demonstrated by `ALTER INSTANCE
  RELOAD TLS`: parsed → `unhandledStatementFeature` → `unsupported_statement`
  `alter_instance`, coverage incomplete — the boundary path works.
- Dialect detection stays separate from semantic judgment
  (`context.dialect_source` flag vs detected in metadata mode).
- Parse-failure paths do not copy sensitive input (verified §3).

## 2. Real offline probe matrix (verbatim outputs in probes/outputs.txt)

| input | mysql | tidb |
|---|---|---|
| `CREATE SERVER golden_srv … PASSWORD 'golden-secret'` | parser_error rc=2 | parser_error rc=2 |
| `ALTER SERVER … OPTIONS (HOST 'db2.example.com')` | parser_error rc=2 | parser_error rc=2 |
| `DROP SERVER [IF EXISTS]` | parser_error rc=2 | parser_error rc=2 |
| `CREATE SPATIAL REFERENCE SYSTEM 4120 …` | parser_error rc=2 | parser_error rc=2 |
| `CREATE OR REPLACE SRS 40001 …` | parser_error rc=2 | parser_error rc=2 |
| `DROP SPATIAL REFERENCE SYSTEM IF EXISTS` | parser_error rc=2 | parser_error rc=2 |
| `ALTER INSTANCE ROTATE INNODB MASTER KEY` | parser_error rc=2 | parser_error rc=2 |
| `ALTER INSTANCE ROTATE BINLOG MASTER KEY` | parser_error rc=2 | parser_error rc=2 |
| `ALTER INSTANCE RELOAD TLS` | unsupported `alter_instance` rc=1 review/incomplete | same |
| `ALTER INSTANCE RELOAD KEYRING` | parser_error rc=2 | parser_error rc=2 |
| `ALTER INSTANCE ENABLE INNODB REDO_LOG` | parser_error rc=2 | parser_error rc=2 |

Golden Path input (verbatim issue input) result:
`CREATE SERVER golden_srv FOREIGN DATA WRAPPER mysql OPTIONS (HOST
'192.0.2.1', DATABASE 'app', USER 'u', PASSWORD 'golden-secret');`
→ exit=2, `diagnostics[0].classification=parser_error`, `statements=0`,
`coverage=incomplete`, `verdict=""`, stderr prints the fixed reason line.
`golden-secret`, `192.0.2.1`, `app`, `u` appear **nowhere** in the output —
the parser_error path is clean. **But** this is not yet the issue oracle:
the required outcome is an explicit incomplete/unsupported *boundary with
identity*, not a bare parser_error (the statement currently vanishes from
the audited denominator, which is exactly what #105 prohibits).

Multi-statement probe:
`CREATE TABLE t (id INT); CREATE SERVER s … PASSWORD 'golden-secret';
DROP TABLE t;` → siblings audited (2 statements, indices 0/1, positions
preserved), parser_error diagnostic at line 1 column 26 for the SERVER
statement, coverage=incomplete, no leak. Statement identity/positions
survive a mid-batch failure.

## 3. Leak-surface findings (current build)

| path | stores input text? | verdict |
|---|---|---|
| `diagnostics[]` (parser_error) | no — fixed strings + keyword-derived GuidanceCode | clean today |
| `diagnostics[]` (unsupported_statement) | no — same fixed-shape | clean |
| `unsupported[].sql` | **yes — entire RawSQL backfilled** (`evaluate.go:33`) | leak channel |
| `statements[].raw_sql`/`normalized_sql` | yes — verbatim echo by design | leak channel |
| stderr | fixed reason line only | clean |
| provider error wraps | n/a this round (no provider load for these objects) | unknown-by-absence |

**Registered contract conflict for reviewer ruling**: today SERVER never
reaches `unsupported[]` (it is parser_error), so nothing leaks — but the
moment a recognizer routes SERVER to any parsed channel (unsupported,
boundary, or audited), both `unsupported[].sql` and `raw_sql` will echo
`PASSWORD 'golden-secret'` verbatim unless the spec or serializer is
redaction-aware. The #105 no-leak oracle cannot be met by classification
choices alone; it needs either (a) statement-level input redaction for
credential-carrying families, or (b) an explicit exemption decision for
echo fields. Worker does not decide this — flagged for ruling.

## 4. 153-probe corpus mapping

`docs/research/2026-09-21-mysql-tidb-ddl-probes.json` (153 probes) contains
**zero** entries for `create server`/`alter server`/`drop server`/
`spatial reference`/`alter instance` → registered as **未映射/基线遗漏**,
matching inventory `status_evidence` "2026-09-22 CLI probe: parser_error"
(now re-verified verbatim this round, §2). The `2026-09-22` denylist
probe file is a different corpus (25 denylist probes, no SERVER/SRS).

## 5. Metadata/unknown-propagation baseline

- `spec.Metadata` has no `Servers`/`SRS` slots; the provider exposes no
  loaders for `mysql.servers` / `ST_SPATIAL_REFERENCE_SYSTEMS` /
  `ST_GEOMETRY_COLUMNS` → every server/SRS premise is structurally absent,
  i.e. UNKNOWN-by-unimplemented-reader, not proven-absent.
- Existing contract (T04/#83): zero-row provider results stay unknown;
  bounded `evidence_gaps` with stable `reason_code` carry missing premises.
- Same-batch ordering: parsed siblings keep identity and later statements
  still audit; a mid-batch parser_error produces a diagnostic and
  `coverage=incomplete` without contaminating siblings' verdicts.

## 6. No-outbound-access evidence (static call-chain, honest scope)

- Offline audit (`context.mode:"offline"`) opens **no** connection —
  `unproven:["column_exists","table_exists"]` records that existence was
  not checked.
- The audited SQL text flows through: tokenizer/parser (in-process) →
  spec declarations → rule evaluation → report serialization. No
  `net.Dial`, HTTP client, file open, or process exec on that path in the
  audited-statement pipeline; the only network consumer is the metadata
  provider, which is driven by `--host/--port` connection flags — never
  by statement content (provider call sites take host/port from
  `connect`, not from parsed DDL).
- Honest boundary: this is static chain review only. It does **not**
  prove runtime behavior (no syscall/network observation); it does not
  cover plugin paths or future code; a future runtime check (e.g.,
  blocked-egress probe under a later authorized native round) is the
  residual verification path — listed in the native plan, not run now.

## 7. Test commands actually run (offline only)

- `go build` → probe binary (sha256 above).
- 26 CLI invocations (2 dialects × 13 inputs) + 1 stderr capture —
  all offline, zero DB/network usage.
- `make ddl-golden TASK=T26`: **未建立** — `Makefile` has `ddl-golden`
  infrastructure (runner `scripts/ddl_golden.py` + `testdata/ddl-golden/`
  manifests) but no `T26.json` manifest exists yet; creating one belongs
  to a later implementation round, so the target was **not** executed.
- `go test ./...` not run this round: no product changes to verify, and
  the work order limits runs to confirmed offline parts of touched scope
  (none touched).

## 8. What R1 does NOT claim

- No support claim, no implementation, no native result, no metadata
  proof. All "officially applicable" statements above are doc-level
  applicability, not DeltaScope capability.
