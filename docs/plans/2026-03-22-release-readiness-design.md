# Release Readiness Design

## Goal

Prepare DeltaScope for its first polished public release after the current `v0.5.0` baseline by:

- reshaping the documentation tree into product-facing and maintainer-facing sections
- rewriting `README.md` and `README_ZH.md` as landing pages instead of internal developer notes
- adding a single trusted GitHub Actions release path
- adding an `install.sh` path that matches published artifacts
- expanding `Makefile` into a small, stable operator surface

The target release version is `v0.6.0`. The repository already ships `v0.5.0`, so version numbers must not move backward.

## Context

DeltaScope now has:

- a complete CLI surface
- metadata-aware live smoke coverage for MySQL and TiDB
- a thin HTTP service
- a stable public Go package

What it does not yet have is a product-quality release and documentation surface. Current gaps:

- `README.md` and `README_ZH.md` still read like implementation notes
- the audit capability matrix lives under `docs/plans/`, which is the wrong long-term home
- there is no release workflow under `.github/workflows/`
- there is no `install.sh`
- `Makefile` only exposes metadata e2e targets

## Non-Goals

This milestone does not:

- add new audit rules
- change HTTP service behavior
- add MCP support
- add package-manager distribution such as Homebrew
- replace the existing CLI or HTTP contracts

## Approaches Considered

### Approach A: Release First, Docs Later

Ship a workflow, archives, and install script immediately, then clean up docs afterward.

Pros:

- fastest path to a tag

Cons:

- release docs and artifact naming drift easily
- homepage still undersells the product
- support burden rises right after release

### Approach B: Docs and Release Together

Restructure docs, rewrite the homepage, then wire the release pipeline and install path around the same artifact contract.

Pros:

- one coherent public surface
- README/install/release assets stay aligned
- lowers post-release confusion

Cons:

- slightly larger milestone

### Approach C: Full Packaging Ecosystem

Do everything in one pass: workflow, install script, package-manager support, release notes, docs restructure, and CI expansion.

Pros:

- strongest launch surface

Cons:

- too much scope for one milestone
- higher risk of release-path churn

## Recommendation

Choose Approach B.

It keeps the milestone focused on one thing: making DeltaScope look and behave like a releasable product. It also matches the existing need to reconcile docs, artifact naming, install instructions, and GitHub release output.

## Design

### 1. Documentation Information Architecture

Create a clear split between product docs and internal planning docs.

Target structure:

- `docs/admin/`
- `docs/concept/`
- `docs/dev/`
- `docs/recipe/`
- `docs/reference/`

Responsibilities:

- `docs/admin/`: release, roadmap, support, security extensions
- `docs/concept/`: product-level architecture, core concepts, metadata-aware mode
- `docs/dev/`: development workflow, testing, implementation architecture
- `docs/recipe/`: task-oriented guides for DBA and developer workflows
- `docs/reference/`: stable lookup docs such as CLI, HTTP API, config, rules, capability matrix

`docs/plans/` remains for internal design, implementation plans, handoff, and milestone records only.

### 2. README Strategy

`README.md` and `README_ZH.md` become product landing pages.

Front-half content:

- project identity and shields
- short positioning
- install
- quick start
- why DeltaScope
- key features
- recipes
- documentation index

Back-half content:

- architecture map and module map required by the L1 three-level-doc contract
- contributing, status, license links

This preserves the L1 requirements without letting them dominate first contact with the project.

### 3. Architecture Diagrams

Use ASCII diagrams in two places:

- `docs/concept/architecture.md`: high-level product workflow
- `docs/dev/architecture.md`: implementation layering and dependency direction

The README should link to these docs instead of embedding large diagrams inline.

### 4. Capability Matrix Relocation

Move the audit capability matrix from `docs/plans/2026-03-21-audit-capability-matrix.md` to a stable reference location, ideally:

- `docs/reference/audit-capability-matrix.md`

The matrix becomes a product/reference artifact, not a transient plan attachment.

### 5. Trusted Release Path

Adopt a single release path:

`tag -> GitHub Actions release workflow -> tested archives -> checksums -> GitHub Release assets`

Use the existing `BinlogVisualizer` workflow pattern as the reference shape:

- run `go test ./...`
- validate packaging config
- build darwin/linux archives for amd64/arm64
- assemble checksums
- publish GitHub Release assets on `v*` tags

Do not maintain multiple half-overlapping release paths.

### 6. Install Path

Add `install.sh` that:

- resolves OS and architecture
- downloads the matching release archive from GitHub Releases
- verifies and extracts the archive
- installs `deltascope` and optionally `deltascope-server` into a target directory

The artifact naming in:

- workflow
- install script
- README install examples
- checksums

must all match exactly.

### 7. Makefile Expansion

Keep `Makefile` small and stable. It should surface common operations such as:

- `test`
- `test-e2e-cli`
- `build`
- `build-cli`
- `build-server`
- `install-local`

Do not turn it into a second build system.

### 8. Release Version

Use `v0.6.0` as the next release target.

Rationale:

- `v0.5.0` already exists in repository history and public version strings
- this milestone is a feature/product-surface upgrade, not just a patch
- bumping to `v0.6.0` preserves semantic ordering and avoids rewriting history

## Acceptance Criteria

This milestone is complete when:

1. The docs tree contains the new product-facing sections and the capability matrix has moved out of `docs/plans/`.
2. `README.md` and `README_ZH.md` read like product landing pages, while preserving L1 module-map requirements near the end.
3. `.github/workflows/release.yml` exists and defines the single trusted release path.
4. `install.sh` matches workflow artifact names and README install instructions.
5. `Makefile` exposes the agreed common targets.
6. Release docs and scripts all consistently target `v0.6.0`.
7. The repository is ready for a real tag-based release path validation.
