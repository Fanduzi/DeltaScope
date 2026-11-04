#!/usr/bin/env python3
"""Capture actual Git ranges; counts never replace the commit/path lists."""

import argparse
import json
from pathlib import Path
import subprocess

ROOT = Path("/Users/fan/GolangProjects/DeltaScope")
BASE = "6ac1e918b3558d6d611d7d8a59a770d73341c2f9"
T19 = "57c207a8486497b49c17dd9f0b8fd0b16dc103a7"
CONTRACT_BASE = "1e630493b47676ba076010be59c28898c16626d8"
MAIN = "9a28cc99c38770e6e40738515994f864166419f9"


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, text=True).strip()


def scope(base, head):
    subprocess.run(["git", "merge-base", "--is-ancestor", base, head], cwd=ROOT, check=True)
    commits = [dict(zip(("sha", "parents", "subject"), line.split("\t", 2)))
               for line in git("log", "--reverse", "--format=%H%x09%P%x09%s", f"{base}..{head}").splitlines()]
    paths = git("diff", "--name-only", f"{base}...{head}").splitlines()
    return {
        "base_sha": base, "head_sha": head,
        "log_command": f"git log --reverse --format='%H %P %s' {base}..{head}",
        "diff_command": f"git diff --name-only {base}...{head}",
        "commits": commits, "commit_count": len(commits),
        "changed_paths": paths,
        "non_verification_paths": [p for p in paths if not p.startswith("docs/verification/")],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--product", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    product = git("rev-parse", args.product + "^{commit}")
    head = git("rev-parse", "HEAD")
    result = {
        "branch": git("branch", "--show-current"),
        "head_sha": head, "product_sha": product, "main_sha": git("rev-parse", "main"),
        "tracked_status": git("status", "--porcelain=v1", "--untracked-files=no").splitlines(),
        "worktree_status": git("status", "--porcelain=v1", "--untracked-files=normal").splitlines(),
        "pre_repair_contract_range": scope(CONTRACT_BASE, BASE),
        "task_product_range": scope(BASE, product),
        "task_delivery_range": scope(BASE, head),
        "t20_cumulative_range_to_product": scope(T19, product),
        "contract_cumulative_range": scope(CONTRACT_BASE, head),
        "evidence_range": scope(product, head),
        "milestone_range": scope(MAIN, head),
    }
    if result["main_sha"] != MAIN or result["pre_repair_contract_range"]["commit_count"] != 10:
        raise ValueError("fixed baseline identity/range drift")
    if result["tracked_status"]:
        raise ValueError("capture requires a fixed tracked/index tree")
    Path(args.output).write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps({k: v["commit_count"] for k, v in result.items()
                      if isinstance(v, dict) and "commit_count" in v}, indent=2))


if __name__ == "__main__":
    main()
