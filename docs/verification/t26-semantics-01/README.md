# T26-SEMANTICS-01 evidence package

Scope: #105 / T26 — semantics-convergence work order: offline semantic
contract consolidation + first implementation-slice determination.
DOCUMENTATION ONLY: no product code, no test runs, no native evidence.
The accepted product object stays `7c2c1f23`; this package is a design
and contract record awaiting reviewer adjudication.

## A. Baseline + document identity

- work baseline: `dfa7852eb9606a11770cdef261e1c50d0d905b75`
  (T26-PROVIDER-01 archive commit)
- accepted product: `7c2c1f23d41960ef5f8d5efeb51c7fd166eb9a8e`
  (tree `cca8af6914cd1a5754d0724c5ac45cb9c1e8bdaa`)
- first document candidate: `8fc066571ab7946a8b42f79137fd832d065fc960`
  (tree `bee5ffb7f4e6f310bddd18407bccae7a566c1871`) — ruled
  **CHANGES REQUIRED** under T26-SEMANTICS-01/R1
- R1 revision: `5640954ee760d92011677a8b37add64579b81ab5`
  (tree `f9ea2a34367c0ba9b90558b24959e503e6423433`) — ruled
  **CHANGES REQUIRED** under T26-SEMANTICS-01/R2; this directory now
  carries the R2 revision (privilege-gap adjudication, unified ordered
  algorithm, SRS single-check merge contract, evidence-layer tagging)
- current document commit: registered in the delivery receipt for this
  round — the hash cannot pre-exist inside the commit that creates it
- changed files: this directory only
- tracked worktree status at commit: clean for tracked paths
  (pre-existing untracked files are unrelated and unstaged)

## File index

| file | answers |
|---|---|
| `b-prior-materials.md` | §B — prior-materials inventory + approval status (approved / applied-ruling / proposal / revised / missing) |
| `c-decision-table.md` | §C — consolidated decision table (R2 revision): R2-A privilege-gap adjudication + unknown barriers, six-step ordered algorithm C17–C22 with symmetric relation classes, SRS 33 rows under the single-check merge contract, full C.4 branch derivation |
| `d-first-slice.md` | §D — first slice `DROP SERVER` (R2 revision): reachable-vs-premise-dependent scope, relation classes, barrier/taint/write-record invalidation, premise-gated `OrderedAbsent` |
| `e-verification-plan.md` | §E — verification plan, 22 cases, plan/unexecuted (R2 revision: three observation layers, cur/dep split, reverse-alias + first-write cases) |
| `f-matrix-correspondence.md` | §F — cli_*/db_* cell correspondence + G-1..G-8 R1 dispositions (R2-updated drop expectations) |
| `g-source-excerpts.md` | §G-src — source excerpts with provenance labels (verbatim/comment/derived): R4 complete contract, R1 A.4/A.5, R2 §1/§5, T04 anchors, invalidation precedent, SRS empty-attr recognizer fragment |

## Source anchors reused (not re-copied)

`docs/verification/t26-r1/*`, `docs/verification/t26-r2/*`,
`docs/verification/t26-r6/`, `docs/verification/t26-provider-01/`,
`docs/decisions/2026-11-15-mysql-server-srs-recognition.md`,
`testdata/ddl-golden/T26.json`.
