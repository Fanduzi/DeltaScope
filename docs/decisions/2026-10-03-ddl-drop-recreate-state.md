# Decision: Bounded DROP absence and independent same-name recreation

Date: 2026-10-03
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79, T05-A3 under #84)
Related decisions: `2026-10-02-ddl-ordered-schema-state-first-path.md`, `2026-10-20-ddl-rename-table-identity-migration.md`
Related tests: `internal/application/audit/batch_state_a3_test.go`, lifecycle rule tests, `testdata/ddl-golden/T05.json`, `scripts/test_ddl_golden.py`
Related docs: `internal/application/audit/README.md`, `internal/domain/rule/ddl/README.md`, `docs/reference/rules.md`, `docs/reference/rules.zh-CN.md`

## Context

The accepted ordered-state model deliberately left DROP as target invalidation.
Consequently CREATE(old_c), DROP, CREATE(id), ADD(c), INDEX(c) lost the absence
fact before the second CREATE. Ordinary CREATE already constructs an independent
shape; rebuilding CREATE or reloading the pre-batch database snapshot would not
fix the missing transition. DROP also needs to invalidate references already
loaded in this request: equal names do not establish that old references remain
valid after a different table definition is created.

## Decision

Precise deletion applies only to MySQL/TiDB, one bindable ordinary table target,
a consistent primary target, no omitted targets, no temporary scope, and no
unsupported marker or unaudited aspect. Target cardinality is checked before
key deduplication, so a repeated written target is not a single-table template.

| Pre-state | Plain DROP post-state | DROP IF EXISTS post-state |
|---|---|---|
| known-present, complete or partial | known-absent | known-absent |
| known-absent | unknown | known-absent |
| unknown, including invalidated entries | unknown | unknown |
| contaminated batch | remains contaminated | remains contaminated |

This is a conservative analysis knowledge threshold, not a prediction of
physical execution results. Unknown conditional existence is not upgraded by
IF EXISTS. A broken plain-DROP premise invalidates instead of simulating a
failed statement's no-op or rollback. Policy blockers do not erase conditional
structural effects.

The DROP transition retains the entry as an absent or unknown tombstone; it
never removes the cache key or reloads stale provider state. A known-absent entry
contains only identity, not the previous shape, options, members, or statistics.
The next ordinary CREATE uses the existing `derivedCreateShape`: only the new
DDL and subsequent modeled operations supply facts. Unprovided statistics stay
unknown rather than being filled with zero. No public generation identifier is
introduced.

RENAME and DROP share read-only selection of loaded references. Explicit
ReferencedSchema wins; an unqualified reference resolves under its owning
entry's schema. All target and dependent identities are collected before any
write. Precise deletion and conservative bound-DROP invalidation both tombstone
related loaded dependents. Multi-target DROP invalidates the entire target and
loaded-dependent union, without partial exact deletion. Same-name recreation
does not restore the child's old reference certainty. Unrelated entries,
provider-owned objects, and earlier statement projections remain unchanged.
Unbound effects retain the existing contamination behavior.

DROP observes cancellation at entry and immediately before publishing targets
and dependents. Provider errors propagate through enrichment before semantic
publication and retain their identity. There is no guarantee for cancellation
arriving after the final check; no locks or rollback mechanism are added.

## Rationale

Reusing the request-local state and existing CREATE logic preserves snapshot
ownership and once-per-identity reads without a second lifecycle engine.
Conservative invalidation keeps unknown effects from becoming facts. This also
avoids projecting one server version's multi-table atomicity onto all anchors:
MySQL 5.7 and 8.x differ when a DROP list includes an absent table.

Strict existence policy is separate from SQL execution legality. The existing
`ddl.table.drop.exists.require` continues to require a present target even for
IF EXISTS. An absent IF EXISTS can therefore execute successfully in the test
database while the product correctly rejects it under that enabled policy.
Neither server Notes nor evidence gaps become product findings.

## Public Contract

- Known-present DROP: no existence finding or gap.
- Known-absent DROP, with or without IF EXISTS: preserve the original one
  existence blocker, message, metadata, and source location; no gap.
- Unknown DROP with the rule enabled: exactly one evidence gap, rule ID
  `ddl.table.drop.exists.require`, reason `unknown_table_state`, required facts
  `[target_table.existence]`; no missing-table finding. The gap lowers coverage
  to unverified and imposes the existing review floor.
- Disabled or inapplicable rules do not produce gaps. The new reporter is
  restricted to this DROP rule ID, drop_table operation, and MySQL/TiDB.
  PostgreSQL, TRUNCATE, and other lifecycle rules keep their existing behavior.
- SDK returns nil error for gap-only audits, HTTP returns 200, MCP isError is
  false; CLI warning/notice thresholds fail on the gap, blocker/none do not.
  Real parser, unsupported, provider, and cancellation errors retain priority.
- The five-statement first path has five complete statements, zero findings,
  gaps, unsupported entries, and parser diagnostics, with verdict pass and CLI
  exit 0 under fail-on warning. Its third statement sees definite absence.

## Verification

The shared parser/application seam proves the state table, explicit absence,
read-once behavior, loaded-reference invalidation, immutable earlier snapshots,
new-member/statistic isolation, policy independence, and cancellation. Public
SDK and representative transport checks preserve result and error contracts.

The independent `t05-drop-recreate-isolated` profile enables five blocker rules:
CREATE existence, DROP existence, ALTER existence, ADD COLUMN existence, and
CREATE INDEX column existence (required=true). Other rules, including notices,
are disabled using the actual CLI catalog, without changing default policy.

Eighteen cases extend the existing 167-case T05 manifest: eight four-anchor
positive paths, four confirmed-absent policy/driver comparisons, four offline
unknown DROP cases, and two old-column negative cases. Driver execute steps
reuse `verify` immediately after DROP and before recreation. Final checks pin
id/c, absence of old_c, PRIMARY(id), idx_c(c), and cleanup. The validator pins
the A3 required oracle independently of the manifest, rejects paired removal
of the intermediate verification, and rechecks raw CLI and driver evidence.

## Deferred Scope

No multi-target precise DROP, full multi-target existence governance, DROP VIEW,
temporary-table lifecycle, permission or FK execution-precondition governance,
new object namespace, dependency graph, reference/constraint rewriting, grants
or trigger reconstruction, column mutation families, USE, or stored bodies.
A1/A2 remain accepted; this slice does not complete #84 or the milestone.

## References

- Parent contract: https://github.com/Fanduzi/DeltaScope/issues/79
- Ordered state: https://github.com/Fanduzi/DeltaScope/issues/84
- MySQL 5.7 DROP: https://docs.oracle.com/cd/E17952_01/mysql-5.7-en/drop-table.html
- MySQL 8.0 DROP: https://dev.mysql.com/doc/refman/8.0/en/drop-table.html
- MySQL 8.4 DROP: https://dev.mysql.com/doc/refman/8.4/en/drop-table.html
- TiDB 8.5 DROP: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-drop-table.md
