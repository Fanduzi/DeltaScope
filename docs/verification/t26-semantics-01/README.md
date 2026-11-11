# T26-SEMANTICS-01 evidence package

Scope: #105 / T26 — semantics-convergence work order: offline semantic
contract consolidation + first implementation-slice determination.
DOCUMENTATION ONLY: no product code, no test runs, no native evidence.
The accepted product object stays `7c2c1f23`; this package is a design
and contract record awaiting reviewer adjudication.

## A. Baseline + document identity

- work baseline: `dfa7852eb9606a11770cdef261e1c50d0d905b75`
  (T26-PROVIDER-01 archive commit; tree
  `3decc0e8e5f1a1c1c14ab7e84d659fadfba21343`)
- accepted product: `7c2c1f23d41960ef5f8d5efeb51c7fd166eb9a8e`
  (tree `cca8af6914cd1a5754d0724c5ac45cb9c1e8bdaa`)
- document commit: registered in the delivery receipt for this round —
  the hash cannot pre-exist inside the commit that creates it
- changed files: this directory only (6 files, all new)
- tracked worktree status at commit: clean for tracked paths
  (pre-existing untracked files are unrelated and unstaged)

## File index

| file | answers |
|---|---|
| `b-prior-materials.md` | §B — prior-materials inventory + approval status (approved / applied-ruling / proposal / revised / missing) |
| `c-decision-table.md` | §C — consolidated semantic decision table: SERVER 16 units, SRS 33 rows mapped to current fact availability, ALTER INSTANCE deps, guard-clause contract |
| `d-first-slice.md` | §D — recommended first slice: `DROP SERVER` audited adjudication — rows, seams, files, order, kept boundaries |
| `e-verification-plan.md` | §E — minimal verification plan, 14 cases, labeled plan/unexecuted |
| `f-matrix-correspondence.md` | §F — cli_*/db_* cell correspondence + §G open items |

## Source anchors reused (not re-copied)

`docs/verification/t26-r1/*`, `docs/verification/t26-r2/*`,
`docs/verification/t26-r6/`, `docs/verification/t26-provider-01/`,
`docs/decisions/2026-11-15-mysql-server-srs-recognition.md`,
`testdata/ddl-golden/T26.json`.
