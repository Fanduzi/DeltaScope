# T06-A17-TYPE-FACTS-BASELINE — external evidence (not remote reviewer evidence)

- baseline.txt — git identity, status, parent chain, locked parser dep, binary sha256, run RCs
- probe_main.go.txt — probe source (run inside a pristine `git archive` copy of a0819096 at /tmp/ds-a17-src)
- go.mod / go.sum — module identity used for the probe build
- build/probe_a17 — probe binary (sha256 in baseline.txt)
- sql/{N1,N2,L1,L2,C}.sql — five fixed fixtures, verbatim
- policy/all-off.yaml — temporary all-rules-disabled policy (copied from prior golden run config)
- extract/*-{mysql,tidb}.json — 10 real parser+extractor runs: AST facts, String/CompactStr/Restore, spec.Column
- audit/audit-*-{mysql,tidb}.json — 10 real AuditSQL runs (all-off, no provider)
- provider/provider-{mysql,tidb}.json — 2 real LoadTableSnapshot runs with controlled database/sql driver (constructed catalog rows, query ledger included)
- run-*.stdout/stderr.log — captured run output; all rc=0
- pairing-table.md — AST → formatted → spec / catalog → snapshot comparison
- minimal-design.md — one recommended minimal repair design

Labels: extract/audit = real product calls; provider inputs = constructed catalog rows;
consumer compatibility = source-code deduction (enrich not re-run).
