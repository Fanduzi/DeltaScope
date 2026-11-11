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
  **CHANGES REQUIRED** under T26-SEMANTICS-01/R2
- R2 revision: `ac5433373b8d2e04e5d400a618d8dad669c4a3b9`
  (tree `7c155e90f0d9f7df9a92da96ead0b58509098c8c`) — ruled
  **CHANGES REQUIRED** under T26-SEMANTICS-01/R3
- R3 revision: `30e6afb82fce6c24fdd84800b6c273c21ee16bf6`
  (tree `6769b87cbc2644b5b6cfeeda824ae3e0951d2071`) — ruled
  **CHANGES REQUIRED** under T26-SEMANTICS-01/R4 (single item:
  the `R(A,B)` distinctness predicate); this directory now carries
  the R4 minimal conservative relation contract — byte-identical or
  registered-ASCII-CI-subset fold-equal ⇒ provably-same, everything
  else unproven, provably-distinct domain empty until a scoped proof
- R4 revision: `b8a67e7b5f7e62a825adace06113a233240d28df`
  (tree `787092acc65b51268ff08ab75902545ebcfb7ef4`) — **APPROVED**;
  acceptance registered in `acceptance.md`
- `acceptance.md` — the approval record: accepted object identities,
  what the contract establishes, explicitly-not-accepted items, and
  the R1–R4 round ledger
- changed files: this directory only
- tracked worktree status at commit: clean for tracked paths
  (pre-existing untracked files are unrelated and unstaged)

## File index

| file | answers |
|---|---|
| `b-prior-materials.md` | §B — prior-materials inventory + approval status (approved / applied-ruling / proposal / revised / missing) |
| `c-decision-table.md` | §C — consolidated decision table (R4 revision): minimal conservative R(A,B) contract (byte-identical / registered-ASCII-CI-subset ⇒ same; all else unproven; distinct domain empty), barriers-before-provider six-step resolveServer + absent-preserving transition table, privilege gap on all non-terminal-refusal drops, SRS capability-vs-external-fact gap split |
| `d-first-slice.md` | §D — first slice `DROP SERVER` (R4 revision): same minimal relation contract, smaller reachable scope (OrderedAbsent moved to future stage), unified order + transitions, D8 dual gaps |
| `e-verification-plan.md` | §E — verification plan, 21 cases, plan/unexecuted (R4 revision: E13 unproven leading-space + single request, E20 unproven `u`/`s1`, explicit [dep] preconditions) |
| `f-matrix-correspondence.md` | §F — cli_*/db_* cell correspondence + G-1..G-8 R1 dispositions (R2-updated drop expectations) |
| `g-source-excerpts.md` | §G-src — source excerpts with provenance labels (verbatim/comment/derived): R4 complete contract, R1 A.4/A.5, R2 §1/§5, T04 anchors, invalidation precedent, SRS return-path fragments (reject branch + success tail) |

## Source anchors reused (not re-copied)

`docs/verification/t26-r1/*`, `docs/verification/t26-r2/*`,
`docs/verification/t26-r6/`, `docs/verification/t26-provider-01/`,
`docs/decisions/2026-11-15-mysql-server-srs-recognition.md`,
`testdata/ddl-golden/T26.json`.
