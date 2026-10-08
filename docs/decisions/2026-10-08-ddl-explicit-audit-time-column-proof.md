# Decision: T06-A9 explicit audit-time-column role proof — facts, not names

Date: 2026-10-08
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A9 proof commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/infrastructure/parser/tidb/extractor_t06a9_test.go`, `internal/application/audit/coverage_t06a9_test.go`, `pkg/deltascope/audit_columns_t06a9_test.go`, `internal/interfaces/http/audit_columns_t06a9_test.go`, `scripts/test_ddl_golden.py` T06-A9 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `testdata/ddl-inventory/inventory.yaml`

## Context

The T06-A9 baseline (`/tmp/ds-t06-a9/`, collected on evidence HEAD
`3c8bac8b`) proved `ddl.table.audit_columns.require` already implements the
desired role semantics end to end on real parser output: a 12-run offline
matrix (six roles × mysql/tidb) plus a real-AST probe showed complete-pair
pass, exact per-kind blockers for each missing role, the two-finding
missing-both shape, `NOW()` acceptance, and a clean all-off control. No
product red existed to fix — what was missing was *committed* proof. The
pre-A9 suite pinned the rule only through synthetic `spec.Column` fixtures,
a single missing-both shape, and corpus fixtures; the synonym
normalization, per-role isolation, counter controls, derived-state flag
carry-over, and four-anchor stored-column oracle had no dedicated evidence.

## Decision

- **Two named roles are team policy, not a column-name convention.**
  `ddl.table.audit_columns.require` requires one *created-time* column
  (time-like type + current-timestamp default + no `ON UPDATE`) and one
  *updated-time* column (time-like type + current-timestamp default +
  `ON UPDATE CURRENT_TIMESTAMP`). Recognition runs on the extracted typed
  flags `DefaultIsCurrentTimestamp` and `OnUpdateCurrentTimestamp` — any
  column name qualifies, and the `created_at`/`updated_at` names in the
  proof fixtures are inputs, not requirements.
- **A column holding `ON UPDATE` counts only toward `updated`.** It never
  doubles as the created role, so a table declaring only an updated column
  still earns the `created` blocker. Conversely `ON UPDATE` without a
  current-timestamp `DEFAULT` satisfies neither role and reports both as
  missing.
- **Spelling normalization is a parser fact, not a product comparison.**
  The TiDB parser normalizes `NOW()`, `LOCALTIME`, `LOCALTIMESTAMP`,
  `CURRENT_TIMESTAMP`, `CURRENT_TIMESTAMP()`, and fractional-second forms
  into one `FuncCallExpr` named `current_timestamp`. The extractor's
  `exprIsCurrentTimestamp` keys off that node kind/name, so synonyms are
  recognized for free. The proof pins this at the AST layer rather than
  assuming helper string equality.
- **A policy reject and a native rejection are separate facts.** Anchored
  B/C cases record the product `reject` verdict *and* the driver-side
  CREATE success: the stored table's `COLUMN_DEFAULT` is whitelisted to
  `current_timestamp`/`current_timestamp()`/`current_timestamp(0)`
  (lowercased, zero precision only) and `EXTRA` is checked per column —
  `created_at` empty or `DEFAULT_GENERATED`, `updated_at` carrying the
  `on update current_timestamp` marker. Textual rendering differences
  between versions are normalized inside the query, not smuggled into the
  expectation.
- **No production code changed.** The slice adds tests, the
  `t06-a9-audit-columns-isolated` manifest profile, 28 frozen cases
  (12 offline + 16 anchored), a multi-finding full-entry expectation in the
  proof contract, and validator mutations — the validator now compares
  complete finding entries as a multiset (index, kind, rule, level,
  message, location, exact metadata map) so duplicated or cross-swapped
  roles cannot pass.

## Public contract

- Frozen case ids: `t06-a9-{mysql|tidb}-{complete-pair|missing-created|missing-updated|missing-both|now-spelling|disabled}` (offline) and `t06-a9-{mysql57|mysql80|mysql84|tidb85}-{complete-pair|missing-created|missing-updated|now-spelling}` (anchored), raising T06 to 256 required cases.
- Messages pinned verbatim: `table should include a created-time audit column with DEFAULT CURRENT_TIMESTAMP` and `table should include an updated-time audit column with DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`, with metadata `{"table":"t","kind":"created"|"updated"}` at 1:1 on statement index 0.
- The rule's shipped default (`warning`, `required=true`) is unchanged; the isolated profile's `blocker` level exists only to isolate the proof.

## Deferred scope

Explicitly out of this slice: TIMESTAMP implicit defaults and
`explicit_defaults_for_timestamp`, time-zone and runtime clock semantics,
fractional-second precision policy (the parser keeps `NOW(3)`'s argument
but fsp>0 legality is unclaimed), other time functions and time-like
types, expression defaults, generated columns, ALTER-side audit-column
governance, `DefaultValue` text for function defaults (empty by design —
`HasDefault` and the typed flags carry the fact), and PostgreSQL behavior.

## Verification evidence

- `go test -count=1 ./internal/infrastructure/parser/tidb ./internal/domain/rule/ddl ./internal/application/audit -run T06A9 -v` — 6 regression groups green.
- `go test` on `pkg/deltascope` and `internal/interfaces/http` — entry representatives green.
- `make ddl-golden-validator-test` — 385 contract cases, 0 failures, including 18 A9 mutations and 10 T06-A9-R1 tightening controls.
- `make ddl-golden TASK=T06` — 256/256 on the four frozen anchors; per-case raw `COLUMN_DEFAULT`/`EXTRA`/`DATETIME_PRECISION` rows preserved in the artifact.

## Revision T06-A9-R1 — strict finding identity and EXTRA whitelist

Two proof-oracle weaknesses were found in review and fixed without
touching production code:

- **Actual findings can no longer borrow defaults.** The multi-finding
  canon previously applied `entry.get("statement_kind", "ddl")` to real
  output, so a fabricated finding with no `statement_kind` was silently
  treated as `"ddl"`. Expected manifest entries keep their compact
  shorthand, but actual findings are now validated strictly:
  `statement_kind` must be present and exactly `"ddl"`, `location` must
  be a real object carrying integer `line`/`column`, `metadata` must be
  a real map, and a present `statement_index` must agree with its
  enclosing statement (the omitempty-absent zero stays legal). Multiset
  comparison still normalizes ordering while preserving multiplicity and
  the message↔`metadata.kind` pairing.
- **The EXTRA oracle is a whole-text whitelist, not substring repair.**
  The old query deleted `default_generated`, `(0)`, and `()` anywhere in
  the text, which would "normalize" malformed strings such as
  `DEFAULT_GENERATED()` or `on update curr()ent_timestamp` into passing
  shapes. The query now folds the complete lower-cased text through a
  SQL `CASE`: created accepts exactly `''`/`'default_generated'`,
  updated accepts the six zero-precision `on update current_timestamp`
  spellings, and anything else — SQL NULL, fsp=3, duplicated markers,
  extra parentheses, unknown `GENERATED` markers — maps to a sentinel
  that satisfies neither role. The same `CASE` expression runs as a
  read-only literal control on every anchor, so legality is proven by
  the shipped query itself. The earlier artifact keeps its old query
  text and SHA as historical evidence; only new runs execute the strict
  query.
