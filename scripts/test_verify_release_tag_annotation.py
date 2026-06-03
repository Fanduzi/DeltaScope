#!/usr/bin/env python3
"""Tests for verify_release_tag_annotation.py.

Creates a temporary git repo to test tag annotation checks in isolation.
Does not depend on the real repository's tags.
"""

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import verify_release_tag_annotation as vrt  # noqa: E402


class _TempRepo:
    """Context manager for a temporary git repository."""

    def __init__(self) -> None:
        self.tmpdir: str = ""
        self._old_cwd: str = ""

    def __enter__(self) -> "_TempRepo":
        self.tmpdir = tempfile.mkdtemp(prefix="tag-annotation-test-")
        self._old_cwd = os.getcwd()
        os.chdir(self.tmpdir)
        subprocess.run(["git", "init", "-q"], check=True)
        subprocess.run(
            ["git", "commit", "--allow-empty", "-m", "init", "-q"],
            check=True,
        )
        return self

    def __exit__(self, *args: object) -> None:
        os.chdir(self._old_cwd)
        subprocess.run(["rm", "-rf", self.tmpdir], check=False)

    def create_annotated_tag(self, name: str) -> None:
        subprocess.run(
            ["git", "tag", "-a", name, "-m", f"Release {name}"],
            check=True,
        )

    def create_lightweight_tag(self, name: str) -> None:
        subprocess.run(["git", "tag", name], check=True)


class TestValidateVersion(unittest.TestCase):
    def test_valid(self) -> None:
        self.assertEqual(vrt.validate_version("v0.251.0"), "v0.251.0")

    def test_multi_digit(self) -> None:
        self.assertEqual(vrt.validate_version("v12.345.678"), "v12.345.678")

    def test_no_v_prefix(self) -> None:
        with self.assertRaises(SystemExit):
            vrt.validate_version("0.251.0")

    def test_empty(self) -> None:
        with self.assertRaises(SystemExit):
            vrt.validate_version("")

    def test_random(self) -> None:
        with self.assertRaises(SystemExit):
            vrt.validate_version("hello")


class TestAnnotatedTagPasses(unittest.TestCase):
    def test_annotated_tag_passes(self) -> None:
        with _TempRepo() as repo:
            repo.create_annotated_tag("v1.0.0")
            # Should not exit
            vrt.check_tag_annotation("v1.0.0")


class TestLightweightTagFails(unittest.TestCase):
    def test_lightweight_tag_fails(self) -> None:
        with _TempRepo() as repo:
            repo.create_lightweight_tag("v1.0.0")
            with self.assertRaises(SystemExit):
                vrt.check_tag_annotation("v1.0.0")


class TestMissingTagFails(unittest.TestCase):
    def test_missing_tag_fails(self) -> None:
        with _TempRepo():
            with self.assertRaises(SystemExit):
                vrt.check_tag_annotation("v9.9.9")


class TestPeeledCommit(unittest.TestCase):
    def test_peeled_commit_reported(self) -> None:
        with _TempRepo() as repo:
            repo.create_annotated_tag("v2.0.0")
            stdout, _ = vrt._git("rev-parse", "v2.0.0^{commit}")
            script = str(Path(__file__).resolve().parent / "verify_release_tag_annotation.py")
            result = subprocess.run(
                [sys.executable, script],
                env={**os.environ, "VERSION": "v2.0.0"},
                capture_output=True,
                text=True,
                cwd=repo.tmpdir,
            )
            self.assertEqual(result.returncode, 0)
            self.assertIn("peeled=", result.stdout)
            self.assertIn(stdout, result.stdout)
            self.assertIn("PASS", result.stdout)


class TestInvalidVersionFails(unittest.TestCase):
    def test_invalid_version_exits(self) -> None:
        with _TempRepo():
            result = subprocess.run(
                [sys.executable, "-m", "verify_release_tag_annotation"],
                env={**os.environ, "VERSION": "not-a-version"},
                capture_output=True,
                text=True,
            )
            self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
