#!/usr/bin/env python3
"""Static docs/examples drift checker for DeltaScope public docs.

This checker is intentionally STATIC and CURATED. It never executes Markdown
or YAML snippets and never touches the network, GitHub APIs, npm, Homebrew,
Docker, databases, or secrets. It only looks for known, high-risk drift
patterns in the current public docs and CI examples:

  1. stale rule-inspection commands (``deltascope rules show`` /
     ``deltascope rules search``),
  2. audit output-format inventory (every current format must appear in the
     files that list formats),
  3. the GitHub Actions example shape (read-only permissions, ``config lint
     --strict``, ``github-actions`` annotations, ``github-summary`` job
     summary, no PR-comment bot behavior, no token handling, and a
     ``DELTASCOPE_VERSION`` pin that matches ``$VERSION`` when supplied),
  4. the GitLab example shape (must use ``gitlab-codequality``, must not use
     the GitHub-specific formats),
  5. affirmative ``severity field`` wording for DeltaScope's public priority
     (DeltaScope uses ``level``; external schemas and negative clarifications
     are allowed).

Usage (from the repository root):

    python3 scripts/verify_docs_examples.py [root]

When ``VERSION=vX.Y.Z`` is set in the environment, the GitHub Actions example
must pin ``DELTASCOPE_VERSION`` to that exact version. Exit 0 on success
(``docs-examples: PASS``), exit 1 on failure (``docs-examples: FAIL`` followed
by one ``path:line: message`` line per finding).

Historical areas are explicitly ignored and may contain old commands as
context: ``docs/decisions/**``, ``docs/releases/**``, and ``CHANGELOG.md``.
"""

from __future__ import annotations

import os
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import List, Optional

# --------------------------------------------------------------------------- #
# Public scope
# --------------------------------------------------------------------------- #

# Markdown / YAML docs scanned for stale commands, format inventory, and
# severity language. Historical areas (decisions, releases, changelog) are
# deliberately excluded.
INVENTORY_FILES = [
    "README.md",
    "README_ZH.md",
    "docs/reference/cli.md",
    "docs/reference/cli.zh-CN.md",
]

SCAN_FILES = [
    "README.md",
    "README_ZH.md",
    "docs/reference/cli.md",
    "docs/reference/cli.zh-CN.md",
    "docs/reference/config.md",
    "docs/reference/config.zh-CN.md",
]

GITHUB_EXAMPLE = "docs/examples/github-actions.yml"
GITLAB_EXAMPLE = "docs/examples/gitlab-ci.yml"
RUNTIME_CONFIG = "docs/examples/runtime-config.yaml"

CURRENT_FORMATS = [
    "markdown",
    "json",
    "github-actions",
    "github-summary",
    "sarif",
    "gitlab-codequality",
]

IGNORED_PREFIXES = ("docs/decisions/", "docs/releases/")
IGNORED_EXACT = {"CHANGELOG.md"}

# Stale rule-inspection commands and their current replacements.
STALE_COMMANDS = [
    (re.compile(r"\brules\s+show\b"), "use `deltascope rules explain <rule-id>`"),
    (re.compile(r"\brules\s+search\b"), "use `deltascope rules list --search <query>`"),
]

# GitHub Actions example: required substrings and forbidden substrings.
GITHUB_REQUIRED = [
    "permissions:",
    "contents: read",
    "deltascope config lint --file deltascope.yaml --strict",
    "--format github-actions",
    "--format github-summary",
    '--fail-on none >> "$GITHUB_STEP_SUMMARY"',
]
GITHUB_FORBIDDEN = [
    "pull-requests: write",
    "GITHUB_TOKEN",
    "github-token",
    "workflow_dispatch",
    "gh pr comment",
    "actions/github-script",
    "issues/comments",
    "pulls/comments",
]

GITLAB_REQUIRED = ["gitlab-codequality"]
GITLAB_FORBIDDEN = ["github-summary", "github-actions"]

