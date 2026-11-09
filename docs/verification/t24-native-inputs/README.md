# T24 native-execution input freeze (R11)

Frozen offline inputs for the T24/#103 native stage. Nothing here
executed: no DB connection, no environment start, no native SQL.

- `anchors.json` — four fixed anchors (mysql:5.7.44, mysql:8.0.46,
  mysql:8.4.10, pingcap/tidb:v8.5.0) + expected environment identity to
  re-verify at preflight.
- `case-map.md` — per-required-ID anchor/SQL/premise/oracle/restore map
  (24 cells: 10 cli + 6 meta + 8 db) + named T24-companion-* probes.
- `fixture-sql.md` — accounts/roles/groups/occupancy-session/read-only
  templates with observe/execute/verify/restore scope.
- `evidence-spec.md` — CPU-affinity + priority-capability collection
  spec pinned to mysql-8.0.46/8.4.10 upstream sources.
- `RUN_PLAN.json` — frozen run plan (fingerprints/digests fill at
  preflight seal).
- `ledger/templates.md` — evidence-index/sql-actions/restore ledger
  schemas + stop conditions + compensation table.
- `authorization-card.md` — the single per-item authorization card
  (P/I/C/O/G/X/T/R/S-R-V/L) awaiting one explicit user approval.

Tooling: `scripts/t24_collect_evidence.py` (observed-record →
DeploymentEvidence JSON + attachment), `scripts/test_t24_evidence_contract.py`
(6 contract tests over `testdata/t24-evidence-fakes/`).
Evidence output paths consumed by manifest args:
`testdata/ddl-golden/evidence/t24-mysql80.json`,
`testdata/ddl-golden/evidence/t24-mysql84.json` (generated at run time;
missing-file behaviour is the established exit-2 contract).

Expect-status note: the two evidence-consuming meta cells keep their
branch-independent manifest `expect`; capability-branch oracles
(cap-on/cap-off) are frozen in case-map.md and selected at run time by
the independent premise observation — never reverse-fit from actuals.
