# MySQL ALTER TABLESPACE ENGINE graduates from parser-upgrade candidate (T57)

Date: 2026-11-15
Status: decided

## Decision

The bounded recognizer for the MySQL tablespace family
(`internal/infrastructure/parser/tidb/mysql_tablespace.go`) now accepts
`ENGINE` as the leading member of a general `ALTER TABLESPACE`
statement, so the documented form

```sql
ALTER TABLESPACE <name> ENGINE [=] <engine>
```

parses with `AlterAction = "engine"` instead of falling back to a
parser error. The six-entry parser-upgrade candidate list shrinks to
five — all PostgreSQL forms (`ALTER CONSTRAINT INHERIT`,
`ALTER CONSTRAINT NO INHERIT`, `ALTER CONSTRAINT NOT ENFORCED`,
`NOT NULL NOT VALID`, `DROP SUBSCRIPTION WITH drop_slot`) remain
parser-error candidates pinned to the libpg_query 17 grammar.

## Rationale

- The form is officially documented MySQL syntax and was the only one
  of the six candidates implementable inside the existing bounded
  recognizer — the five PostgreSQL forms require a libpg_query bump,
  which is a different class of change.
- `ENGINE=InnoDB` now reaches the real rule surface:
  `ddl.mysql.tablespace.alter.warn` fires, and the
  `unknown_tablespace_state` evidence gap is reported honestly since
  tablespace existence is not proven offline.
- `ENGINE=NDB` and other non-InnoDB engines stay an explicit
  `alter_tablespace` engine boundary (`{boundary: "engine"}`) — the
  statement parses but the vendor engine is declined, never silently
  accepted.
- `ALTER TABLESPACE ts1 ENGINE` (no value) remains a parser error; the
  recognizer keeps its strict whole-input claim.

## Public contract

- Catalog entry `mysql.alter_tablespace.engine` moved from
  `parser_error`/`parser_upgrade_candidate` to `finding_covered` with
  `ddl.mysql.tablespace.alter.warn`; regenerated catalogs
  (`internal/application/audit/catalogdata/` and
  `docs/reference/ddl-coverage-catalog.json`) are synchronized.
- MySQL now has zero `parser_error` catalog entries and zero remaining
  parser-upgrade candidates; PostgreSQL retains five.
- No-leak diagnostics behavior is unchanged — the parser-error probe
  used by SDK/CLI/HTTP/MCP tests moved to the still-unrecognized
  `ALTER TABLESPACE ... ADD UNDOFILE` form, which keeps
  `guidance_code: parser_upgrade_candidate` coverage honest.

## Verification

- `go test ./...` green; `go test -tags postgresql` green.
- `python3 scripts/test_ddl_golden.py` — 896/896 contract cases.
- `make ddl-inventory-gate` green.
- SQL corpus fixture `mysql_tablespace_lifecycle_rules.sql` gained the
  `ENGINE=INNODB` alter line.

## Links

- Parser: `internal/infrastructure/parser/tidb/mysql_tablespace.go`,
  tests in `mysql_tablespace_test.go`.
- Corpus: `testdata/sql-corpus/mysql/ddl/findings/mysql_tablespace_lifecycle_rules.sql`.
- Catalog: `internal/application/audit/catalogdata/ddl-coverage-catalog.json`.
- Docs: `docs/reference/ddl-coverage.md`, `cli.md` (+ zh-CN).
