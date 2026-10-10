# Decision: T06-A10 AUTO_INCREMENT declaration and init-value proof

Date: 2026-10-10
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A10 proof commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a10_test.go`, `internal/domain/rule/ddl/auto_increment_t06a10_test.go`, `internal/application/audit/batch_state_t06a10_test.go`, `internal/infrastructure/metadata/mysql/provider_t06a10_test.go`, `pkg/deltascope/audit_auto_increment_t06a10_test.go`, `internal/interfaces/http/audit_auto_increment_t06a10_test.go`, `scripts/test_ddl_golden.py` T06-A10 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `testdata/ddl-inventory/inventory.yaml`

## Context

The T06-A10 baseline (`/tmp/ds-t06-a10-baseline/`, collected on evidence
HEAD `1e75b360`) proved both target rules already implement the frozen
semantics on real parser output: a 16-run offline matrix (eight roles ×
mysql/tidb) plus a real-AST probe showed the single-member PK
`auto_increment` blocker, the composite-PK member-count skip, the disabled
controls staying clean, and the exact-equality init-value check (`8` passes,
`9` blocks, omission is silent). No product red existed — what was missing
was committed proof: the declaration facts, the anchored stored-column
oracle, and the separation of declared value vs catalog allocator vs next
allocated id had no dedicated evidence.

## Decision

- **Two rules, two separate declaration facts.** The column-level
  `AUTO_INCREMENT` clause lands on `Column.AutoIncrement`; the table-level
  `AUTO_INCREMENT=n` option lands on `DDL.Options["auto_increment"]`.
  Neither is synthesized from the other, and an omitted option yields an
  absent key — never a defaulted value.
- **The PK rule checks one bound member only.**
  `ddl.table.primary_key.auto_increment.require` binds the declared PK
  member list back to `spec.Column` and evaluates `AutoIncrement` only when
  exactly one member binds. A composite PK skips the check — the skip is a
  member-count gate, not proof the composite shape satisfies the policy.
  `required=false` (and `enabled=false`) silences the rule entirely.
- **The init rule is exact equality, not a floor or a presence check.**
  `ddl.table.auto_increment.init_value.require` compares the declared
  option string parsed as an integer against `value` (default 1, minimum 1).
  `AUTO_INCREMENT=8` passes under `value=8`; `9` rejects with
  `{table, required_value:8, actual_value:9}`; an omitted option is silent
  and emits no finding, gap, or metadata. The rule has no `required`
  parameter.
- **Declaration, catalog state, and next id are three different layers.**
  The derived CREATE post-state carries the *declared* option value; the
  provider's `information_schema.TABLES.AUTO_INCREMENT` read lands on the
  same `Options` key but is a *catalog observation*; and the id a future
  INSERT would return is never inferred. Anchored cases record the
  allocator value and SHOW CREATE verbatim as bound observations —
  shape-checked, target-bound, never frozen as an oracle.
- **A policy reject and a native rejection are separate facts.** Anchored
  pk-no-auto and init-mismatch cases record the product `reject` verdict
  *and* the driver-side CREATE success; the stored column then proves
  `EXTRA` byte-exact (`auto_increment` for declaring roles, empty for the
  composite and pk-no-auto controls) via the A9-R2 CAST-to-BINARY pattern —
  a distinct auto_increment/empty role, never the A9 time whitelist.
- **No production code changed.** The slice adds tests, two isolated
  manifest profiles (`t06-a10-pk-auto-increment-isolated`,
  `t06-a10-init-value-isolated`), 36 frozen cases (16 offline + 20
  anchored), record-only execute observations (verbatim EXTRA members,
  allocator row, SHOW CREATE), a dedicated artifact validator for their
  shapes, and validator mutations.

## Public contract

- Frozen case ids: `t06-a10-{mysql|tidb}-{pk-auto|pk-no-auto|composite|pk-off|init-match|init-mismatch|init-omitted|init-off}` (offline) and `t06-a10-{mysql57|mysql80|mysql84|tidb85}-{pk-auto|pk-no-auto|composite|init-match|init-mismatch}` (anchored), raising T06 to 292 required cases.
- Messages pinned verbatim: `primary key column "id" must use auto_increment` (metadata `{table:t, column:id, type:bigint(20)}` — the parser's own rendering, not a required native `COLUMN_TYPE`) and `table auto_increment init value must be 8` (metadata `{table:t, required_value:8, actual_value:9}`), both blocker, statement index 0 at 1:1.
- Isolated profiles load exactly one rule (`loaded=1`, `applicable=1`); the all-off control may omit `rule_summary` per the existing serialization contract.
- Both rules' shipped defaults (level, `required`/`value` params) are unchanged.

## Deferred / out of scope

Explicitly out of this slice: allocator advance, next-INSERT ids,
continuity/monotonicity, allocation cache, concurrency, restart recovery,
`AUTO_INCREMENT=0` server semantics, unindexed AUTO_INCREMENT legality,
composite-index first-column legality, multiple AUTO_INCREMENT columns,
`AUTO_RANDOM`, `AUTO_ID_CACHE`, ALTER-side auto_increment changes, and any
unified provenance framework for the shared `Options` key. The catalog
counter is observed and recorded, not validated; SHOW CREATE is a
normalized rendering, not declaration provenance.

## Verification evidence

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A10 -v` — extraction, rule-boundary, and post-state groups green.
- `go test -count=1` on `internal/infrastructure/metadata/mysql`, `pkg/deltascope`, `internal/interfaces/http` — provider provenance and SDK/HTTP representatives green.
- `make ddl-golden-validator-test` — 422 contract cases, 0 failures, including 29 T06-A10 mutations (missed/forged blockers, rule substitution, profile swaps and re-parametrization, composite-shape forgery, observation deletion/forgery, driver-replay skip, denominator shrinkage).
- `make ddl-golden TASK=T06` — 292/292 on the four frozen anchors; per-case verbatim EXTRA members, allocator rows, and SHOW CREATE preserved in the artifact.
