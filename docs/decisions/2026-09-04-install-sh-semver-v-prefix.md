# Decision: Prefix Bare Semver in the POSIX Installer Download URL

Date: 2026-09-04
Status: Accepted
Related milestone/version: Issue #66
Related commits:
Related tests: `scripts/test_install.sh`
Related docs: `install.sh`, `scripts/README.md`

## Context

`install.sh` interpolates `DELTASCOPE_VERSION` directly into the GitHub
release download path. GitHub release tags are `vMAJOR.MINOR.PATCH`, so
`DELTASCOPE_VERSION=0.510.3` requested
`.../releases/download/0.510.3/...` and 404ed (curl exit 22).
`DELTASCOPE_VERSION=v0.510.3` already succeeded. Operators and CI pins
commonly omit the leading `v`.

## Decision

After a version is resolved, if it matches exact `MAJOR.MINOR.PATCH`
(`^[0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*$`), prefix `v` before composing
the GitHub release download URL. Values that already start with `v`, and
any other version string, are left unchanged.

## Rationale

GitHub tags stay `vMAJOR.MINOR.PATCH`; the installer absorbs the missing
`v` rather than renaming tags or adding a second download path. Exact
`MAJOR.MINOR.PATCH` matching avoids rewriting pre-release, two-part, or
symbolic pins such as `latest`. Prefixing the resolved version once keeps
the displayed version, archive name (`VERSION#v`), and download tag on
the same release identity.

## Public Contract

- `DELTASCOPE_VERSION=0.510.3` downloads `.../releases/download/v0.510.3/...`.
- `DELTASCOPE_VERSION=v0.510.3` still downloads `.../releases/download/v0.510.3/...`.
- Non-semver values (for example `latest`) are not rewritten.
- Unpinned discovery is unchanged and still yields `vMAJOR.MINOR.PATCH` tags.
- Archive filenames remain `deltascope_<major.minor.patch>_<os>_<arch>.tar.gz`.

## Deferred / Out Of Scope

- Changing GitHub tag format.
- npm or Homebrew install paths.
- Rewriting pre-release or other non-exact-semver version strings.
- Changing release asset naming or archive integrity behavior.

## Verification Evidence

`bash scripts/test_install.sh` covers curl and wget for API discovery,
redirect fallback, bounded failure, prefixed pins, unprefixed
`0.510.3` → `v0.510.3` download URLs, and a non-semver `latest` pin that
is not rewritten. Tests use only local fake clients and a local archive
fixture.

## Consequences

Future installer version selectors must keep exact `MAJOR.MINOR.PATCH`
prefixing and must not rewrite other strings. Changes to this contract
must update `scripts/test_install.sh` and this decision.

## Links

- Tests: `scripts/test_install.sh`
- Docs: `install.sh`, `scripts/README.md`
- Related: `docs/decisions/2026-08-30-release-installer-latest-fallback.md`
