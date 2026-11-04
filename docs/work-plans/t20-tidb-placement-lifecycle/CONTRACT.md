# T20 — complete placement bindings and partition members

Issue [#99](https://github.com/Fanduzi/DeltaScope/issues/99), parent
[#79](https://github.com/Fanduzi/DeltaScope/issues/79).
This contract fixes the B1/B2 acceptance failures at product
`57476dc2ea0078e7b07efc988c3505d93702aba3`; its evidence parent is
`6ac1e918b3558d6d611d7d8a59a770d73341c2f9`.
It defines expected behavior, not an acceptance or release claim.

## Structured coverage

Every authored assignment participates in a single structural collection:
database/table/RANGE direct bindings, CREATE table and partition definitions,
each ALTER direct assignment, each ALTER member definition, and partition
scheme definitions. DEFAULT is an assignment with its own position.
The collection preserves schema, table, member, action and definition indices.
Repeated references to one policy do not collapse distinct bound members.
Existence lookups may be cached. SQL text is never rescanned to guess bindings.

TiDB 8.5 is the placement execution anchor. MySQL's bare TiDB syntax remains
a vendor boundary; MySQL feature comments remain ignored under the established
contract. No TiDB subpartition implementation is added.

## Public decisions

`ddl.tidb.placement.exists.require` checks every named reference. A proven
missing policy produces its configured finding regardless of its position or
the warning rule's switch. An unknown reference is not a missing reference.
Any sufficient static refusal dominates an unrelated unknown premise.

For an atomic CREATE/ADD/REORGANIZE, a proven refusal suppresses placement
binding-change warnings and irrelevant topology gaps, while unrelated rules
retain their own findings. All applicable static premises must hold before
the prospective binding change can be published. Policy existence and
topology satisfaction remain separate facts.

An applicable named binding produces one conditional risk warning per
authored binding occurrence, carrying bounded object identity. A statement
with several members can therefore have several warnings, including repeated
policy names on different members. A missing topology fact produces one
deduplicated `placement_policy.topology` gap for the warning rule. Unproven
catalog premises instead produce `placement_policy.catalog` gaps on each
enabled applicable placement rule. DEFAULT alone needs neither policy
existence nor topology evidence.

Source locations use the source spans actually available to the product.
Action/member identity supplements statement locations; invented member
offsets and raw region/constraint values are prohibited. Disabled rules emit
no findings or gaps of their own and never change prospective state.
Provider failures, cancellation and deadlines remain errors.

## Partition transitions and references

T14 continues to own method, member-name, boundary and covered-span judgments.
Placement does not bypass these checks. A refused ADD/REORGANIZE preserves
the exact prior directory, definitions, explicit bindings, reference proofs
and statistics. No successful prefix is published.

On ADD, new definitions and explicit overrides are appended while old members
and table binding remain. On REORGANIZE, only source members and their
references retire; replacements receive their own declarations. An omitted
or DEFAULT assignment establishes no explicit member override. It does not
copy the retired member's override. Effective table-to-partition inheritance
continues through the unchanged table binding.

An applied PARTITION BY replaces the directory and all old member references
for that table. Its scheme definitions use the same collected assignments.
The fixed TiDB 8.5.0 control proves named/DEFAULT replacements, retirement of
the old policy's references, and 8239 refusal preserving the prior scheme.

An undecided operation withholds the affected directory and new bindings.
Only potentially changed member/reference proofs lose certainty; unrelated
table facts and unaffected member bindings remain. A known positive reference
is sufficient to refuse DROP. An incomplete empty reference set cannot
authorize DROP. Completeness, emptiness and positive membership are distinct.
Schema/table/member identity prevents equal partition names on different
objects from sharing a reference.

When CREATE or a member operation might add a binding, every candidate
policy's reference set loses exhaustive certainty, including a known policy
beside an unknown one. No new positive reference is inserted. Known target
identity remains usable for retiring a possibly replaced reference when the
table snapshot itself is unavailable. Proven member retirement follows the
directory change even when the explicit binding map was unread.

The global policy-catalog completeness model remains conservative: an
unvouched catalog is not projected as a complete partial map. Current-target
proofs can preserve independently established existence or reference facts
without treating untouched keys as absent. OR REPLACE never turns an unknown
reference set into a proven empty set. Snapshots and provider inputs are owned
copies.

ALTER DATABASE's previously verified charset-before-placement partial
application remains separate from atomic table/partition operations.
T19 TTL semantics are unchanged.

## Subpartition boundary

The official release-8.5 compatibility list excludes SUBPARTITION. The fixed
8.5.0 native probe accepts the tested CREATE forms with Warning 8200:
`Unsupported subpartitioning, only using RANGE partitioning`.
It discards child definitions and child policy references, retaining the
primary RANGE definition and independently valid table binding.

The product must retain a structural capability marker and report incomplete
analysis with the bounded `tidb_subpartition` vendor aspect. It does not set
whole-statement unsupported status that would suppress sibling rules. This
is not a parser error or an ordinary missing-evidence gap. No ignored child
policy produces a missing-policy blocker or reference. Execution-state
normalization uses an owned declaration copy only for CREATE TABLE with
primary RANGE at TiDB 8.5.0. Supported parent/sibling facts remain analyzable.
This observation is not generalized to MySQL or unverified TiDB versions.

## Evidence and gates

The reconstructed permanent red tests use public AuditSQL plus real
parse/extract/enrich companions. They are not the deleted reviewer's logs.
The finite matrix includes position, DEFAULT, repeated policy, known missing,
unknown-before-missing, rule switches, custom levels, current findings and
successor directory/binding/reference facts.

`testdata/ddl-golden/T20.json` retains the original 25 case definitions and
adds seven metadata cases and three offline controls. Native driver queries
prove database behavior; they do not stand in for private product projections.
The latter are asserted by companions. Assertion totals are measured from the
completed artifact, never inferred from test counts.

The new native work concerns legal SQL, catalog atomicity, explicit bindings,
reference changes and the subpartition boundary. It uses TiDB 8.5.0 unistore
only for those facts. Earlier PD/TiKV receipts remain the separate source for
real topology behavior; no new scheduling claim is made.

Validation includes affected T14/T19/T20 and parser/spec/rule tests, final
test/vet/internal race/affected PG gates, SQL and Query Access corpus,
inventory/catalog, Golden validator, parser/unsupported, privacy, docs/ADR,
nonempty committed-diff L2 and unchanged-algorithm lint identity comparison.
Command, HEAD, stdout, stderr and return code are captured from execution.

Primary sources, checked 2026-11-04:

- [Placement rules, release-8.5](https://github.com/pingcap/docs/blob/release-8.5/placement-rules-in-sql.md)
- [MySQL compatibility, release-8.5](https://github.com/pingcap/docs/blob/release-8.5/mysql-compatibility.md)
- [Partitioned tables, release-8.5](https://github.com/pingcap/docs/blob/release-8.5/partitioned-table.md)
