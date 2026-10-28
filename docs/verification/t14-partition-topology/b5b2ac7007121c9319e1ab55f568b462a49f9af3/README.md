# T14 evidence bundle — b5b2ac70

Acceptance-candidate evidence for T14 (#93): partition definition,
reorganization, and maintenance semantics. Product commit
`b5b2ac7007121c9319e1ab55f568b462a49f9af3` (parent `b54a0544`, the
T13-R3 evidence-closeout commit — T14's task baseline). Contract:
`docs/work-plans/t14-partition-topology/CONTRACT.md` (frozen before
implementation); ADR:
`docs/decisions/2026-11-07-ddl-partition-topology-maintenance.md`.

## Scope delivered

- **Normalized partition model** — declaration-side
  `PartitionDefs`/`PartitionBound`/`BoundLiteral` (RANGE/LIST bounds,
  COLUMNS forms, `MAXVALUE`, `LIST DEFAULT`, expression bounds kept
  unproven) and catalog-side projection of `PARTITION_DESCRIPTION`
  into the same boundary vocabulary, so declared and observed
  partitions compare on one representation. `Subpartitioned` keeps
  leaf-level catalog rows from masquerading as top-level members.
- **Topology actions** — ADD PARTITION (definition list and
  `PARTITIONS n` forms bounded by `maxPartitionMembers = 8192`),
  REORGANIZE PARTITION (named source members replaced in place by the
  declared targets, ordered directory preserved), COALESCE PARTITION,
  PARTITION BY redefinition, and REMOVE PARTITIONING. Proven refusal
  preserves exact pre-state; proven execution publishes the modeled
  post-state; unproven outcomes unvouch affected partition facts
  while retaining unrelated structural facts (columns, indexes, PK,
  constraints).
- **Maintenance actions** — CHECK (read-only, notice), REBUILD,
  REPAIR, OPTIMIZE, IMPORT PARTITION TABLESPACE, DISCARD PARTITION
  TABLESPACE (blocker — destroys the member's tablespace file).
  Member-directory facts are preserved; data-derived statistics are
  cleared or kept per probed semantics. MySQL soft-fail behavior
  (warning row, no structural change) emits warning/notice findings
  with `soft_fail`/`member_absent` metadata — never a fabricated
  blocker.
- **Engine/version split** — MySQL 5.7 / 8.0 / 8.4 and TiDB 8.5 carry
  separate behavior where native evidence differs: TiDB refuses
  REBUILD/REPAIR/OPTIMIZE/CHECK/IMPORT/DISCARD at vendor boundary
  (action-specific `alter_table.<action>` unsupported features with
  incomplete coverage), TiDB bare `ADD PARTITION` on HASH/KEY is a
  probed no-op (not a +1), and TiDB refusal classes (8200/1567/1493)
  are used instead of inherited MySQL codes.
- **Rules** — 30 registered/defaulted rules: 11 governance
  (per-action warn/notice/blocker), 11 applicability (proven-fact
  binding only, per-rule `target_table.*` evidence gaps), 8 target
  (member names bind only to a proven directory). Example config
  lists all 30; `rules list` catalogs them.
- **Ordered-state handoff** — successors see the post-state:
  REORGANIZE p0→p1/p2 retires `p0` and publishes `p1`,`p2`;
  REMOVE PARTITIONING publishes the proven plain table (T13
  non-partitioned paths engage); PARTITION BY publishes the new
  scheme; proven-refused actions leave the exact pre-state; unproven
  actions leave gaps instead of stale directories.

## Red baseline

`red/` — captured on the pre-implementation product `b54a0544` in an
isolated worktree (`git worktree`, `CGO_ENABLED=0 go build`); the
same unsupported/incomplete behavior was observed live before
implementation began, and this archive reproduces it on the exact
parent SHA. All failures are behavioral (audited semantics missing),
none compile-driven.

- `audit.sql` — the frozen first-golden REORGANIZE statement plus ten
  representative T14 actions (ADD, COALESCE, PARTITION BY, REMOVE
  PARTITIONING, CHECK, REBUILD, REPAIR, OPTIMIZE, IMPORT, DISCARD).
- `audit.json` / `audit.stderr.txt` — `deltascope audit --dialect
  mysql --file audit.sql --format json` → rc=1, verdict `review`,
  coverage `incomplete`; every statement lands in `unsupported[]`
  under its T03 feature id (`alter_table.reorganize_partition`,
  `alter_table.add_partitions`, …) with per-statement `incomplete`
  coverage — the exact boundary the frozen contract moves to audited
  findings/gaps/verdicts.

## Gates

`gates/` — 19 required gates + 1 full-repo lint diagnostic, all on
`b5b2ac70`, each with real argv, stdout, stderr, and an rc saved
immediately after exit. See `gates/INDEX.md`: 17 green, `lint-scoped`
at exactly the 13-issue accepted baseline (normalized-diff identical
to the T13-R3 record), `gofmt` listing only the two accepted T07
baseline files, `lint-fullrepo` recorded verbatim as a 37-issue
diagnostic outside the accepted scope.

## Golden

`golden/` — six serial suite runs on `b5b2ac70`, each artifact
recording `head_sha=b5b2ac700712` and its own CLI sha256:

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 59/59 | 917 | PASS |

T14's manifest is `testdata/ddl-golden/T14.json` (59 cases:
metadata, CLI, multi-statement chains, policy isolation, and
syntax-negative cells). During manifest construction one development
run archived 12 parser-boundary failures — modifier+REORGANIZE
combinations the locked parser cannot express (`ALGORITHM=<v>|
LOCK=<v>, REORGANIZE PARTITION …`); those cells are documented as
unreachable in `CONTRACT.md` §8 rather than silently dropped, and
the frozen 59-case set contains no parser-dependent cell the parser
cannot express.

## Native evidence

`native/` — archived copies of the committed probe logs:
`p13_topology_maintenance.log` (full topology + maintenance matrix
across MySQL 5.7.44 / 8.0.46 / 8.4.10 and TiDB v8.5.0:
REORGANIZE/ADD/COALESCE/PARTITION BY/REMOVE and all six maintenance
actions, per-statement rc/stdout/stderr, post-state directories,
routing assertions, and `SHOW WARNINGS` on warning-sensitive cells)
and `p14_nonpart_maintenance.log` (the non-partitioned-table
maintenance matrix pinning the MySQL soft-fail vs 1505 hard-refusal
split and the TiDB vendor-boundary refusals). Probe script and the
supplementary cells are committed under
`docs/work-plans/t14-partition-topology/probes/`.

Unproven/unobserved boundaries kept explicit: expression bounds stay
unproven (gap, not fabricated equality), IMPORT/EXPORT tablespace
artifacts were not generated (classification bounded by probed
precondition/member-presence behavior), `ANALYZE PARTITION` lives
outside the ALTER action space (separate statement kind), and
modifier+REORGANIZE cells remain unreachable under the locked
parser.
