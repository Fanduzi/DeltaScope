# Decision: PostgreSQL column storage/compression and observed system targets

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D7)
Baseline: `90a68a8b` historical D6 closeout; shipped D6 code `fa37a41b`

## Context

At 90a, 42 native PostgreSQL 17.7 storage/compression shapes and 84 real offline/
metadata-aware CLI audits found 20 native successes, 15 0A000, five 22023 and
two 42703 refusals. All audits passed with generic notices, including invalid
modes/methods, inapplicable types and absent columns. A five-shape/ten-CLI
system-target follow-up found five 0A000 refusals; guarded DROP ctid incorrectly
passed as a no-op, while other existing checks incorrectly claimed absence.

The directory queries positive pg_attribute.attnum values only. Its complete
**user** member set is not proof that a system attribute is absent. Compression
AST method payload exists but is discarded; storage_kind is already retained.
Actual type typstorage, not an integer width/type-name guess or the column's
current attstorage, establishes TOAST readiness. Compression default is legal
on fixed-length types; a toastable text column with STORAGE PLAIN can still
receive a nondefault compression declaration.

The operator explicitly approved two default blockers and preserving actual
system facts, separately from the existing user-column directory. This is not
authorization for parser expansion, privilege/dependency/expression/data
analysis or a PostgreSQL prospective-state model.

## Decision

Use the existing storage_kind AST projection; add a private authored compression
method without SQL-text recovery. Keep method case ownership: STORAGE follows
PG17 case-insensitive comparison; COMPRESSION names are case-sensitive.

Extend private column facts with observed pg_type.typstorage and a system
marker from **negative attnum**, with system members in a separate directory.
Preserve the existing user Columns nil/known-empty contract, including real
zero-user-column tables. Never synthesize a member/type/nullability fact from
its name. The six exact PG17 reserved names only constrain the proof domain
when system observations are missing; they never create members or absence. Missing, ambiguous or stale system facts cannot establish absence or
a guarded no-op. Exact lower-case system names and quoted upper-case user
names must stay distinct. Clone both directories without aliasing nested facts.

Read the lz4 capability from the enumvals of default_toast_compression in the
**existing pg_settings query**. Explicit known/supported bits distinguish a
complete false observation from no observation. No pg_config read is added:
ordinary-reader probes can read enumvals, while pg_config is denied 42501.
The upstream enum itself is conditionally built under USE_LZ4, unlike a current
setting of pglz, which does not prove lz4 unavailable.

## Public Contract

Enabled by default, parameterless, observed PG17:

| Rule | Level | Bounded predicate |
|---|---|---|
| `ddl.pg.alter.column.storage.valid` | blocker | Proven invalid storage mode, absent user column, system-target inapplicability, or non-PLAIN strategy on a proven non-TOAST-aware type |
| `ddl.pg.alter.column.compression.valid` | blocker | Proven invalid case-sensitive method, absent user column, system-target inapplicability, nondefault method on a non-TOAST-aware type, or lz4 unavailable in a complete observed enum |

PLAIN and DEFAULT are valid storage modes; DEFAULT uses the actual type's
strategy. Pglz/lz4/default have distinct compression semantics: **default does
not require TOAST readiness or lz4 support**, but it still requires an applicable
column. Current STORAGE PLAIN does not make a toastable type inapplicable.
Unknown/unmodeled/missing/stale/misbound/ambiguous facts remain attributable
unverified/review gaps, not guessed blockers. Intrinsic declaration refusals
require the observed version; target/type/build conclusions need their real
premises. The ordinary persistent non-inheritance/non-partition table stage
is retained. Related preceding DDL/parser failures and earlier ALTER actions
withhold old directories without simulating a post-state.

Existing D5 generation and D6 statistics/options/DROP target checks use the
same observed system classification: a system member is not absent and IF
EXISTS does not make it a no-op. Retain their IDs and approved levels; use a
bounded inapplicable-system reason when proved, otherwise a gap. The deletion
advisory shares that assessment and must not imply a proved non-executable
system deletion. Independent forbids and MySQL/TiDB behavior remain unchanged.

New finding fields contain only bounded action/identifiers/reason, not arbitrary
method strings, configuration text, expressions, account/connection values or
provider errors. Private authored/catalog/build facts do not expand audit JSON
envelopes. SET STORAGE/COMPRESSION does not itself rewrite existing rows;
declaration validity is not a compression-ratio, data-recompression, dependency,
privilege, duration or successful-execution guarantee.

## Verification And Evidence

Historical clean-baseline study (not D7 implementation/final evidence):

- `/tmp/ds-pg-d7-storage-scope-90a68a8b/evidence.json`: 42 native / 84 CLI.
- `/tmp/ds-pg-d7-system-scope-90a68a8b/evidence.json`: five native / ten CLI.
- `/tmp/ds-pg-d7-ast.log`: exact AST payloads and current compression collapse.
- Each audit preserved catalog, row fingerprints, relation file identities and
  guard position; driver-only DDL was transactional and rollback-checked.
  Owned namespaces/reader role were proved absent before creation and removed
  afterward; unrelated services were preserved.

The first facts-only candidate (90a + dirty source, not a final D7 HEAD) passed
build, default and PG-tagged tests, inventory/corpus/coverage contracts, both
lint lanes and format/diff checks. The unchanged old T37 passed **136/136,
2,186 assertions** at `/tmp/ddl-golden-pg-column-format-facts-90a68a8b/T37/artifact.json`.
Tests cover exact/default/case AST method facts and JSON exclusion; actual
signed-attnum separation and zero-user-column preservation; complete/false/
unobserved/duplicate/error enum arms; SDK/application catalog clone ownership.
This is model/provider compatibility evidence, **not** a system-target rule or
D7 validity delivery; the existing system-target verdicts still need the next
shared-rule commit and final same-HEAD canonical evidence.

