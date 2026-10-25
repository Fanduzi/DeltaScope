# T10-R5 closeout — 2f37fbd4

## Verdict

T10-R5 independent-acceptance candidate delivered. All three scoped
defect classes fixed with red→green tests, all rounds of independent
dual-axis review findings closed in place (including one defect found
in the final fix-verification pass), bounded native verification
reproduced for every load-bearing claim, and the directly affected
golden suites (T09, T10) re-run green at the bound SHA.

## Fixes vs. scoped defect classes

| Finding | Disposition |
|---------|-------------|
| R5-A: name-keyed consumers bound `primary` literally; TiDB stores renamed PK objects under their real name | Fixed — `spec.ResolveIndexNameBinding`/`ResolvePrimaryNameBinding` shared across ordered-state add/create collision checks, drop/rename/visibility applies, `snapshotHasIndex`, existence predicates, and evidence-gap mirrors; member-first, PK-object-by-stored-name, handle-identity, freed-name, and ambiguous outcomes explicit |
| R5-A: provider reload lost the actual PK object name | Fixed — `resolveTiDBPrimaryName` separates constraint presence (`table_constraints`) from object attribution (unique STATISTICS folds + `tidb_indexes.CLUSTERED` + `key_column_usage`); ambiguity keeps presence but withholds identity (`PrimaryKey.NameUnknown`) and demotes candidates to `IndexKindUnknown` |
| R5-A: authored `CONSTRAINT symbol` conflated with object name | Fixed — the parser keeps the authored symbol for naming rules; derived snapshots and published PK objects normalize the stored name to `primary` |
| R5-B: provider probe failures silently withheld facts | Fixed — actual query/scan/connection failures propagate as load errors (all-rules-disabled included); clean absence, NULL, and inconsistent successful reads withhold only dependent facts |
| R5-C: `StoredIndexNames` missed MySQL's 61-byte suffix truncation | Fixed — dialect-split suffix branch (MySQL `61c+"_N"`, TiDB full-base), verified at the covered bases: MySQL 62c (→`61c_2`) and 63c (control), TiDB 62c (→`62c_2`) and 64c (→1059) |

## Fixes vs. independent-review findings

| Finding | Disposition |
|---------|-------------|
| Ambiguous `primary` rename unvouched member names but left `PrimaryKey.Name` proven — a successor `DROP INDEX \`PRIMARY\`` could publish a fabricated PK drop | Fixed — the rename unvouches the member name space AND `PrimaryKey.NameUnknown` separately; `PrimaryKeyObjectName` refuses the stale name and the successor binds ambiguous |
| `IndexNamesUnknown` consulted as PK-name-unproven | Fixed — the flag contract covers the member name space only; PK stored-name provability lives on `PrimaryKey.NameUnknown` (R4 fixtures with member-name uncertainty + proven `primary` now regress-pinned) |
| `indexExistenceRule.EvidenceGaps` owed no PK-side gap for non-`primary` ambiguous targets | Fixed — the ambiguous-binding scan covers all targets; `target_table.primary_key` emitted only where PK-side facts are unvouched, and only on TiDB/`primary` targets (MySQL member ambiguity owes `indexes` only — R4 matrix preserved) |
| Member `RENAME INDEX` destinations checked `primary` only — `RENAME INDEX s TO pk2` onto the PK object's stored name published an impossible coexistence | Fixed (final re-review defect) — destination freedom now resolves through `ResolveIndexNameBinding`: `BindsPKObject`/`BindsMember` refuse, `BindsAmbiguous` unvouches the member name, handle/freed names stay free; red-first test `TestT10R5MemberRenameOntoPKObjectNameRefused` pinned both arms |
| Demoted `IndexKindUnknown` candidates published invisible flips / satisfied DROP PRIMARY KEY member drops / passed add-create collision checks | Fixed — candidate-kind guards, member-first DROP PRIMARY KEY routing with PK-presence unvouch on demoted drops, and PK-uncertainty collision arms on add/create |
| Provider constraint probe failure conflated with `ErrNoRows` | Fixed — all real probe errors propagate; `ErrNoRows` is the stub's clean-absence signal only |
| Standards findings (inline triple-condition duplication, stale headers, holder name-text compares, `HolderIndex` fallback) | Fixed — shared `IndexNameSpaceUnvouched()` helper, L3 headers updated, all identity compares use pointer identity |

## Evidence ledger

- Golden at bound SHA: T09 **125/125** cases (3442 assertions), T10
  **126/126** cases (3175 assertions) — `golden/T09-artifact.json`,
  `golden/T10-artifact.json`, both recording `head_sha=2f37fbd4` and a
  consistent `cli.sha256` (`identity/golden-cli-sha256.txt`).
- Offline gates at bound SHA (`gates/`, rc quads): `go test ./...` = 0,
  `go vet ./...` = 0, `go test -race ./internal/...` = 0,
  three-level-doc = 0, `gofmt -l` flags only the same baseline files as
  R4, `golangci-lint` reports baseline classes only (the two R5-diff
  findings were fixed pre-archive).
- Native verification (`native-probe/r5-bounded-probes.log`): 16
  idempotent run blocks on mysql:8.0.46 + pingcap/tidb:v8.5.0 — 1
  version check plus 15 behavior probes covering renamed-PK `pk2`
  successors (VISIBLE rc=0, INVISIBLE 3522, common-handle `DROP INDEX
  pk2` rc=0 with `pk_left=0` independently proving object deletion,
  re-rename pk2→pk3 on a fresh table), freed `primary` (1091
  target-miss, member takeover rc=0, PK rename-back 1061), DROP
  PRIMARY KEY member-first on a handle table, nonclustered
  member-`primary` 1061 coexistence refusal, and the covered suffix
  bases (MySQL 62c→`61c_2`, 63c control; TiDB 62c→`62c_2`,
  64c→1059).
- Red→green (`red/`): the new R5 test files replayed on a `1d60d578`
  worktree produce 28 top-level behavioral FAILs (spec/rule/provider/
  audit), all green on the R5 tree — every failure is an assertion,
  not a compile error.

## Evidence caveats (kept honest)

- The original in-session native-probe and red-test transcripts were
  lost with the pre-commit session log. Both were reproduced: the probe
  script re-ran live against the same anchors (`native-probe/`), and
  the red set was replayed on a clean `1d60d578` worktree (`red/`).
  Nothing in the archive claims the original transcript bytes.
- Wording corrections appended in T10-R6 (no evidence re-run needed):
  the `a1c2` block stopped at its first statement — `DROP PRIMARY KEY`
  answered 1091 and the trailing `RENAME INDEX pk2 TO pk3` never
  executed, so it is not a "double successor confirmation"; the object
  deletion is proven independently by `pk_left=0` in `a1c`. The suffix
  probes cover exactly the bases listed above, not a full 61–64-byte
  matrix on either anchor. The `red/` replay produced 28 top-level
  behavioral FAILs — a count of failed test units, not of every inner
  assertion.
- Golden ran twice: once pre-commit on identical content and once at
  the amended bound SHA — the archived artifacts are the bound-SHA
  run.
