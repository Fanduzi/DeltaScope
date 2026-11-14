# TiDB ALTER TTL no-config refusal → vendor boundary

## Status

Accepted (T19 R4, commit `10b3df5d`).

## Context

On real TiDB 8.5.0, `ALTER TABLE t TTL_ENABLE='ON' | 'OFF'` and
`ALTER TABLE t TTL_JOB_INTERVAL='<interval>'` on a table that has no TTL
configuration are refused natively with `ERROR 8150`. Before R4, DeltaScope
reported these statements as `pass`/`complete` (or an unverified gap for the
interval form), while the CREATE-side equivalent already published a bounded
vendor-boundary aspect via `TTLCreateRefusalReason`.

## Decision

`spec.TTLAlterRefusalReason(dialect, declaration, prior)` mirrors the CREATE
helper. Coverage publishes an unsupported vendor-boundary aspect with
`refusal=no_ttl_config` only when all four premises hold:

1. dialect is TiDB;
2. the bound prior snapshot exists and `Exists` is true;
3. `TTLKnown` is true and `TTL == nil` (prior state explicitly proves no TTL
   config — never inferred from an absent field);
4. the declaration is a standalone, fully modelled `ttl_enable` or
   `ttl_job_interval` member with no unmodelled content and is not
   `TTLClauseSourceUnproven`.

Public contract:

- verdict `review`, top-level and statement coverage `incomplete`;
- unsupported feature `alter_table.table_option.ttl.ttl_enable` /
  `...ttl_job_interval` with metadata `aspect=ttl`, `action=table_option`,
  `member=<member>`, `refusal=no_ttl_config`;
- no `ddl.tidb.ttl.warn` finding is fabricated for a native refusal;
- CLI exit follows existing vendor-boundary/`--fail-on` semantics.

## Boundaries kept

- Unknown prior state (provider miss, `TTLKnown=false`) stays an evidence gap.
- REMOVE TTL on a no-TTL table remains a native no-op (not a refusal).
- Existing-TTL ON→OFF silent / OFF→ON warn / interval rules are unchanged.
- A refused ALTER preserves the proven no-TTL pre-state so later batch
  statements inherit the refusal premise rather than polluted state.
- MySQL `/*T![ttl] ... */` feature comments remain the R1 vendor boundary.
- Composite ALTER (`TTL_ENABLE` + another action) is native ERROR 8200
  (multi-schema-change); it is not claimed by this boundary.

## Verification

- Native probes: `t19-r4` evidence tree `native/` records ERROR 8150 for all
  three standalone member forms and ERROR 8200 for composite ALTER on TiDB
  8.0.11-TiDB-v8.5.0.
- Golden: 44/44 cases including provider-backed no-TTL refusal, unknown-prior
  gap, existing-TTL controls, and prospective REMOVE→ENABLE refusal.
- Unit: `TestT19AlterTTLNoTTLConfigRefusal` covers member forms, unknown
  prior, TTL-present controls, prospective inheritance, and successor-state
  pollution guards.
- Parity: CLI/SDK/HTTP/MCP produce identical structured semantics for the
  refusal case.

## Links

- `internal/domain/spec/ttl.go` — `TTLAlterRefusalReason`
- `internal/application/audit/coverage.go` — ALTER TTL aspect publication
- `internal/application/audit/batch_state_t19_test.go` — `TestT19AlterTTLNoTTLConfigRefusal`
- `testdata/ddl-golden/T19.json` — refusal/gap/control cases
- `docs/decisions/2026-11-11-tidb-ttl-lifecycle.md` — TTL lifecycle model
