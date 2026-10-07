# Decision: T06-A5 declared CHAR/VARCHAR length policy proof

Date: 2026-10-07
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A5 implementation commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a5_test.go`, `internal/domain/rule/ddl/type_family_rules_t06a5_test.go`, `internal/application/audit/batch_state_t06a5_test.go`, `pkg/deltascope/audit_char_length_t06a5_test.go`, `internal/interfaces/http/audit_char_length_t06a5_test.go`, `scripts/test_ddl_golden.py` T06-A5 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, ALTER policy ownership: issue #87 / T08

## Context

T06-A5 is the first slice that turns `ddl.column.char.max_length` and
`ddl.column.varchar.max_length` from earlier-evidence rules into a frozen
four-anchor contract. Two questions had to be answered before the proof was
honest:

1. **What does the compared number mean?** `Column.Length` is the TiDB
   parser's `FieldType.GetFlen()` — the *declared display width*, in
   characters for CHAR/VARCHAR. It is not a byte budget, not stored-data
   size, and not the `row_size`/`key_length` byte estimates (those consumers
   convert characters to bytes separately and keep their own contracts).
2. **What does a threshold breach mean?** A `limit=8` blocker on
   `CHAR(9)`/`VARCHAR(9)` is a team policy verdict. All four anchors
   (MySQL 5.7.44 / 8.0.46 / 8.4.10, TiDB 8.5.0) accept the same DDL
   natively — the two facts must never merge into one claim.

## Decision

- **The compared unit is declared characters.** The rule compares
  `spec.Column.Length` (the declared `(N)` width) against `params.limit`.
  The anchored oracle observes `CHARACTER_MAXIMUM_LENGTH` and
  `CHARACTER_OCTET_LENGTH` as two independent information_schema fields
  (`utf8mb4`/`utf8mb4_bin` columns make the split concrete: N characters
  vs 4·N octets). No test substitutes octets, row bytes, index bytes, or
  data length for the declared character count.
- **Policy threshold and native legality stay separate evidence.** Every
  anchored A5 case asserts the product verdict AND a post-audit absence
  check (the product never executed the DDL) AND a driver-side native
  `CREATE` with `expect_rc=0` — including N=9, where `reject`/exit 1 and a
  successful native replay coexist in the same case record.
- **Scope is CREATE TABLE only.** Both rules' `AppliesTo` accepts only
  CREATE column statements; `ALTER ... ADD/MODIFY/CHANGE COLUMN` length
  policy consistency is owned by #87/T08 and is pinned as *not applicable*
  at the rule-method layer rather than widened.
- **Shipped defaults are unchanged.** Char stays `limit=64`/`warning`,
  varchar stays `limit=16383`/`blocker`. The proof profiles
  (`t06-a5-char-length-isolated`, `t06-a5-varchar-length-isolated`) enable
  exactly one rule at `limit=8`/`blocker` so a finding can only come from
  the rule under test; the all-rules-disabled profile pins the off
  control. `limit<1` is a constructor error in both rules, and no policy
  upper bound mirroring a database maximum was added.
- **Missing and explicit-zero lengths are distinct facts.** Bare `CHAR`
  carries the parser's `UnspecifiedLength` (-1) verbatim into
  `Column.Length` (its rendered `Type` already canonicalizes to
  `char(1)`); `CHAR(0)` records 0. Both are under any positive limit, but
  they are never merged, and no length-known flag was introduced. Bare
  `VARCHAR` and the `CHAR BYTE` alias are recorded as real parser refusals.
- **No production code changed.** The slice adds tests, the frozen golden
  contract, validator mutations, and documentation only.

## Consequences

- `T06.json` grows from 84 to 124 required cases: the preserved 84 plus a
  16-cell offline matrix (2 types × 2 dialects × {below,at,above,off}) and
  24 anchored cases (4 anchors × 2 types × {7,8,9}).
- The validator gains A5 mutation coverage: threshold flips in either
  direction, swapped/removed `limit`/`actual` metadata, char↔varchar
  rule/message/column/statement mix-ups, off↔above and cross-anchor slot
  swaps, weakened profile limit/level/rule bindings, characters recorded
  as octets, wrong collation or `DATA_TYPE`, skipped or falsified native
  replay, oracle legs deleted on both sides, and paired-side case
  deletion.
- The `Length / precision` traceability row is *partially* golden-proven:
  declared CHAR/VARCHAR character length only. DECIMAL precision/scale,
  other type args, `row_size`/`key_length` byte estimation, and ALTER-side
  length policy keep earlier-evidence status — the row is not marked
  complete.

## Deferred scope

- `CHARACTER`/`CHAR VARYING` and other spelling aliases carry no A5
  evidence; they were not equivalently verified and no case claims them.
- Whether ALTER `ADD`/`MODIFY`/`CHANGE COLUMN` should enforce the same
  thresholds is #87/T08.
- Bare `VARCHAR`/`CHAR BYTE` parser refusals are pinned as-is; promoting
  or normalizing them is out of scope.
- The `decision-record-gate` SIGPIPE/pipefail masking noted in the A4
  record is an independent maintenance item, untouched here.
