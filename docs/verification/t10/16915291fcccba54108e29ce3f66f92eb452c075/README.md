# T10-R4 verification archive — 16915291

Bound SHA: `16915291fcccba54108e29ce3f66f92eb452c075`
(`fix(ddl): T10-R4 per-member name provability, primary binding, post-state truth`)

Base: `daad03bbb0a482b2b99ac90b5c2b3af89c417039` (T10-R3 archive commit;
this round is the fact-propagation/post-state correction pass on top of
the accepted R3 surface — no T10 expansion, no T11 work)

Scope: T10-R4 independent-acceptance candidate for issue #89 — four
bounded defect classes plus two rounds of independent-review
corrections:

- **A — post-state truth under an unproven name space.** When
  `IndexNamesUnknown` holds while `IndexesUnknown` does not, name-keyed
  mutations keep member truth: a matched authored name applies, an
  unmatched name unvouches instead of retaining stale members, and no
  transition retains an index name the product may never have stored.
- **B — stored-name provability.** `spec.StoredIndexNames` derives
  product stored names only where native naming rules support the
  conclusion (MySQL first-plain-column / `functional_index` /
  `primary_2` collision suffix; TiDB `expression_index` and authored
  name pre-occupation); anything unprovable marks the member
  `Index.NameUnknown` so name-keyed consumers bind only proven names.
- **C — PRIMARY addressability vs clustered storage.** TiDB
  `PrimaryKeyIsHandle` (row-id promotion, no operable index object) is
  modeled independently from `PrimaryKeyClustered` (the DROP PRIMARY
  KEY 8200 refusal class covering PKIsHandle and common-handle shapes
  alike) — a clustered non-single-integer primary key stays an
  addressable real object while its drop stays refused.
- **D — member fidelity under partial modeling.** Expression members
  keep kind, expression identity, multi-valued marks, and aggregate
  counters (`PrefixParts`/`DescParts`/`UnmodeledKeyParts`) when catalog
  direction is unproven or key parts are only partially modeled;
  provider `NULL`/`'NULL'` classification never demotes a member.

Review-pass corrections folded into the same diff:

- One shared `spec.ResolvePrimaryNameBinding` tri-state
  (member / PK / ambiguous) answers every `primary` name-keyed consumer
  across post-state publishers, the three `*TargetExists` rule
  predicates, and the advanced-index assessment — member-first on TiDB
  because a secondary can literally own `primary`.
- TiDB PK-object presence is proven by the
  `information_schema.table_constraints` `PRIMARY KEY` row — the TiDB
  v8.5.0 anchor proved `tables.tidb_pk_type` conflates PK-less tables
  with real nonclustered primaries (both NONCLUSTERED). Zero
  `primary`-fold STATISTICS rows prove no PK object; a proven
  constraint plus exactly one fold promotes it; proven-absent keeps
  folds secondary; multi-fold or unanswerable-probe-with-candidate
  states withhold `PrimaryKeyUnknown`/`PrimaryKeyHandleUnknown`/
  `PrimaryKeyClusteredUnknown` together. Probe failures are themselves
  the unanswerable case (`count(*)` can never be empty) — they withhold
  rather than fail the table load, context cancellation aside.
- Hidden collisions unvouch; proven collisions keep. A member-adding
  transition under an unproven member name space can hit a hidden 1061
  — the member set unvouches rather than appending a maybe-absent
  member — while an authored name provably held by an existing member
  keeps the shape verbatim (1061 refused, or `IF NOT EXISTS` no-ops).
  `CREATE INDEX \`primary\`` checks the 1280 refusal before any entry
  lookup, so a provably untouched table keeps whatever state it held.
- Rename honesty: an unresolved `RENAME … TO \`primary\`` destination
  unvouches the matched member's stored name
  (`markIndexMemberNameUnknown`) instead of leaving the old name
  falsely bindable; ambiguous `primary` bindings unvouch the member set
  plus the PK facts where a droppable object cannot be ruled out.
- Per-target evidence gaps: `indexMemberExistenceGaps` owes
  `target_table.indexes` only for targets with no proven match under an
  unproven sibling name (or an unvouched member set) — a statement
  whose targets all bind proven names answers without paying an
  unrelated collection gap, and a `primary` target on the resolving
  variant owes the member fact exactly when the binding is ambiguous.
  The implicit-primary visibility assessment binds its target by
  `FindProvenIndex` and the holder by `HolderIndex` identity.

## Layout

- `identity/` — bound head SHA (`head-sha.txt`), toolchain version
  (`work-cli-goversion.txt`), worktree status at archive time
  (`worktree-status.txt`), and the two distinct CLI identities:
  `golden-cli-sha256.txt` records `8a226831…`, the sha256 of the binary
  the golden runner built (as recorded in both
  `golden/*-artifact.json` files), while `work-cli-sha256.txt` records
  `48fb6366…`, the sha256 of the separately built work CLI at
  `bin/deltascope`.
- `golden/` — `ddl_golden.py` artifacts re-run at the bound SHA:
  **T09 125/125 cases (3442 assertions), T10 126/126 cases (3175
  assertions)**. The T10 manifest equals the frozen independent oracle
  in `scripts/t10_contract.py` (`manifest strict_equal: True`,
  including the `PRIMARY_GAPS_MYSQL`/`PRIMARY_GAPS_TIDB` fact sets
  graduated this round).
- `gates/` — offline gates at the bound SHA: `go test ./...`, `go vet
  ./...`, `go test -race ./internal/...`, `gofmt -l`, `golangci-lint`
  on the touched packages, and the three-level documentation loop
  check — each captured as command/stdout/stderr/rc quads.
- `native-probe/` — bounded native verification scripts and verbatim
  transcripts against mysql:8.0.46 and pingcap/tidb:v8.5.0:
  `r4-bounded-probes.sh`/`.log` (M8/M9 member-fact and refusal
  matrices), `r4-m10-primary-name-probes.sh`/`.log` (the `primary`
  name-space matrix — member-first hits, handle coexistence, rename
  refusal classes, constraint naming after rename), and
  `r4-pk-presence-probe.sh`/`.log` (the `tidb_pk_type` conflation and
  `table_constraints`/`tidb_indexes` discriminating probes that drove
  the provider correction). Probe fixtures were created under names
  `pp_*`/`t10*` and cleaned up after capture.

## Gate summary (all at bound SHA)

`go test ./...` 0 failures across all packages; `go vet ./...` clean;
`go test -race ./internal/...` clean; three-level-doc OK; T09+T10
golden 251/251 cases. `golangci-lint` on the touched packages reports
the same 20 pre-existing baseline findings as the R3 baseline (none in
this diff's new code paths), and `gofmt -l` flags the same baseline
files untouched by this diff — both recorded verbatim in `gates/`.

## Independent review

Two dual-axis (standards + spec) reviews ran against the frozen
`daad03bb..HEAD` diff. Round one: standards PASS-with-nits (fixed in
place); spec FAIL — two majors (stale member name after unresolved
TiDB rename-to-`primary`; proven-1061 tombstoning) plus one minor
(collection-granular existence gating) — all fixed with regression
tests. Round two: standards PASS (no hard violations; judgement items
fixed or documented); spec PASS-with-edges — the unanswerable-probe
arm's unreachable `ErrNoRows`, the `applyCreateIndex` refusal ordering
on unknown entries, and an `if_exists`/`ConstraintsUnknown` interaction
were each verified and either fixed (first two) or documented with the
coupled-flag invariant (third, plus the bare-flag nit in
`ResolvePrimaryNameBinding`).
