# T11-R1 verification — 216e187c

Product HEAD `216e187cb2266dd4c8bc5f5c0590ed9bf99a995b` — the test-only
oracle commit appended on product commit `20649919`, itself appended
after the T11 evidence commit `661e5756`. Scope: the T11-R1 hardening
round under issue #90 — six root-cause groups closed against the same
T11 contract (issue #79 parent).

| Directory | Contents |
|-----------|----------|
| gates/ | Per-gate quad files (`<gate>.{command,stdout,stderr,rc}.txt`) executed against the bound SHA on this host |
| green/ | `go test -count=1` logs for the packages touched by the R1 product diff on `216e187c` |
| red/ | Revert-stub replay evidence: five `REDSTUB` mutations disable one R1 seam each; the focused R1 tests must fail — 21 audit FAILs, 4 SDK FAILs, 4 rule FAILs. Marked REPLAY; see `red/README.md` |
| golden/ | `T11-artifact.json` (40/40, 505 assertions), `T09-artifact.json` (125/125, 3442), `T10-artifact.json` (126/126, 3175), plus per-task run-log summaries derived from each artifact JSON (artifact-derived digests, not verbatim runner stdout) — all `head_sha=216e187c` |
| native-probe/ | Raw R1 four-anchor probe logs (MySQL 5.7.44/8.0.46/8.4.10, TiDB v8.5.0) with the corrected fact matrix |
| identity.txt | Product SHA, parent chain, CLI sha256, golden totals, anchor versions, credential-env convention |

## Gate index (all bound to 216e187c)

Every gate under `gates/` records the exact command, stdout, stderr,
and exit code — **17 gate quads as archived**. Summary: `go test ./...`
0, `go vet ./...` 0, `go test -race -count=1 ./internal/...` 0
(CGO_ENABLED=1), `gofmt` no new issues — the two accepted T07 baseline
files remain — `sql-corpus-gates` 0, `query-access-corpus-gates` 0,
`ddl-inventory-gate` 0, `ddl-coverage-catalog-test` 0 (catalog JSON
regenerated: the two PostgreSQL name-only PK stub items moved from
finding_covered to normalized_silent — the R1-A correction stops a
zero-type stub from fabricating `ddl.table.primary_key.bigint.require`),
`ddl-golden-validator-test` 0, `parser-error-unsupported-contract-test`
0, `unsupported-diagnostics-evidence-test` 0, `docs-example-gates` 0,
`release-dialect-hygiene-gates` 0, `decision-record-gate` 0,
`pg-confidence-gates` 0 (unit gates plus Docker-backed E2E suites —
three downstream oracles updated in `216e187c`, same root cause),
`three-level-doc` OK, and `lint` rc=1 with the pre-existing baseline
only — 15 issues, 0 added by this round.

## Golden

T11 **40/40 cases, 505 assertions**; T09 **125/125 cases, 3442
assertions**; T10 **126/126 cases, 3175 assertions** — all frozen
denominators preserved, all `head_sha=216e187c`, CLI sha256
`4b366a4f10cd…` identical across the three artifacts and the locally
rebuilt `CGO_ENABLED=0` binary (deterministic build, go1.26.1).
Anchors: shared ddl-golden compose (MySQL 5.7.44 / 8.0.46 / 8.4.10,
TiDB v8.5.0), serial use, runner-managed lifecycle.

## R1 scope map

| Group | Root cause closed | Where verified |
|-------|-------------------|----------------|
| R1-A | PK structural rules skipped ALTER-declared members (PG-only stub path) | `batch_state_t11r1_pk_test.go`; oracle fixes in `216e187c` |
| R1-B | SDK clone collapsed empty collections to nil; FK existence gaps used a blanket collection premise; `ConstraintsUnknown` gated proven PK facts | `audit_t11r1_test.go` (public+internal parity), `constraint_fk_t11_test.go`, `batch_state_a5_r1_test.go` |
| R1-C | Referenced-parent provider read used the request schema instead of the child's effective schema | `batch_state_t11r1_c_test.go` (two-database probe fixtures) |
| R1-D | FK adds published post-state without the native landing premise (usable referenced key, refer actions) | `batch_state_t11r1_de_test.go`; native-probe logs 1215/1822/6125/1830 |
| R1-E | Implicit supporting index not derived on CREATE; undecidable adds kept proven index facts; 1553 gate covered only plain `drop_index`; covering eligibility ignored index kind | same file + `batch_state_index.go` gates at every removal entry |
| R1-F | TiDB `DROP CONSTRAINT` premise was wrong (probe misread rc=0 as removal); MySQL keyword divergence unmodeled | `batch_state_t11r1_f_test.go`; `r1-probe2.log` KCU verification |

## Honest caveats

- Red evidence is a **revert-stub replay** on the final SHA (five
  `REDSTUB` marks, each killing one R1 seam). The original development
  red runs were terminal output; the replay is labeled REPLAY in
  `red/*.stdout.txt`.
- `docs/work-plans/t11-unified-constraints/r1/*.log` holds the raw
  native sessions; `native-probe/` keeps a copy beside this README.
- Two prior-bundle probe notes were disproven by R1 probes and are
  corrected in `native-probe/README.md` (6125 belongs to MySQL 8.4,
  TiDB DROP CONSTRAINT never removes an FK). The `11d64391` bundle is
  kept verbatim — history is not rewritten.
