# Lead-Owned Evidence Packaging Recipe

This is a packaging recipe, not a gate or delivery result. Execute only after
the lead has checked final-SHA raw receipts. Never recompute prior results
while rendering the evidence; read the frozen lead-checked JSON captures.

Destination:
docs/verification/t18-view-lifecycle/2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb

## Ownership And Content

- README.md and INDEX.md state product/base identities, evidence commit as
  the commit containing the entrypoint, seven inventory dimensions, actual
  final gates, explicit boundaries, reviewer status, preservation and delivery
  constraints. The lead authors final factual claims after checking receipts.
- baseline/: original commands/outputs/rc/identity and protected-path snapshots.
- red/: all real red and failed-control quads plus baseline-compatible test
  source snapshots. Preserve failures, replay labels and execution identities.
- green/: developmental passes and failed intermediate attempts, explicitly
  distinguished from final-SHA results. Keep full WIP run 1 failure and run 2
  success without relabeling them as final-product executions.
- native/: the actual lead-authored dedicated-object probes, frozen SQL,
  complete valid outputs, images/versions/warnings/catalog and cleanup.
  Native facts are not counted as internal prospective-state assertions.
- gates/: all 23 final gate receipts and execution identity, scoped formatting,
  committed-range L2 checker, decision-record checks and lint comparisons.
- golden/: final T16/T17/T18 artifacts, original seed artifact, full WIP failed/
  corrected artifacts and exact policy YAMLs/fingerprints. Preserve original
  bytes and embedded absolute execution paths; do not rebind old artifacts.
- analysis/: final lead-authored generator, immutable seed and original failed
  full manifest/source, corrected full manifest, exact lint comparator and its
  sensitivity tests, artifact accounting scripts and lead-checked captures.
- review/: actual review records and final closure, including historical
  ITERATE findings with the final resolved verdict separately identified.
- preservation/: actual before/after protected-resource snapshots, foreign
  paths, worktrees, source hashes, historical artifact preservation and
  publication preflight. No CI/merge/issue-closure claim.

## Mechanical Rules

Do not copy compiled binaries, __pycache__, exploratory codegraph dumps, internal
lead notes, or unrelated root/debug resources. Preserve original temporary
artifacts rather than deleting them after copying. Source snapshots use
.go.txt, not compiled .go; an original overlay source may be archived as text
with its original execution path recorded. Do not introduce Go packages under
docs that alter go test ./... behavior.

Provide a concise README.md for every directory containing executable helper
code, including nested native probe directories. Export/dependency L2 follows
repository policy; retired L3 declarations are not added.

All copied scripts, queries, manifests, lint definitions and assertion logic
are byte-identical lead-authored artifacts. Do not invent new semantic proof,
query, threshold, sampling or validator logic while packaging. Module READMEs
may describe those already frozen artifacts.

Scan for secrets without printing sensitive values. Original disposable
fixture credentials must not be published as passwords; existing sanitization
markers are historical redactions, not executable credentials. If a raw
receipt needs redaction, preserve the original privately and record a
deterministic redaction ledger with the original SHA, field/category and
sanitized SHA. Do not redact synthetic private sentinels needed to audit the
no-leak tests, legitimate SQL fixture names, image digests or Git identities.

Create a SHA256 file manifest over archived files (excluding itself), then
check copied bytes. Exclude historical plans from final-status placeholder
claims: the final entrypoints describe only checked facts; raw historical
failed attempts remain truthful. The lead checks the complete evidence diff
before an appended evidence commit. No product edits, commits, pushes, PRs,
merges, issue closure, releases, deployments or T19 work in this packaging step.
