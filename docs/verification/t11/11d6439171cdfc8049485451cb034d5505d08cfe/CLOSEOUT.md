# T11 closeout — issue #90

## What shipped

Unified PK / UNIQUE / FK policy behavior across CREATE-table and ALTER
add/drop paths, with provider-loaded constraint membership and
ordered-state FK transitions. Product commit
`11d6439171cdfc8049485451cb034d5505d08cfe` (55 files, +4492/−151).

### First public golden (frozen contract §4)

`ALTER TABLE child ADD CONSTRAINT fk_c FOREIGN KEY (c) REFERENCES
parent(id);` with only `ddl.table.foreign_key.forbid` enabled at
blocker → exactly one forbid blocker carrying table, constraint, local
column, referenced table/column, and source-location metadata;
`verdict=reject`; `coverage=complete`; zero evidence gaps, parser
errors, or unsupported entries; CLI exit 1 — verified in
`golden/T11-artifact.json` cases `t11-cli-fk-alter-firstpath`,
`t11-mysql80-firstpath`, `t11-tidb85-firstpath`. Never degrades to
`ddl.alter.add_constraint.notice` alone.

### Controls

`ADD COLUMN` without an FK emits zero FK-forbid findings
(`t11-cli-addcol-control`); disabling the rule removes the blocker
(`t11-cli-fk-forbid-off`); CREATE-table and ALTER-add FK pairs match
(`t11-cli-fk-create-pair`).

## Acceptance-matrix map

| Matrix area | Where verified |
|-------------|----------------|
| CREATE/ALTER parity — PK, UNIQUE, FK; single/composite; named/bare | T11.json cli_cases `t11-cli-*` (20 cases); rule tests `constraint_fk_t11_test.go`; catalog JSON rows |
| References & dependency facts — ref schema/table/columns, ON DELETE/UPDATE, composite ordering, second constraint | `t11-cli-fk-schema-ref`, `t11-cli-fk-alter-composite-actions`, `t11-cli-fk-alter-two`, meta `t11-mysql80-composite-actions`; spec/extractor tests |
| Constraint/index ordered state — implicit index, drop keeps index, needed-index refusal, rename/drop invalidation | `batch_state_fk.go` + 19 unit tests `batch_state_t11_fk_test.go`; meta cases ordered-drop/drop-needed-index/rename-parent |
| Unknown vs proven — constraints unknown, unvouched name space, CHECK-bearing tables | meta cases `t11-mysql80-dropfk-constraints-unknown`, `t11-mysql80-dropfk-check-bearing`; provider tests |
| New rule `ddl.alter.drop_foreign_key.exists.require` | registered/defaulted at blocker; offline + mysql80 meta cases (present/absent/unknown/tombstone quadrants) |
| Four anchors | ddl_steps + syntax_negative on 5.7.44/8.0.46/8.4.10/v8.5.0; meta on mysql80+tidb85; native probe logs |

## Verified vs not verified

**Verified:** all of the above on `11d64391`; deterministic CLI build
(sha256 `fa77a477…` identical across the work build and all three
golden artifacts); T09/T10 denominators preserved byte-for-byte.

**Not verified / bounded:**
- TiDB `DROP CONSTRAINT` on a named FK — T12 boundary, recorded in the
  ADR and native probe notes.
- Provider-load golden coverage binds mysql80 + tidb85 only (5.7/8.4
  provider loads share the same information_schema vocabulary; no
  connect-bearing cases target them this round).
- Lazy invalidation is bounded to already-loaded dependents — recorded
  in CONTRACT §6 and exercised by `rename-parent-invalidates`.
- `MATCH` remain unsupported evidence (existing green control
  `t11-cli-fk-match-evidence`), unchanged from T10.

## Gate summary

17 gate quads under `gates/`, all bound to `11d64391`: 16 × rc=0 plus
`lint` rc=1 identical to the accepted T10 baseline modulo line numbers
(15 issues / 52 output lines, zero added). `gofmt` adds no files beyond
the two accepted T07 baseline entries. `ddl-coverage-catalog-test` and
`pg-confidence-gates` initially failed on a stale embedded catalog —
the catalog was regenerated and the product SHA re-frozen before this
run; both now pass.

## Remote / lifecycle status

Local branch `milestone/mysql-tidb-ddl-completion` at `11d64391`
(product) + evidence commit on top. Fast-forward push to
`origin/workfree` per the standing authorization. No PR, merge, issue
closure, tag, release, or deployment performed or claimed; T11 stops at
the independent-acceptance candidate.
