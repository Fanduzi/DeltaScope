# T07 versioned sources and bounded observations

Research date: 2026-10-19 UTC. Scope: MySQL 5.7.44, 8.0.46, 8.4.10 and TiDB
8.5.0 only; no image, parser dependency, SQL mode, or server configuration
upgrade is proposed. These sources inform the contract; documentation is not
native execution evidence, and the old product's output is not the new oracle.

## Primary sources

| Source / version | Exact URL | Decisive facts |
|---|---|---|
| MySQL 5.7 official archived reference | https://downloads.mysql.com/docs/refman-5.7-en.pdf | Header identifies the 5.7 manual (document revision 84735, generated 2026-07-24, describing through 5.7.52); generated-column overview and reserved-keyword history. Our native anchor is still exactly 5.7.44, not every patch described by the manual. |
| MySQL 5.7.6 release notes | https://dev.mysql.com/doc/relnotes/mysql/5.7/en/news-5-7-6.html | Generated columns were introduced in 5.7.6, including VIRTUAL/STORED and catalog facts. The boundary is used only in constructed version controls; no 5.7.5/5.7.6 server is claimed tested. |
| MySQL 8.0 defaults | https://dev.mysql.com/doc/refman/8.0/en/data-type-defaults.html | Expressions are supported from 8.0.13; the prior behavior is literal-only except current timestamp for temporal columns. Column references have ordering/auto-increment restrictions; subqueries, parameters, variables, stored/loadable functions are not valid expression defaults. |
| MySQL 8.0 visibility | https://dev.mysql.com/doc/refman/8.0/en/invisible-columns.html | VISIBLE/INVISIBLE and ALTER COLUMN SET visibility are available from 8.0.23. Invisible columns still exist and may be explicitly indexed/referenced. EXTRA and LIMIT-0 result columns reveal effective visibility. |
| MySQL 8.4 generated CREATE | https://dev.mysql.com/doc/refman/8.4/en/create-table-generated-columns.html | Omitted storage is VIRTUAL; deterministic expressions and static local references are allowed, generated-to-generated references must point backward, and forbidden constructs are not made legal by parser acceptance. |
| MySQL 8.4 generated ALTER | https://dev.mysql.com/doc/refman/8.4/en/alter-table-generated-columns.html | ADD/MODIFY/CHANGE have distinct restrictions; VIRTUAL/STORED conversion is not a generic ordinary-column replacement. MySQL supports ADD STORED and ordinary-to-STORED, unlike the tested TiDB anchor. |
| TiDB release-8.5 generated columns | https://raw.githubusercontent.com/pingcap/docs/release-8.5/generated-columns.md | CREATE VIRTUAL/STORED and indexes on generated columns; ALTER ADD STORED and stored-expression changes are restricted. Nondeterministic and stateful functions are forbidden. The stored-to-ordinary discrepancy below must not be silently hidden. |
| TiDB release-8.5 default values | https://raw.githubusercontent.com/pingcap/docs/release-8.5/data-type-default-values.md | Function-expression support is restricted, not arbitrary MySQL arithmetic/column-reference parity. JSON_OBJECT/ARRAY/QUOTE are documented positives for JSON defaults. CREATE support does not establish ALTER SET DEFAULT support. |
| TiDB release-8.5 compatibility | https://raw.githubusercontent.com/pingcap/docs/release-8.5/mysql-compatibility.md | Product differences must remain explicit; no MySQL positive may substitute for TiDB evidence. |

The retrieved TiDB branch resolved to commit
`5aa2014145162f6c26e1d9bbbe2d1ae3b7972ec4`; raw files, response headers,
retrieval timestamps, final URLs, status/redirect records and hashes are under
`/tmp/ds-t07-proof/references/`. The live `release-8.5` branch can contain later
8.5 patch documentation; the 8.5.0 native observations are independently pinned.

The 5.7 HTML endpoints returned a newer 26.7 reference through web retrieval.
They were rejected as 5.7 evidence. The official archive PDF was downloaded
and decoded using the existing macOS PDFKit, without installing dependencies.
An attempted official `.info.gz` URL returned 404; that failure is retained.
Context7 was used to locate relevant documentation, but its purported 5.7
source snippets resolved to `trunk` and are not used as version-pinned proof.

## Local source baseline

- Task base: `7a65fe162695ce1248d6edfb825b04e6b81b48b8`.
- T06 closeout docs commit: `1997dac1ae5ec5fe6abefdb7f9868e1a15999a47`.
- Locked parser: `github.com/pingcap/tidb/pkg/parser`
  `v0.0.0-20260622124716-26db2d85d6cd`.
- The AST retains generated expression and Stored facts, but the extractor
  marks generated as unextracted. Current `Column` has no MySQL generated
  dependencies or column visibility facts; its `GeneratedWhen` is an existing
  PostgreSQL fact and must not be overloaded.
- The locked AST has index visibility but no column-visibility option. Valid
  MySQL visibility definitions currently fail parsing. The T07 parser adapter
  must add a narrow structural grammar, not regex inference from parse errors.
