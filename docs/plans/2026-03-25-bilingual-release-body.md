# Bilingual Release Body Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make GitHub Release pages publish complete English-first bilingual notes and backfill existing `v0.6.1` and `v0.6.2` releases.

**Architecture:** Keep `docs/releases/release-notes-vX.Y.Z.md` and `docs/releases/release-notes-vX.Y.Z.zh-CN.md` as the source of truth. In the release workflow, resolve both files from the pushed tag, concatenate them into one temporary markdown file with a separator, and explicitly sync the GitHub Release body with `gh release edit --notes-file` after GoReleaser publishes assets.

**Tech Stack:** GitHub Actions workflow YAML, GoReleaser action, GitHub CLI (`gh`), Markdown release-note files.

---

### Task 1: Add bilingual release-body assembly to the workflow

**Files:**
- Modify: `.github/workflows/release.yml`
- Check: `docs/releases/release-notes-v0.6.2.md`
- Check: `docs/releases/release-notes-v0.6.2.zh-CN.md`

**Step 1: Write the failing expectation**

Expectation: a tag release should fail if either the English or Chinese release-note file is missing.

**Step 2: Encode file resolution in workflow**

Add shell steps that resolve:
- `docs/releases/release-notes-${TAG}.md`
- `docs/releases/release-notes-${TAG}.zh-CN.md`

and exit non-zero if either file is absent.

**Step 3: Assemble a bilingual notes file**

Create a temporary combined markdown file in the workflow with this order:
1. English release notes
2. blank line
3. `---`
4. blank line
5. Chinese release notes

**Step 4: Update release publishing sync step**

Use `gh release edit "${GITHUB_REF_NAME}" --notes-file "${COMBINED_RELEASE_NOTES_FILE}"` after GoReleaser publishes assets.

**Step 5: Verify YAML remains valid**

Run: `python3 - <<'PY'
import yaml, pathlib
path = pathlib.Path('.github/workflows/release.yml')
yaml.safe_load(path.read_text())
print('YAML OK')
PY`
Expected: `YAML OK`

### Task 2: Backfill bilingual notes into existing releases

**Files:**
- Check: `docs/releases/release-notes-v0.6.1.md`
- Check: `docs/releases/release-notes-v0.6.1.zh-CN.md`
- Check: `docs/releases/release-notes-v0.6.2.md`
- Check: `docs/releases/release-notes-v0.6.2.zh-CN.md`

**Step 1: Build combined notes for `v0.6.1`**

Create a temporary markdown file containing EN first, then `---`, then ZH.

**Step 2: Update GitHub Release `v0.6.1`**

Run: `gh release edit v0.6.1 --notes-file <combined-file>`
Expected: GitHub returns the release URL.

**Step 3: Build combined notes for `v0.6.2`**

Create a temporary markdown file containing EN first, then `---`, then ZH.

**Step 4: Update GitHub Release `v0.6.2`**

Run: `gh release edit v0.6.2 --notes-file <combined-file>`
Expected: GitHub returns the release URL.

**Step 5: Verify both release bodies are bilingual**

Run:
- `gh release view v0.6.1 --json body`
- `gh release view v0.6.2 --json body`

Expected: each body contains both the English heading and the Chinese heading.

### Task 3: Commit the workflow fix

**Files:**
- Modify: `.github/workflows/release.yml`
- Create: `docs/plans/2026-03-25-bilingual-release-body.md`

**Step 1: Review the diff**

Run: `git diff -- .github/workflows/release.yml docs/plans/2026-03-25-bilingual-release-body.md`
Expected: only workflow changes and the plan document are present.

**Step 2: Commit the change**

Run:
`git add .github/workflows/release.yml docs/plans/2026-03-25-bilingual-release-body.md && git commit -m "ci: publish bilingual GitHub release notes"`

**Step 3: Push the change**

Run: `git push origin main`
Expected: remote `main` advances successfully.
