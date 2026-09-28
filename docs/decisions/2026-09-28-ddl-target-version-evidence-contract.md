# Decision: target_version and observed-version evidence contract for version-dependent DDL rules

Date: 2026-09-28
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T04-B/#83)
Related commits: this task's commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/domain/spec/version_test.go`, `internal/domain/rule/ddl/key_length_version_test.go`, `internal/application/audit/version_t04b_test.go`, `internal/interfaces/{cli,http,mcp}/audit_version_t04b_test.go`, `pkg/deltascope/audit_version_t04b_test.go`, `make ddl-golden TASK=T04 ARTIFACT_DIR=...`
Related docs: `testdata/ddl-golden/T04.json`, `docs/decisions/2026-09-23-ddl-incomplete-coverage-contract.md` (T04-A amendment), `docs/dev/testing.md`

## Context

Task T04-A established metadata evidence gaps: enabled, required, applicable
rules that cannot obtain declared facts report bounded `evidence_gaps`
instead of silently passing. Task T04-B (#83) extends the same contract to
version facts. Before this change, `ddl.index.key_length.max_bytes.require`
either resolved nothing offline or consumed whatever instance string the
provider happened to return, with no canonical product/version identity,
no caller-supplied target constraint, and no reconciliation between the two.
A missing version silently evaluated as `complete`/`pass`, which is exactly
the failure mode #79 requires the audit model to expose.

## Decision

1. **One shared parser-neutral version identity.** `spec.VersionIdentity`
   carries `product`, canonical `version`, `major`/`minor`/`patch`,
   `source` (`target` | `observed`), and `validated_range`. All version
   judgment lives in `internal/domain/spec` (parsing) and
   `internal/application/audit` (attachment/reconciliation); SDK, CLI,
   HTTP, and MCP only forward the input and map the shared typed errors.

2. **Strict target grammar.** `target_version` accepts `[v]MAJOR.MINOR.PATCH`
   only: surrounding whitespace trimmed, optional single `v`/`V` prefix,
   exactly three non-negative decimal integers, canonicalized to
   `MAJOR.MINOR.PATCH`. Two-component forms, suffixes, banners, signs,
   overflow, and empty `v` are input errors (`ErrInvalidTargetVersion`),
   not audit results. Malformed or conflicting input produces no findings,
   gaps, unsupported entries, or parser diagnostics.

3. **Validated series is evidence, not syntax.** The milestone-verified
   series are MySQL 5.7.x / 8.0.x / 8.4.x and TiDB 8.5.x. A syntactically
   valid version outside the series parses successfully with
   `validated_range=false`; version-dependent rules then emit
   `target_version_out_of_validated_range` (required_facts:
   `target.version.validated_range`) instead of an input error.

4. **Observed identity is authoritative online.** Provider-reported
   product/version is canonicalized once from the raw banner
   (`ParseObservedVersion`): MySQL banners resolve their leading
   `MAJOR.MINOR.PATCH`; TiDB compatibility banners such as
   `8.0.11-TiDB-v8.5.0` resolve to `product=tidb, version=8.5.0` — the
   MySQL compatibility prefix is never the TiDB version. The observed
   product is derived from the banner itself, never injected from the
   request dialect. A caller `target_version` may only constrain observed
   identity: equality proceeds, a version inequality returns
   `ErrTargetVersionMismatch`, and a dialect/product conflict returns
   `ErrDialectProductMismatch`; the caller value can never override the
   observed fact. A missing or unparseable observed banner remains
   missing evidence; nothing is fabricated. Provider/connection errors
   keep their existing precedence and never degrade into gaps or
   mismatches.

5. **Missing versions never default.** An absent target/observed version
   leaves `Metadata.Version` unset; the result carries no `version` block.
   The wired rule (`ddl.index.key_length.max_bytes.require`) reports
   `missing_target_version` (required_facts: `target.version`) — coverage
   drops to `unverified`, the verdict floors to `review`, and gaps carry
   warning-equivalent `--fail-on` weight without counting as findings.

6. **First version-dependent rule bound, with candidate bounds.** For
   `ddl.index.key_length.max_bytes.require`, bounds verified against
   versioned official documentation: MySQL COMPACT/REDUNDANT → 767 bytes
   in every validated series (no further facts needed). MySQL
   DYNAMIC/COMPRESSED scales with `innodb_page_size`: 4KB → 768, 8KB →
   1536, 16KB and larger applicable pages → 3072; an unobserved page size
   expands the bound to the candidate family {768, 1536, 3072} instead of
   an assumed 16KB. MySQL 5.7 additionally needs `innodb_large_prefix`:
   proven OFF caps every row format at 767, proven ON applies the
   page-size bound, unknown keeps 767 as a candidate alongside the page
   family. TiDB 8.5.x reads the tidb-server `max-index-length` config via
   `SHOW CONFIG`; a known value is the bound, an unknown value yields the
   documented candidate range {3072, 12288} — the 3072 default is never
   asserted as fact. Missing instance facts produce a candidate bound
   family: the check resolves only when every candidate agrees, and any
   total in between reports `missing_instance_fact` naming the absent
   facts (`instance.innodb_page_size`,
   `instance.innodb_large_prefix_enabled`,
   `instance.innodb_default_row_format`, `instance.tidb_max_index_length`).
   A total exceeding every candidate is still a proven finding alongside
   the gap, and a resolved finding never contradicts a gap.

7. **Public projection is canonical-only and zero-exact.**
   `report.Result.Version` and the SDK `Result.Version` expose the
   resolved `VersionIdentity`; `major`/`minor`/`patch` are always
   serialized so `8.0.46` keeps `minor: 0` and `8.5.0` keeps `patch: 0`.
   The raw provider banner stays inside internal/artifact evidence
   records (`version_evidence`) and never reaches findings, gaps, or
   result identity fields. Golden artifacts record `target_raw`,
   `target_canonical`, `observed_raw`, `observed_canonical`, `resolved`,
   and — for online cases — live `instance_facts` per case; the validator
   re-derives every one of them from the manifest, raw stdout, and the
   recorded anchor banner.

8. **Transport mapping is uniform, and preflight precedes connection.**
   `spec.ValidateTargetVersion` is the single shared syntax check every
   transport runs before any connection lookup, prepare, or open: CLI
   before `resolveConnectionOptions`, HTTP before the registry lookup,
   MCP before `ResolveAuditConnection`. A malformed target is an input
   error even when the connection is unreachable; a valid out-of-range
   target proceeds into audit and degrades to the range gap. SDK:
   `Request.TargetVersion`, `Result.Version`, typed input errors. CLI:
   `--target-version`, exit 2 on malformed/mismatch. HTTP:
   `target_version` JSON field, 400 on the same errors. MCP:
   `target_version` input, `isError=true` on the same errors. No
   transport owns version logic.

## Public contract changes

- Audit request gains optional `target_version` / `TargetVersion` /
  `--target-version` on all four surfaces.
- Audit result gains optional top-level `version` (`VersionIdentity`);
  its `major`/`minor`/`patch` numeric components are always emitted,
  including zero.
- New typed errors: `ErrInvalidTargetVersion`,
  `ErrTargetVersionMismatch`, `ErrDialectProductMismatch`.
- New evidence-gap reason codes: `missing_target_version`,
  `target_version_out_of_validated_range`, `missing_instance_fact`
  (with `instance.innodb_page_size`,
  `instance.innodb_large_prefix_enabled`,
  `instance.innodb_default_row_format`, and
  `instance.tidb_max_index_length` facts).
- `InstanceFacts` gains explicit known bits —
  `InnoDBPageSizeKnown`/`InnoDBPageSizeBytes`,
  `InnoDBLargePrefixKnown`/`InnoDBLargePrefixOn`,
  `TiDBMaxIndexLengthKnown`/`TiDBMaxIndexLengthBytes` — so a proven
  OFF/zero differs from an unobserved fact, and an unparseable or absent
  provider value stays unknown rather than defaulting.

## Deferred scope

- Prospective Schema State (#84/T05) is untouched: this task adds request
  input, version facts, and one rule's bound resolution only.
- Only `ddl.index.key_length.max_bytes.require` consumes the version fact;
  T06–T30 semantic families and the remaining metadata rules keep their
  existing oracles.
- Query Access version contracts are unchanged.
- TiDB `max-index-length` is read from `SHOW CONFIG` with every matching
  row (one per tidb-server instance) verified for a single consistent
  value; zero rows, unparsable/non-positive values, and disagreeing
  instances stay unknown rather than fabricating the 3072 default, while
  query/scan/iteration failures propagate as provider errors — a failed
  read is an error, never a gap.

## Verification

- Golden: `testdata/ddl-golden/T04.json` — 163 cases total, preserving
  all 131 pre-existing cases. T04-B adds the
  `t04b-key-length-isolated` policy profile, offline `cli_cases`
  (missing/valid/`v`-prefix/out-of-range/5.7-instance-fact/
  proven-blocker/TiDB matrix, page-size candidates, unknown-limit
  candidates), `metadata_cases` across six anchors including two
  task-scoped auxiliary fixtures (`mysql84-4k` with
  `innodb_page_size=4096`, `tidb85-12288` with
  `max-index-length=12288`), and `error_cases` (malformed target with
  unreachable connection, version mismatch, same-version-different-
  product dialect conflict on both banners).
- Validator (`scripts/ddl_golden.py validate`) independently
  canonicalizes target inputs and observed banners, requires numeric
  components to exist (rejects a missing `minor` in `8.0.46` or missing
  `patch` in `8.5.0`), requires declared `instance_facts` with live-read
  values, requires observed banner evidence on anchored error cases,
  rejects request/observed conflicts recorded as success, out-of-range
  marked `complete`, TiDB resolved as `8.0.11`, malformed targets
  laundered into connection failures, deleted or tampered
  `instance_facts`, tampered `target_raw`/`target_canonical`, missing
  `version_evidence`, and error stdout carrying an audit result;
  `make ddl-golden-validator-test` covers these mutations offline
  (75 contract cases).
- Anchor evidence: MySQL 5.7.44 / 8.0.46 / 8.4.10, TiDB
  `8.0.11-TiDB-v8.5.0` → `tidb 8.5.0`, plus auxiliary MySQL 8.4.10 at
  `innodb_page_size=4096` and TiDB 8.5.0 at `max-index-length=12288`,
  all verified against live containers.
