# 双语 Release 正文实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标：** 让 GitHub Release 页面发布完整的“英文在前”的双语说明，并补写已有 `v0.6.1` 与 `v0.6.2` release 的正文。

**架构：** 保持 `docs/releases/release-notes-vX.Y.Z.md` 与 `docs/releases/release-notes-vX.Y.Z.zh-CN.md` 作为事实来源。在 release workflow 中，根据 pushed tag 同时解析两份文件，将它们按分隔符拼接成一个临时 markdown 文件，并在 GoReleaser 发布资产后通过 `gh release edit --notes-file` 显式同步 GitHub Release 正文。

**技术栈：** GitHub Actions workflow YAML、GoReleaser action、GitHub CLI (`gh`)、Markdown release-note 文件

---

### 任务 1：在 workflow 中增加双语 release 正文拼装

**Files:**
- Modify: `.github/workflows/release.yml`
- Check: `docs/releases/release-notes-v0.6.2.md`
- Check: `docs/releases/release-notes-v0.6.2.zh-CN.md`

**Step 1: 写出失败预期**

预期：如果英文或中文任意一份 release-note 文件缺失，则 tag release 应失败。

**Step 2: 在 workflow 中编码文件解析逻辑**

增加 shell 步骤，解析：
- `docs/releases/release-notes-${TAG}.md`
- `docs/releases/release-notes-${TAG}.zh-CN.md`

若任一文件不存在，则以非零状态退出。

**Step 3: 组装双语 notes 文件**

在 workflow 中创建一个临时的合并 markdown 文件，顺序如下：
1. 英文 release notes
2. 空行
3. `---`
4. 空行
5. 中文 release notes

**Step 4: 更新发布同步步骤**

在 GoReleaser 发布资产之后，使用 `gh release edit "${GITHUB_REF_NAME}" --notes-file "${COMBINED_RELEASE_NOTES_FILE}"`。

**Step 5: 校验 YAML 仍然合法**

Run: `python3 - <<'PY'
import yaml, pathlib
path = pathlib.Path('.github/workflows/release.yml')
yaml.safe_load(path.read_text())
print('YAML OK')
PY`
Expected: `YAML OK`

### 任务 2：为已有 release 回填双语说明

**Files:**
- Check: `docs/releases/release-notes-v0.6.1.md`
- Check: `docs/releases/release-notes-v0.6.1.zh-CN.md`
- Check: `docs/releases/release-notes-v0.6.2.md`
- Check: `docs/releases/release-notes-v0.6.2.zh-CN.md`

**Step 1: 为 `v0.6.1` 构建合并 notes**

创建一个临时 markdown 文件，内容为英文在前，接着 `---`，再接中文。

**Step 2: 更新 GitHub Release `v0.6.1`**

Run: `gh release edit v0.6.1 --notes-file <combined-file>`
Expected: GitHub 返回 release URL。

**Step 3: 为 `v0.6.2` 构建合并 notes**

创建一个临时 markdown 文件，内容为英文在前，接着 `---`，再接中文。

**Step 4: 更新 GitHub Release `v0.6.2`**

Run: `gh release edit v0.6.2 --notes-file <combined-file>`
Expected: GitHub 返回 release URL。

**Step 5: 验证两个 release 正文都为双语**

Run:
- `gh release view v0.6.1 --json body`
- `gh release view v0.6.2 --json body`

Expected: 每个正文都同时包含英文标题与中文标题。

### 任务 3：提交 workflow 修复

**Files:**
- Modify: `.github/workflows/release.yml`
- Create: `docs/plans/2026-03-25-bilingual-release-body.md`

**Step 1: 查看 diff**

Run: `git diff -- .github/workflows/release.yml docs/plans/2026-03-25-bilingual-release-body.md`
Expected: 只出现 workflow 变更与计划文档。

**Step 2: 提交变更**

Run:
`git add .github/workflows/release.yml docs/plans/2026-03-25-bilingual-release-body.md && git commit -m "ci: publish bilingual GitHub release notes"`

**Step 3: Push 变更**

Run: `git push origin main`
Expected: 远端 `main` 成功前进。