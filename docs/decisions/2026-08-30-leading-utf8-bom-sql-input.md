# Decision: Accept One Leading UTF-8 BOM in SQL Input

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #44
Related commits:
- `46a1160b7a2df06d2c2f3a75eccb3b3d1514e3d4`
- `75787aba5b77ded7ae97bafd251a12b13bf1ec5d`
- `e93ac4c`
Related tests:
- `TestParseLeadingUTF8BOMUsesVisibleSQLLocations`
- `TestAuditLeadingUTF8BOMMatchesBOMFreeInput`
- `TestAuditOnlyStripsLeadingUTF8BOM`
- `TestAuditBOMOnlyInputIsEmpty`
- `TestAnalyzeQueryAccessLeadingUTF8BOMMatchesBOMFreeInput`
- `TestAnalyzeQueryAccessBOMOnlyInputIsRejectedAsEmpty`
- `TestAnalyzeQueryAccessBOMFreeEmptyInputKeepsZeroStatements`
- `TestAuditCommandAcceptsLeadingUTF8BOMFile`
- `TestSQLCorpusMySQLAndTiDB` (`leading_utf8_bom` fixture)
Related docs:
- `internal/application/README.md`
- `internal/application/audit/README.md`
- `internal/application/queryaccess/README.md`
- `pkg/deltascope/README.md`
- `internal/interfaces/cli/README.md`

## Context

SQL files beginning with the UTF-8 byte-order mark (`EF BB BF`) reached the
dialect parser with the marker treated as SQL syntax. Valid SQL consequently
returned a parser error, while the same text without the marker succeeded.
The audit and Query Access entry paths share SQL as an input contract, and
audit empty-input validation must recognize marker-only files as empty.

## Decision

The shared application input boundary removes exactly one leading UTF-8 BOM
before audit empty-input validation and parser dispatch. Query Access removes
the same marker before extraction and rejects the input only when that marker
was present and the remaining text is empty after whitespace trimming. All
public audit surfaces and Query Access session routes therefore inherit the
behavior through their existing application services.

## Rationale

`strings.TrimPrefix` with the exact UTF-8 BOM removes one marker without
altering ordinary UTF-8, CRLF, or later BOM characters. Keeping normalization
at the application boundary covers file, inline, HTTP, MCP, CLI, and SDK
requests without duplicating transport logic or changing parser recovery.

## Public Contract

- One leading UTF-8 BOM followed by valid SQL has the same audit or Query Access semantics as BOM-free SQL.
- Marker-only and marker-plus-whitespace audit or Query Access input is rejected as empty.
- BOM-free Query Access empty input keeps its existing `zero_statements` result.
- BOM characters after the first decoded character are not removed.
- Statement locations are calculated from the visible SQL after the marker.
- UTF-16/UTF-32 detection, malformed UTF-8 repair, arbitrary zero-width stripping, and parser recovery remain unsupported.

## Deferred / Out Of Scope

BOM-free Query Access keeps its existing empty-input result contract
(`zero_statements`) to avoid changing the separate Query Access empty-input
public contract. Only BOM-prefixed input that becomes empty is rejected here.
Issue #43 parser recovery is unchanged.

## Verification Evidence

Public SDK audit and Query Access regression tests, CLI BOM-file coverage, the
byte-level SQL corpus fixture, focused package tests, and the repository full
test gates verify this boundary.

## Consequences

Future SQL entrypoints must route through the application boundary before
validation or parsing. Do not broaden this normalization to arbitrary Unicode
format characters or other encodings without a separate decision.

## Links

- Commits: `46a1160b7a2df06d2c2f3a75eccb3b3d1514e3d4`, `75787aba5b77ded7ae97bafd251a12b13bf1ec5d`, `e93ac4c`
- Issue: https://github.com/Fanduzi/DeltaScope/issues/44
- Tests: `pkg/deltascope/audit_test.go`, `pkg/deltascope/query_access_test.go`, `internal/interfaces/cli/cli_test.go`, `testdata/sql-corpus/mysql/dml/clean/leading_utf8_bom.sql`
- Docs: `internal/application/README.md`, `internal/application/audit/README.md`, `internal/application/queryaccess/README.md`, `pkg/deltascope/README.md`, `internal/interfaces/cli/README.md`
- Implementation: `internal/application/sql_input.go`
