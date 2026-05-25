# DeltaScope v0.180.0 Release Notes

## Summary — Release Surface Consistency Gates

v0.180.0 is a release engineering milestone that adds a release surface consistency checker to the release gate pipeline. It does **not** add new SQL parser support, new audit rules, or new product behavior.

## What Changed

- New release surface consistency checker (`scripts/verify_release_consistency.py`) validates that release-domain facts are consistent across all release surfaces: landing page, release notes EN/ZH, changelog, roadmap, rules reference, and capability matrix.
- `make release-version-surface-gates` now runs both the existing shell version surface checker and the new Python release semantic consistency checker.
- `make release-contract-gates` inherits the new checker through dependency.
- New `make release-consistency-test` target runs the consistency checker's test suite.
- Decision record: `docs/decisions/2026-05-25-v0.180.0-release-surface-consistency-gates.md`.

## Protected Surfaces

The consistency checker enforces:

- Landing recent release cards do not drift to stale version sequences.
- Residual census numbers (e.g., `finding_covered`) match the version-specific facts.
- Stale wording patterns (e.g., "unsupported_boundary N→N") are caught.
- PG ALTER TABLE rule count does not drift between versions.
- EN/ZH rule ID and numeric parity is maintained.
- Release overclaim patterns are flagged outside no-leak/non-goal contexts.
- Forbidden payload terms only appear inside no-leak context.

## Negative Test Coverage

The test suite (`scripts/test_verify_release_consistency.py`) covers:

- Stale recent release cards in landing page.
- Wrong `finding_covered` count (e.g., 64 instead of 60).
- Stale "unsupported_boundary N→N" wording.
- Missing ZH rule ID in release notes.
- Positive overclaim in release notes.
- Forbidden payload outside no-leak context.

## Unchanged Metrics

v0.180.0 does not change SQL parser, rule, or product behavior:

- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- Residual census: **66/60/2/0/4/0** (unchanged).
- SQL corpus: **535/535**, **100.0%**, **243 YAML files** (unchanged).

## Non-Goals

- Not a new SQL rule or parser feature release.
- Not full PostgreSQL ALTER TABLE support.
- Not PostgreSQL 18 parser support.
- Not runtime/live validation.
- Not rewrite duration estimate.
- Not v1.0/stable API contract claim.
