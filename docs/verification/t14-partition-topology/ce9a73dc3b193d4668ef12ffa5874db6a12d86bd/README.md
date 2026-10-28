# T14-R2 evidence bundle — ce9a73dc

Independent-acceptance evidence for T14-R2 (#93 remediation): the
A/B/C/D defect groups closed under the frozen ownership contract,
plus the E-group evidence corrections. Product commit
`ce9a73dc3b193d4668ef12ffa5874db6a12d86bd` (parent `17539dbb`, the
accepted T14-R1 evidence-closeout commit). Contract: CONTRACT.md
§10; ADR: `docs/decisions/2026-10-28-ddl-partition-topology-r2-remediation.md`.

## Scope delivered

- **A — frozen ADD ownership restored.**
  `ddl.alter.add_partition.target` again owns the observed-directory
  bound contract: non-increasing RANGE ADD vs catalog → 1493, ADD
  after a MAXVALUE tail → 1481, provable LIST value duplication vs
  existing members → 1495. Applicability keeps declared-form duties
  only (1492 form / 1499 overflow / 1517 same-batch duplicates).
  `Evaluate` and `EvidenceGaps` consume the same shared adjudication
  (`spec.LessThanPairOutcome`, `spec.InBoundsOverlap`,
  `firstDuplicateDefName`); both-enabled never duplicates a finding,
  disabling a rule removes only its own gap. Verified across
  target-only / applicability-only / both / all-off policies.
- **B — undecidable comparisons disclosed on the current statement.**
  `BoundComparable` certifies shape, not decidability. Undecidable
  comparisons now emit `unknown_table_state` evidence gaps on the
  owning rule (RANGE/LIST COLUMNS string bounds, `PARTITION BY`
  string bounds, catalog bound shells missing rows); pure literal
  undecidable minimums report unverified/review instead of a false
  complete/pass. Pairwise adjudication keeps a later provable
  violation even when an earlier member is undecidable.
- **C — bounded partition adjudication reaches the public entry.**
  `partitionStateAdmissible` admits a statement whose only unmodeled
  coverage aspects are partition-bound or probed vendor-refusal
  aspects; everything else stays fail-closed. A proven same-batch
  duplicate (1517) preserves the exact pre-state including
  statistics; `CREATE TABLE IF NOT EXISTS` on a known-present table
  stays a no-op before body aspects; a bound-only undecidable ADD
  degrades just directory/definition facts and partition-dependent
  statistics — successors owe `target_table.partitions`/
  `partition_defs`, not table existence.
- **D — OPTIMIZE scope conditional on proven engine.** InnoDB keeps
  the whole-table rebuild/reanalysis wording; other proven engines
  keep named-member scope; unproven engine emits the conditional
  wording consistent with the `target_table.engine` gap contract.

## Red baseline

`red/` — captured on the unremediated product `17539dbb` before the
R2 fixes landed. `go-test-t14r2.stdout.txt` records the real failure
run of `batch_state_t14r2_test.go`: **27 FAIL markers / 11 failing
top-level tests** covering all four groups — zero-finding/zero-gap/
complete under target-only on a provable 1493, applicability wrongly
owning bound findings, missing `unknown_table_state` gaps on string
bounds and catalog shells, an undecidable member swallowing a later
provable violation, tombstoned pre-states on proven refusals,
`CREATE IF NOT EXISTS` invalidating known facts, and OPTIMIZE
asserting named-member scope without engine facts. The pre-fix run's
rc and stderr were not separately captured; the recorded command is
archived as `go-test-t14r2.command.txt` and every FAIL marker is a
behavioral miss on the public `AuditSQL` seam — no failure is
compile-driven.

Post-remediation, six R1-test expectations were corrected to the
restored ownership (fixture/ownership test-side fixes — profile
switches, a renamed colliding fixture member, a metadata-key
assertion) — not product regressions.

## Gates

`gates/` — 19 required gates + 3 committed-diff/final-tree L2 checks
+ 1 full-repo lint diagnostic, all on `ce9a73dc`, each with real
argv, stdout, stderr, and rc. See `gates/INDEX.md`: 17 required
green, `lint-scoped` at exactly the 13-issue accepted baseline
(file-list diff vs the R1 record identical — two self-introduced
findings were fixed before this SHA), `gofmt` listing only the two
accepted T07 files, `lint-fullrepo` recorded verbatim as a 37-issue
diagnostic outside the accepted scope.

## Golden

`golden/` — six serial suite runs on `ce9a73dc`, each artifact
recording `head_sha=ce9a73dc3b19`, `cli.build.head_sha` equal, one
shared CLI sha256 (`a7a1c1da9c0d…`), required==executed case sets,
no duplicate IDs, `compose_down_rc=0`, `residual_containers=[]`:

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 68/68 | 1066 | PASS |

## E-group corrections recorded this round

- **E1 (gates & committed-diff L2).** The R1 archive's 22 tuples are
  restated honestly: 18 rc=0, scoped lint rc=1 at the 13-item
  baseline, full-repo lint rc=1 at 37 diagnostics, and **both
  committed-diff checks rc=1** — a clean-tree "no changed files" run
  is not a committed-diff pass and is not claimed as one. The R1
  committed-diff `.command.txt` recorded only `git diff --name-only`
  while its stdout came from a bespoke checker; that checker script
  is not recoverable and is recorded as missing. The current-rule
  committed-range variant `gates/l2_committed_range_check.sh` is
  archived with full argv/stdout/stderr/rc and run on the R2 product
  range, the cumulative T14 range, and the final tree — all rc=0.
- **E2 (red/green statistics & test layers).** Regenerated from the
  archived `red/go-test-t14r1.stdout.txt`: **66 FAIL markers, 65
  unique failing paths, 18 unique parent tests, 47 unique subtest
  (leaf) full paths (46 unique leaf names)** — these are separate
  dimensions and are never summed; the earlier "36 cells" summary is
  withdrawn, and the previously recorded "50 leaf paths" figure does
  not reproduce under recounting. The R1 matrix mixed product-
  behavior failures with later test-side corrections (fixture,
  assertion, ownership) and nested-helper duplicate prints; the
  R2 matrix is additionally layer-explicit: the primary cells run
  the public `AuditSQL` seam, `t14r1` parser-boundary cells run the
  internal `parseSQL` helper for native-code evidence, and
  diagnostic-category cells run parser diagnostic assertions — the
  suite is not a uniform public-`AuditSQL` pass. Green-run counts
  cite the verbose stdout of `go test -run TestT14R2 -v`
  (36 `=== RUN` markers, all T14R2) and the full `go test ./...`
  tuple — nothing is inferred from the non-verbose red log.
- **E3 (huge-input evidence).** The regression cell runs inside a
  resource-limited subprocess with a timeout — **no hard memory
  ceiling is enforced**, stated as-is. The 9.4 GB RSS / 175 % CPU
  figure reported in R1 has **no archived collection artifact** and
  is recorded as previously-reported-but-unverified; the dangerous
  pre-fix allocation path was not re-run to manufacture evidence.
- **E4 (golden additions).** From the actual `14850fb2..c29ebc38`
  manifest diff the nine added IDs are:
  `t14r1-mysql80-add-dup-batch-name`, `t14r1-mysql80-add-count-overflow`,
  `t14r1-mysql80-coalesce-zero`,
  `t14r1-mysql80-create-decreasing-stays-absent`,
  `t14r1-mysql80-pby-expr-bound-incomplete`,
  `t14r1-tidb85-vendor-check-preserves`,
  `t14r1-mysql57-myisam-discard-1031`,
  `t14r1-mysql80-rcol-decreasing-tuple`,
  `t14r1-mysql80-string-domain-unproven` — containing the COALESCE
  zero and PARTITION BY expression cells. The earlier description
  naming a LIST-expression cell and an OPTIMIZE-scope cell among the
  nine was wrong and is corrected; the LIST-expression and OPTIMIZE
  behaviors are covered by the frozen 59 and R1 test matrix instead.
- **E5 (native log scope).** p15's 292 recorded calls include
  fixture-creation and catalog queries — they are not 292
  independent behavior assertions. mysql80/mysql84 MyISAM CREATE
  refused 1178 and the following four 1146s are `golden`-database
  setup misses on those anchors — they are **not** IMPORT/DISCARD
  engine-behavior evidence; the 5.7 1031 observations remain valid.
  `SHOW WARNINGS` output on a new connection is not attributed to a
  prior connection's DDL. The direct OPTIMIZE status rows remain
  valid scope evidence. probes README's p14 note is scoped: 1505
  applies to actual MySQL cells; TiDB's initial 1045 is invalid
  (setup), the valid TiDB replay is 8200. No p15 re-run was
  performed for these wording corrections; original logs are not
  rewritten.

## Verified / unverified boundaries

Verified on this SHA: ownership split under four policy
combinations, current-statement gap disclosure, pairwise
adjudication, bounded state admission, OPTIMIZE conditional scope,
19 gates + 3 L2 checks + six golden suites. Unverified/deferred:
expression bound items are never evaluated (capability aspect,
stricter than the native fold); string ordering without a proven
collation domain stays unproven; multi-action ALTERs and non-
partition aspects stay fail-closed; tablespace artifacts, row
routing, INTERVAL evaluation, and subpartition modeling remain
deferred to T15+.
