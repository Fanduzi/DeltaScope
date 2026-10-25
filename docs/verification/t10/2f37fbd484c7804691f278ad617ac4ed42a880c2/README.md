# T10-R5 verification archive — 2f37fbd4

Bound SHA: `2f37fbd484c7804691f278ad617ac4ed42a880c2`
(`fix(ddl): T10-R5 actual PK object names, provider error propagation, MySQL suffix bounds`)

Base: `1d60d57849978050b519b8cdc856142606f9b045` (T10-R4 archive commit;
this round is the actual-stored-name / provider-fidelity / suffix-boundary
correction pass on top of the accepted R4 surface — no T10 expansion,
no T11 work)

Scope: T10-R5 independent-acceptance candidate for issue #89 — three
bounded defect classes plus three rounds of independent-review
corrections:

- **R5-A — actual PK object names through every consumer.** TiDB stores
  a renamed primary index object under its real catalog name (verified
  v8.5.0: `RENAME INDEX \`PRIMARY\` TO pk2` moves the object to `pk2`
  while constraint identity stays `PRIMARY`). Ordered-state successors,
  provider reload, and every name-keyed predicate now share one binding
  resolver (`spec.ResolveIndexNameBinding` / `ResolvePrimaryNameBinding`)
  with explicit member-first / PK-object / handle-identity /
  freed-name / ambiguous outcomes. A freed `primary` binds nothing, a
  member literally named `primary` wins member-first on TiDB, a handle
  PK claims logical identity without an operable object, and no path
  fabricates presence, absence, or a successor transition on an
  unvouched name space. Authored `CONSTRAINT symbol PRIMARY KEY` stays
  on the parsed spec for naming rules while derived snapshots and
  published PK objects normalize the stored object name to `primary`.
- **R5-B — provider error propagation and unknown semantics.** TiDB
  primary attribution separates constraint presence
  (`information_schema.table_constraints`) from object identity
  (unique STATISTICS folds disambiguated by `tidb_indexes.CLUSTERED`
  then `key_column_usage` column lists). Exactly one unique candidate
  promotes; ambiguous attribution keeps PK presence but withholds
  object identity (`PrimaryKey.NameUnknown`) and demotes candidate
  folds to `IndexKindUnknown`. Actual query/scan/connection failures
  propagate as load errors — including with every rule disabled —
  while clean `count(*)=0`, NULL, and inconsistent successful reads
  withhold only the dependent facts.
- **R5-C — MySQL auto-generated suffix bounds.** `StoredIndexNames`
  applies MySQL's `make_unique_key_name` truncation: colliding bases
  >61 bytes store `61c+"_N"` (verified 8.0.46: 62-byte base →
  `61c+"_2"`=63). TiDB does not truncate — a 62-byte base derives
  `62c+"_2"`=64 (verified), while 63/64-byte derivations exceed the
  identifier limit natively (1059 verified) and stay unprovable.

Review-pass corrections folded into the same diff (rounds noted in
`red/` and commit history; final re-review verified all dispositions):

- `applyRenameIndex` `PrimaryNameBindsPK` + unvouched member space now
  unvouches the member name space AND `PrimaryKey.NameUnknown`
  separately — the rename either moved the object or refused on a
  hidden 1061, so the stored name is unprovable; a successor
  `DROP INDEX \`PRIMARY\`` can no longer publish a fabricated PK drop.
- Member `RENAME INDEX` destinations check the unified binding for
  every name — `RENAME INDEX s TO pk2` where the PK object is stored
  as `pk2` refuses (1061) instead of publishing an impossible member.
- `IndexNamesUnknown` documents and covers the member name space only;
  the PK object's stored name has its own marker — the earlier flag
  blend that mis-titled member-name uncertainty as PK-name uncertainty
  was corrected and is pinned by regression tests.
- `indexExistenceRule.EvidenceGaps` mirrors `Evaluate` per target:
  a `primary` or TiDB non-`primary` target whose ambiguity could
  resolve through the PK side owes `target_table.primary_key`, while
  MySQL ordinary-member ambiguity owes only `target_table.indexes`.
