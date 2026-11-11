# T26-SEMANTICS-01 §F — stage-matrix correspondence + open items (R1 revision)

## F.1 `cli_*` cells — per-cell judgment (R1-E.4)

All 23 remain verified against their R6 candidate `16eba8a0`; none is
re-registered to a later commit. The slice changes expected behavior
for the two DROP cells below — they must be re-versioned in the
implementation work order's manifest update (new expectations
recorded; the R6 pass record stays bound to `16eba8a0`):

| case id | manifest SQL | R6 expectation | post-slice expectation (offline, no provider) |
|---|---|---|---|
| `T26.cli.mysql-drop-server` | `DROP SERVER golden_srv` | `unsupported`+`unaudited`, coverage incomplete, exit 1/review | audited path: existence unbound (provider-not-loaded) → notice + gap `server.existence` → review / unverified; `unsupported` drops to 0 |
| `T26.cli.mysql-drop-server-if-exists` | `DROP SERVER IF EXISTS golden_srv` | same | notice + gap → review / unverified; modeled post conditional-absent |

**`mysql-server-mixed-batch` cell — unchanged (verified against the
actual manifest entry).** Its manifest row is:

```json
{ "id": "mysql-server-mixed-batch", "dialect": "mysql",
  "sql": "CREATE TABLE t (id INT); CREATE SERVER golden_srv FOREIGN DATA WRAPPER mysql OPTIONS (PASSWORD 'golden-secret', HOST '192.0.2.1'); DROP TABLE t",
  "expect": { "exit": 1, "verdict": "review", "statements": 3,
    "findings": 0, "diagnostics": 1, "unsupported": 1,
    "coverage": "incomplete",
    "statement_coverage": ["complete","incomplete","complete"],
    "unsupported_entries": [ { "index": 1, "feature": "create_server",
        "metadata": {"boundary":"unaudited","object":"server"} } ],
    "statement_sql": ["CREATE TABLE t (id INT);","CREATE SERVER <redacted>","DROP TABLE t"] } }
```

Judgment: the unit's SERVER member is `CREATE SERVER` — the slice
audits only `DROP SERVER` on `NameIdent` forms. The CREATE keeps its
`unaudited` boundary, so `unsupported`=1, per-statement coverage
`[complete, incomplete, complete]`, redaction, and exit/verdict all
stand. It does NOT contain a DROP that would move from boundary to
audited path. (Post-slice it additionally taints any provably-same
ordered entry — none exists in this batch.)

Cells unchanged by the slice: `mysql-create-server-*`,
`mysql-alter-server`, `tidb-*`, `srs-*`, `alter-instance-*`.
Changed-cell count after the per-cell check: **exactly two**.

Future manifest additions the implementation order may stage (PLAN
only): `DROP s1; DROP S1` alias pair, `DROP s1; CREATE s1; DROP s1`
invalidation pair, literal-name drop taint pair — each as new cells,
never a rewrite of the R6 records.

## F.2 `db_*` cells — 8 required units, unexecuted

`T26.db.mysql57.ddl`, `T26.db.mysql57.syntax_negative`,
`T26.db.mysql80.ddl`, `T26.db.mysql80.syntax_negative`,
`T26.db.mysql84.ddl`, `T26.db.mysql84.syntax_negative`,
`T26.db.tidb85.ddl`, `T26.db.tidb85.syntax_negative` — unexecuted under
this order; no offline stub result is registered as a DB PASS. Native
oracles for: INE ordering (row 20), OR REPLACE absent (row 21), option
dup/truncation adjudication (C14/C15), per-version `server_name`
collation.

## F.3 Open items — R1 dispositions (G-1..G-8 ruled this round)

| id | item | R1 ruling | state in this revision |
|---|---|---|---|
| G-1 | literal-name DROP | **accepted**: `unaudited` boundary kept; identity_unproven binding + zero catalog queries stay | C6/D5/E3 |
| G-2 | SRS conflict wiring | **provider-only** continues; no authored spelling enters conflict qualifiers until a trusted-identity basis exists | C.2 rows 31–32 |
| G-3 | SRS drop-ok | empty-deps arm stays unreachable; positive-deps/warning/refusal relation corrected (C.2 row 25 three arms) | C.2 row 25 |
| G-4 | ordered-absence representation | **accepted direction** — explicit provenance required; field design pending knownness/mutual-exclusion/consumer contract | D.4 `OrderedAbsent` contract |
| G-5 | unknown + IF EXISTS | **not approved as blanket "unknown"** — per-branch derivation required | C.4/D4/E5: conditional absent from convergent arms |
| G-6 | SERVER compare rule | exact collation stays PEND; **first slice must handle aliases** — verbatim object key not reusable for ordered channel | C17/C18 canonical ASCII fold + `server.batch_effects` residual |
| G-7 | SRS row 13 | stays proposal — **per-check version applicability**, not blanket static-only | C.2 row 13 revised |
| G-8 | ALTER INSTANCE | named boundary + ownership kept; registered RELOAD TLS members stay T26-owned — not moved to sibling scope | C.3 |