The shared-rule candidate (4f157146 + dirty source, not final) replays **56
native / 112 CLI** shapes: 20 successes, 27 0A000, seven 22023 and two 42703.
Every metadata-aware success/refusal matches native and the original 42 native
SQLSTATEs are unchanged. Evidence at
`/tmp/ds-pg-d7-rules-scope-4f157146/evidence.json` records base/dirty status,
tracked-diff, new-rule-source and binary SHA256. It is not a clean-HEAD canonical
substitute. Related system operations are not all 0A000: ADD IDENTITY on ctid
reaches the integer-type refusal **22023**; the inapplicable-system finding must
not be mistaken for a copied native SQLSTATE or general execution simulation.
The Unicode lookalike `"PLAİN"` also rejects 22023 and is not canonicalized to
PLAIN by Unicode lowercase. System guard/no-op and deletion/clamp-risk tests
were red before the shared correction and green afterward.

The final shared-rule candidate v2 (still 4f157146 + dirty source, not delivery)
passes build, full default/PG-tagged tests, inventory/corpus/coverage contracts,
both lint lanes and format/diff checks. Original T37 remains byte-for-byte
unchanged at **136/136, 2,186 assertions** in
`/tmp/ddl-golden-pg-column-format-rules-v2-4f157146/T37/artifact.json`.
`/tmp/ds-pg-d7-rules-scope-v2-4f157146/evidence.json` repeats all 56 native/112
CLI shapes with identical SQLSTATEs and 56/56 live success/refusal matches at
the candidate's new binary/source hashes. Application/SDK/HTTP/MCP tests pin
shared findings/default controls and unverified build gaps without private
method metadata; the application also pins prior-DDL/parser/sibling invalidation.
A separate red/green test confirms unknown authored method/strategy facts do
not hide an independently proved absent or inapplicable system target.
The first lint retry corrected only an import group; no check was disabled.
Canonical additions, exact inventory binding and final all-task proof remain
for the next evidence task, not implied by these compatibility gates.

The candidate's first default/PG full suites failed only for the two missing
new corpus rule targets; the missing fixtures are added rather than excluding
the new rules or weakening the coverage gate. Seven additive corpus pairs cover
invalid/type/control/system/build-unavailable/unknown and non-PG cases. A native
without-lz4 binary has not been run: negative-build unit/corpus facts and the
upstream USE_LZ4 enum gate are distinct from the actual anchor's positive lz4
read. This limitation does not convert an unobserved build into proved false.

As verified 2026-11-18, the project remains pg_query_go/v6.2.2/libpg_query17 with
a pinned native PG17.7 anchor. The official current stable release is 18.7 and
current 17 minor is 17.12. Neither is substituted into the parser/fixture or
claimed supported by memory; no dependency/image upgrade is part of D7.

Freeze these **20 additive T37 IDs before implementation** (136 -> 156), retaining
all existing IDs/cases/profiles:

- `T37.cli.column-storage-offline`
- `T37.cli.column-compression-offline`
- `T37.cli.column-format-default-offline`
- `T37.cli.column-format-system-offline`
- `T37.cli.column-format-storage-case-offline`
- `T37.cli.column-format-compression-case-offline`
- `T37.cli.column-format-disabled`
- `T37.cli.column-format-mysql`
- `T37.cli.column-format-tidb`
- `T37.cli.column-format-parser-retention`
- `T37.meta.pg-column-storage-validity`
- `T37.meta.pg-column-compression-validity`
- `T37.meta.pg-column-format-system-targets`
- `T37.meta.pg-column-format-system-empty`
- `T37.meta.pg-column-format-quoted-case`
- `T37.meta.pg-column-format-prior-ddl`
- `T37.meta.pg-column-format-prior-parser`
- `T37.meta.pg-column-format-scope`
- `T37.meta.pg-column-format-no-rewrite`
- `T37.meta.pg-column-format-enum-reader`

Evidence proceeds through model/parser/provider/clone tests, shared rule and
cross-surface/corpus controls, owned native catalog/refusal/no-execution/cleanup
oracles, exact original inventory selectors and the matrix generator. Final
same-HEAD all-task gates/validate, independent read-only review, FF-only merge,
normal push and exact-SHA CI follow. Facts or baseline notices alone do not
complete the rule slice, original row or F09.

## Rationale And Remaining Scope

Separating system observations avoids breaking known-empty user directories
and fixes false absence at its common fact boundary. Type storage strategy and
current column strategy are independent; using the latter would reject legal
compression. Public enum observation proves build capability without adding a
privileged configuration read or guessing from the selected default.

All original denominators remain. Broader table kinds, table options/access
method/tablespace/logging/RID, rename/schema/owner and other F09 dimensions,
permissions/dependencies and general sequential state continue in the queue,
not permanent deferrals or whole-family completion by association.

## Sources

- [PG17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17 pg_type and typstorage](https://www.postgresql.org/docs/17/catalog-pg-type.html)
- [PG17.7 GetAttributeStorage/GetAttributeCompression](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PG17.7 compression name/build mapping](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/access/common/toast_compression.c)
- [PG17.7 USE_LZ4-gated enum](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/misc/guc_tables.c)
- [PG17 pg_config access boundary](https://www.postgresql.org/docs/17/view-pg-config.html)
- [Current official version table](https://www.postgresql.org/support/versioning/)
- [D6 target/freshness/private-value contract](2026-11-18-postgresql-column-tuning-and-drop-guards.md)
