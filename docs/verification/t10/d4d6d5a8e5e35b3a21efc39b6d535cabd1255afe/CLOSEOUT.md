# T10-R2 closeout — deferred acceptance restored for issue #89

Base: 8d75e6b2212f4696e06a9acb2c039414ec147a2b (milestone base, T10 R0
delivered; this round finishes the #89 re-audit responsibilities)
Product code: 5a918ac2 + d4d6d5a8 (style-only, behavior-neutral)
Contract/oracle: 8cba6c1c (126-case oracle, CONTRACT/ADR/L2 sync) +
1135e3f5 (T03–T05 shared-corpus expects for graduated member forms)
Bound SHA: d4d6d5a8e5e35b3a21efc39b6d535cabd1255afe
(T03–T05 artifacts record head 1135e3f5 — they ran before the
behavior-neutral style commit; see archive README for the split note)

## Scope delivered

- `PrimaryRoleResolution` four-state model: explicit primary,
  proven implicit holder, proven no candidate, and unknown/unresolved.
  Holder identity carries the member/index pointer (`HolderIndex`) so
  two unnamed UNIQUE indexes never collide on the empty name.
- `UnresolvedTransition` publication gate: any feature-bearing
  transition whose native outcome cannot be proven (candidate-order
  gaps, unresolvable handle, missing required member facts, missing
  version for a gated form) tombstones the ordered post-state instead
  of publishing success. `InvalidTransition` likewise withholds
  publication for determined refusals.
- TiDB PK-handle tri-state: `PrimaryKeyIsHandle` +
  `PrimaryKeyHandleUnknown`. Explicit `CLUSTERED`/`NONCLUSTERED`
  overrides session mode; unstated declarations consume observed
  `tidb_enable_clustered_index` via `InstanceFacts`; missing mode,
  missing clustered evidence, or unmatchable member shape stays unknown
  — never guessed. `ResolveIndexPrimaryObject` answers real-object vs
  PKIsHandle vs unresolved for `ALTER INDEX PRIMARY`.
- Kind-aware member legality (`assessIndexMembers`): FULLTEXT text
  members bypass the generic prefix-required rule; non-text FULLTEXT
  members produce `fulltext_column_type`; SPATIAL requires geometry
  family + NOT NULL (`spatial_column_type`/`spatial_nullable`); JSON
  direct indexing is `json_column_index`; prefix legality checks
  zero (`prefix_zero`, any kind — FT(0) natively refused),
  non-string (`prefix_non_string`), and declared-length overflow
  (`prefix_too_long`); expression members verify resolvable column
  references (`expression_column_missing`); TiDB GLOBAL requires
  unique/primary kind plus partitioned table (`global_not_unique` /
  `global_not_partitioned`, unknown partition → gap).
- Provider member classification triage: SQL NULL catalog member →
  expression part; literal `NULL` with expression text → expression;
  literal matching a real column (including a real column named
  ``NULL``) → column member; anything else → unmodeled. Members never
  fabricated from name collisions.
- Multi-valued cast facts: `CAST(x AS <T> ARRAY)` element-type capture
  through `MultiValuedCastFacts` on both declared parts (extractor) and
  catalog expressions (provider) — with the dialect-pinned allow/deny
  matrix (MySQL accepts DECIMAL rejects DOUBLE; TiDB accepts DOUBLE
  rejects DECIMAL).
- Coverage graduation: expression, prefix, descending, multi-valued,
  and TiDB GLOBAL member forms left blanket-unsupported aspect status
  and are audited; the remaining bounded surfaces are
  `unmodeled_parts`, predicates, MySQL GLOBAL, MySQL `primary_key_type`,
  unmodeled options, TiDB FULLTEXT/SPATIAL, and vector/columnar kinds.
- Ordered-state member facts: graduated members persist into post-state
  and next-statement pre-state; non-plain members remain excluded from
  plain-key pair comparison, keeping `target_table.indexes.key_parts`
  evidence gaps on duplicate/left-prefix comparisons.
- T03–T05 hand-frozen manifests resynced for the shared CLI corpus
  (10 cases × 3): graduated forms now expect audited pass/complete;
  MySQL GLOBAL, `unique_global` column option, `SET GLOBAL`, and
  temporary-table boundaries unchanged.

## Verified at the bound SHA d4d6d5a8

- Golden: T10 **126/126** cases, 3,175 assertions; regression T03 118,
  T04 163, T05 247, T06 496, T07 207, T08 172, T09 125 — all pass
  (golden/*/artifact.json, golden/regression-all.log). T03–T05 bind
  `1135e3f5`, T06–T10 bind `d4d6d5a8`; the delta is a style-only commit.
- Unit/spec: `go test ./...` all ok / 0 FAIL; `pg-unit-test-gates` and
  `pg-confidence-gates` green (unit + e2e CLI/HTTP/MCP); focused T10-R2
  suites green across spec/audit/provider; race clean.
- Contract: 938 constructed validator checks / 0 failures
  (`test_ddl_golden.py`), 33/33 T10 control checks, contract drift
  test green.
- Gates: sql-corpus, query-access-corpus, inventory, catalog,
  parser-error + unsupported-diagnostics evidence, docs examples,
  decision-record, three-level-doc, go vet, dialect-hygiene — rc=0
  each (gates/*.log).
- `release-gofmt-gate`: 5 violations, all pre-existing baseline files
  outside this diff (two `batch_state_t07_*_test.go` + three archived
  t06 `probe/main.go` scratch fixtures). `golangci-lint` on the touched
  packages: four R2 items fixed in `d4d6d5a8`; the remainder are
  baseline findings on pre-milestone commits.
- Native pins: `native-probe/r2-consolidated-pins.log` — implicit-PK
  promotion matrix, explicit-PK-frees-later-unique, TiDB
  PRIMARY-object truth table (1176 on PKIsHandle),
  `tidb_enable_clustered_index` mode observation, member-legality error
  table (1170/1089/1391/3152/1283/1687/1252/1054/8200/3522/1176),
  FT(5)-accepted / FT(0)-refused, TiDB descending-accepted-stored-
  ascending, multi-valued cast allow/deny per dialect.

## Corrections found in this round

- The blanket `expr`/`prefix`/`desc`/`multi_valued` unsupported aspects
  were stale posture, not a product boundary — members are auditable;
  graduated with kind-aware legality instead of statement-level
  unsupported.
- Explicit-primary statements previously collapsed to "no holder" —
  the four-state model separates explicit PK, proven holder, proven
  no-candidate, and unresolved.
- Unnamed unique indexes collided on name-keyed holder identity —
  holder now binds the member pointer.
- TiDB literal `NULL` catalog members could masquerade as columns —
  triage now separates SQL NULL, expression text, and a real column
  named ``NULL``.
- TiDB PK handle was binary true/false — replaced by the tri-state
  (explicit keyword / session mode / unknown) so `ALTER INDEX PRIMARY`
  unresolved outcomes withhold publication.
- FULLTEXT text members were rejected by the generic prefix-required
  path — kind-aware routing fixed the classification.

## Known bounds (unchanged posture)

- `unmodeled_parts`, predicates, MySQL GLOBAL, MySQL `primary_key_type`,
  unmodeled index options, TiDB FULLTEXT/SPATIAL (8200), and
  vector/columnar index kinds stay bounded unsupported evidence.
- Multi-valued member/query *semantics* beyond cast identity stay
  bounded.
- Expression-member internal equivalence is not evaluated.
- `UnmodeledKeyParts` members produce bounded gaps, never guessed
  answers.

## Environment notes

- T07–T10 anchors need `T07/T08/T09/T10_MYSQL_PASSWORD` exported
  (fixture-credential contract).
- The Docker VM disk-exhaustion cascade (dangling anonymous volumes
  from `rm -f` cleanup) is documented in the archive README; resolved
  by removing 121 dangling volumes (~17.5 GB). Named volumes untouched.

## Delivery state

Two-axis review ran mid-round; all blocking findings closed. This
archive closes T10-R2 only — no merge, tag, release, or issue-closure
action; branch `milestone/mysql-tidb-ddl-completion` fast-forwards to
`origin/workfree` as the T10-R1/T10-R2 candidate. T11 is out of scope.
