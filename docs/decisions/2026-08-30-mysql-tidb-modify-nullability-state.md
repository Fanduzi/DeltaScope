# Decision: Compare MySQL/TiDB MODIFY Nullability With Known State

Date: 2026-08-30
Status: Accepted
Related issue: [GitHub #47](https://github.com/Fanduzi/DeltaScope/issues/47)
Implementation: [fix(ddl): compare MODIFY nullability with metadata](https://github.com/Fanduzi/DeltaScope/commit/1ac5749abc827e70c7e826ef24a400443a0d92c2) and [publicly clarify the hybrid advisory metadata scope](https://github.com/Fanduzi/DeltaScope/commit/c174408d80cb11b68bb5df5a743b5a5bdbee27c8)

## Context

MySQL and TiDB `MODIFY COLUMN` syntax commonly repeats the current `NULL` or
`NOT NULL` attribute while changing a type. Treating every explicit attribute
as a transition rejected safe type widening when the live column already had
the requested nullability. The parser already preserves the requested target
state and whether nullability was explicitly written, while the metadata
provider already normalizes `information_schema.columns.is_nullable` into the
current column state.

## Decision

Keep `ddl.alter.modify_column.explicit_nullability_change.forbid` as the
configured blocker, but make it compare the requested `Definition.NotNull`
with a confirmed source column from `Metadata.TargetTable`. It fires only when
the values differ. A matching `NULL` or `NOT NULL` restatement is silent for
this rule.

When the prior state cannot be confirmed, emit the separate notice-level rule
`ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory`.
The advisory records bounded `prior_state: unknown` and the requested
nullability, and never claims that a transition occurred. Omitted nullability
does not enter either path. `CHANGE COLUMN` keeps its existing action and
explicit-change policies, and type compatibility rules continue to evaluate
independently.

## Rationale

The existing parser-neutral contract already distinguishes explicit intent
from an omitted attribute, so adding another nullability enum would duplicate
state. Requiring both an existing table snapshot and a matching source column
prevents a zero-value or incomplete metadata response from being interpreted as
nullable. A separate advisory keeps offline audits useful without weakening the
live transition blocker.

## Public Contract

- Confirmed MySQL/TiDB restatements do not produce the transition blocker.
- Confirmed nullable→`NOT NULL` and `NOT NULL`→nullable changes produce the
  configured blocker.
- Offline or incomplete-state explicit `NULL`/`NOT NULL` produces the stable,
  non-blocking unknown-prior-state advisory.
- The advisory is enabled by default at `notice` level and is discoverable in
  the rule, config, capability, and DDL coverage catalog surfaces.
- Findings remain bounded to table/action/column identifiers, change kind,
  prior-state classification, and requested nullability; raw SQL is not added.

## Deferred / Out Of Scope

- Automatic safety decisions for arbitrary type changes.
- Any change to `CHANGE COLUMN` policy or semantics.
- Treating absent, failed, or incomplete metadata as proof of a source state.
- New default levels or unrelated MySQL/TiDB ALTER policies.

## Verification Evidence

- [Application transition and offline tests](https://github.com/Fanduzi/DeltaScope/blob/1ac5749abc827e70c7e826ef24a400443a0d92c2/internal/application/audit/service_test.go)
  cover all four live state combinations, omitted nullability, unknown
  metadata, and independent compatibility findings.
- [Parser extraction tests](https://github.com/Fanduzi/DeltaScope/blob/1ac5749abc827e70c7e826ef24a400443a0d92c2/internal/application/audit/extract_test.go)
  cover explicit `NULL` and `NOT NULL` targets for MySQL and TiDB.
- [MySQL metadata provider tests](https://github.com/Fanduzi/DeltaScope/blob/1ac5749abc827e70c7e826ef24a400443a0d92c2/internal/infrastructure/metadata/mysql/provider_test.go)
  lock `is_nullable` normalization.
- [MySQL corpus fixtures](https://github.com/Fanduzi/DeltaScope/tree/1ac5749abc827e70c7e826ef24a400443a0d92c2/testdata/sql-corpus/mysql)
  and [TiDB corpus fixtures](https://github.com/Fanduzi/DeltaScope/tree/1ac5749abc827e70c7e826ef24a400443a0d92c2/testdata/sql-corpus/tidb)
  cover offline and metadata-aware cases. The CLI metadata E2E harness carries
  live restatement and transition assertions for both dialects.

## Consequences

Metadata-aware callers must continue to provide a trustworthy table snapshot
with the source column when they want transition enforcement. Offline callers
receive a reviewable notice rather than a false blocker. Future source-aware
MODIFY rules should reuse the same confirmed-source boundary and should not
infer state from a missing column or a failed lookup.

## Links

- [GitHub #47](https://github.com/Fanduzi/DeltaScope/issues/47)
- [Exact implementation commit](https://github.com/Fanduzi/DeltaScope/commit/1ac5749abc827e70c7e826ef24a400443a0d92c2)
- [Exact public-contract clarification commit](https://github.com/Fanduzi/DeltaScope/commit/c174408d80cb11b68bb5df5a743b5a5bdbee27c8)
