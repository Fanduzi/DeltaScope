# T10-R1 closeout — scope restoration for issue #89

Base: 8d75e6b2212f4696e06a9acb2c039414ec147a2b (milestone base, T10 R0
delivered; this round restores the deferred #89 responsibilities)
Product code: e70881d0 + 64e750dc + 2e7b63fb
Contract/oracle: 52925171 (123-case freeze) + 4c725708 (126-case freeze,
expression-unique split pins) + 3a92a02a (post_verify phase fix)
Bound SHA for all golden artifacts: 3a92a02a5d0c8bac7c26871b684d8f5aa838e3ab

## Scope delivered

- `Index.KeyParts` member-level facts: ordinal, kind
  (column|expression|unmodeled), bound column, `PrefixLength *int`
  (zero distinguishable), descending flag, structured `ExpressionFacts`
  (identity, references, functions, top form, fingerprint, knownness),
  multi-valued flag. Aggregate marks (`PrefixParts`, `DescParts`,
  `ExpressionCount`, `MultiValuedParts`, `UnmodeledKeyParts`) derive via
  `SetKeyParts`. Extractor (source declarations) and provider (catalog)
  both populate it.
- Implicit primary-key role (`ResolvePrimaryRole`): first qualifying
  `UNIQUE NOT NULL` index carries the role with no explicit PK —
  qualifying includes prefix members; expression members split by
  dialect (TiDB hidden columns never qualify → definite disqualify;
  MySQL hidden generated-column nullability is unprovable → bounded
  gap). Multiple candidates need declaration/catalog order;
  provider name-sorted snapshots emit `target_table.indexes.ordinal`
  instead of guessing. `HasHolder` distinguishes resolved-no-holder
  from an empty-named holder.
- TiDB PRIMARY-object resolution (`ResolveIndexPrimaryObject`):
  NONCLUSTERED and VARCHAR/commonHandle clustered PKs are real index
  objects — `VISIBLE` is a successful no-op, `INVISIBLE` refuses (3522);
  single-integer clustered PKIsHandle has no PRIMARY index object (1176).
  Provider loads `PrimaryKeyIsHandle` from
  `information_schema.tidb_indexes.CLUSTERED` + single-integer PK shape.
- Member-legality family inside
  `ddl.index.advanced.compatibility.require`: `prefix_required`
  (missing prefix on TEXT/BLOB members → issue, explicit/zero prefix
  distinguishable), JSON direct index classification
  (`json_column_index` under prefix_length handling), FULLTEXT
  text-family members, SPATIAL geometry-family + NOT NULL members,
  expression-member dependency columns, multi-valued `CAST(... ARRAY)`,
  descending member-order positions (`(a DESC,b)` ≠ `(a,b DESC)`),
  TiDB GLOBAL legality (unique/primary kind + partitioned table;
  unknown partition state → bounded gap; MySQL never probes partitions
  for it). Feature-gap attribution follows the fact's owning feature.
- Ordered batch state: `IndexOrderCatalog` marks declaration/catalog
  ordering; invalid transitions tombstone instead of publishing illegal
  post-state. `Partitioned *bool` keeps unknown state honest.
- Provider fidelity: TiDB literal-`"NULL"` `STATISTICS.column_name`
  classified as an expression member (not a fabricated column named
  NULL); expression text/multi-valued preserved; SPATIAL/FULLTEXT
  `INDEX_TYPE`, `SUB_PART`, collation direction, member ordering kept.
- Every original case ID retained; contract evolved append-only
  81 → 123 → 126 (frozen ID SHA-256
  `3d526e8ef8199b18fbbf39fa1af024192948fee3a0ebefeea6d086c330329311`).

## Verified at the bound SHA 3a92a02a

- Golden: T10 **126/126** cases, 3,214 assertions; regression T03 118,
  T04 163, T05 247, T06 496, T07 207, T08 172, T09 125 — all pass,
  every artifact `head_sha=3a92a02a5d0c` (golden/*/artifact.json,
  golden/regression-all.log).
- Unit/spec: `go test ./...` all ok / 0 FAIL; `pg-unit-test-gates`
  green; focused T10-R1 suites 29 tests green; race clean on the five
  touched packages.
- Contract: 758 constructed validator checks / 0 failures
  (`test_ddl_golden.py`), 33/33 T10 control checks, T07/T08/T09
  contract validators green.
- Gates: inventory, corpus, coverage catalog, census, parser-error
  contract, unsupported-diagnostics evidence, parser-error feasibility,
  docs examples, decision-record, three-level-doc — rc=0 each
  (gates/*.log).
- Native pins: `native-probe/r2-consolidated-pins.log` — MySQL/TiDB
  implicit-PK promotion matrix, the dialect split on expression-unique
  promotion (MySQL 3522 vs TiDB allow), TiDB PRIMARY-object truth
  table, `TIDB_INDEXES.CLUSTERED`/`TIDB_PK_TYPE` shapes, and the
  member-legality error table (1170/3152/1283/1687/1252/1054/8200).

## Corrections found in this round

- Contract wording "TiDB PRIMARY does not exist" was a false
  generalization — only PKIsHandle lacks the object; corrected by
  product observation (ncp/vpk probes) + TiDB source pins.
- MySQL promotes expression-member UNIQUE indexes to the implicit role
  (hidden generated column is NOT NULL) but the hidden column's role
  facts are unobservable to us — modeled as a bounded gap, not a
  disqualification; TiDB deterministically disqualifies.
- Review fixes: `isIntegerType` normalizes `INT(11)`-style spellings;
  `ResolvePrimaryRole` refuses to order-guess on name-sorted provider
  catalogs; GLOBAL legality evaluated before member-column collection
  so gaps attribute to the right feature family; MySQL never probes
  partition state; TiDB descending is not version-gated; primary
  invisibility is version-independent (3522 regardless of version
  knowledge).

## Known bounds (unchanged posture)

- Multi-valued member/query semantics stay bounded unsupported
  evidence, not a version-only answer.
- Expression-member internal equivalence is not evaluated.
- TiDB FULLTEXT/SPATIAL remain vendor boundaries (8200); MySQL
  `primary_key_type` is a TiDB-only boundary.
- `UnmodeledKeyParts` members are unprovable → bounded gaps, never
  silent disqualification.

## Delivery state

Two-axis review ran mid-round; all blocking findings closed (fixes
listed above). This archive closes T10-R1 only — no merge, tag,
release, or issue-closure action; branch
`milestone/mysql-tidb-ddl-completion` fast-forwards to `origin/workfree`
as the T10-R1 candidate. T11 is out of scope.
