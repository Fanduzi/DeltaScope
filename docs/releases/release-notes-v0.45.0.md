# DeltaScope v0.45.0 Release Notes

## Summary

DeltaScope now emits GitLab Code Quality reports directly, so merge-request pipelines can surface SQL audit findings as inline code-quality annotations without any post-processing.

## What's New

### GitLab Code Quality Output Format

- `--format gitlab-codequality` CLI flag produces a JSON array matching the [GitLab Code Quality report](https://docs.gitlab.com/ee/ci/testing/code_quality.html) contract.
- Each DeltaScope finding maps to a Code Quality entry with `check_name`, `description`, `severity`, `fingerprint`, and `location` fields.
- File paths from `--file` propagate to `location.path`; inline SQL uses the audit input filename.
- Add the report as a CI artifact under `artifacts:reports:codequality` to see findings directly in merge-request diffs.

### Contract Tests & Release Gate

- Contract characterization tests lock the required JSON shape and semantic field guarantees.
- Unit tests cover the renderer with zero findings, single finding, and multiple finding cases.
- `make release-gitlab-codequality-smoke` gate validates the built CLI binary against the contract in the release pipeline.
- `make release-contract-gates` now includes the GitLab Code Quality smoke.

### Documentation

- New recipe: [Using DeltaScope in GitLab CI](../recipe/use-deltascope-in-gitlab-ci.md) with step-by-step `.gitlab-ci.yml` setup.
- CLI reference updated with `--format` flag documentation.
- Audit capability matrix updated to list GitLab Code Quality as a supported output format.

## Upgrade Notes

- No breaking changes to CLI flags, public API, rule behavior, or existing output formats.
- `--format json` (default) behavior is unchanged.
- The new `--format gitlab-codequality` flag is additive; existing CI configurations continue to work without modification.

## Scope Confirmation

- No parser, spec, domain-rule, or policy changes.
- No HTTP, MCP, or `pkg/deltascope` production-code changes.
- No new dependencies.
