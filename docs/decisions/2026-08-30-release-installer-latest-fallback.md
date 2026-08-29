# Release Installer Latest-Version Fallback

Date: 2026-08-30
Status: Accepted
Related milestone/version: Issue #40
Related commits: 345618a5a982eaad89c5ccd846e9c261a2e2bc30
Related tests: `scripts/test_install.sh`
Related docs: `README.md`, `README_ZH.md`, `install.sh`

## Context

The unpinned POSIX installer used only the unauthenticated GitHub REST
`releases/latest` endpoint to discover a tag. API rate limiting or another
metadata failure therefore prevented the installer from reaching a release
archive even when GitHub's public release redirect was available.

## Decision

Keep `DELTASCOPE_VERSION` as the first, unchanged path. For an unpinned
install, accept only `vMAJOR.MINOR.PATCH` tags from the REST response. If that
lookup fails or has no valid tag, query the public
`https://github.com/<repo>/releases/latest` redirect using the selected curl or
wget client, and accept only a redirect to the configured repository's release
tag. If both lookups fail, return a bounded error that tells the operator to
set `DELTASCOPE_VERSION=vX.Y.Z`.

## Rationale

The redirect is a separate public GitHub path and does not depend on the
unauthenticated REST API rate-limit bucket. Strict repository and tag
validation prevents metadata or redirect content from becoming an archive
download path. Suppressing raw response output keeps failures bounded and
avoids exposing untrusted bodies in installer diagnostics.

## Public Contract

- Unpinned installs try REST discovery, then the public latest-release redirect.
- curl and wget use equivalent fallback behavior.
- If wget cannot perform the redirect inspection, the bounded failure names the
  supported curl alternative or the version pin bypass.
- Only configured-repository `vMAJOR.MINOR.PATCH` release tags are accepted.
- Failed discovery reports the `DELTASCOPE_VERSION=vX.Y.Z` bypass hint without
  dumping response bodies.
- Pinned installs skip discovery and retain the existing archive/download path.

## Deferred / Out Of Scope

- Parsing the GitHub releases HTML page.
- Adding authenticated API credentials or changing API rate-limit policy.
- Changing Homebrew, npm, Windows, release asset naming, or archive integrity
  behavior.
- Permanently pinning the README's unpinned install command.

## Verification Evidence

`bash scripts/test_install.sh` runs 26 hermetic assertions across curl and wget:
API success, API failure and invalid API fallback, invalid/empty/non-release
redirect rejection, bounded total failure, and pinned discovery bypass. The
tests use only local fake clients and a local archive fixture.

## Consequences

The installer uses the existing `sed`, `head`, and `tail` utilities while
using wget's server-response headers to observe the final redirect. Future
changes to release URL shape, client capabilities, or tag policy must update
the installer contract test and this decision.

## Links

- Commits: 345618a5a982eaad89c5ccd846e9c261a2e2bc30
- Tests: `scripts/test_install.sh`
- Docs: `README.md`, `README_ZH.md`, `scripts/README.md`
