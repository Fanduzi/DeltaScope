# T11-R2 verification — 988560c2

Product HEAD `988560c2abff75e52d31cf1a249712ae53a95eb8` — appended on
`d0c7cf33` (L3 header-convention retirement), itself on the T11-R1
evidence chain (`ec9629d8` → `216e187c` → `20649919` → `661e5756`).
Scope: the T11-R2 hardening round under issue #90 — four red→green
groups (A same-statement PK facts, D strict referenced-key anchor,
E self/existing binding dependencies, F DROP CHECK|CONSTRAINT kind
coverage) plus the mandated evidence-precision corrections. B/C stay
closed from R1.

| Directory | Contents |
|-----------|----------|
| gates/ | Per-gate quad files (`<gate>.{command,stdout,stderr,rc}.txt`) executed against the bound SHA on this host, plus `three-level-doc-diff.{command,scope,stdout,rc}.txt` — the doc check replicated over the committed `d0c7cf33..988560c2` diff because the script itself reads only the worktree |
| green/ | `go test -count=1` logs for the packages touched by the R2 product diff plus the focused `TestT11R2` verbose run (52 PASS) on `988560c2` |
| red/ | `baseline_red.log` — the real first-run red on the pre-fix base (25 `--- FAIL` markers, 17 parent tests, all four groups) — and `README.md` mapping every failing leaf to its group. Not a replay |
| golden/ | `T11-artifact.json` (40/40, 505 assertions), `T09-artifact.json` (125/125, 3442), `T10-artifact.json` (126/126, 3175), plus per-task run-log digests — all `head_sha=988560c2`, denominators identical to the R1 bundle |
| native-probe/ | Raw R2 probe scripts + logs with real per-statement client rc (`r2_probe.*`, `r2_probe2.log`, `r2_tidb_flag_probe.*`), including the complete TiDB `tidb_enable_check_constraint` OFF/ON cell |
| identity.txt | Product SHA, parent chain, CLI sha256, golden totals, anchor versions, credential-env convention, remote status |

## Gate index (all bound to 988560c2)

17 gate quads: `go test ./...` 0, `go vet ./...` 0,
`go test -race -count=1 ./internal/...` 0 (CGO_ENABLED=1), `gofmt` no
new issues — the two accepted T07 baseline files remain —
`sql-corpus-gates` 0, `query-access-corpus-gates` 0,
`ddl-inventory-gate` 0, `ddl-coverage-catalog-test` 0,
`ddl-golden-validator-test` 0, `parser-error-unsupported-contract-test`
0, `unsupported-diagnostics-evidence-test` 0, `docs-example-gates` 0,
`release-dialect-hygiene-gates` 0, `decision-record-gate` 0,
`pg-confidence-gates` 0, `three-level-doc` 0 (worktree run; the
meaningful check is `three-level-doc-diff` over the committed range —
rc=0), and `lint` rc=1 with the accepted baseline only — 15 issues,
0 added by this round (none on R2-touched lines).

## Golden

T11 **40/40 cases, 505 assertions**; T09 **125/125 cases, 3442
assertions**; T10 **126/126 cases, 3175 assertions** — all frozen
denominators preserved, all `head_sha=988560c2`, CLI sha256
`246d9de8…` identical across the three artifacts. Anchors: shared
ddl-golden compose (MySQL 5.7.44 / 8.0.46 / 8.4.10, TiDB v8.5.0),
serial use, runner-managed lifecycle.

## R2 scope map

| Group | Root cause closed | Where verified |
|-------|-------------------|----------------|
| R2-A | PK structural rules read only pre-state column facts — same-statement ADD/MODIFY/RENAME/DROP declarations invisible to the key action | `orderedColumnVerdict` tri-state + `statementColumnDecls` folding in `common.go`, shared by Evaluate/EvidenceGaps; `batch_state_t11r2_a_test.go`; native: MySQL folds effective declarations (8.0/8.4), TiDB refuses same-statement touches (1072/8200) |
| R2-D | Referenced-key premise treated a leftmost prefix as a usable key — partial unique/primary references published under observed restrict ON | `referencedKeyMatch` + `indexMatchesColumnsExactly` strict anchor; `batch_state_t11r2_d_test.go`; native: 6125 on partial unique/PK/secondary under ON, accepts under OFF, TiDB accepts partial (all probed) |
| R2-E | Drop dependencies skipped the drop target's own entry (self-ref) and reused the creatability answer for loaded bindings | `referencedDropDependency` scans all loaded tables + `supportingIndexCandidate` binding question; `batch_state_t11r2_e_test.go`; native: 1553 on self-ref sole coverer (both anchors), OFF-built non-unique binding still refuses under ON |
| R2-F | `applyDropCheck` unvouched only proven FK members — proven unique/PK targets stayed readable after `DROP CONSTRAINT` | `unvouchIndexMember`/`unvouchPrimaryKeyObject` arms; `batch_state_t11r2_f_test.go`; native: `DROP CONSTRAINT` removes FK/unique/PK/CHECK on 8.x, `DROP CHECK` only CHECK (3821/3940) |

## Evidence-precision corrections shipped alongside

- R1 `red/README.md` corrected: 21 `--- FAIL` markers = 16 leaf tests /
  9 parents (audit), SDK 1 leaf, rule 1 leaf — the "A/C/D/E/F all red"
  claim was wrong (C has no stub and stayed green), and the replay is a
  mutation demonstration, not a baseline-red rerun.
- R1 `native-probe/README.md` annotated with the rc-convention
  limitation (combined-command rc vs per-statement client rc); R2 logs
  capture real per-statement client rc.
- R1 `CLOSEOUT.md` restrict-OFF caveat annotated as superseded — both
  flag arms probed in R2.
- `CONTRACT.md` R2 section + E4 row, ADR R2 amendment appended —
  historical entries kept verbatim.

## Honest caveats

- `three-level-doc` quad shows the worktree-scope script output
  (`OK` on the then-pending diff when archived pre-commit shows the
  real file set; the committed-range equivalent is
  `three-level-doc-diff`, rc=0). The script reads worktree only — that
  is a tooling limitation recorded, not a gate change.
- `origin/milestone/mysql-tidb-ddl-completion` drifted to `d0c7cf33`
  ahead of the declared `1d60d578` baseline via an external push —
  recorded, untouched.
- Binding-side scans stay lazy (already-loaded entries only); an
  unloaded child's binding remains out of dependency scope by design.
- Lint 15-issue baseline and the two T07 gofmt files are accepted
  debt, unchanged.
