# T18 Evidence Index

The frozen facts are in [analysis/FINAL-FACTS.json](analysis/FINAL-FACTS.json).
This package is ready for independent review; it is not an acceptance, merge,
CI, release, deployment or issue-closure claim.

## Identities and final results

- Base: `7f67c5de42fb6096d9c80f51e2d0f217eb34e528`.
- Runtime product: `879c49f58d8df02120875fffc901dc2d672210f0`.
- Final product: `2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb` on
  `milestone/mysql-tidb-ddl-completion`; the task has 96 owned paths.
- The evidence identity is the commit containing this index and the
  [entrypoint](README.md), not a self-referential hash.
- The [final gate receipts](gates/) contain 23 entries: 21 rc 0 and two
  inherited lint rc 1, with zero lint identity deltas.
- [Final Golden artifacts](golden/) contain T16 60 cases / 832 assertions,
  T17 52 / 834 and T18 149 / 3,755. Companion Go proofs are separate.
- Frozen unresolved P1/P2 count: zero. Independent acceptance is not asserted.

## Navigate the evidence

| Directory | Contents |
|---|---|
| [baseline/](baseline/) | Original identity, resources and untracked-root baseline |
| [red/](red/) and [green/](green/) | Developmental failures, passes, snapshots and intermediate attempts |
| [native/](native/) | Disposable-anchor probe sources, raw outcomes and cleanup |
| [research/](research/) | Previously captured primary-source research receipts |
| [gates/](gates/) | Corrected final-product gate quads and lead checks |
| [golden/](golden/) | Final, WIP and seed artifacts plus copied policies |
| [attempts/](attempts/) | Earlier broad failure, WIP preflights and test/doc repair receipts |
| [analysis/](analysis/) | Frozen facts, generators, manifests and accounting tools |
| [review/](review/) | Historical reviews and the final-review/repair handoff |
| [preservation/](preservation/) | Source identity, protected resources and approved cleanup |

The [earlier review](review/FINAL-REVIEW.md) covered the 91-path product before
the five-file test/doc repair. Its resolution is the
[repair diff](attempts/gate-repair/green/five-file.patch) and the corrected
[final gate pass](gates/). Failed attempts remain labeled as such; planning
manifests in [analysis/plans/](analysis/plans/) were not executed as Goldens.

## Provenance and boundaries

[COPYMANIFEST.json](COPYMANIFEST.json) maps each copied source to its archived
path and both SHA-256 values. [SHA256SUMS](SHA256SUMS) covers the archive,
excluding itself. Source snapshots ending in `.go` were archived as `.go.txt`;
their original paths and overlay mappings remain in the copied receipts.

[The redaction ledger](preservation/redaction-ledger.json) identifies the
fixture-credential redactions and private original hashes. Three native helper
sources contain redacted credential literals and are archival, not runnable
credentials. Outside the 17 ledger-listed redactions, copied files retain their
original bytes. Synthetic private sentinels remain intact for privacy-evidence
review.

Original Golden artifact bytes and their embedded absolute execution paths are
unchanged; archived policies do not rebind those paths. The original foreign
root listing is preserved, but no historical foreign-content byte baseline is
claimed. Query-model and authorization boundaries are stated in the
[entrypoint](README.md).
