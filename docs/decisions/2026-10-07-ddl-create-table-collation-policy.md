# Decision: T06-A7 table-level COLLATE policy and column charset/collation proof

Date: 2026-10-07
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A7 implementation commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/domain/rule/ddl/table_collation_t06a7_test.go`, `internal/application/audit/coverage_t06a7_test.go`, `internal/infrastructure/parser/tidb/extractor_t06a7_test.go`, `pkg/deltascope/audit_table_collation_t06a7_test.go`, `internal/interfaces/http/audit_table_collation_t06a7_test.go`, `internal/interfaces/mcp/audit_table_collation_t06a7_test.go`, `scripts/test_ddl_golden.py` T06-A7 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, ALTER scope: issue #94 / T15

## Context

`CREATE TABLE ... COLLATE=<name>` was already extracted into
`spec.DDL.Options["collate"]` by the shared TiDB parser, but no rule
consumed it — every such statement reported the bounded unsupported
evidence `create_table.option.collate` and `coverage=incomplete` even
with all rules disabled. The sibling `charset` option has had a consumer
(`ddl.table.charset.allowlist`) since long before this milestone, leaving
the paired declaration governed on one side only. Column-level
`CHARACTER SET`/`COLLATE` declarations already fed three rules
(`ddl.column.charset.allowlist`, `ddl.column.collation.allowlist`,
`ddl.column.charset_collation.match.require`) that were correct but only
carried earlier corpus evidence, never an isolated per-rule proof.

Two design questions had honest answers before implementation:

1. **Is a missing policy finding a capability gap?** No. Capability
   classification (does the audited semantics layer consume this
   extracted fact?) is independent of whether any policy rule is enabled.
   The old unsupported evidence existed because no consumer existed —
   once `Options["collate"]` has a real consumer, `CREATE TABLE ...
   COLLATE` is fully governed input regardless of that rule's `enabled`
   flag.
2. **Does the table rule check native legality?** No. It is a team
   allowlist over the declared option value. A `utf8mb4_general_ci`
   declaration all four anchors accept natively can still be a policy
   blocker, and a `latin1_swedish_ci` value the team allowlist forbids
   says nothing about whether a specific anchor's collation catalog
   contains it.

## Decision

- **One new rule, reusing the existing helper.**
  `ddl.table.collation.allowlist` is registered through
  `newTableOptionAllowlistRule` with `optionKey="collate"`,
  `label="collation"`, fallback `values=["utf8mb4_bin"]`, fallback level
  `blocker`. It consumes `params.values` (case-insensitive membership,
  same as the charset sibling) and `params.require_explicit` (bool,
  default `true`); `enabled`/`level` follow the normal policy contract.
  No `required` parameter, no paired charset/collation table rule, no
  collation catalog or ordering logic.
- **Finding contract mirrors the sibling rules.** A declared value in
  the allowlist is silent; a disallowed value reports exactly one finding
  `table collation must be one of [<values>]` with metadata
  `{table, option: "collate", actual: <declared value>, allowed: [...]}`.
  With `require_explicit=true` an absent declaration reports the same
  finding with `actual: ""`; with `false` it is silent. The rule never
  queries a server default to fill `actual` and never mutates
  `statement.DDL.Options` or the policy array.
- **Scope is MySQL/TiDB CREATE TABLE only.** `AppliesTo` gates
  PostgreSQL out explicitly for this rule ID (the generic table-option
  gate is a per-rule-ID switch — the new ID was added to it) and accepts
  only create-table statements: `CREATE DATABASE`, `ALTER DATABASE`, and
  `ALTER TABLE` never produce this finding.
- **Shipped default is disabled.** The default policy carries the rule
  `enabled=false` at `blocker` with the fallback params — the same
  initial-enablement posture a new governance dimension takes. Tests and
  the golden profiles enable it explicitly; the default-off posture never
  re-creates the retired incomplete classification.
- **Capability classification is rule-independent.** `extractedOptionGap`
  drops only `DDLOperationCreateTable` from the `collate` branch; the
  schema operations (`create_schema.option.collate`,
  `alter_schema.option.collate`) keep their unsupported evidence
  unchanged. Under an all-rules-disabled policy
  `CREATE TABLE t (c INT) COLLATE=utf8mb4_bin` now reports
  complete/pass with zero findings, zero gaps, and zero unsupported
  entries — the S4 baseline that previously returned
  `review`/`incomplete`/`create_table.option.collate`.
- **Declared facts and resolved facts stay separate.** An empty
  `Column.Charset`/`Column.Collation` means "not declared" — the
  extractor performs no server-default or table-option inheritance. The
  anchored oracle separately proves the resolved metadata
  (`CHARACTER_SET_NAME`/`COLLATION_NAME`/`TABLE_COLLATION`), so a bare
  column under `COLLATE=utf8mb4_bin` is product-side "no column
  declaration" and database-side `utf8mb4:utf8mb4_bin` in the same case.
- **The column rules are unchanged.** All three keep their IDs, params,
  defaults, messages, and metadata. The proof adds isolation: each rule
  runs on its own profile so findings attribute to exactly one rule —
  `P_ALL`-style aggregate counts are not used as per-rule evidence.

## Consequences

- `T06.json` grows from 124 to 188 required cases: 32 new offline CLI
  roles (16 roles × mysql/tidb) and 32 new anchored metadata cases
  (8 roles × 4 anchors: MySQL 5.7.44, 8.0.46, 8.4.10, TiDB 8.5.0).
- The anchored roles pin the two-layer fact split: `column-partial`
  (column `COLLATE` only) resolves natively to `utf8mb4:utf8mb4_bin`
  while the product reports the must-declare-together blocker;
  `column-mismatch` (`utf8mb4` + `latin1_bin`) is a real native negative
  bound to ERROR 1253 with the table staying absent;
  `table-inheritance` proves a bare column resolves through the table
  declaration without the product treating the inherited value as
  declared.
- The validator gains A7 mutation coverage: charset↔collation rule-id
  swaps, reject-as-pass flips, native success recorded as failure,
  unknown-collation/permission stderr substituted for ERROR 1253,
  TABLE_COLLATION↔COLUMN_COLLATION swaps, inherited-vs-declared role
  conflation, the table rule carrying column finding identity, retired
  unsupported evidence reappearing, disabled-rule incomplete
  restoration, schema-boundary smuggling, profile/param weakening,
  off↔strict swaps, cross-anchor reuse, offline demotion, and
  both-sides case deletion.
- Prior expected behavior changed in exactly one place: a `CREATE TABLE`
  carrying `COLLATE=` no longer emits `create_table.option.collate`
  unsupported evidence or `incomplete` coverage. Corpus fixtures, the T03
  coverage test, the inventory evidence strings, and this traceability
  row were updated at the same sites; `CREATE/ALTER DATABASE` collate and
  all other unsupported entries are untouched.
- The case-count contract now binds 188: the original 124 identities,
  inputs, and controls are preserved verbatim — the upgrade adds cases
  and profile bindings, it does not relabel or shrink old expectations.

## Deferred scope

- `CREATE DATABASE`/`ALTER DATABASE` collation stays unaudited boundary
  evidence; `ALTER TABLE` collation and option-change governance is
  T15/#94, not absorbed here.
- Collation catalog completeness per version, ordering/sorting behavior,
  `NATIONAL`/alias spellings (`NCHAR`, `NVARCHAR`, `ASCII`, `UNICODE`,
  `BINARY` charset forms), `CHARACTER`/`CHAR VARYING` aliases, and
  `CONVERT TO` are out of scope — allowlist membership is a team policy
  boundary, never a claim about a target version's native support.
- Table-level `charset`/`collate` pairing is deliberately not required:
  the new rule governs one option independently and no match-style table
  rule was added.
