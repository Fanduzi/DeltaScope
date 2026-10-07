# Decision: T06-A4 metadata provider default NULL identity fidelity

Date: 2026-10-07
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A4 implementation commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/metadata/mysql/provider_t06a4_test.go`, `internal/application/auditmeta/audit_t06a4_test.go`, `internal/application/audit/batch_state_t06a4_test.go`, `pkg/deltascope/audit_provider_defaults_t06a4_test.go`, `internal/interfaces/http/audit_provider_defaults_t06a4_test.go`, `internal/interfaces/cli/audit_t06a4_test.go`, `scripts/test_ddl_golden.py` T06-A4 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `docs/decisions/2026-10-06-ddl-default-null-literal-fidelity.md` (A3; this record closes the provider half of its deferred metadata item)

## Context

T06-A4-PLAN traced the drop-column conservative boundary to a provider-side
fact error. `loadColumns` in the MySQL/TiDB metadata provider populated
`DefaultIsNull` by comparing the non-NULL `COLUMN_DEFAULT` text against
`"null"` case-insensitively. On a real MySQL 8.4 fixture this produced the
inverted identity: a column declared `DEFAULT 'NULL'` (string literal, stored
bytes `4E554C4C`) surfaced as `DefaultIsNull=true`, while a real `DEFAULT NULL`
surfaces as a NULL row (`COLUMN_DEFAULT IS NULL`) that set no flag at all and
is indistinguishable from an omitted `DEFAULT` clause.

The proven consumer was again the T05-A6 seam:
`dropOtherColumnMayReference` treats `HasDefault && !DefaultIsNull &&
!DefaultIsCurrentTimestamp` as an unprovable column reference. The inverted
flag made a stored `'NULL'` literal read as dependency-free, so
`DROP COLUMN d` followed by `CREATE INDEX idx_b(b)` published a precise
`pass/complete` under provider-loaded state while the parsed-source view of
the same schema stayed conservatively `review/unverified` — the same defect
A3 fixed on the extraction side, mirrored on the snapshot side.

## Decision

`loadColumns` no longer interprets default text: the `EqualFold("null")` →
`DefaultIsNull` mapping is removed. A non-NULL `COLUMN_DEFAULT` is always the
stored representation of some default clause — it can never prove the SQL
NULL datum, because that datum is exactly the NULL row where
`COLUMN_DEFAULT IS NULL` holds. The field contract is now:

- `HasDefault = COLUMN_DEFAULT IS NOT NULL` (a stored default exists);
- `DefaultValue = COLUMN_DEFAULT` verbatim — raw bytes, no added quoting,
  no case folding;
- `DefaultIsNull` is never set by this path. Neither side of the NULL row
  can claim it: a NULL catalog value is consistent with both an omitted
  `DEFAULT` clause and an explicit `DEFAULT NULL`, and promoting either
  reading would fabricate declaration provenance the catalog does not keep;
- `DefaultIsCurrentTimestamp`, `OnUpdate`, and all other column, index,
  version, error, and statistics reads are unchanged.

Two source-level facts stay deliberately separate: on the input-AST path
`HasDefault` means "an explicit `DEFAULT` clause was written"; on the
metadata path it means "a non-NULL default is stored". Neither is required
to agree with the other field-for-field, and no field may be read as
recovering the original declaration. `SHOW CREATE` cannot help: it emits
the normalized `DEFAULT NULL` for nullable columns regardless of what was
historically declared.

The PostgreSQL provider is explicitly out of scope: its `column_default`
comes from `pg_get_expr`, which renders a stored expression as SQL text —
a different source kind, not the same nullable `COLUMN_DEFAULT` column.
Its lookalike text compare is dormant for exactly this reason and is left
untouched; only its existing regression suite runs.

No rule, spec field, gap vocabulary, state machine, or transport changed.
`dropOtherColumnMayReference` keeps the A6 conservative boundary verbatim —
it now receives the truthful flag: a stored `'NULL'` or `'<nil>'` literal is
an unprovable reference and stays conservative, while a true NULL default
(the NULL row) stays precise.

`testdata/ddl-golden/T06.json` extends the frozen set to 84 cases: the 72
accepted cases verbatim plus 12 anchored metadata cases —
`t06-a4-{anchor}-{drop-d-text-null|drop-c-text-nil|drop-d-null-control}` —
under a new three-blocker isolated profile
`t06-a4-provider-defaults-isolated`. Unlike every earlier metadata case, the
physical `CREATE TABLE` lives in setup before the audit, so the audited
two-statement `DROP COLUMN` + `CREATE INDEX` batch can only be answered from
the live `information_schema` snapshot — the input AST carries no defaults.
The setup and post-audit oracles pin the COLUMN_DEFAULT NULL flags and raw
HEX bytes, mid-flight checks run between the driver DROP and the driver
CREATE INDEX, and `--fail-on blocker` keeps `exit=0` while the conservative
variants record `review`/`unverified` — exit 0 is not pass.

## Public Contract

Consumers can rely on: for MySQL/TiDB metadata snapshots, `HasDefault=false`
means only that no non-NULL default is stored — it must not be read as "the
original SQL omitted `DEFAULT`". `DefaultIsNull` from this source is always
`false`; the typed `DEFAULT NULL` identity exists only on the input-AST path
established by A3. `DefaultValue` is the stored representation text, not a
reconstructed declaration; text conventions differ from parser output
(parser literals keep their quotes, provider values are raw bytes) and no
consumer may compare across sources.

## Deferred / Out Of Scope

- Declaration provenance is unrecoverable from `COLUMN_DEFAULT` or
  `SHOW CREATE`; no source/provenance framework is introduced, and no
  current consumer needs one — the drop-column boundary only consumes the
  decidable "non-NULL stored default" fact.
- Expression defaults, temporal implicit defaults, `DEFAULT` execution
  semantics, generated columns, and the PostgreSQL `pg_get_expr` text
  interpretation stay with their owning tasks (T07/T08/T12/T27–T30).
- `DefaultValue` quote conventions are intentionally not unified across
  sources; no consumer reads the text today.

## Verification Evidence

- `go test -run T06A4` on mysql provider / audit / auditmeta / SDK / HTTP /
  CLI — real `Provider.LoadTableSnapshot` field matrix over injected driver
  rows (SQL NULL, `NULL`/`null`/`NuLl`, `'<nil>'`, `''`, `'0'`), the
  audit's A red→green flip and unchanged B/C controls, parsed-vs-provider
  literal parity at the drop seam, error-channel preservation,
  `default.require` input-only behavior, and the four-level fail-on exit
  table on the A result shape.
- `python3 scripts/test_ddl_golden.py` — contract suite extended to 84
  required cases plus T06-A4 mutations (old pass/complete revival, gap
  deletion/misattribution/required_facts reorder, A↔C role swap, literal
  bytes forged as SQL NULL in both directions, setup-CREATE removal or
  relocation into audited input, post-audit and mid-flight oracle deletion,
  cross-anchor rebind, offline demotion, profile swap, paired-side case
  deletion).
- `make ddl-golden TASK=T06` — 84 cases across the four locked anchors
  (evidence recorded with the task evidence commit).

## Consequences

- The drop-column conservative boundary now agrees across evidence sources:
  parsed `DEFAULT 'NULL'` and stored `'NULL'` both stay conservative; only a
  true NULL datum (parser `DEFAULT NULL` or catalog NULL row) is
  dependency-free.
- Audits that previously produced `pass/complete` on provider-loaded state
  with a literal `'NULL'` sibling now correctly produce
  `review`/`unverified` with one `unknown_table_state` gap on the follower
  statement. This is a correctness tightening, not a new restriction.
- `HasDefault` on provider-loaded columns narrows slightly in what it can
  be claimed to mean; the A3 decision's deferred "metadata default-source
  semantics" item is resolved to the bounded contract above.

## Links

- Tests: `internal/infrastructure/metadata/mysql/provider_t06a4_test.go`,
  `internal/application/auditmeta/audit_t06a4_test.go`,
  `internal/application/audit/batch_state_t06a4_test.go`,
  `pkg/deltascope/audit_provider_defaults_t06a4_test.go`,
  `internal/interfaces/http/audit_provider_defaults_t06a4_test.go`,
  `internal/interfaces/cli/audit_t06a4_test.go`
- Docs: `testdata/ddl-golden/T06.json`,
  `testdata/ddl-inventory/T06-base-traceability.md`
