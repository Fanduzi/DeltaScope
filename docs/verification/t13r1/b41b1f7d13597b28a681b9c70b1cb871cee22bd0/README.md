# T13-R1 evidence bundle — b41b1f7d

Acceptance-candidate evidence for T13-R1 (#92 remediation): partition
applicability checks, EXCHANGE counterpart ordered-state fix, and the
evidence-bounded multi-spec refusal classifier. Product commit
`b41b1f7d13597b28a681b9c70b1cb871cee22bd0` (parent `d65ac868`, the T13
evidence-bundle commit). Contract amendment:
`docs/work-plans/t13-partition-risk/CONTRACT.md` §9 (frozen before
implementation); ADR:
`docs/decisions/2026-11-04-ddl-partition-applicability-and-ordered-state-remediation.md`.

## Scope delivered

- **A — applicability + evidence:** partition catalog facts
  (`Partitioned`/`PartitionType`/`PartitionNames`, nil = unobserved,
  non-nil empty = observed-empty) load on BOTH dialects from
  `information_schema.partitions`; CREATE TABLE publishes the declared
  catalog into ordered state. Seven default-blocker rules bind the
  facts: `ddl.alter.{drop,truncate,exchange}_partition.applicability`
  (1505 not-partitioned; 1512 hash/key-gated DROP),
  `ddl.alter.{drop,truncate,exchange}_partition.target` (1507/1735
  missing member; `ALL` consumes no name), and
  `ddl.alter.exchange_partition.counterpart` (1146 absent / 1732
  partitioned / 1736 bounded column+PK-definition mismatch — a clean
  compare never claims the engine validated). Unobserved premises emit
  per-consumed-fact `unknown_table_state` evidence gaps on the owning
  rule; disabled rules owe nothing; risk findings stay AST-only.
- **B — counterpart ordered state:** `enrichStatements` preloads the
  EXCHANGE counterpart through `preState` under its own identity and
  binds it to `statement.Metadata.ExchangeTable`; the post-state
  invalidation then covers both identities, so a first-touch successor
  never resurrects batch-prior statistics.
- **C — refusal narrowing:** `partitionMixedOutcome` replaces the
  blanket multi-spec refusal — >1 partition action, structural siblings,
  and `table_option` siblings stay refused (MySQL 1064 / TiDB 8200 or
  1064 by order); one preceding `algorithm`/`lock` modifier follows the
  probed matrix (`LOCK=DEFAULT`/`ALGORITHM=DEFAULT` combine everywhere;
  `ALGORITHM=INPLACE` combines everywhere except MySQL 5.x DROP → 1846;
  no version fact → unproven); `placement_policy` and multi-modifier
  mixes tombstone.
- Public-SDK boundary: `cloneTableSnapshot` deep-copies the partition
  pointer/slice fields — no provider-snapshot aliasing, observed-empty
  member sets never collapse to unobserved.

## Red baseline

`red/` — replayed on the pre-T13-R1 line `d65ac868` (temporary
worktree, test files copied in; the worktree was deleted after the
runs):

- `red_rules_compile.log` / `red_applicability_compile.log` /
  `red_extract_compile.log` / `red_pkg_clone_compile.log` — compile red:
  `spec.TableSnapshot.PartitionType`/`PartitionNames`,
  `spec.DDL.PartitionType`/`PartitionNames`, and the seven rule
  constructors do not exist on the base.
- `red_state_behavior.log` — real behavior red on the base:
  `TestT13R1ExchangeQualifiedCounterpart` and
  `TestT13R1ExchangeFirstTouchCounterpartClearsStats` fail because the
  counterpart's stale `table_rows=42` answered the successor TRUNCATE;
  `TestT13R1AlgorithmDropPartitionExecutes`/`...VersionGate` fail
  because `ALGORITHM=INPLACE, DROP PARTITION` parses as an
  unrepresentable multi-spec; the `StructuralSiblingStillRefused`
  control stays PASS (the proven refusal is preserved before and
  after).

## Gates

`gates/` — 18 command tuples (`.command.txt`/`.stdout.txt`/
`.stderr.txt`/`.rc.txt`): 17 × rc=0; `lint` rc=1 carries the same
13-issue baseline as the `890ae531` bundle (one entry re-pointed
`ddl.go:466→476` after the added spec fields — same gocritic
`typeDefFirst` finding, zero new issues). `gofmt` lists only the two
accepted T07 files.

## Golden

`golden/<TASK>/` — five serial runs sharing anchors, each bound to
`b41b1f7d` (`head_sha` and `cli.build.head_sha` in every artifact.json;
one CLI sha256 `5c3754e255d574bf` across all five). `cli.path` records
the runtime build path; the binary itself is not committed (`bin/` is
gitignored) — rebuild from `b41b1f7d` with
`CGO_ENABLED=0 go build -o bin/deltascope ./cmd/deltascope` and compare
against the recorded sha256.

| Task | Cases | Assertions |
|------|-------|------------|
| T09  | 125   | 3442 |
| T10  | 126   | 3175 |
| T11  | 40    | 506  |
| T12  | 25    | 282  |
| T13  | 38    | 538  |

All PASS; `executed_count` equals the required set for every task;
`residual_containers=[]`, `compose_down_rc=0`. T09–T12 denominators are
byte-stable against the previous bundle. T13 keeps the original 24
cases under the unchanged isolated profiles and adds 14 T13-R1 cells:
3 offline CLI gap cells (drop applicability on mysql+tidb, exchange
counterpart existence) and 11 live-anchor cells asserting the blocker
finding (rule_id + reason_code + exists/native_error metadata) plus a
native replay of the audited statement — including the B pair
(`exchange-clears-counterpart-stats` silent vs
`truncate-rows-control` blocker on the same fixture) and the C pair
(5.7 `ALGORITHM=INPLACE, DROP` keeps exact pre-state → successor still
answers statistics; 8.0 executes → successor silent).

## Native archives

`native/` — this round's bounded probe logs (all four anchors:
mysql 5.7.44/8.0.46/8.4.10 + tidb v8.5.0):

