# TiDB composite ALTER TTL refusal boundary (T19-R6)

Status: accepted
Date: 2026-11-23
Issue: #98 (parent #79)
Work order: T19-ISSUE98-R6-COMPOSITE-TTL-REFUSAL-AND-PROVENANCE-CLOSEOUT-01

## Context

R4 published the standalone ALTER refusal boundary: `TTL_ENABLE` /
`TTL_JOB_INTERVAL` on a proven no-TTL table report `no_ttl_config`
(TiDB ERROR 8150) instead of a silent pass. R5 evidence then proved a
sibling defect on the multi-spec path: `spec.Alter.TTL` members inside a
composite `ALTER TABLE ... <ttl member>, <other action>` were recognized
by the parser and then silently consumed — `ttl_rules.go` returned an
empty outcome for `len(ddl.Alter) != 1` and coverage published nothing,
so natively refused statements reported `pass/complete`.

Native TiDB 8.5.0 probes (`pingcap/tidb@sha256:490861bd...`) closed the
refusal matrix:

| composite shape | no-TTL table | TTL table |
|---|---|---|
| `TTL_ENABLE`/`TTL_JOB_INTERVAL` + other action | ERROR 8150 | ERROR 8200 |
| `TTL=<def>` + other action | ERROR 8200 | ERROR 8200 |
| multi-TTL-member composite | ERROR 8150* | ERROR 8200 |
| `REMOVE TTL` + other action | parser ERROR 1064 | parser ERROR 1064 |
| non-TTL multi-action | rc=0 | rc=0 |

*on a no-TTL table the 8150 no-config check fires before the
multi-schema-change check.

## Decision

1. `spec.TTLAlterCompositeRefusalReason` is the adjudicator for TTL
   members inside multi-spec ALTER statements. It returns one of three
   bounded refusal tiers:
   - `no_ttl_config` — proven existing table + proven absent TTL config
     (ERROR 8150 fires first);
   - `multi_schema_change` — proven TTL table carrying an
     enable/interval member, or any `ttl=` definition member on a proven
     existing table (ERROR 8200);
   - `composite` — the refusal itself is provable (every native branch
     refuses) but the bound pre-state cannot prove which error code
     fires (unknown TTL state, unproven/absent table).
   The tiers never merge: an unknown prior never fabricates a specific
   error code, and an unmodeled member keeps its evidence-gap contract.
2. `coverage.go` selects the composite adjudicator whenever
   `len(ddl.Alter) > 1`; the published aspect keeps the existing
   `alter_table.<action>.ttl.<member>` feature shape plus
   `refusal=<tier>` metadata. Verdict `review`, coverage `incomplete`,
   zero fabricated `ddl.tidb.ttl.*` findings.
3. `ttl_rules.go` still returns an empty finding outcome for composites —
   the statement never executes, so no lifecycle warning fires — but the
   comment now names the coverage boundary so the clause is not hidden.
4. Ordered state already withholds every mutation for TiDB multi-spec
   ALTERs carrying TTL members (`batch_state.go`, "8200, probed" guard),
   so a refused composite cannot pollute successor bindings; subsequent
   statements inherit the exact pre-state.
5. MySQL is unchanged: composite TTL members keep their vendor-boundary
   aspect without a TiDB refusal tier.

## Contract

- Unsupported metadata carries `statement index`, `sql`, `member`, and
  the proven refusal tier — never a bare `reason_code`.
- `REMOVE TTL` composites remain a parser boundary (native ERROR 1064;
  DeltaScope reports `parser_error`), not a coverage refusal.
- Runtime TTL deletion and performance claims remain out of scope.

## Verification

- Native probe transcripts (ER1): full argv/rc/error-code/before-after
  SHOW CREATE records for both refusal classes plus the reverse
  controls.
- Unit matrix `TestT19AlterCompositeTTLRefusal`: proven no-TTL →
  `no_ttl_config`, proven TTL → `multi_schema_change`, unproven/nil
  prior → `composite`, non-TTL multi-action control, successor-state
  inheritance, MySQL boundary guard, zero fabricated findings.
- Golden T19 cases, oracle matrix, validator mutations, and four-surface
  parity each carry composite coverage in the R6 evidence archive.

Links: `internal/domain/spec/ttl.go`,
`internal/application/audit/coverage.go`,
`internal/domain/rule/ddl/ttl_rules.go`,
`internal/application/audit/batch_state_t19_test.go`,
`docs/decisions/2026-11-15-tidb-alter-ttl-no-config-refusal.md`.
