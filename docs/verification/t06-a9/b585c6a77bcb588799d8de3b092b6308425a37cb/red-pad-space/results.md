# T06-A9-R1 EXTRA 比较语义定向核对 — 结果

Code under review: ad8f74227b19d42575c0494b68960e420396acb5
Repo HEAD at probe time: edc5bf079c29ef9297c394cd71895912a7412720
Fixture: deltascope-ddl-golden-mysql84, mysql:8.4.10, image sha256:c592c15aaf4a1961e15d82eb31ea5987dda862d1c4b1e93424438c0e91dc1f8d (本轮新建, probe 后已拆除)
Helper: real `t06_a9_extra_case` imported from scripts/ddl_golden.py at HEAD edc5bf0 (in-repo module is identical function used by t06_a9_extra_normalized)

## Baseline (00-baseline.out)

```
VERSION() = 8.4.10   @@collation_connection = utf8mb4_0900_ai_ci
CHARSET(EXTRA) = utf8mb3   COLLATION(EXTRA) = utf8mb3_general_ci   (information_schema.COLUMNS)
```

## Results

Group A — plain session literals (expression collation utf8mb4_0900_ai_ci, NO PAD):

| label | input_hex | expr collation | result_text | expected | verdict |
|---|---|---|---|---|---|
| A.legal-0 | (empty, len 0) | utf8mb4_0900_ai_ci | (empty = created marker) | created marker | OK |
| A.legal-1 | 44454641554C545F47454E455241544544 | utf8mb4_0900_ai_ci | (empty = created marker) | created marker | OK |
| A.legal-2 | 4F4E205550444154452043555252454E545F54494D455354414D50 | utf8mb4_0900_ai_ci | on update current_timestamp | updated marker | OK |
| A.legal-3 | 44454641554C545F47454E455241544544206F6E207570646174652043555252454E545F54494D455354414D50283029 | utf8mb4_0900_ai_ci | on update current_timestamp | updated marker | OK |
| A.illegal-0 | 20 | utf8mb4_0900_ai_ci | `<unrecognized-extra>` | `<unrecognized-extra>` | OK |
| A.illegal-1 | 64656661756C745F67656E65726174656420 | utf8mb4_0900_ai_ci | `<unrecognized-extra>` | `<unrecognized-extra>` | OK |
| A.illegal-2 | 6F6E207570646174652063757272656E745F74696D657374616D7020 | utf8mb4_0900_ai_ci | `<unrecognized-extra>` | `<unrecognized-extra>` | OK |
| A.illegal-3 | 64656661756C745F67656E657261746564206F6E207570646174652063757272656E745F74696D657374616D7028302920 | utf8mb4_0900_ai_ci | `<unrecognized-extra>` | `<unrecognized-extra>` | OK |

Group B — expression explicitly converted to EXTRA's charset/collation (utf8mb3 / utf8mb3_general_ci, PAD SPACE):

| label | input_hex | expr collation | result_text | expected | verdict |
|---|---|---|---|---|---|
| B.legal-0 | (empty, len 0) | utf8mb3_general_ci | (empty = created marker) | created marker | OK |
| B.legal-1 | 44454641554C545F47454E455241544544 | utf8mb3_general_ci | (empty = created marker) | created marker | OK |
| B.legal-2 | 4F4E205550444154452043555252454E545F54494D455354414D50 | utf8mb3_general_ci | on update current_timestamp | updated marker | OK |
| B.legal-3 | 44454641554C545F47454E455241544544206F6E207570646174652043555252454E545F54494D455354414D50283029 | utf8mb3_general_ci | on update current_timestamp | updated marker | OK |
| B.illegal-0 | 20 | utf8mb3_general_ci | (empty = created marker) | `<unrecognized-extra>` | **FAIL — whitelist hit** |
| B.illegal-1 | 64656661756C745F67656E65726174656420 | utf8mb3_general_ci | (empty = created marker) | `<unrecognized-extra>` | **FAIL — whitelist hit** |
| B.illegal-2 | 6F6E207570646174652063757272656E745F74696D657374616D7020 | utf8mb3_general_ci | on update current_timestamp | `<unrecognized-extra>` | **FAIL — whitelist hit** |
| B.illegal-3 | 64656661756C745F67656E657261746564206F6E207570646174652063757272656E745F74696D657374616D7028302920 | utf8mb3_general_ci | on update current_timestamp | `<unrecognized-extra>` | **FAIL — whitelist hit** |

## Interpretation (evidence only, no fix applied)

`utf8mb3_general_ci` is a PAD SPACE collation: `IN (...)` comparisons pad the
shorter operand with U+0020, so `' '` = `''`, `'default_generated '` =
`'default_generated'`, etc. Under the real catalog query the CASE compares
against `EXTRA`, whose column collation is exactly `utf8mb3_general_ci` on
this anchor — group B reproduces that path faithfully. Under
`utf8mb4_0900_ai_ci` (NO PAD, group A) every malformed input correctly falls
through to `<unrecognized-extra>`.

Generated SQL for the four failing rows: see 20-group-b.sql lines 5-8
(labels B.illegal-0..3). All 8 statements in each group were produced by the
real `t06_a9_extra_case`; the only difference between groups is the literal
expression (`'<s>'` vs `CONVERT('<s>' USING utf8mb3) COLLATE utf8mb3_general_ci`).
Deprecation warnings (1287) for utf8mb3 introducer-equivalent CONVERT are
recorded in the .out files; they do not affect comparison semantics.
