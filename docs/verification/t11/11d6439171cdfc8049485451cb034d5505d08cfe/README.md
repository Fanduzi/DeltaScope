# T11 verification — 11d64391

Product commit `11d6439171cdfc8049485451cb034d5505d08cfe`, appended
after the T10-R6-tail evidence commit `8b6f3098`. Scope: T11 / issue #90
— unified primary-key, unique-key, and foreign-key CREATE/ALTER checks
under parent issue #79.

| Directory | Contents |
|-----------|----------|
| gates/ | Per-gate quad files (`<gate>.{command,stdout,stderr,rc}.txt`) executed against the bound SHA on this host |
| green/ | Verbose `go test -count=1 -v` logs for all nine packages touched by the product diff on `11d64391` |
| red/ | Worktree-replay red evidence: the T11 manifest run against a detached `8b6f3098` worktree — registry red (rule unknown) plus 15/32 behavioral FAILs. See `red/README.md` |
| golden/ | `T11-artifact.json` (40/40, 505 assertions), `T09-artifact.json` (125/125, 3442), `T10-artifact.json` (126/126, 3175), plus per-task run-log summaries derived from each artifact JSON (they are artifact-derived digests, not the runner's verbatim stdout) — all `head_sha=11d64391` |
| native-probe/ | Raw four-anchor FK probe logs (MySQL 5.7.44/8.0.46/8.4.10, TiDB v8.5.0) recorded before oracle freeze |
| identity.txt | Product SHA, parent chain, CLI sha256, golden totals, anchor versions, credential-env convention |

## Gate index (all bound to 11d64391)

Every gate under `gates/` records the exact command, stdout, stderr, and
exit code — **17 gate quads as archived**. Summary: `go test ./...` 0,
`go vet ./...` 0, `go test -race -count=1 ./internal/...` 0 (CGO_ENABLED=1),
`gofmt` reports no new formatting issues this round — the two accepted
T07 baseline files (`batch_state_t07_matrix_test.go`,
`batch_state_t07_test.go`) remain — `sql-corpus-gates` 0,
`query-access-corpus-gates` 0, `ddl-inventory-gate` 0,
`ddl-coverage-catalog-test` 0 (catalog JSON regenerated this round:
ALTER ADD FOREIGN KEY forms now record `ddl.table.foreign_key.forbid`
coverage — the duplicated rule id on the PostgreSQL
`ADD CONSTRAINT FOREIGN KEY` form is two corpus statements under one
form, not a double emit), `ddl-golden-validator-test` 0,
`parser-error-unsupported-contract-test` 0,
`unsupported-diagnostics-evidence-test` 0, `docs-example-gates` 0,
`release-dialect-hygiene-gates` 0 (dialect-isolation policy check — not
a release authorization), `decision-record-gate` 0,
`pg-confidence-gates` 0 (unit gates plus the three Docker-backed
PostgreSQL E2E suites), `three-level-doc` OK (archived stdout is the
script's real changed-files run on this working tree, not a
post-commit "no changed files" output; the only L1 message is the
standing soft reminder), and `lint` rc=1 with the pre-existing baseline
only — byte-identical to the T10-R6-tail accepted baseline modulo
line-number shifts (15 issues / 52 output lines, 0 added).

## Golden

T11 **40/40 cases, 505 assertions**; T09 **125/125 cases, 3442
assertions**; T10 **126/126 cases, 3175 assertions** — T09/T10 frozen
oracle denominators unchanged; T11 is a new manifest. All three
artifacts record `head_sha=11d6439171cd…` and CLI sha256 `fa77a477e29c…`
(identical to the locally rebuilt `CGO_ENABLED=0` binary — deterministic
build, go1.26.1). Anchors: shared ddl-golden compose (MySQL 5.7.44 /
8.0.46 / 8.4.10, TiDB v8.5.0), serial use, runner-managed lifecycle.
`T09/T10/T11_MYSQL_PASSWORD` exported per each manifest's
`exec_password_env` convention (fixture root password `root`).

## Honest caveats

- Red evidence is a **worktree replay** on `8b6f3098`; the intermediate
  development golden runs were overwritten before this archive existed.
  The replay required pruning the two profiles that name the new rule —
  their eight cases' red is the registry-level failure itself. See
  `red/README.md`.
- T11 metadata/provider-load golden evidence binds `mysql80` and
  `tidb85` anchors only (the manifest's connect-bearing cases); MySQL
  5.7/8.4 cover the DDL/native-behavior rows, not provider loads.
- TiDB `DROP CONSTRAINT` for named FKs is recorded as a T12 boundary —
  not modeled this round.
- `pg-confidence-gates` ran on this host with Docker; its three E2E
  suites self-provision their anchors.
- `release-dialect-hygiene-gates` is a policy hygiene check, not a
  release authorization; no tag, release, PR, merge, or issue closure
  is claimed anywhere in this archive.
