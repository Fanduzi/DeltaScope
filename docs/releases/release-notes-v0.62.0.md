# DeltaScope v0.62.0 Release Notes

## Summary

v0.62.0 adds structured logging for server and MCP services, log file rotation, metadata connect timeout configuration, and parser benchmark coverage. It improves code maintainability by splitting large rule and extractor files into focused modules, fixes context propagation in boundary errors and impact estimation, and restricts log file permissions to owner-only. No new audit rules, parser features, or public API changes.

## New Features

| Feature | Description |
|---------|-------------|
| Structured logging | Server and MCP now accept `-log-output`, `-log-level`, and `-log-file` flags for structured log output |
| Log file rotation | Configurable rotation with `--log-rotate`, `--log-max-size`, `--log-max-age`, `--log-max-backups`, and `--log-compress` |
| Metadata connect timeout | `--metadata-connect-timeout` CLI flag and `MetadataConnectTimeout` field on library `Request` |
| Parser benchmarks | Hot-path benchmark coverage for rule evaluation and rendering |

## Reliability

- Log file and directory permissions restricted to owner-only (`0750` for directories, `0600` for files)
- Context propagation improved in boundary error wrapping and impact estimation

## Codebase

- `defaults.go` split into 5 focused files by rule category
- `extractor.go` split into 7 focused files by statement type

## Non-Goals

- No new rule IDs, parser features, or public API changes.
- No MySQL/TiDB/PostgreSQL audit behavior changes.
- No release asset naming or install workflow changes.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.62.0/install.sh | \
  DELTASCOPE_VERSION=v0.62.0 sh
```
