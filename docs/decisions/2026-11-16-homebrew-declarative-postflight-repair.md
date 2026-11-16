# Repair the Homebrew declarative postflight migration

Date: 2026-11-16
Status: Accepted

## Context

Release `v0.514.0` is an annotated, provenance-valid tag at
`eec3bca5c1f56f17419ab7b4ef6be48ca252e709`. Release run
[43812771712](https://github.com/Fanduzi/DeltaScope/actions/runs/43812771712)
published all platform assets, npm, and the Homebrew cask, but its install
verifier failed while loading the cask: `staged_path` is undefined in
`Homebrew::InstallSteps::DSL`.

Commit `84be045c` renamed `postflight` without migrating its body. The new
DSL does not accept the legacy Ruby `staged_path` accessor or
`system_command`. The old static cask check covered only version, URLs,
checksums, and the binary stanza, so it missed the incompatibility.

## Decision

Use declarative `postflight_steps` with `on_macos`, `run`, and the literal
install-time token `{{staged_path}}/deltascope`. Preserve the same `xattr`
executable, arguments, and binary-only target. Extend the rendered-cask
contract and release hygiene gate with positive and legacy-regression tests.

For this incident, repair only `Casks/deltascope.rb` in the published tap,
using the reviewed renderer fix and checksums from the existing release.
Then rerun only the failed install-verification job. The normal recovery
workflow deliberately renders from the verified tag target; dispatching it
here would reproduce the broken renderer. This one-time tap repair does not
weaken that provenance contract or introduce a configurable source override.

## Rationale

Moving the published tag, replacing archives, or republishing npm would
change unrelated, already-correct artifacts. Reverting to legacy Ruby flight
blocks would restore the deprecation rather than complete the migration.
A narrow tap correction preserves release identity while fixing installation.

## Public Contract

SQL audit behavior, output shape, release version, archive URLs/checksums,
and npm contents are unchanged. Homebrew retains explicit trust before
installation and clears quarantine only on the staged CLI binary on macOS.
No system-wide quarantine setting or Gatekeeper policy is changed.

## Deferred / Out Of Scope

No tag movement, asset replacement, npm overwrite, full publisher rerun,
or recovery-provenance exception. Static regression checks cover this emitted
DSL shape, not every Homebrew API; real Homebrew install verification remains
necessary.

## Verification Evidence

- The local Homebrew content loader reproduced the exact `staged_path`
  failure from the published cask, then loaded the corrected cask with a
  `Cask::Artifact::PostflightSteps` artifact present.
- `python3 scripts/test_homebrew_cask.py`: failed before the fix; passed
  after it, including four legacy-construct mutations.
- `make release-workflow-hygiene-gates`: passed after the fix.
- Verified against local Homebrew `7.0.13-140-g1cd7b3e`, source
  `1cd7b3e1341b257955f5d31106637d7696019129`, and the official cookbook
  at source `7d562e1c8f8d233c822eb029c33a6ce1a4a147d6` on 2026-11-16.

## Consequences

A routine recovery of the original tag would restore its old renderer;
operators must account for this incident before recovering its Homebrew
channel. Subsequent releases include the corrected renderer and regression
gate. The final tap commit and successful install run belong in the release
delivery evidence, not in an invented claim before publication.

## Links

- [Official Homebrew flight steps and interpolation contract](https://github.com/Homebrew/brew/blob/7d562e1c8f8d233c822eb029c33a6ce1a4a147d6/docs/Cask-Cookbook.md#stanza-flight_steps)
- [Renderer](../../scripts/render_homebrew_cask.sh)
- [Cask verifier](../../scripts/verify_homebrew_cask.sh)
- [Regression tests](../../scripts/test_homebrew_cask.py)
- [Existing trust contract](2026-07-27-release-homebrew-trust-contract.md)
- [Release recovery contract](../dev/release-recovery.md)
