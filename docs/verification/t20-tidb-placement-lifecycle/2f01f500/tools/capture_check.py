#!/usr/bin/env python3
"""Capture a lead-run verification command without piping away its exit code."""

import argparse
import datetime
import json
from pathlib import Path
import shlex
import subprocess
import sys


ROOT = Path("/Users/fan/GolangProjects/DeltaScope")


def head():
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--name", required=True)
    parser.add_argument("--dir", required=True)
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command
    if command and command[0] == "--":
        command = command[1:]
    if not command:
        raise ValueError("verification command required")
    directory = Path(args.dir)
    if not directory.is_dir():
        raise ValueError("receipt directory must already exist")
    prefix = directory / args.name
    if prefix.with_suffix(".rc.txt").exists():
        raise ValueError("existing receipt must not be overwritten")
    prefix.with_suffix(".command.txt").write_text(shlex.join(command) + "\n")
    before = head()
    prefix.with_suffix(".head-before.txt").write_text(before + "\n")
    started = datetime.datetime.now(datetime.timezone.utc).isoformat()
    result = subprocess.run(command, cwd=ROOT, text=True, capture_output=True, check=False)
    prefix.with_suffix(".stdout.txt").write_text(result.stdout)
    prefix.with_suffix(".stderr.txt").write_text(result.stderr)
    prefix.with_suffix(".rc.txt").write_text(str(result.returncode) + "\n")
    prefix.with_suffix(".head-after.txt").write_text(head() + "\n")
    prefix.with_suffix(".execution.json").write_text(json.dumps({
        "command": command, "head_before": before, "head_after": head(),
        "started_at": started,
        "ended_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "returncode": result.returncode,
    }, indent=2) + "\n")
    sys.stdout.write(result.stdout)
    sys.stderr.write(result.stderr)
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
