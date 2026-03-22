# Release Readiness Task Prompts

## Global Execution Rules

- Work on `main` unless a dedicated execution worktree is created.
- Keep three-level-doc as a hard requirement.
- Prefer one trusted release path; do not create parallel packaging logic.
- Keep artifact names consistent across workflow, install script, README, and checksums.
- Treat `v0.6.0` as the target version unless a later explicit version decision replaces it.

## Reviewer Return Template

Every task-level reviewer should report:

- pass or fail
- exact blocker or residual risk
- files inspected
- tests/commands checked
- commit hash under review

## Task 1 Prompt

Build the initial product docs skeleton under `docs/admin`, `docs/concept`, `docs/dev`, `docs/recipe`, and `docs/reference`. Update `README.md` and `README_ZH.md` just enough to link into the new tree. Do not rewrite the full landing pages yet.

Acceptance:
- the five docs sections exist
- both READMEs link to them
- three-level-doc still makes sense

## Task 2 Prompt

Move the audit capability matrix into `docs/reference/audit-capability-matrix.md` and update links so product docs no longer treat it as a transient plan artifact.

Acceptance:
- the matrix lives under `docs/reference`
- the READMEs point to the new location

## Task 3 Prompt

Add concept and architecture docs, including ASCII diagrams for both product-level and implementation-level views.

Acceptance:
- `docs/concept/architecture.md` exists with a product-level ASCII diagram
- `docs/dev/architecture.md` exists with an implementation-level ASCII diagram
- supporting concept docs exist and are linked

## Task 4 Prompt

Add the first recipe and reference sets. Recipes must be task-oriented and practical for DBA/developer workflows; reference docs must be stable lookup material.

Acceptance:
- recipe docs exist for offline, metadata-aware, DDL migration, CI DML guard, AI-agent usage, and rules/config inspection
- reference docs exist for CLI, config, rules, and HTTP API

## Task 5 Prompt

Rewrite `README.md` as a product landing page. Keep the L1 module/architecture map, but move it toward the end. Replace `go run` as the first-contact path with installation-first guidance.

Acceptance:
- the top half reads like a product homepage
- installation comes before development-oriented commands
- L1/module-map content still exists near the end

## Task 6 Prompt

Rewrite `README_ZH.md` to mirror the English landing page structure and quality level.

Acceptance:
- the Chinese landing page matches the English information architecture
- examples and links are kept in sync

## Task 7 Prompt

Add the single trusted release workflow under `.github/workflows/release.yml`, align version metadata to `v0.6.0`, and keep CHANGELOG/SECURITY in sync with the new release target.

Acceptance:
- workflow is tag-driven on `v*`
- tests remain passing
- default version becomes `v0.6.0`

## Task 8 Prompt

Add `install.sh` and align it with artifact names and README installation docs.

Acceptance:
- `install.sh` passes `sh -n`
- artifact naming is consistent across workflow, install script, README, and changelog

## Task 9 Prompt

Expand `Makefile` into a small stable operator surface without turning it into a second build system.

Acceptance:
- `test`, `build`, `build-cli`, `build-server`, and CLI e2e targets exist
- README/dev docs explain them succinctly

## Task 10 Prompt

Run final verification, update handoff/progress/decisions, and close the milestone.

Acceptance:
- verification commands are recorded
- handoff/progress/decision docs reflect the release-ready state
- no leftover ambiguity about the trusted release path
