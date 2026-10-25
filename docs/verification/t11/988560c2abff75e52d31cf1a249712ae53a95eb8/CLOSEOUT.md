# T11-R2 closeout — issue #90 hardening round 2

## What shipped

Four red→green groups closed on top of the T11-R1 chain. Product HEAD
`988560c2abff75e52d31cf1a249712ae53a95eb8` on `d0c7cf33` (L3 header
retirement) — 9 files, +1249/−50, including 4 new focused test files
and the two module READMEs.

### R2-A — PK structural rules read the statement's ordered column facts

`orderedColumnFact` now returns a tri-state verdict — resolved /
proven-absent / undecidable — shared verbatim by Evaluate and
EvidenceGaps. `statementColumnDecls` folds same-statement column
actions in authored order: MySQL resolves the effective statement-local
declaration (native-probed — `ADD COLUMN`+`ADD PRIMARY KEY` lands in
either order on 8.0.46/8.4.10, `MODIFY`+`ADD PRIMARY KEY` reads the new
definition), a `RENAME`/`CHANGE` carries the prior proven fact under
the new spelling, and a removed or unresolvable member is undecidable.
TiDB — which refuses same-statement member touches natively
(1072/8200) — keeps only pre-state facts for untouched members and
answers undecidable for touched ones. A proven-absent member is the
native refusal (no finding, no gap); only the undecidable middle owes
`target_table.columns`. bigint/unsigned/auto_increment share the
verdict.

### R2-D — complete keys vs partial references

`referencedKeyMatch` adds the strict-anchor complete-key test on top of
leftmost-prefix coverage, for NEW MySQL references only: observed
`restrict_fk_on_non_standard_key=ON` refuses a partial unique or
partial primary coverage with the 6125 class — identical to the
non-unique refusal (probed on 8.4.10, both flag arms). An exact match
needs no flag fact; an unobserved flag or unproven version leaves a
prefix-only match undecidable rather than guessing ON/OFF; pre-8.4
keeps the proven prefix rule; TiDB accepts partial references
(probed v8.5.0).

### R2-E — drop dependencies ask the binding question

`referencedDropDependency` now scans every loaded table including the
drop target itself — a self-reference's referenced-side sole coverer
binds, so `DROP PRIMARY KEY` and the child supporting index refuse
1553 on both anchors and the repeated drop still answers present. The
dependency check answers with `supportingIndexCandidate` kinds plus
physical prefix coverage — the physical-binding question — so a
provider-proven relationship keeps binding a non-standard member
under observed restrict ON (probed: an OFF-built non-unique parent
index still refuses its drop with 1553 after the flag flips ON).
Creatability governs new references only.

### R2-F — `applyDropCheck` covers every kind `DROP CONSTRAINT` removes

Probed on 8.0.46/8.4.10: `DROP CONSTRAINT` removes an FK, a unique
member, the stored `PRIMARY` object, and a real CHECK; `DROP CHECK`
removes only a real CHECK (3821 on bound non-CHECK names, 3940 on
unmatched). The authored keyword is unrecoverable from the shared AST
node, so a proven unique member or `PRIMARY` object now unvouches
like the FK arm — `unvouchIndexMember`/`unvouchPrimaryKeyObject` drop
the projection and unvouch the affected collection; presence-keyed
consumers owe the set gap instead of binding a maybe-gone member. A
proven secondary member provably survives both keywords, an unmatched
name refuses both, untargeted facts stay put, and TiDB's
OFF-warning-1105 / ON-3940 no-op arms are unchanged (verified with a
complete fixture/SQL/rc/warnings/KCU/SHOW CREATE cell).

## Verified vs not verified

**Verified:** all of the above on `988560c2`; targeted probes on
8.0.46/8.4.10/v8.5.0 plus the 5.7 syntax-refusal arm (`native-probe/`
with real per-statement client rc); deterministic CLI build
(`246d9de8…` identical across the three golden artifacts); T09/T10/T11
denominators preserved (125/126/40 cases, 3442/3175/505 assertions);
17 gates — 16 × rc=0 + lint rc=1 identical to the accepted baseline
(15 issues, none on R2 lines).

**Not verified / bounded:**
- The binding scan stays lazy (already-loaded entries) — a child that
  never became a statement target cannot bind; the R2-E test loads it
  through a prior statement exactly like the real audit path.
- `orderedColumnFact` sees the normalized spec model — parser-side
  multi-action column edges beyond add/modify/change/rename/drop stay
  on the undecidable arm.
- TiDB partial-reference acceptance is probed only on v8.5.0 (no
  restrict concept observed); older TiDB semantics unverified.
- `DROP CHECK`/`DROP CONSTRAINT` on a real CHECK member remains
  T12 scope — the model unvouches around it rather than tracking the
  member.

## Gate summary

17 gate quads under `gates/` bound to `988560c2`: 16 × rc=0 plus
`lint` rc=1 identical to the accepted baseline. `three-level-doc`
supplemented by `three-level-doc-diff` — the same L2 rule evaluated
over the committed `d0c7cf33..988560c2` file set (rc=0), since the
script reads worktree scope only and a committed tree shows no files.

## Remote / lifecycle status

Local branch `milestone/mysql-tidb-ddl-completion` at `988560c2`
plus this evidence commit. Fast-forward push to `origin/workfree`
per the standing authorization. No PR, merge, issue closure, tag,
release, or deployment performed or claimed; T11 stops at the
independent-acceptance candidate.
