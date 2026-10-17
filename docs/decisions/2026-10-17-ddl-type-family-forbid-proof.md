# Decision: T06-A13 type-family forbid policy proof

Date: 2026-10-17
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A13 proof commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a13_test.go`, `internal/domain/rule/ddl/type_forbid_t06a13_test.go`, `internal/application/audit/batch_state_t06a13_test.go`, `pkg/deltascope/audit_type_forbid_t06a13_test.go`, `internal/interfaces/http/audit_type_forbid_t06a13_test.go`, `scripts/test_ddl_golden.py` T06-A13 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, T06-A13 baseline `/tmp/ds-t06-a13-baseline/`

## Context

The T06-A13 baseline (offline, collected on evidence HEAD `84e37e32`)
proved the five ordinary CREATE TABLE type-forbid rules already implement
the frozen semantics on real parser output: a 32-run offline matrix
(sixteen roles × mysql/tidb) plus a real-AST probe showed the per-column
blocker attribution, the blob/text exclusion of JSON, the timestamp
exclusion of DATETIME, the independent float/double finding shape, and the
three distinct `rule_summary` states (loaded+applicable,
loaded+inapplicable, absent). MySQL and TiDB finding output was
byte-identical. No product red existed — what was missing was committed
proof: the real-extraction → per-column attribution → isolated-profile
finding identity chain had no dedicated evidence, and no anchored native
observation bound the stored `DATA_TYPE` set to the policy verdict.

## Decision

- **Five independent team policies, one common mechanism.** BLOB/TEXT,
  JSON, BIT, and TIMESTAMP forbid rules share the generic
  `columnTypeForbiddenRule` (`params.forbid`, `baseType` matching,
  `{table,column,type,family,charset}` metadata);
  `ddl.column.float_double.forbid` is an independent implementation with
  its own `switch` over `baseType` and a deliberately smaller
  `{table,column,type}` metadata shape. Each is proven under its own
  isolated profile — the same mixed SQL under five profiles is five
  distinct roles bound by (sql, profile, rule_id, verdict), never
  interchangeable records.
- **Applicable-but-clean, forbid=false, and disabled are three different
  states.** The safe input under an ON profile is `loaded=1,
  applicable=1` with zero findings; `forbid=false` is `loaded=1,
  applicable=0`; `all-rules-disabled` never loads the rule and
  `rule_summary` is absent per the existing serialization contract.
  None of the three is "the rule found nothing on the mixed input".
- **Two findings keep per-column identity.** The blob/text and
  float/double roles each emit exactly two findings; the contract matches
  complete finding entries as a multiset (rule_id, level, message,
  metadata map, statement index, location), so collapsing to one,
  duplicating one entry over the other, or cross-pairing message and
  metadata columns all fail.
- **The exclusion boundaries are part of the predicate.** The registered
  blob/text predicate is `isBlobTextLike(column) && !json` — the
  `isBlobTextLike` set itself still contains `json` (that is the A12
  exemption set, not the forbid set), so `c_json` is never absorbed by
  `ddl.column.blob_text.forbid`. The timestamp predicate matches
  `baseType=="timestamp"` only, so `c_datetime` is never absorbed by
  `ddl.column.timestamp.forbid`.
- **Declared parser fields and native `DATA_TYPE` are separate layers.**
  The extractor records `Type` from `Tp.String()` lowercased — `BLOB`
  renders `blob binary`, `BIT(1)` renders `bit(1)`, and the built-in
  binary charset on BLOB/JSON leaves `spec.Charset` empty because no
  charset was declared. The anchored oracle instead records the stored
  `information_schema.COLUMNS.DATA_TYPE` (plus column name and ordinal):
  `blob`, `text`, `json`, `bit`, `float`, `double`, `timestamp`,
  `datetime`, `int`. Neither layer falsifies the other.
- **A policy reject and a native rejection are separate facts.** Every
  anchored `mixed-hit` case records the product `reject` verdict and the
  driver-side CREATE success over the same SQL: the policy is a team
  threshold, never a claim that the anchors reject the DDL. `TIMESTAMP
  NULL` is a fixed input fact; no implicit-default, sql_mode, or
  runtime-conversion claim follows from it.
- **No schema changes.** `spec.Column` gains no fields; `rule_summary`
  gains no field; the shipped defaults (warning level, `forbid` default
  true) are unchanged — the isolated blocker levels live only in the
  frozen profiles.

## Public contract

- Frozen case ids: `t06-a13-{mysql|tidb}-<family>-{mixed-hit|safe|forbid-false}` plus `t06-a13-{mysql|tidb}-mixed-off` (offline, 32) and `t06-a13-{mysql57|mysql80|mysql84|tidb85}-<family>-mixed-hit` (anchored, 20), raising T06 to 412 required cases.
- Generic-rule message pinned verbatim: `column "<name>" must not use the <family> type family` with metadata `{table:t, column:<name>, type:<parser-rendered>, family:<blob/text|json|bit|timestamp>, charset:""}`; float/double messages `column "c_float" should not use float` / `column "c_double" should not use double` with metadata `{table:t, column, type}` only.
- Isolated profiles: `t06-a13-<family>-on-isolated` (`enabled`, `level=blocker`, `params.forbid=true`) and `t06-a13-<family>-false-isolated` (`forbid=false`); `all-rules-disabled` is reused for the disabled control.
- Anchored structure oracle: ordered `GROUP_CONCAT` of `schema:table:column:DATA_TYPE:ordinal` over the nine declared columns, plus independent table/column counts, zero PK constraints, zero primary index parts, no secondary indexes, and InnoDB on MySQL anchors.
- The rules' shipped defaults are unchanged; nothing in this slice widens the PG `dialect=common` predicate scope or narrows it either.

## Deferred / out of scope

Explicitly out of this slice: BLOB/TEXT size variants
(TINYBLOB/MEDIUMBLOB/LONGBLOB and TEXT equivalents beyond the two fixed
columns), BIT length ranges and bit values, FLOAT precision args /
rounding / REAL aliases / sql_mode, JSON document semantics / default
expressions / JSON indexes, TIMESTAMP implicit defaults / ranges / time
zones / `explicit_defaults_for_timestamp`, DECIMAL precision/scale,
ALTER-side type governance, runtime values, and any auto-rewrite of
rejected declarations.

## Verification evidence

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A13 -v` — real-extraction, rule-boundary, audit-role-matrix, and post-state groups green.
- `go test -count=1` on `pkg/deltascope` and `internal/interfaces/http` — SDK/HTTP representatives green (float/double two-finding reject + safe pass; blob/text two-finding reject + safe pass as HTTP 200).
- `make ddl-golden-validator-test` — contract cases green, including the T06-A13 mutation group (missed/forged findings, absorbed c_json/c_datetime, collapsed/duplicated/cross-paired findings, dropped or added metadata keys, same-SQL family swaps, switch-state laundering, weakened profiles, column-set corruptions, skipped or failed driver replay, denominator shrinkage).
- `make ddl-golden TASK=T06` — 412/412 on the four frozen anchors; per-case driver replay, ordered nine-column sets, and structure oracles preserved in the artifact.
