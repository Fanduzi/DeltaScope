# DeltaScope v0.510.0 Release Notes

Status: Official GitHub Actions run 33302045413 passed provenance then failed in `TestAuditCommandLoadsTLSCAFile` before any assets. This is not a successful published GitHub Release. v0.510.1 republishes this source work plus the Linux test-contract correction; see `docs/releases/release-notes-v0.510.1.md`.

## Summary - Release Channel Parity, Compact CLI JSON, Parser Review Floor, and Connection Refusal

The v0.510.0 candidate contains the #54–#57 source work that landed after v0.500.0. The tag-triggered workflow would publish `@fanduzi/deltascope-mcp` even if Homebrew cask publish or install verification fails, provided provenance and platform builds succeed; the recorded run stopped earlier in release tests. Default CLI JSON compact-skips dialect-heavy skip lists into `{reason, count}` aggregates; `--include-skipped-rules` restores the per-rule list. Mixed migrations that would have verdict `pass` despite an unaudited `parser_error` diagnostic are floored to `review` across SDK, CLI, HTTP, and MCP. Metadata-aware CLI TCP refusal is a bounded `connection refused` exit 3.

This is still static analysis. DeltaScope does not execute submitted SQL, retrieve query results, or decide authorization, grants, RLS, or masking. MCP still has no Query Access tool. The registered audit rule catalog is unchanged at 373 rules. Supported rule-and-dialect fixture coverage remains 586/586, 100.0%, across 286 YAML files; that is not SQL syntax or grammar coverage. Recovered `@fanduzi/deltascope-mcp@0.500.0` is a separate historical release and remains unchanged by v0.510.0.

## What Changed

### Release Channel Parity (#54)

- `publish-mcp-launcher-package` waits only on the four platform-build jobs and, transitively, on provenance.
- Homebrew publish and Homebrew install verification continue to run; they no longer gate npm.
- `make release-workflow-hygiene-gates` rejects a workflow graph that recouples npm to Homebrew or drops the platform-build / provenance prerequisites.

### Compact CLI JSON Skipped Rules (#55)

- Default CLI JSON emits `rule_summary.skipped` as a deterministic reason-sorted array of `{reason, count}` objects, or `[]` when no rule was skipped, and omits `rule_summary.skipped_rules`.
- Audit-only `--include-skipped-rules` adds `rule_summary.skipped_rules` with objects shaped `{rule_id, reason}` while retaining the aggregate `skipped` field.
- `--quiet --format json` remains byte-for-byte equal to ordinary JSON for the same other flags.
- SDK, HTTP, MCP, Markdown, GitHub Actions, SARIF, and GitLab Code Quality output are unchanged.

### Parser Review Floor (#56)

- At the shared application result seam, a partial parse with any `audited=false` `parser_error` diagnostic applies a `review` floor only when finding aggregation computed `pass`.
- Existing `review` and `reject` verdicts never downgrade. Wholly unparseable inputs retain their existing #24/#43 behavior.
- SDK, CLI, HTTP, and MCP serialize the same floored verdict. The application and SDK still return a non-nil parser error; CLI still exits 2; HTTP and MCP still signal an error.

### CLI Connection Refusal (#57)

- Metadata-aware CLI classifies typed `syscall.ECONNREFUSED` as `connection refused` with exit 3.
- Other non-TLS dial failures remain `connection failed` with exit 3. Authentication, timeout, TLS, password-source, and PostgreSQL port mappings are unchanged.
- Portable CLI output never includes host, port, user, database, schema, DSN, password, raw driver text, filesystem path, or version.

## What Stayed the Same

- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.
- Query Access contracts from v0.500.0 are unchanged: empty `--sql`, audit-only flags, MySQL/TiDB schema binding, PG17 version boundary, and fail-closed default/offline paths.
- `level` remains the public audit priority field; no severity field is introduced.
- The registered audit rule catalog, SQL corpus fixture coverage, and DDL coverage catalog counts are unchanged from v0.500.0.
- Existing release tags, GitHub Releases, npm packages, and Homebrew casks are untouched until this tag publishes.

## Non-Goals

- Not a recovery publish of `@fanduzi/deltascope-mcp@0.500.0`.
- Not an MCP Query Access tool.
- Not authorization, grants, roles, RLS, masking, rewrite, SQL execution, or data-returning APIs.
- Not a new verdict enum, fallback grammar, or semantic guessing for parser-failed statements.
- Not a non-TLS protocol/handshake category, and not leaking connection internals.
- Not changing SDK, HTTP, or MCP skipped-rule JSON shape.
- Not a registered-rule catalog change and not SQL syntax or grammar coverage.
- Not a severity field; not a change to any previously published artifact or existing tag.

## Rule Catalog Facts

The registered audit rule catalog is unchanged relative to v0.500.0.

| Metric | Count |
|--------|------:|
| Total rules | **373** |
| blocker | 73 |
| warning | 142 |
| notice | 158 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |
| mysql and tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 362 |
| dml | 11 |

## Corpus and Catalog Facts

- Supported rule-and-dialect fixture coverage: **586/586**, **100.0%**, **286** YAML fixture files. This is not SQL syntax or grammar coverage.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **407** entries (mysql 62, tidb 55, postgresql 290, parser_upgrade_candidate 18).

## Decision Records

- `docs/decisions/2026-08-30-release-channel-npm-homebrew-parity.md` (this release; #54)
- `docs/decisions/2026-08-30-cli-json-skipped-rule-compaction.md` (this release; #55)
- `docs/decisions/2026-08-30-partial-parser-error-verdict-review-floor.md` (this release; #56)
- `docs/decisions/2026-08-30-cli-connection-error-categories.md` (this release; #57)
- `docs/decisions/2026-08-30-partial-parser-error-recovery.md` (v0.500.0; #43)
- `docs/decisions/2026-08-17-cli-metadata-connection-exit-mapping.md` (v0.490.0; #23)