# Severity language. The trigger is the literal "severity field" phrase; the
# negative/external allow-lists are matched against the same line.
SEVERITY_TRIGGER = re.compile(r"severity\s+field", re.IGNORECASE)
SEVERITY_ALLOW = [
    re.compile(r"no\s+severity\s+field", re.IGNORECASE),
    re.compile(r"not\s+add\s+(?:a\s+)?severity\s+field", re.IGNORECASE),
    re.compile(r"GitLab\s+Code\s+Quality", re.IGNORECASE),
    re.compile(r"external\s+severity", re.IGNORECASE),
    re.compile(r"fail-on\s+severity", re.IGNORECASE),
]

VERSION_PIN_RE = re.compile(r"DELTASCOPE_VERSION:\s*[\"']?(v\d+\.\d+\.\d+)", re.IGNORECASE)


# --------------------------------------------------------------------------- #
# Data types
# --------------------------------------------------------------------------- #

@dataclass(frozen=True)
class File:
    """A collected in-scope file: repo-relative posix path and full text."""

    rel_path: str
    text: str


@dataclass(frozen=True)
class Failure:
    """A single drift finding with a repo-relative path and 1-based line.

    A line of 0 means the finding is file-level (no specific line applies).
    """

    path: str
    line: int
    message: str


# --------------------------------------------------------------------------- #
# Pure helpers
# --------------------------------------------------------------------------- #

def is_ignored_path(path: str) -> bool:
    """True for historical areas that may carry old commands as context."""
    norm = path.replace("\\", "/").lstrip("./")
    if norm in IGNORED_EXACT:
        return True
    return any(norm.startswith(prefix) for prefix in IGNORED_PREFIXES)


def line_number(text: str, index: int) -> int:
    """Return the 1-based line number of ``index`` within ``text``."""
    if index <= 0:
        return 1
    return text.count("\n", 0, index) + 1


def _read(root: Path, rel: str) -> Optional[str]:
    path = root / rel
    if not path.is_file():
        return None
    return path.read_text(encoding="utf-8")


def collect_files(root: str) -> List[File]:
    """Return every existing in-scope public file under ``root``.

    Historical areas are excluded so stale commands there do not count. Recipe
    docs are globbed so both English and zh-CN variants are covered.
    """
    rootp = Path(root)
    candidates: List[str] = list(SCAN_FILES)
    candidates.append(GITHUB_EXAMPLE)
    candidates.append(GITLAB_EXAMPLE)
    candidates.append(RUNTIME_CONFIG)
    recipe_dir = rootp / "docs" / "recipe"
    if recipe_dir.is_dir():
        for item in sorted(recipe_dir.iterdir()):
            if item.is_file() and item.name.endswith(".md"):
                candidates.append("docs/recipe/" + item.name)

    files: List[File] = []
    for rel in candidates:
        if is_ignored_path(rel):
            continue
        text = _read(rootp, rel)
        if text is None:
            continue
        files.append(File(rel_path=rel.replace("\\", "/"), text=text))
    return files


# --------------------------------------------------------------------------- #
# Checks
# --------------------------------------------------------------------------- #

def check_stale_commands(files: List[File]) -> List[Failure]:
    failures: List[Failure] = []
    for f in files:
        for pattern, hint in STALE_COMMANDS:
            for match in pattern.finditer(f.text):
                failures.append(
                    Failure(
                        path=f.rel_path,
                        line=line_number(f.text, match.start()),
                        message="stale command `%s`; %s" % (
                            match.group(0).strip(), hint),
                    )
                )
    return failures


def check_format_inventory(files: List[File]) -> List[Failure]:
    by_path = {f.rel_path: f for f in files}
    failures: List[Failure] = []
    for rel in INVENTORY_FILES:
        f = by_path.get(rel)
        if f is None:
            continue
        for fmt in CURRENT_FORMATS:
            if fmt not in f.text:
                failures.append(
                    Failure(
                        path=rel,
                        line=0,
                        message="audit output-format inventory missing `%s`; "
                                "expected all of %s" % (fmt, ", ".join(CURRENT_FORMATS)),
                    )
                )
    return failures


