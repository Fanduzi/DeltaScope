# Decision: T06-A15 row-size field-level fact resolution

Date: 2026-10-18
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A15 fix commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/domain/rule/ddl/size_rules_t06a15_test.go`, `internal/domain/rule/ddl/size_rules_test.go` (`TestSizeRulesWithoutInstanceFacts`), `internal/domain/rule/catalog/catalog_test.go` (`TestLookupMarksRowSizeInstanceDependency`), `internal/application/audit/batch_state_t06a15_test.go`, `pkg/deltascope` and `internal/interfaces/http` A15 seam tests, `scripts/test_ddl_golden.py` T06-A15 mutation group
Related docs: `testdata/ddl-golden/T06.json`, `docs/reference/rules.md`, `docs/reference/config.md`, `docs/reference/audit-capability-matrix.md` (+ zh-CN mirrors), T06-A15 baseline `/tmp/ds-t06-a15-baseline/`

## Context

The T06-A15 baseline (offline, collected on evidence HEAD `73717598`)
proved `ddl.table.row_size.max_bytes.require` was gated on the *existence
of* an `InstanceFacts` pointer rather than on the facts the estimate
actually consumes:

- Offline audits (no provider at all) and nil-instance requests silently
  produced `pass`/`complete`/`applicable:1` on a 65534-character latin1
  VARCHAR — the over-limit estimate never ran, and nothing distinguished
  "evaluated and safe" from "missing facts, never evaluated".
- Nil and empty instance objects diverged: an empty-but-present object
  executed the estimate (declared options made fallbacks unreachable)
  while a nil pointer skipped it — pointer shape, not fact availability,
  decided the outcome.
- The rule's catalog metadata declared `MetadataKindTargetTable`, but the
  real dependency is the optional instance-default fields
  (`default_charset`, `innodb_default_row_format`) — never a target-table
  snapshot.

This violated the milestone's central requirement: fixture coverage must
be able to prove audit semantics, evidence gaps, and recognized-but-
unsupported states separately instead of collapsing every missing fact
into an indistinguishable `pass`.

## Decision

- **Declaration-first, field-level resolution.** The row-size rule keeps
  its existing scope gates (required flag, CREATE TABLE applicability,
  declared non-InnoDB engine skip) and then resolves each fact
  independently: column `CHARACTER SET` over the declared table charset
  over `instance.default_charset`; a declared `row_format` over
  `instance.innodb_default_row_format`. No source exists → the field is
  unknown; unknown is never filled with `utf8mb4` or `DYNAMIC` guesses.
- **`ROW_FORMAT=DEFAULT` is unresolved, not DYNAMIC.** A `DEFAULT`
  keyword, an absent option, and an unrecognized keyword are all routed
  to the instance default and stay unknown when that field is empty.
- **No instance object does not mean "cannot compute"; an empty object
  does not mean "facts are known".** A nil pointer, `&InstanceFacts{}`,
  and a fully populated object produce identical findings/gaps whenever
  the declared inputs suffice — and identical gaps when they do not.
- **Findings and gaps coexist.** When the resolved facts already prove
  `total > 65535`, the blocker reports with `row_format=""` in metadata
  (the field that never resolved) *and* the statement carries one
  `missing_instance_fact` gap naming `instance.innodb_default_row_format`
  — coverage drops to `unverified`, verdict stays `reject`. A gap never
  erases a determined finding; a finding never launders a gap.
- **At most one gap per statement, field-ordered.** `required_facts`
  lists only the facts that were actually needed and actually missing,
  always in the order `instance.default_charset` then
  `instance.innodb_default_row_format`. A fact the estimate never
  consumed is never listed: fixed-width columns cannot invent a charset
  gap, and when every charset-sensitive column declares its own
  `CHARACTER SET`, an undeclared table charset is simply unused — it may
  stay `""` in `metadata.charset`.
- **`metadata.charset` keeps its table-level meaning.** It records the
  resolved *table* charset (declared option or instance fallback), not
  "the charset all columns used". Column-level `CHARACTER SET` still
  wins inside `estimatedColumnBytes` — e.g. a utf8mb4 column under a
  latin1 table estimates at 4 bytes/char while `metadata.charset` stays
  `latin1`.
- **The estimate stays a rough estimate.** Thresholds and formulas are
  frozen: 65535-byte row limit, COMPACT 8126 / REDUNDANT 8000 payload
  bounds with the `min(bytes, 768)` accumulator, and the existing
  `estimatedColumnBytes`/`charsetBytes` dispatch unchanged. The estimate
  is a conservative preflight bound, not a physical-storage model —
  NULL bitmaps, row headers, page splitting, and engine-internal
  encodings remain out of scope.
- **Explicit non-InnoDB stays a scope skip, not a safety proof.** A
  declared `ENGINE=MyISAM` (or other non-InnoDB value) still returns
  before any fact lookup — the rule does not check those engines, which
  is never a claim that the row is physically safe.
- **TiDB inherits the same declared/field-level contract.** TiDB parses
  and largely ignores `ROW_FORMAT`; the estimate models the declared
  input under the same bounds. No TiDB page-size, storage, or
  TiKV-internal model is added or implied.

## Rationale

The alternative — keeping the whole-object gate and documenting the
silent skip — permanently loses the distinction the milestone exists to
provide, and forces every offline audit of a wide table to report
`complete` for a check that never ran. Field-level resolution is the
smallest change that makes missing evidence *visible* without inventing
facts: the shared `resolvedTableCharset`/`resolvedRowFormat` helpers keep
their legacy fallbacks (and the index-key-length rule keeps its own
whole-object gate — that contract was already accepted separately), so
only the row-size rule learns the new contract.

## Public Contract

- `ddl.table.row_size.max_bytes.require` now emits
  `missing_instance_fact` evidence gaps on MySQL/TiDB with
  `required_facts` ⊆ `{instance.default_charset,
  instance.innodb_default_row_format}` in that fixed order — at most one
  gap per statement, only for fields the estimate needed and could not
  resolve. Gap-only statements report `coverage=unverified`,
  `verdict=review`; a determined over-limit finding beside a gap reports
  `reject`. Exit codes follow the existing `--fail-on` mapping.
- `required=false` and `enabled=false` keep their existing semantics
  (loaded-not-applicable vs absent `rule_summary`) and emit no gaps.
- `metadata.charset` remains the resolved *table-level* charset;
  `metadata.row_format` stays `""` when unresolved, and
  `metadata.estimated`/`limit` keep their existing names and values.
- Catalog metadata: the rule's kind is now `MetadataKindInstance` — an
  optional defaults source, not a connectivity requirement — and its
  notes state that declaration-complete statements evaluate offline.
- PostgreSQL and the index-key-length rule are byte-for-byte unchanged:
  non-MySQL-family statements keep the whole-object instance gate, and
  the key-length rule keeps its previously accepted missing-instance
  silence.
- Frozen case ids: `t06-a15-{mysql|tidb}-<role>` for roles
  `below|equal|above|column-override|non-innodb-skip|missing-charset|
  missing-format-safe|missing-format-over|missing-both|
  column-only-charset|not-required|disabled` (24 cases), raising T06 to
  496 required cases. Two isolated profiles ship in the test manifest
  only: `t06-a15-row-size-isolated` (required=true) and
  `t06-a15-row-size-not-required-isolated` (required=false).

## Deferred / Out Of Scope

- No physical row-size model, page-size model, NULL-bitmap/row-header
  accounting, index-page accounting, or per-engine accuracy claims.
- No charset-universe or alias framework, and no collation-derived
  charset inference — a declared charset name is trusted as declared.
- No default-engine observation: an omitted `ENGINE` stays inside the
  conditional InnoDB estimate and is not promoted to "server default
  observed".
- The index-key-length rule's whole-object missing-instance silence is
  untouched; broadening it is a separate decision.
- PostgreSQL row-size coverage and other dialects' fact models stay as
  before — the gap projection never leaks outside MySQL/TiDB.

## Verification Evidence

- Red-on-baseline proof: the A15 test files compiled and failed on the
  pre-fix tree (pointer-shape divergence, absent `EvidenceReporter`,
  silent pass) before the implementation landed.
- `go test -count=1 ./internal/domain/rule/ddl ./internal/domain/rule/catalog ./internal/application/audit -run T06A15 -v` — field matrix, precedence, gap ordering, finding+gap coexistence, scope skip, pointer-shape equivalence, and provider-error/cancellation propagation green on both dialects.
- `make ddl-golden-validator-test` — 728 checks, including the T06-A15
  mutation group (forged/missed findings, fabricated-complete missing
  facts, wrong/reordered/duplicated `required_facts`, declared-fact
  laundering, dropped determined finding, column-vs-table charset
  laundering, profile/record swaps, fabricated `rule_summary`, and
  denominator shrinkage).
- `make ddl-golden TASK=T06` — 496/496 on the four frozen anchors (the
  24 new cases are offline roles; the anchored slices are regression,
  not A15 physical evidence).
- SDK (`pkg/deltascope`) and HTTP seam tests exercise the same contract
  through public entrypoints.

## Consequences

- Any future row-size work (physical models, more engines, version-
  conditioned formats) must preserve the field-level gap contract: a
  fact that cannot be resolved is reported, never guessed.
- `metadata.charset` consumers must keep reading it as the table-level
  resolution, not a per-column attribution.
- Rules needing instance defaults should follow this resolver pattern —
  declaration first, per-field fallback, gap-per-statement — rather than
  re-adding whole-object presence gates.

## Links

- Commits: T06-A15 code commit and evidence commit on
  `milestone/mysql-tidb-ddl-completion`
- Tests: listed in the header block
- Baseline: `/tmp/ds-t06-a15-baseline/` (binary + 8 offline CLI runs +
  14 controlled `AuditSQL` calls proving the pre-fix gate)
