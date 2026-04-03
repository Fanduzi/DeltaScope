# Security Policy

DeltaScope is distributed under the Apache License 2.0. See [LICENSE](LICENSE).

## Supported Versions

Security fixes are currently planned for the latest released tag and the active development branch.

```text
Version     Supported
v0.14.x     yes
main        yes
older tags  no
```

## Reporting A Vulnerability

Please do not open a public GitHub issue for a suspected vulnerability.

Report security issues by contacting the maintainer privately through one of these paths:

- GitHub Security Advisories, if enabled for the repository
- the maintainer contact listed on the repository profile

When reporting, include:

- affected DeltaScope version or commit
- deployment shape: library, CLI, HTTP service, or MCP server
- reproduction steps or proof of concept
- impact assessment
- whether credentials, SQL text, or metadata access are required

## Response Expectations

- Initial triage target: within 7 days
- If reproduced, a fix or mitigation plan will be communicated before public disclosure
- Please avoid publishing exploit details until a patched release is available or the issue has been assessed as non-exploitable
