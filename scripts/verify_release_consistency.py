#!/usr/bin/env python3
"""Release surface consistency checker.

Validates that release-domain facts are consistent across all release surfaces:
landing page, release notes EN/ZH, changelog, roadmap, rules reference,
capability matrix.

Full implementation deferred to Task 3. This stub provides the public contract
so tests can express expectations.
"""


class ReleaseConsistencyError(Exception):
    """Raised when a release surface consistency check fails."""


def validate_all(root, version):
    raise NotImplementedError("release consistency checker not implemented")
