# Release Readiness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reshape DeltaScope's documentation and release surface so the project is ready to publish a polished `v0.6.0` release.

**Architecture:** Keep one trusted release path and one product-facing documentation hierarchy. First fix the docs structure and landing pages, then wire the release workflow, install script, and build entrypoints around a single artifact contract.

**Tech Stack:** Go, Cobra CLI, GitHub Actions, shell install script, Markdown docs, Makefile

---

### Task 1: Create the Product Docs Skeleton

**Files:**
- Create: `docs/admin/README.md`
- Create: `docs/concept/README.md`
- Create: `docs/dev/README.md`
- Create: `docs/recipe/README.md`
- Create: `docs/reference/README.md`
- Modify: `README.md`
- Modify: `README_ZH.md`

**Step 1: Write the failing check mentally**

Current failure:
- docs are not grouped into product-facing sections
- README has no stable links into `admin/concept/dev/recipe/reference`

**Step 2: Add the directory skeleton**

Create short README/index files for each new docs section so the tree is navigable immediately.

**Step 3: Update the top-level READMEs**

Add a docs navigation section that links to the new directories without yet rewriting the whole homepage.

**Step 4: Verify**

Run:

```bash
rg -n "docs/(admin|concept|dev|recipe|reference)" README.md README_ZH.md
```

Expected:
- links exist in both landing pages

**Step 5: Commit**

```bash
git add README.md README_ZH.md docs/admin/README.md docs/concept/README.md docs/dev/README.md docs/recipe/README.md docs/reference/README.md
git commit -m "docs: add product docs structure"
```

### Task 2: Move the Capability Matrix Into Reference Docs

**Files:**
- Create: `docs/reference/audit-capability-matrix.md`
- Modify: `docs/reference/README.md`
- Modify: `README.md`
- Modify: `README_ZH.md`

**Step 1: Copy the canonical capability matrix**

Move the stable content out of `docs/plans/2026-03-21-audit-capability-matrix.md` into `docs/reference/audit-capability-matrix.md`.

**Step 2: Rewrite links**

Point product docs to the new reference location.

**Step 3: Verify**

Run:

```bash
rg -n "audit-capability-matrix" README.md README_ZH.md docs/reference
```

Expected:
- product docs point at `docs/reference/audit-capability-matrix.md`

**Step 4: Commit**

```bash
git add docs/reference/audit-capability-matrix.md docs/reference/README.md README.md README_ZH.md
git commit -m "docs: move capability matrix into reference docs"
```

### Task 3: Add Architecture and Concept Docs

**Files:**
- Create: `docs/concept/architecture.md`
- Create: `docs/concept/core-concepts.md`
- Create: `docs/concept/metadata-aware-mode.md`
- Create: `docs/dev/architecture.md`
- Modify: `docs/concept/README.md`
- Modify: `docs/dev/README.md`

**Step 1: Write the product architecture doc**

Include a high-level ASCII flow covering:
- SQL input
- policy/config
- parser/extractor
- optional metadata enrichment
- rule evaluation
- CLI / HTTP / library outputs

**Step 2: Write the implementation architecture doc**

Include an ASCII layering view covering:
- cmd
- interfaces
- application
- domain
- infrastructure
- pkg

**Step 3: Add supporting concept docs**

Keep them concise and user-facing.

**Step 4: Verify**

Run:

```bash
rg -n "ASCII|metadata-aware|rule|verdict" docs/concept docs/dev
```

Expected:
- docs exist and contain the expected concept anchors

**Step 5: Commit**

```bash
git add docs/concept docs/dev
git commit -m "docs: add concept and architecture references"
```

### Task 4: Add Recipe and Reference Docs

**Files:**
- Create: `docs/recipe/audit-sql-offline.md`
- Create: `docs/recipe/audit-sql-with-metadata.md`
- Create: `docs/recipe/review-ddl-before-migration.md`
- Create: `docs/recipe/guard-dml-in-ci.md`
- Create: `docs/recipe/use-with-ai-agents.md`
- Create: `docs/recipe/inspect-rules-and-config.md`
- Create: `docs/reference/cli.md`
- Create: `docs/reference/config.md`
- Create: `docs/reference/rules.md`
- Create: `docs/reference/http-api.md`
- Modify: `docs/recipe/README.md`
- Modify: `docs/reference/README.md`

**Step 1: Add the recipe set**

Each recipe should be short, task-oriented, and command-driven.

**Step 2: Add the reference set**

Keep these lookup-oriented, not tutorial-style.

**Step 3: Verify**

Run:

```bash
find docs/recipe docs/reference -maxdepth 1 -type f | sort
```

Expected:
- the planned recipe and reference files exist

**Step 4: Commit**

