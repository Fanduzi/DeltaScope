#!/usr/bin/env python3
"""Release assets preflight helper.

Verifies that a GitHub Release for VERSION exists, is not draft/prerelease,
has all required assets, and that checksum files reference the correct archives.

Read-only — never uploads, deletes, or publishes anything.
"""

import json
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path


REQUIRED_PLATFORMS = [
    "darwin_amd64",
    "darwin_arm64",
    "linux_amd64",
    "linux_arm64",
]

DEFAULT_REPO = "Fanduzi/DeltaScope"


def strip_v(version: str) -> str:
    return version.lstrip("v")


def validate_version(version: str) -> str:
    if not re.match(r"^v\d+\.\d+\.\d+$", version):
        print(
            "release-assets: FAIL — VERSION must look like vX.Y.Z, "
            f"got {version!r}"
        )
        sys.exit(1)
    return version


def required_asset_names(version_no_v: str) -> list[str]:
    names: list[str] = [f"deltascope_{version_no_v}_checksums.txt"]
    for plat in REQUIRED_PLATFORMS:
        names.append(f"deltascope_{version_no_v}_{plat}.tar.gz")
        names.append(f"deltascope_{version_no_v}_{plat}_checksums.txt")
    return names


def _gh(*args: str) -> str:
    result = subprocess.run(
        ["gh", *args],
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(
            f"gh {' '.join(args)} failed (exit {result.returncode}): "
            f"{result.stderr.strip()}"
        )
    return result.stdout


def fetch_release(version: str, repo: str) -> dict:
    stdout = _gh(
        "release", "view", version,
        "--repo", repo,
        "--json", "tagName,isDraft,isPrerelease,assets,url",
    )
    return json.loads(stdout)


def check_release_state(release: dict, version: str) -> list[str]:
    errors: list[str] = []
    if release.get("tagName") != version:
        errors.append(
            f"tagName mismatch: expected {version}, "
            f"got {release.get('tagName')}"
        )
    if release.get("isDraft"):
        errors.append("release is draft")
    if release.get("isPrerelease"):
        errors.append("release is prerelease")
    return errors


def check_assets(
    release: dict, version_no_v: str
) -> tuple[list[str], set[str], set[str]]:
    required = set(required_asset_names(version_no_v))
    actual = {a["name"] for a in release.get("assets", [])}
    missing = required - actual
    extra = actual - required
    return sorted(missing), actual, extra


def download_checksum(
    version: str, repo: str, pattern: str, tmpdir: str
) -> str:
    _gh(
        "release", "download", version,
        "--repo", repo,
        "--pattern", pattern,
        "--dir", tmpdir,
        "--clobber",
    )
    path = Path(tmpdir) / pattern
    return path.read_text(encoding="utf-8")


def check_checksums(
    version: str, version_no_v: str, repo: str, actual_assets: set[str]
) -> list[str]:
    errors: list[str] = []

    with tempfile.TemporaryDirectory(prefix="deltascope-preflight-") as tmpdir:
        # Check generic checksums file references at least the linux_amd64 archive
        generic_name = f"deltascope_{version_no_v}_checksums.txt"
        if generic_name in actual_assets:
            content = download_checksum(version, repo, generic_name, tmpdir)
            linux_archive = f"deltascope_{version_no_v}_linux_amd64.tar.gz"
            if linux_archive not in content:
                errors.append(
                    f"{generic_name} does not reference {linux_archive}"
                )
        else:
            errors.append(f"{generic_name} missing from release assets")

        # Check per-platform checksums reference their archive
        for plat in REQUIRED_PLATFORMS:
            plat_checksum = f"deltascope_{version_no_v}_{plat}_checksums.txt"
            plat_archive = f"deltascope_{version_no_v}_{plat}.tar.gz"

            if plat_checksum not in actual_assets:
                errors.append(f"{plat_checksum} missing from release assets")
                continue

            content = download_checksum(version, repo, plat_checksum, tmpdir)
            if plat_archive not in content:
                errors.append(
                    f"{plat_checksum} does not reference {plat_archive}"
                )

    return errors


def main() -> None:
    version = os.environ.get("VERSION", "").strip()
    if not version:
        for i, arg in enumerate(sys.argv[1:], 1):
            if arg.startswith("--version="):
                version = arg.split("=", 1)[1]
            elif arg == "--version" and i < len(sys.argv) - 1:
                version = sys.argv[i + 1]

    validate_version(version)
    version_no_v = strip_v(version)
    repo = os.environ.get("REPO", DEFAULT_REPO)

    print(f"release-assets: VERSION={version}")

    # Fetch release metadata
    try:
        release = fetch_release(version, repo)
    except RuntimeError as e:
        print(f"release-assets: FAIL — {e}")
        sys.exit(1)

    # Check release state
    state_errors = check_release_state(release, version)
    if state_errors:
        for err in state_errors:
            print(f"release-assets: FAIL — {err}")
        sys.exit(1)

    # Check assets
    missing, actual, extra = check_assets(release, version_no_v)
    total_required = len(required_asset_names(version_no_v))
    found_count = total_required - len(missing)

    if missing:
        for name in missing:
            print(f"release-assets: FAIL — missing asset {name}")
    if extra:
        for name in sorted(extra):
            print(f"release-assets: FAIL — unexpected asset {name}")

    print(f"release-assets: assets {found_count}/{total_required} "
          f"{'OK' if not missing and not extra else 'INCOMPLETE'}")

    if missing or extra:
        sys.exit(1)

    # Check checksums
    checksum_errors = check_checksums(version, version_no_v, repo, actual)
    if checksum_errors:
        for err in checksum_errors:
            print(f"release-assets: FAIL — {err}")
        sys.exit(1)

    print("release-assets: checksums OK")
    print("release-assets: PASS")


if __name__ == "__main__":
    main()
