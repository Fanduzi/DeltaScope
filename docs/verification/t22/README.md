# T22/#101 offline evidence — TiDB RU resource-group lifecycle

Scope: offline implementation candidate only. No native (Docker/DB)
execution was authorized or performed; `testdata/ddl-golden/T22.json`
is authored and schema-loadable, not a Golden PASS record.

## Baseline identity

- Repository: Fanduzi/DeltaScope
- Branch: `milestone/mysql-tidb-ddl-completion`
- Pre-implementation HEAD (red baseline): `0b4e66db1398c4c63d239d240635b1f531639df6`
  (the accepted T21-A1 product commit — T22 started on it)

## Red receipts (pre-implementation, captured on the baseline)

Frozen input:

```sql
CREATE RESOURCE GROUP golden_rg RU_PER_SEC = 100;
ALTER RESOURCE GROUP golden_rg RU_PER_SEC = 200;
```

Baseline behavior, reproduced by the permanent public red test
(`internal/application/audit/audit_t22_first_test.go`):

- Both statements audited as `kind=unknown` under `--dialect tidb`;
  `AuditSQL` returned `ErrUnsupportedStatement` with unsupported entries
  `create_resource_group` and `alter_resource_group`;
  aggregate `coverage=incomplete`.
- The same CREATE under `--dialect mysql` parsed silently with no
  boundary metadata — TiDB RU semantics would have been audited as
  native MySQL behavior.
- `DROP RESOURCE GROUP rg` under TiDB produced only
  `ddl.drop_resource_group.notice` — no existence, reserved-group, or
  capability adjudication.
- `CREATE RESOURCE GROUP rg TYPE=USER VCPU=1` under MySQL stayed a
  parser error (the TiDB grammar rejects MySQL CPU fields), so the T24
  family was never silently implemented.

## Green receipts (post-implementation, offline)

- `go test ./internal/application/audit -run TestT22 -count=1`:
  22/22 PASS — first path `kind=ddl` + `unknown_resource_group_capability`
  gap + zero unsupported; complete-premises golden path reaches
  `coverage=complete` with no gaps and both notices and `RuleSummary`
  proves the dedicated rules applied (not skipped); the CREATE batch
  projection publishes the fixed `NewResourceGroupSettings` defaults and
  `RU_PER_SEC=UNLIMITED` pins burst enabled; ALTER merges member-wise at
  the top level while a declared `QUERY_LIMIT` rebuilds the nested group
  and `NULL`/`()` clears it; the successor projection exposes the
  authored `100`→`200` change; MySQL `ErrUnsupportedStatement` with
  `boundary=vendor` metadata; reserved `default` drop is a proven
  refusal with the notice suppressed; guard no-ops emit no success
  claim; resolver errors propagate uncached; disabled rules emit no
  gaps.
- `go test ./internal/infrastructure/parser/tidb -run TestT22 -count=1`:
  10/10 PASS — authored `100`/`200` kept distinct at the extractor seam,
  `UNLIMITED` stays a token, bare `BURSTABLE` projects the 8.5 enabled
  flag, and the legal 8.5 `BURSTABLE [EqOpt] <TRUE|FALSE|NUM>` arm
  extracts authored enabled/disabled through the resource-group
  supplement (same-width blank → reparse → one-to-one typed rebind;
  statement text/positions preserved verbatim; multi-statement input
  works) while the post-8.5 keyword forms (`MODERATED`/`UNLIMITED`/`OFF`)
  stay bounded `unsupported_form` members and non-Boolean operands
  (strings, negatives, decimals, identifiers) plus duplicates keep the
  base parse error, BACKGROUND stays a bounded member, MySQL keeps the
  vendor-boundary marker.
- `go test ./internal/infrastructure/metadata/mysql -run TestT22 -count=1`:
  8/8 PASS — official RESOURCE_GROUPS reader publishes definitions,
  `UNLIMITED` distinct from numeric rates, zero rows stay unknown,
  catalog `BURSTABLE` `YES`/`NO` map to enabled/disabled with any other
  token unproven, `QUERY_LIMIT` distinguishes confirmed-absent (NULL on a
  complete RU-mode row) from configured text and unread incomplete rows,
  errors propagate uncached, MySQL never emits the TiDB catalog query,
  `tidb_enable_resource_control` publishes with an explicit known bit.
- `go test ./internal/application/auditmeta -run TestT22 -count=1`:
  5/5 PASS — real provider + stub driver feeds the burstable premise,
  the known-OFF gate refuses before existence adjudication, zero rows
  owe the state gap, catalog errors propagate through the public path,
  and the first path stays `unverified`+state-gap per statement with
  both lifecycle notices firing.
- `go test ./...`: all packages PASS.
- `make build`: `bin/deltascope`, `bin/deltascope-server`,
  `bin/deltascope-mcp` built clean.
- `make ddl-golden-validator-test`: 758 constructed contract cases +
  23 isolation negative controls, 0 failures.
- `make ddl-inventory-gate` (`TestDDLInventoryContract`): PASS.
- CLI receipt (built binary, provider-free): the frozen two-statement
  input returns `kind=ddl` ×2, one lifecycle notice each,
  `unknown_resource_group_capability` + `unknown_resource_group_state`
  gaps each, zero unsupported entries, `coverage=unverified`; under
  `--dialect mysql` the same CREATE returns one unsupported entry with
  `metadata.boundary=vendor` and `coverage=incomplete`.

## Native-pending branches (explicitly NOT claimed)

- Real TiDB 8.5 `information_schema.RESOURCE_GROUPS` row shape and the
  `tidb_enable_resource_control=OFF` native refusal error class.
- Catalog-backed definition reads on a live anchor (`YES`/`NO` burst
  flag, QUERY_LIMIT column text and its NULL-on-unset form).
- Golden manifest execution (`make ddl-golden TASK=T22`) — requires the
  T22 native authorization card; the eight historical Docker volumes
  stay protected.
