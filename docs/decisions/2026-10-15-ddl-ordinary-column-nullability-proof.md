# Decision: T06-A12 ordinary-column NOT NULL policy proof

Date: 2026-10-15
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A12 proof commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a12_test.go`, `internal/domain/rule/ddl/not_null_t06a12_test.go`, `internal/application/audit/batch_state_t06a12_test.go`, `pkg/deltascope/audit_column_not_null_t06a12_test.go`, `internal/interfaces/http/audit_column_not_null_t06a12_test.go`, `scripts/test_ddl_golden.py` T06-A12 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `testdata/ddl-inventory/inventory.yaml`, T06-A12 baseline `/tmp/ds-t06-a12-baseline/`

## Context

The T06-A12 baseline (offline, collected on evidence HEAD `907ce213`)
proved `ddl.column.not_null.require` already implements the frozen
semantics on real parser output: a 16-run offline matrix (eight roles ×
mysql/tidb) plus a real-AST probe showed the ordinary nullable-column
blocker, the `NOT NULL` pass, the time-like and BLOB/TEXT-family
exemption paths, per-column attribution on a mixed table, and the three
distinct rule_summary states (loaded+applicable, loaded+inapplicable,
absent). No product red existed — what was missing was committed proof:
the real-extraction → per-column attribution → isolated-profile finding
identity chain had no dedicated evidence, and no anchored native
observation bound the stored `IS_NULLABLE` fact to the policy verdict.

## Decision

- **Structural nullability and policy exemption are separate
  dimensions.** `spec.Column.NotNull` is a declared fact extracted from
  the AST; `required`/`allow_time_null` are policy switches; the stored
  `IS_NULLABLE` is a database fact. The proof binds all three
  independently: a policy `reject` on `CREATE TABLE t (c INT)` does not
  change the extracted `NotNull=false`, and the anchored replay shows
  the stored column stays `IS_NULLABLE=YES` — the verdict never rewrites
  the declaration, and the declaration never falsifies the verdict.
- **required=false and disabled are different mechanisms.** Under
  `t06-a12-not-null-not-required-isolated` the rule is loaded
  (`loaded=1`) but `AppliesTo` returns false (`applicable=0`); under
  `all-rules-disabled` the rule never loads and `rule_summary` is absent
  per the existing serialization contract. Neither is "the rule found
  nothing": `time-exempt`/`text-exempt` are loaded and applicable yet
  emit zero findings because each column is exempted. All three states
  are pinned per role.
- **Mixed tables are evaluated per column.** `time-allow` exempts
  `time_c DATETIME NULL` and `payload TEXT` without suppressing the
  `ordinary INT` finding; under `strict` the same input produces two
  findings in declaration order. Findings carry complete identity
  (rule_id, level, message, `{table, column, type}` metadata, statement
  index, location) bound to the owning column — a one-finding count is
  never sufficient.
- **Explicit ordinary NULL and omitted nullability share one normalized
  boolean.** The AST distinguishes `ColumnOptionNull` from no clause;
  the extractor treats explicit `NULL` as a no-op because NULL is the
  default nullability, so `spec.Column.NotNull` is `false` for both.
  This provenance loss is recorded as current behavior, not promoted to
  an A12 defect, and no provenance field is added.
- **A policy reject and a native rejection are separate facts.**
  Anchored `ordinary-nullable`, `time-required`, and
  `mixed-attribution` cases record the product `reject` verdict *and*
  the driver-side CREATE success; `time-exempt` and `time-required`
  share one SQL under opposite verdicts, so role identity is bound by
  (sql, profile, verdict), never by SQL text alone. `IS_NULLABLE` is
  observed as stored: the rejected `c`/`ordinary` columns record `YES`,
  the `NOT NULL` declaration records `NO`.
- **DATETIME and TEXT are representatives, not the full exemption
  matrix.** `isTimeLike` covers DATETIME/TIMESTAMP/DATE/TIME/YEAR and
  `isBlobTextLike` covers the BLOB/TEXT families plus JSON; this slice
  pins one representative per family and does not claim the matrices.
- **The A2 primary-key boundary stays intact.** PK members' implicit
  nullability and the A2 explicit-NULL conflict control are untouched;
  this slice covers non-PK ordinary columns only and reuses the A2
  regressions as the isolation control.
- **No schema changes.** `spec.Column` gains no nullability-provenance
  field; `rule_summary` gains no field — the `applicable` value was
  already serialized and is now validated per A12 role.

## Public contract

- Frozen case ids: `t06-a12-{mysql|tidb}-{ordinary-nullable|ordinary-not-null|time-exempt|time-required|text-exempt|mixed-attribution|required-false|disabled}` (offline) and `t06-a12-{mysql57|mysql80|mysql84|tidb85}-{ordinary-nullable|ordinary-not-null|time-exempt|time-required|mixed-attribution}` (anchored), raising T06 to 360 required cases.
- Message pinned verbatim: `column "<name>" should be declared NOT NULL` (metadata `{table:t, column:<name>, type:<parser-rendered>}`), blocker, statement index 0 at 1:1.
- Isolated profiles: `t06-a12-not-null-strict-isolated` (`required=true`, `allow_time_null=false`), `t06-a12-not-null-time-allow-isolated` (`required=true`, `allow_time_null=true`), `t06-a12-not-null-not-required-isolated` (`required=false`); `all-rules-disabled` is reused for the disabled control.
- Anchored structure oracle: ordered `GROUP_CONCAT` of `schema:table:column:DATA_TYPE:ordinal:IS_NULLABLE`, plus independent column count, zero PK constraints, zero primary index parts, no secondary indexes, and InnoDB on MySQL anchors.
- The rule's shipped defaults (`warning`, `required=true`, `allow_time_null=true`) are unchanged; isolated blocker levels live only in the frozen profiles.

## Deferred / out of scope

Explicitly out of this slice: runtime INSERT NULL behavior, SQL mode,
TIMESTAMP implicit defaults, expressions/generated columns, the full
time-like and blob-text-like exemption matrices, ALTER-side nullability
changes, stored-data validity, PostgreSQL behavior beyond the existing
coverage-corpus inclusion, explicit-NULL provenance modeling, default
handling, and any auto-rewrite of rejected declarations.

## Verification evidence

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A12 -v` — extraction, rule-boundary, audit-attribution, and post-state groups green.
- `go test -count=1` on `pkg/deltascope` and `internal/interfaces/http` — SDK/HTTP representatives green.
- `make ddl-golden-validator-test` — contract cases green, including the T06-A12 mutation group (missed/forged blockers, re-attributed and cross-paired findings, same-SQL role swaps, required=false/disabled laundering, weakened profiles, IS_NULLABLE forgeries, column-set/count corruptions, skipped driver replay, denominator shrinkage).
- `make ddl-golden TASK=T06` — 360/360 on the four frozen anchors; per-case driver replay, ordered column sets, and structure oracles preserved in the artifact.

## R1 — strict `rule_summary` shape (2026-10-16)

The A12 check originally compared `parsed.get("rule_summary")` values with
`==`, which conflated an absent key with an explicit `null`, let `True`/`1.0`
launder as the integer `1`/`0`, and crashed with `AttributeError` on truthy
non-object summaries (`1`, `true`, `"bad"`, `[1]`). `t06_a12_artifact_
failures` now requires: "absent" means the key does not exist on the top-level
object (a present `null`/`{}`/`[]`/`false` is a forged summary); enabled roles
must carry a JSON object with `loaded` and `applicable` keys; both counters
must be JSON integers (`type(value) is int`) equal to the frozen per-role
values. The two generic `rule_summary` count reads in
`cli_case_expect_checks` gained the same object/integer guard via
`rule_summary_int_count`, so malformed summaries reject as failure lists
through the full `validate_artifact` entry — never as exceptions. Pre-fix runs
recorded the wrong acceptances and crashes on the original 360-case artifact;
post-fix runs reject all of them while the unchanged artifact still validates.
No product or database behavior changed; the original artifact, binary, and
policies were reused.