```bash
git add docs/recipe docs/reference
git commit -m "docs: add recipes and product references"
```

### Task 5: Rewrite README.md as a Product Landing Page

**Files:**
- Modify: `README.md`

**Step 1: Rewrite the top half**

Use this order:
- hero + shields
- short positioning
- install
- quick start
- why DeltaScope
- key features
- recipes
- documentation links

**Step 2: Keep L1 information near the end**

Preserve the architecture/module map required by three-level-doc, but move it to the lower half of the file.

**Step 3: Replace `go run` first-contact examples**

Teach installation and released binaries first. Move development-oriented instructions to dev docs.

**Step 4: Verify**

Run:

```bash
sed -n '1,220p' README.md
```

Expected:
- product-first top half
- module map retained near the end

**Step 5: Commit**

```bash
git add README.md
git commit -m "docs: rewrite english landing page"
```

### Task 6: Rewrite README_ZH.md as a Product Landing Page

**Files:**
- Modify: `README_ZH.md`

**Step 1: Mirror the English structure**

Do not leave the Chinese landing page lagging behind the English one.

**Step 2: Verify**

Run:

```bash
sed -n '1,220p' README_ZH.md
```

Expected:
- Chinese landing page mirrors the product-first structure

**Step 3: Commit**

```bash
git add README_ZH.md
git commit -m "docs: rewrite chinese landing page"
```

### Task 7: Add the Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`
- Optional Create: `.goreleaser.yml`
- Modify: `CHANGELOG.md`
- Modify: `SECURITY.md`
- Modify: `pkg/deltascope/version.go`

**Step 1: Choose one trusted path**

Use a tag-driven GitHub Actions workflow modeled after the proven `BinlogVisualizer` release shape.

**Step 2: Implement the release job**

It should:
- trigger on `v*` tags
- run `go test ./...`
- validate packaging config if used
- build darwin/linux archives for amd64/arm64
- create checksums
- publish GitHub Release assets

**Step 3: Set the next release version**

Update the default version to `v0.6.0`.

**Step 4: Verify**

Run:

```bash
sed -n '1,260p' .github/workflows/release.yml
go test ./...
```

Expected:
- workflow exists
- tests still pass

**Step 5: Commit**

```bash
git add .github/workflows/release.yml .goreleaser.yml CHANGELOG.md SECURITY.md pkg/deltascope/version.go
git commit -m "build: add release workflow"
```

### Task 8: Add install.sh and Artifact Contract Docs

**Files:**
- Create: `install.sh`
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: `CHANGELOG.md`

**Step 1: Implement the install script**

Support:
- OS/arch detection
- GitHub release download
- archive extraction
- install destination selection

**Step 2: Align README install instructions**

The README and install script must agree on:
- archive names
- binaries installed
- destination assumptions

**Step 3: Verify**

Run:

```bash
sh -n install.sh
rg -n "install.sh|curl|tar.gz|v0.6.0" README.md README_ZH.md CHANGELOG.md install.sh
```

Expected:
- shell syntax passes
- naming is consistent

**Step 4: Commit**

```bash
git add install.sh README.md README_ZH.md CHANGELOG.md
git commit -m "build: add install script"
```

### Task 9: Expand Makefile for Local Operator Workflows

**Files:**
- Modify: `Makefile`
- Modify: `docs/dev/testing.md`
- Modify: `README.md`
- Modify: `README_ZH.md`

**Step 1: Add stable targets**

At minimum:
- `test`
- `build`
- `build-cli`
- `build-server`
- `test-e2e-cli`
- `test-e2e-cli-mysql`
- `test-e2e-cli-tidb`

**Step 2: Document them**

Keep the target set small and clearly explained.

**Step 3: Verify**

Run:

```bash
make -n test
make -n build
make -n build-cli
make -n build-server
```

Expected:
- all targets resolve cleanly

**Step 4: Commit**

```bash
git add Makefile README.md README_ZH.md docs/dev/testing.md
git commit -m "build: expand make targets"
```

### Task 10: Final Verification and Milestone Closeout

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`

**Step 1: Run verification**

Run:

```bash
go test ./...
make -n test
make -n build
make -n test-e2e-cli
sh -n install.sh
/Users/fan/.codex/skills/check-three-level-doc/scripts/check_three_level_doc.sh
```

**Step 2: Update handoff and decisions**

Record:
- new docs layout
- release path
- install path
- target version `v0.6.0`

**Step 3: Commit**

```bash
git add docs/plans/2026-03-20-overnight-handoff.md docs/plans/2026-03-20-autonomous-progress.md docs/plans/2026-03-20-deltascope-v1-decisions.md
git commit -m "docs: close release readiness milestone"
```
