# T10-R4 closeout — 16915291

## Verdict

T10-R4 independent-acceptance candidate delivered. All four scoped
defect classes fixed with red→green tests, both rounds of independent
dual-axis review findings closed in place, bounded native verification
preserved for every load-bearing claim, and the directly affected
golden suites (T09, T10) re-run green at the bound SHA.

## Fixes vs. scoped defect classes

| Finding | Disposition |
|---------|-------------|
| R4-A: name-keyed mutations under `IndexNamesUnknown` retained stale members/names | Fixed — per-member `Index.NameUnknown` + `FindProvenIndex`; matched authored names apply, unmatched names unvouch, no stale name survives an unresolved transition |
| R4-B: `StoredIndexNames` projected names beyond native naming rules | Fixed — dialect-bound derivation (MySQL first-column/`functional_index`/ordered `_N`; TiDB `expression_index`/authored pre-occupation); unprovable members keep `NameUnknown` so lookups bind only proven names |
| R4-C: PRIMARY addressability conflated with clustered-storage refusal | Fixed — `PrimaryKeyIsHandle` (operable-object existence) split from `PrimaryKeyClustered` (8200 class incl. common-handle); `table_constraints` proves the PK object — `tidb_pk_type` conflates PK-less with nonclustered (verified v8.5.0) |
| R4-D: direction-unknown / partially modeled members lost facts | Fixed — `DirectionUnknown` members keep kind, expression identity, multi-valued marks, and aggregate counts end to end |
| R3 archive label corrections | Corrected — R3 CLOSEOUT/README/identity amended without fabricating historical evidence |

## Fixes vs. independent-review findings (round 1)

| Finding | Disposition |
|---------|-------------|
| Ambiguous `primary` rename could publish a post-state | Fixed — ambiguous binding unvouches the name space / member set + PK facts |
| Unresolved `RENAME … TO \`primary\`` left the matched member's old name proven | Fixed — `markIndexMemberNameUnknown` unvouches only that member's stored name |
| Hidden 1061 collision on add/create appended a maybe-absent member | Fixed — `IndexesUnknown` unvouches; merge still publishes when every member name is proven |
| Proven 1061 collision tombstoned the table | Fixed — `applyAddIndex`/`applyCreateIndex` keep the shape verbatim (refused before touching the table; `IF NOT EXISTS` no-ops) |
| TiDB `primary`-fold catalog row could mis-bind a member to PrimaryKey | Fixed — `resolveTiDBPrimaryName` probes `table_constraints`; multi-fold/unanswerable states withhold all three primary facts |
| TiDB `ADD PRIMARY KEY` under ambiguous `primary` binding | Fixed — only a proven `PrimaryNameBindsPK` publishes; anything else invalidates |
| Collection-granular existence gate skipped provable names | Fixed — `snapshotHasIndex`/`FindProvenIndex` bind proven names per member |

## Fixes vs. independent-review findings (round 2)

| Finding | Disposition |
|---------|-------------|
| `resolveTiDBPrimaryName` withhold arm unreachable (`count(*)` never `ErrNoRows`) | Fixed — any non-context probe error is the unanswerable case and withholds the three facts; the `tidb_indexes` clustered probe withholds its two on the same posture |
| `applyCreateIndex` published present-incomplete before the 1280 check | Fixed — the authored-`primary` refusal is checked before any entry lookup, mirroring `applyAddIndex` |
| `if_exists` early-return vs `ConstraintsUnknown` | Verified — `PrimaryKey==nil` under `!PrimaryKeyUnknown` is already a proven absence (every PK-scrubbing writer couples the flag); `!HasPrimaryKey()` can only defer to the conservative path — documented in place |
| Bare `IndexNamesUnknown` flag made `ResolvePrimaryNameBinding` ambiguous under all-proven names | Fixed — the member-owned question answers from the per-member bits; the bare flag alone no longer blocks `PrimaryNameBindsPK` |
| Standards nits (stale comment, helper naming, test banner) | Fixed — `applyAddIndex` merge comment corrected, `provenIndexMatch` documented as the positional form, `t10r4Apply` picks a dialect-matched banner |

## Evidence ledger

- Golden at bound SHA: T09 **125/125** cases (3442 assertions), T10
  **126/126** cases (3175 assertions) — `golden/T09-artifact.json`,
  `golden/T10-artifact.json`; T10 manifest `strict_equal` against the
  frozen oracle in `scripts/t10_contract.py`.
- Offline gates at bound SHA (`gates/`, rc quads): `go test ./...` = 0,
  `go vet ./...` = 0, `go test -race ./internal/...` = 0,
  three-level-doc = 0, `gofmt -l` flags only baseline files outside
  this diff, `golangci-lint` on touched packages reports the same 20
  baseline findings as R3 (none in new code paths).
- Native verification (`native-probe/`): `r4-bounded-probes` (M8/M9),
  `r4-m10-primary-name-probes` (member-first `primary` hits, handle
  coexistence, rename refusal classes, post-rename constraint naming —
  mysql:8.0.46 + pingcap/tidb:v8.5.0), `r4-pk-presence-probe`
  (`tidb_pk_type` NONCLUSTERED conflation vs `table_constraints`
  discrimination — the direct evidence for the provider correction).
  Every command and rc captured; probe fixtures cleaned after capture.
- Binary identities: golden-runner binary `8a226831…` (per
  `golden/*-artifact.json` `cli.sha256`, built at head_sha
  `16915291…`); work CLI `48fb6366…` (`bin/deltascope`, this tree).

## Deferred / explicitly out of scope

- T11 not started — this archive closes T10 at the R4 candidate only.
- `release-gofmt`-style baseline lint/gofmt findings are unchanged
  historical debt (recorded, not fixed here).
- Flag-only `IndexNamesUnknown` states (collection flag set, all member
  names proven) remain reachable through namespace-level unvouch paths;
  they are now answered by the per-member bits rather than treated as
  member uncertainty.