def check_severity_language(files: List[File]) -> List[Failure]:
    failures: List[Failure] = []
    for f in files:
        for match in SEVERITY_TRIGGER.finditer(f.text):
            start = match.start()
            line_start = f.text.rfind("\n", 0, start) + 1
            line_end = f.text.find("\n", start)
            if line_end == -1:
                line_end = len(f.text)
            line_text = f.text[line_start:line_end]
            if any(allow.search(line_text) for allow in SEVERITY_ALLOW):
                continue
            failures.append(
                Failure(
                    path=f.rel_path,
                    line=line_number(f.text, start),
                    message="avoid affirmative `severity field` language for "
                            "DeltaScope public docs (DeltaScope uses `level`, not "
                            "`severity`); external-schema and negative "
                            "clarifications are allowed",
                )
            )
    return failures


def extract_version_pin(text: str) -> Optional[str]:
    """Return the ``DELTASCOPE_VERSION`` value (e.g. ``v0.330.0``) or None."""
    match = VERSION_PIN_RE.search(text)
    if match is None:
        return None
    return match.group(1)


def check_github_actions_example(
    text: str, expected_version: Optional[str]
) -> List[Failure]:
    failures: List[Failure] = []
    for token in GITHUB_REQUIRED:
        idx = text.find(token)
        if idx == -1:
            failures.append(
                Failure(
                    path=GITHUB_EXAMPLE,
                    line=0,
                    message="GitHub Actions example missing required `%s`" % token,
                )
            )
    for token in GITHUB_FORBIDDEN:
        idx = text.find(token)
        if idx != -1:
            failures.append(
                Failure(
                    path=GITHUB_EXAMPLE,
                    line=line_number(text, idx),
                    message="GitHub Actions example must not include `%s`" % token,
                )
            )
    if expected_version is not None:
        pin = extract_version_pin(text)
        if pin is None:
            failures.append(
                Failure(
                    path=GITHUB_EXAMPLE,
                    line=0,
                    message="VERSION is set to %s but no DELTASCOPE_VERSION pin "
                            "found" % expected_version,
                )
            )
        elif pin != expected_version:
            failures.append(
                Failure(
                    path=GITHUB_EXAMPLE,
                    line=line_number(text, VERSION_PIN_RE.search(text).start()),
                    message="DELTASCOPE_VERSION pin %s does not match VERSION %s"
                            % (pin, expected_version),
                )
            )
    return failures


def check_gitlab_example(text: str) -> List[Failure]:
    failures: List[Failure] = []
    for token in GITLAB_REQUIRED:
        if token not in text:
            failures.append(
                Failure(
                    path=GITLAB_EXAMPLE,
                    line=0,
                    message="GitLab CI example missing required `%s` output format"
                            % token,
                )
            )
    for token in GITLAB_FORBIDDEN:
        idx = text.find(token)
        if idx != -1:
            failures.append(
                Failure(
                    path=GITLAB_EXAMPLE,
                    line=line_number(text, idx),
                    message="GitLab CI example must not include `%s`" % token,
                )
            )
    return failures


def run_checks(root: str, expected_version: Optional[str]) -> List[Failure]:
    """Run every curated check and return all failures (no early exit)."""
    files = collect_files(root)
    failures: List[Failure] = []
    failures.extend(check_stale_commands(files))
    failures.extend(check_format_inventory(files))
    failures.extend(check_severity_language(files))

    github_text = _read(Path(root), GITHUB_EXAMPLE)
    if github_text is not None:
        failures.extend(check_github_actions_example(github_text, expected_version))

    gitlab_text = _read(Path(root), GITLAB_EXAMPLE)
    if gitlab_text is not None:
        failures.extend(check_gitlab_example(gitlab_text))

    return failures


# --------------------------------------------------------------------------- #
# Entry point
# --------------------------------------------------------------------------- #

def _default_root() -> Path:
    """Repository root derived from this script's location (scripts/../)."""
    return Path(__file__).resolve().parent.parent


def main(argv: Optional[List[str]] = None) -> int:
    argv = sys.argv[1:] if argv is None else argv
    root = argv[0] if argv else str(_default_root())
    expected_version = os.environ.get("VERSION") or None

    failures = run_checks(root, expected_version)
    if not failures:
        print("docs-examples: PASS")
        return 0

    print("docs-examples: FAIL")
    for failure in failures:
        print("%s:%d: %s" % (failure.path, failure.line, failure.message))
    return 1


if __name__ == "__main__":
    sys.exit(main())
