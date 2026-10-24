# T10-R3 verification archive — ffa7d2b2

Bound SHA: `ffa7d2b2b1d3f54902c462eb68356e34c8e05865`
(`fix(ddl): T10-R3 consumer correctness — declared-pair gaps, PK identity, member fidelity`)

Base: `ff5a759b46a7f72d1525cde7a0a0fd7c585a2ec6` (T10-R2 archive commit;
this round is the independent-review hardening pass on top of the
accepted R2 surface — no T10 expansion, no T11 work)

Scope: T10-R3 independent-acceptance candidate for issue #89 — three
bounded defect classes confirmed by independent review:

- **A — CREATE TABLE declared-pair comparison gaps.** When two
  different declarations inside one `CREATE TABLE` pair under
  duplicate, left-prefix, or unique-overlap semantics and either side
  carries a non-plain member, the rule now emits `unknown_table_state`
  with required fact `target_table.indexes.key_parts` instead of
  silently completing. Self-pairing is excluded, lone non-plain
  indexes/non-pairing members/provable plain pairs/disabled rules keep
  their prior coverage, and the CREATE INDEX / ALTER ADD INDEX member
  controls are unchanged.
- **B — primary-key identity across all consumers.**
  `PrimaryKeyHandleUnknown` now participates in the knownness premise
  alongside `PrimaryKeyUnknown`/`ConstraintsUnknown`: exists-only,
  compat-only, and combined runs emit a `target_table.primary_key`
  evidence gap instead of fabricating absence or silently passing.
  Derived CREATE TABLE snapshots derive stored names through
  `spec.StoredIndexNames` (first column member, ordered `_N` collision
  suffix; expression-first/empty/authored-collision →
  `IndexNamesUnknown`), so unnamed-member successors resolve real
  names and name-keyed consumers gap rather than proving absence from
  an empty name. `IndexNamesUnknown` gates only name-keyed consumers —
  member-shape consumers (add-index merge, collision-free member
  append) still use authored facts. TiDB `int_only` defers to the
  `alter-primary-key` instance fact loaded through the verified all-row
  `SHOW CONFIG` path; observed `true`/`false`/unobserved are all
  tested. `DROP PRIMARY KEY` on a proven handle table keeps state
  (native 8200 refusal) while an unproven handle answer invalidates.
- **C — provider expression-member direction fidelity.**
  `classifyExpressionMember` reads `STATISTICS.COLLATION`: `D` marks the
  member descending, `A` ascending, NULL/unrecognized leaves direction
  unmodeled while preserving expression identity — never guessing
  ascending. Direction and expression facts survive
  source → provider → ordered post-state.

## Layout

- `identity/` — bound head SHA, worktree status, toolchain version, CLI
  sha256 of the binary the golden runner built.
- `golden/` — `ddl_golden.py` artifacts for the directly affected
  tasks re-run at the bound SHA: **T09 125/125 cases (3442
  assertions), T10 126/126 cases (3175 assertions)** plus the run log
  (`regression-t09-t10.log`). The remaining six tasks were not re-run:
  R3 changed no T03–T08 oracle, manifest, or behavior those suites pin
  (verified by full `go test ./...` + validator suite below).
- `gates/` — offline gates at the bound SHA: `make test` (all
  packages), `go vet`, race on the four touched packages,
  `pg-unit-test-gates`, `pg-confidence-gates` (unit + e2e CLI/HTTP/MCP),
  `sql-corpus-gates`, `query-access-corpus-gates`, `ddl-inventory-gate`,
  coverage-catalog, parser-error, unsupported-diagnostics,
  docs-examples, dialect-hygiene, decision-record, three-level-doc,
  `release-gofmt-gate`, the golden validator suite
  (`ddl-golden-validator-test.log`: 938 checks — 758 contract cases +
  86 T07 + 25 T08 + 36 T09 + 33 T10 constructed controls, 0 failures),
  and the focused T10-R3/T09-R1 unit runs.
- `native-probe/` — real bounded re-verification of the matrices the
  R2 archive mis-attributed to a stale R1 pin log (see below):
  `r3-bounded-probes.sh` is the executed script and
  `r3-bounded-probes.log` its verbatim transcript.

## Native re-verification (why it exists)

The R2 archive claimed `native-probe/r2-consolidated-pins.log` carried
four new matrices (FT prefix length, MV DECIMAL/DOUBLE dialect split,
TiDB clustered defaults, explicit-PK-then-UNIQUE). That file is in fact
byte-identical to the R1 pin log (same git blob); the original R2 probe
scripts were not preserved. This round re-verified the missing claims
bounded, plus the two new R3 facts, against live anchors:

| Matrix | Anchor | Result in `r3-bounded-probes.log` |
|--------|--------|----------------------------------|
| M1 FT prefix 5 ok / 0 rejected | mysql:8.0.46 | rc=0 / ERROR 1391 |
| M2 MV cast dialect split | mysql:8.0.46 + tidb:v8.5.0 | DECIMAL ok on MySQL / ERROR 1235 on TiDB; DOUBLE reverses |
| M3 clustered default + `alter-primary-key` + `int_only` | tidb:v8.5.0 | default ON, `alter-primary-key=false`, int PK still CLUSTERED under `int_only` |
| M4 explicit PK then UNIQUE INVISIBLE | mysql:8.0.46 | real secondary unique index `c` |
| M5 unnamed index stored names | mysql:8.0.46 | `a`,`b`; duplicate `c` → `c`,`c_2` |
| M6 expression member direction | mysql:8.0.46 | `((a+b)) DESC` stores `COLLATION=D` |
| M7 ALTER ADD/DROP PRIMARY object form | tidb:v8.5.0 | ADD PK → NONCLUSTERED even with `alter-primary-key=false`; DROP PK on handle table → ERROR 8200; DROP on nonclustered PK succeeds |

Probe time: 2026-10-24 local, code identity `ffa7d2b2`; run before the
archive commit against the working tree that became this SHA.

## Gate summary (all at bound SHA)

`make test` 0 failures across all packages; affected-package race
clean; validator suite 938/938; T09+T10 golden 251/251 cases; every
listed corpus/catalog/inventory/docs/contract gate green. The only
nonzero gate is `release-gofmt-gate`, which flags the same five
pre-existing baseline files (three t06 archive probes + two t07 test
files) — none introduced or touched by this diff; confirmed identical
to the R2 archive's recorded state.