- The locked DEFAULT grammar also rejects `(1+2)` and `(a+1)` in the valid
  MySQL8 native cases. The supplement must preserve real expression ASTs, not
  merely recover visibility or copy a literal placeholder into the model.
- Existing ordered state clones known CREATE facts and conservatively refuses
  unrecomputable dependencies. The T07 work must deepen those existing seams,
  not introduce a second state machine or transport-specific analyzer.

## Native observation provenance

The baseline was built from a reduced `git archive` of the task-base commit,
plus the explicitly hashed fixed probe source. Actual `go version -m` contains
no VCS fields for these archive binaries; none are invented. Source archive,
probe source, build command, binary hashes, and raw build info are separate
records. A first archive pathspec mistakenly included an absent VERSION file;
its rc=128/stderr are retained. The corrected archive used existing paths only.

Raw observations: `/tmp/ds-t07-proof/native-baseline/<anchor>/<role>/`.
All 34 roles were collected on each anchor, serially, using only self-created
empty `golden.t07_probe` tables, no INSERT, no arbitrary expression execution,
and optional LIMIT-0 projection checks. Current parser/extractor/AuditSQL
records are separate from native DDL results. Collection rc=0 means records
were captured, not that a feature is supported or the old product is correct.
Native errors retain actual MySQL number, SQLSTATE and message; query/scan/
Rows.Err/write/cleanup errors are not interpreted as feature negatives.
Four additional roles per anchor are retained under native-supplement:
G17/G18 referenced-base CHANGE/RENAME and D10/D11 UUID setter/JSON-default DROP.
Together these are 38 roles × four anchors = 152 observation bindings, not 152
product acceptance cases. MySQL5.7 G17 returns1054 (unknown referenced name),
while the other anchors return3108; G18 returns1064 on5.7 and3108 elsewhere.
TiDB8.5.0 accepts ALTER UUID on VARCHAR and DROP a CREATE-defined JSON default.
These distinct ALTER positives do not establish ALTER SET JSON support.

The catalog's column-label case differs across products. A lead review uses a
label-normalized view only; no value, member byte, SQL, raw record or hash is
lowercased or changed. The initial review's KeyError is retained as a review
script failure, not a database or product result.

## Decisive differences already observed

- All four anchors accept the generated STORED seed and reject dropping its
  referenced base column with error 3108. The unrelated-c control succeeds.
- VIRTUAL, omitted-storage VIRTUAL, direct/short-transitive dependencies,
  quoted identifiers, string-literal nonreferences and generated-column
  indexes have separate observations.
- TiDB ADD STORED and stored-expression MODIFY reject with 3106; same-expression
  stored CHANGE renaming is accepted. VIRTUAL-to-STORED rejects on all four.
- MySQL 5.7.44 rejects the expression-default candidates with syntax 1064.
  MySQL 8.0.46/8.4.10 accept constant and column-reference expressions; dropping
  a referenced default-expression base rejects with 3766.
- TiDB 8.5.0 rejects arbitrary arithmetic/column-reference CREATE defaults, but
  accepts JSON_OBJECT in CREATE. Its ALTER SET JSON expression default rejects
  with 1101; that is not a valid TiDB ALTER positive.
- Explicit column visibility is accepted by MySQL 8.0.46/8.4.10 and rejected
  with syntax 1064 by MySQL 5.7.44 and TiDB 8.5.0. No successful-but-ignored raw
  VISIBLE/INVISIBLE form was observed; the contract must not invent one.
- On MySQL 8.0.46/8.4.10, MODIFY/CHANGE with omitted visibility resets an
  invisible column to visible. This effective-state transition has native
  catalog and LIMIT-0 projection evidence; it is not an assumed inheritance.

MySQL8.0/8.4 raw generation/default catalog text also escapes quote delimiters
(e.g. `_utf8mb4\'a\'` and `json_object(_utf8mb4\'k\',1)`). Preserve
both text and HEX; a label-normalized view does not decode or rewrite them.
The provider's derived AST facts must handle that metadata representation
without globally dropping literal backslashes or declaring unknown empty.

### Recorded documentation/native discrepancy

The TiDB release-8.5 generated-columns page says stored-generated to ordinary
conversion is unsupported. On the fixed 8.5.0 image, the exact empty-table
G14 input succeeds and the following catalog read shows both generated EXTRA
and GENERATION_EXPRESSION removed. This is a real definition-level difference,
not merely an ignored attribute or an exit-code-only inference. The T07
contract must pin the observed 8.5.0 behavior and explicitly avoid inferring
support for all other 8.5 patches. No previously frozen #79 public result
contract defines that conversion, so this finding can be settled internally
before freezing T07; it is not a reason to alter accepted public statuses.

The final contract and independent oracle will reference the bounded native
records and primary claims above. They will not copy the old product's missing
finding/unsupported/parser-error outputs into future expected behavior.