- `p4_nonpart.log` — TRUNCATE/EXCHANGE on a non-partitioned table:
  1505 on all four anchors.
- `p5_validation_arms.log` + `p5_validation_tidb_post.log` — the
  corrected three-arm EXCHANGE validation rerun with per-arm fixture
  resets: absent clause = `WITH VALIDATION` (1526 mysql / 1737 tidb),
  `WITHOUT VALIDATION` lands violating rows on both sides.
- `p6_mixed_matrix.log` — multi-action mixes (TiDB 8200 ordering
  split), HASH partition actions (DROP 1512, TRUNCATE/EXCHANGE
  execute), missing names (1507/1735), exchange counterpart classes
  (1146/1732/1736).
- `p7_modifiers.log` — `ALGORITHM=INPLACE` + DROP (1846 on 5.7,
  executes 8.0/8.4/tidb), + TRUNCATE/EXCHANGE (executes everywhere),
  `ALGORITHM=DEFAULT`/`LOCK=DEFAULT` (combine everywhere), post-action
  modifiers rejected by grammar.
- `p8_tableoption.log` — `ENGINE=..., DROP PARTITION`: 1064 mysql,
  8200 option-first / 1064 partition-first on tidb.
- `p1_all.log`/`p2_exchange.log`/`p3_validate_mixed.log` — carried over
  from the baseline archive for claims this round still relies on
  (drop classes, both-direction exchange data movement); p3's original
  absent-validation conclusion is superseded by the p5 three-arm
  rerun — kept, annotated in `probes/README.md`.
- `golden-T*.log` — the five `make ddl-golden` runner outputs for this
  bundle.

## Verified / unverified boundaries

Verified this round: absent-validation = WITH VALIDATION (4 anchors);
WITHOUT lands violating rows; 1505 non-partitioned TRUNCATE/EXCHANGE;
HASH DROP 1512 while TRUNCATE/EXCHANGE execute; member-name misses
1507/1735; counterpart classes 1146/1732/1736; the modifier and
table-option matrices above; counterpart first-touch statistics
invalidation through the public Audit path.

Unverified / not modeled: row-content compatibility solving (a clean
bounded compare never claims the engine validated the exchange);
sub-partitioned DROP; `PARTITIONS n` beyond implicit `p0..p(n-1)`
naming; `placement_policy` siblings and multi-modifier mixes
(tombstone); explicit `WITH VALIDATION` vs absent is
AST-indistinguishable (both record `with` — now the native-correct
collapse); unprobed `LOCK`/`ALGORITHM` values; T14 topology actions
(ADD/REORGANIZE/COALESCE/REMOVE PARTITIONING) and T15 standalone
modifier governance.