- Demoted `IndexKindUnknown` candidates cannot publish invisible
  flips or satisfy DROP PRIMARY KEY member drops without unvouching
  PK presence; add/create index under unprovable PK presence or name
  unvouches the member set (hidden 1061 may mean the write never
  landed).

## Layout

- `identity/` — bound head SHA (`head-sha.txt`), toolchain version
  (`work-cli-goversion.txt`), worktree status at archive time
  (`worktree-status.txt`), and the two distinct CLI identities:
  `golden-cli-sha256.txt` records `ed42a3fd…`, the sha256 of the binary
  the golden runner built (as recorded verbatim in both
  `golden/*-artifact.json` files), while `work-cli-sha256.txt` records
  `8a9af301…`, the sha256 of the separately built work CLI at
  `bin/deltascope`.
- `golden/` — `ddl_golden.py` artifacts: **T09 125/125 cases (3442
  assertions), T10 126/126 cases (3175 assertions)** — identical case
  and assertion counts to the R4 baseline. The T09/T10 runs at this
  archive were executed against the committed product tree
  (post-amend, head=2f37fbd4); the runner records `head_sha` and
  `cli.sha256` inside each artifact.
- `gates/` — offline gates at the bound SHA: `go test ./...`,
  `go vet ./...`, `go test -race ./internal/...`, `gofmt -l`,
  `golangci-lint` on the touched packages, and the three-level
  documentation loop check — each captured as command/stdout/stderr/rc
  quads.
- `native-probe/` — `r5-bounded-probes.sh`/`.log`, verbatim transcript
  against mysql:8.0.46 and pingcap/tidb:v8.5.0 of every R5-load-bearing
  native fact: renamed-PK `pk2` successor ops (VISIBLE no-op / INVISIBLE
  3522 / common-handle `DROP INDEX pk2` rc=0 deleting the object /
  re-rename), freed-`primary` outcomes (1091 target-miss, member
  takeover rc=0, PK rename-back 1061), DROP PRIMARY KEY member-first
  routing on a handle table, the nonclustered member-`primary` 1061
  coexistence refusal, and the MySQL-vs-TiDB 61/62/63/64-byte
  auto-name suffix boundaries. The earlier in-session probe transcript
  was lost with the pre-commit session log; this script re-runs the
  same probe set against the same anchors — every case is idempotent
  and the log is a real current transcript.
- `red/` — red→green replay evidence: the new `*_t10r5_test.go` files
  copied verbatim onto a `git worktree` of `1d60d578` (R4 product
  code) produce 28 top-level behavioral FAILs; the same suite is green
  on the R5 tree. See `red/README.md` for provenance notes.

## Gate summary (all at bound SHA)

`go test ./...` 0 failures across all packages; `go vet ./...` clean;
`go test -race ./internal/...` clean; three-level-doc OK; T09+T10
golden 251/251 cases at R4-equal counts. `golangci-lint` on the touched
packages reports only the pre-existing baseline classes (unparam /
unused / gocritic / staticcheck style items also present at R4); the
two findings the R5 diff itself introduced (a De Morgan inline
condition and a never-used suffix variable) were fixed before this
archive. `gofmt -l` flags the same baseline files as R4 — all recorded
verbatim in `gates/`.

## Independent review

Dual-axis (standards + spec) reviews ran against the frozen
`1d60d578`-relative diff across three rounds. Round 1: standards
clean; spec found the visibility/gap/constrained-name defects — all
fixed. Round 2: two standards hard-finds (evidence-gap hole for
non-`primary` ambiguous targets; stale tri-state header) plus four
spec "implemented-but-wrong" findings (`IndexNamesUnknown`/`NameUnknown`
conflation, ambiguous-rename stored-name hole, add/create withheld-PK
arm, demoted-candidate publishes) — all fixed and regression-pinned.
Final re-review of the fix delta: 5/6 items VERIFIED-OK; the one
defect found (member rename destination not checked against the PK
object's actual stored name) was fixed with a red-first test and the
shared binding resolver — the fix closes the last publish path that
could fabricate an impossible member/PK name coexistence.
