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
  **CHANGES REQUIRED** under T26-SEMANTICS-01/R1; this directory now
  carries the R1 revision
- current document commit: registered in the delivery receipt for this
  round — the hash cannot pre-exist inside the commit that creates it
- changed files: this directory only
- tracked worktree status at commit: clean for tracked paths
  (pre-existing untracked files are unrelated and unstaged)

## File index

| file | answers |
|---|---|
| `b-prior-materials.md` | §B — prior-materials inventory + approval status (approved / applied-ruling / proposal / revised / missing) |
| `c-decision-table.md` | §C — consolidated decision table (R1 revision): premise-itemized SERVER units + ordered-channel contract C17–C22 + corrected SRS 33 rows + per-branch guard derivation |
| `d-first-slice.md` | §D — first slice `DROP SERVER` (R1 revision): premise-itemized rows, ordered identity + invalidation, `OrderedAbsent` contract, seams |
| `e-verification-plan.md` | §E — verification plan, 21 cases, plan/unexecuted (R1 revision: resolver-vs-query split, alias + taint + guard cases) |
| `f-matrix-correspondence.md` | §F — cli_*/db_* cell correspondence + G-1..G-8 R1 dispositions |
| `g-source-excerpts.md` | §G-src — verbatim historical excerpts (R4 complete contract, R1 A.4/A.5, R2 §1/§5, T04 anchors, invalidation precedent) with path + record identity |

## Source anchors reused (not re-copied)

`docs/verification/t26-r1/*`, `docs/verification/t26-r2/*`,
`docs/verification/t26-r6/`, `docs/verification/t26-provider-01/`,
`docs/decisions/2026-11-15-mysql-server-srs-recognition.md`,
`testdata/ddl-golden/T26.json`.
