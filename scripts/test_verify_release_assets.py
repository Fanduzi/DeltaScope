#!/usr/bin/env python3
"""Offline unit tests for verify_release_assets.py pure functions.

No network access — only tests logic, not gh CLI integration.
"""

import sys
import unittest
from pathlib import Path

# Import from the sibling module
sys.path.insert(0, str(Path(__file__).resolve().parent))
import verify_release_assets as vra  # noqa: E402


class TestStripV(unittest.TestCase):
    def test_strips_v_prefix(self) -> None:
        self.assertEqual(vra.strip_v("v0.230.0"), "0.230.0")

    def test_no_v_prefix(self) -> None:
        self.assertEqual(vra.strip_v("0.230.0"), "0.230.0")

    def test_double_v(self) -> None:
        self.assertEqual(vra.strip_v("vv0.230.0"), "0.230.0")


class TestValidateVersion(unittest.TestCase):
    def test_valid_version(self) -> None:
        self.assertEqual(vra.validate_version("v0.230.0"), "v0.230.0")

    def test_valid_version_multi_digit(self) -> None:
        self.assertEqual(vra.validate_version("v12.345.678"), "v12.345.678")

    def test_missing_v_exits(self) -> None:
        with self.assertRaises(SystemExit):
            vra.validate_version("0.230.0")

    def test_empty_exits(self) -> None:
        with self.assertRaises(SystemExit):
            vra.validate_version("")

    def test_random_string_exits(self) -> None:
        with self.assertRaises(SystemExit):
            vra.validate_version("hello")

    def test_partial_version_exits(self) -> None:
        with self.assertRaises(SystemExit):
            vra.validate_version("v0.230")


class TestRequiredAssetNames(unittest.TestCase):
    def test_count(self) -> None:
        names = vra.required_asset_names("0.230.0")
        self.assertEqual(len(names), 9)

    def test_generic_checksums(self) -> None:
        names = vra.required_asset_names("0.230.0")
        self.assertIn("deltascope_0.230.0_checksums.txt", names)

    def test_platform_archives(self) -> None:
        names = vra.required_asset_names("0.230.0")
        for plat in vra.REQUIRED_PLATFORMS:
            self.assertIn(f"deltascope_0.230.0_{plat}.tar.gz", names)

    def test_platform_checksums(self) -> None:
        names = vra.required_asset_names("0.230.0")
        for plat in vra.REQUIRED_PLATFORMS:
            self.assertIn(
                f"deltascope_0.230.0_{plat}_checksums.txt", names
            )

    def test_different_version(self) -> None:
        names = vra.required_asset_names("1.2.3")
        self.assertIn("deltascope_1.2.3_checksums.txt", names)
        self.assertIn("deltascope_1.2.3_linux_amd64.tar.gz", names)


class TestCheckReleaseState(unittest.TestCase):
    def _make_release(
        self,
        tag: str = "v0.230.0",
        draft: bool = False,
        prerelease: bool = False,
    ) -> dict:
        return {
            "tagName": tag,
            "isDraft": draft,
            "isPrerelease": prerelease,
            "assets": [],
            "url": "https://example.com",
        }

    def test_valid_release(self) -> None:
        errors = vra.check_release_state(
            self._make_release(), "v0.230.0"
        )
        self.assertEqual(errors, [])

    def test_tag_mismatch(self) -> None:
        errors = vra.check_release_state(
            self._make_release(tag="v0.229.0"), "v0.230.0"
        )
        self.assertTrue(any("tagName mismatch" in e for e in errors))

    def test_draft(self) -> None:
        errors = vra.check_release_state(
            self._make_release(draft=True), "v0.230.0"
        )
        self.assertTrue(any("draft" in e for e in errors))

    def test_prerelease(self) -> None:
        errors = vra.check_release_state(
            self._make_release(prerelease=True), "v0.230.0"
        )
        self.assertTrue(any("prerelease" in e for e in errors))


class TestCheckAssets(unittest.TestCase):
    def _make_assets(self, version_no_v: str) -> list[dict]:
        names = vra.required_asset_names(version_no_v)
        return [{"name": n} for n in names]

    def test_all_present(self) -> None:
        release = {"assets": self._make_assets("0.230.0")}
        missing, actual, extra = vra.check_assets(release, "0.230.0")
        self.assertEqual(missing, [])
        self.assertEqual(extra, set())

    def test_missing_one(self) -> None:
        assets = self._make_assets("0.230.0")
        assets = [a for a in assets if "darwin_amd64.tar.gz" not in a["name"]]
        release = {"assets": assets}
        missing, _, _ = vra.check_assets(release, "0.230.0")
        self.assertEqual(len(missing), 1)
        self.assertIn("deltascope_0.230.0_darwin_amd64.tar.gz", missing)

    def test_extra_asset(self) -> None:
        assets = self._make_assets("0.230.0")
        assets.append({"name": "extra_file.txt"})
        release = {"assets": assets}
        _, _, extra = vra.check_assets(release, "0.230.0")
        self.assertIn("extra_file.txt", extra)

    def test_no_assets(self) -> None:
        release = {"assets": []}
        missing, _, _ = vra.check_assets(release, "0.230.0")
        self.assertEqual(len(missing), 9)


class TestCheckChecksumsOffline(unittest.TestCase):
    """Test checksum validation logic using mock data.

    The check_checksums function calls gh to download, so we only
    test the structure expectations here. Full integration tested
    via VERSION=v0.230.0 make release-recovery-preflight.
    """

    def test_platform_count(self) -> None:
        self.assertEqual(len(vra.REQUIRED_PLATFORMS), 4)

    def test_platform_names(self) -> None:
        expected = {
            "darwin_amd64", "darwin_arm64",
            "linux_amd64", "linux_arm64",
        }
        self.assertEqual(set(vra.REQUIRED_PLATFORMS), expected)


if __name__ == "__main__":
    unittest.main()
