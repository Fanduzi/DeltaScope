# T17-R1-tail: schema default-collation review candidate

Final product: `c6eebc09b64a4c9ffc8d1dc73689f505da06d61c`.
Implementation parent: `84762d9d6c9085dbe91307516e14a984efa76375`.
Task/evidence baseline: `9da9f8e02221b1e51bf275cf012ffc2ac88b594a`.
Original R1 product: `83125653d7cfc4343221d6f47e89df4311101efe`.

This is a narrow independent-review candidate, not T17 acceptance, a release,
or T18. The default-collation error existed in original T17; R1 did not
introduce it. B1/B2/B3, the shared PostgreSQL error correction, touched-unknown
table guards, and proven-present duplicate CREATE preservation remain closed.
No original evidence bundle or published commit is rewritten.
The final product includes a test-only bytes.Equal follow-up: the first
lint pass identified one new stringXbytes diagnostic, which was fixed
rather than accepted into the inherited baseline. Initial receipts remain
under gates-pre-lint and lint-fix, with their actual execution identities.

## Exact change

CREATE and ALTER share the same `schemaPostAttrs` derivation:

- Explicit COLLATE remains authored evidence and is never overwritten.
- A validated MySQL 5.7 identity derives utf8mb4_general_ci, not 0900.
- Other MySQL versions require an observed default_collation_for_utf8mb4
  value; knowing 8.0/8.4 alone does not prove a configurable default.
- Validated TiDB 8.5 retains utf8mb4_bin. Unproven/future TiDB identities
  without a sufficient premise withhold this derivation.
- Missing collation removes only that attribute; charset, unrelated
  attributes, proven existence, and ownership retain their proper state.
- Bare CREATE uses individually observed character_set_server and
  collation_server, not a version-to-default-configuration guess.
  Legacy DefaultCharset (MySQL character_set_database) is not substituted.

The provider adds those three variables to its existing single read.
Three additive string InstanceFacts fields carry the observations; empty
means unknown. SDK aliases and existing cloning preserve them. No complete
charset engine, collation comparator, or table conversion feature is added.
Other charset mappings remain the pre-existing bounded table.

## Genuine baseline red

`red/identity.txt` records execution at the unmodified R1 product
`83125653...` in a detached worktree, tracked-clean except the supplemental
permanent test. `red/proof.*` is rc1 from exactly the wrong collation,
not compilation or fixture failures.

| Projection test | Provider premise | SQL first statement | Baseline second-statement pre-state | Fixed pre-state |
|---|---|---|---|---|
| CREATE | g explicitly absent | CREATE DATABASE g CHARACTER SET utf8mb4 | present, utf8mb4 / utf8mb4_0900_ai_ci | present, utf8mb4 / utf8mb4_general_ci |
| ALTER | g present, latin1 / latin1_swedish_ci | ALTER DATABASE g CHARACTER SET utf8mb4 | present, utf8mb4 / utf8mb4_0900_ai_ci | present, utf8mb4 / utf8mb4_general_ci |

Both chains end with ALTER DATABASE g COLLATE utf8mb4_bin. Tests prove the
second projection stays general_ci even after that final transition, the
first projection and original provider snapshot stay unchanged, and the
final entry is bin with correct ownership. VersionIdentity comes through
parseAuditTargetVersion, including canonical product/version/source/range;
it is not an incomplete hand-built version.

`T17TAIL_PROOF` JSON logs are internal parse/extract/enrich evidence.
Separate `T17TAIL_PUBLIC_CONTROL` records use AuditSQL and observed instance
version parsing. Public output intentionally omits internal SchemaTarget:
those controls prove the public path and do not claim to observe collation.

The permanent matrix covers 5.7.44; observed 8.0.46/8.4.10 defaults; a
configured-general 8.0 case; missing version/configuration; future series;
foreign-product and invalid observations; TiDB 8.5.0; explicit COLLATE;
known-present INE; observed and partial bare-CREATE defaults; snapshot/fact
immutability; and enabled/disabled real registries. Policy checks evaluate
the actual enriched statements, verify loaded-rule counts differ, and show
neither registry mutates metadata.

## Native correspondence

The 5.7 primary source is the existing committed
`docs/work-plans/t17-database-schema-lifecycle/probes/p17_schema_matrix.log`,
`mysql57 :: charset-lifecycle-steps` (lines 8-12):
successful CREATE CHARACTER SET utf8mb4 yields utf8mb4_general_ci.
The historical sys-mysql-alter result agrees; it is reused, not rerun as
an additional probe.

`native/defaults.sql` is a new bounded, lead-authored business-schema probe
on a dedicated MySQL 8.0.46 container. Its raw receipt proves:

- Default configuration makes both CREATE and ALTER yield 0900.
- Setting default_collation_for_utf8mb4 to general_ci changes both.
- Observed server charset=latin1 / collation=latin1_bin makes bare CREATE
  inherit that pair, despite the same 8.0.46 version.

No system schema appears in this additional SQL. These observations are
frozen receipts; presentation does not rerun the derivation.

## Golden and gate identity

The original T17 manifest is byte-identical to `9da9f8e0`: all 52 existing
IDs, SQL, and oracles are preserved. Net new CLI/database cases: zero.
The new companion projection/control tests run in the canonical T17
Make target, before the real CLI/database runner, and are not added to
the CLI artifact's assertion count.

Final command: `make ddl-golden TASK=T17 ARTIFACT_DIR=/tmp/ddl-golden`.
The artifact and CLI build bind to the full product SHA above. Expected
and verified denominator: 52/52, 834 CLI/database assertions. No previous
52/834 artifact is overwritten; its preserved location and hash manifests
are recorded.

Safety exception explicitly confirmed by the user: original Golden
system-schema cases may run only in its disposable fixtures. No historical
p17 script or additional system-schema experiment is authorized or run.
This distinguishes the required formal Golden from the extra business probe.

The first final-product Golden attempt returned rc2 because the worker
incorrectly used redaction markers as fixture passwords. Its 1045 failures
and artifact are preserved as environment-failure evidence, not counted as
behavioral red or a product defect. The corrected run uses the actual
disposable Compose credential; code, manifest, and oracles do not change.

`gates/INDEX.md` lists actual commands, identities, and outcomes. Full tests,
vet, internal race, canonical PostgreSQL-tagged unit gates, corpus, inventory,
catalog, validator, parser/unsupported, docs/ADR, committed-range L2, and
formal T17 Golden have raw command/stdout/stderr/rc tuples.

PG scope is deliberate: the MySQL-only observations are empty on PG;
canonical tagged unit tests cover parser, audit, auditmeta, CLI/HTTP/MCP,
and SDK shared types. PostgreSQL queries/error behavior do not change, so
Docker PG confidence and historical T09-T16 Golden suites are not
mechanically re-gated. Their committed evidence remains at the old SHAs.

Lint is not claimed green. Identity comparison with the current R1
`9da9f8e0` baseline checks file plus complete message/linter multiplicities:
22 scoped and 39 full-repository inherited findings, no added/removed
identities. Scoped formatting touches only changed Go files; the separate
T07 baseline is preserved.

## Packaging and preservation

Source snapshots use `.go.txt` to avoid adding invalid Go packages under
docs. The initial baseline source is separate from the expanded final
regression source; restore it only into the original package of a disposable
worktree when replaying red.

Product and evidence are separate appended commits. The evidence commit is
the commit containing this README, avoiding a self-referential SHA.
Final receipts preserve the 12 historical untracked paths, other worktrees
and containers, and protected remote milestone. Only task-owned probe,
Golden fixtures, and the detached red worktree are cleaned.

Unverified boundaries remain explicit: absent observed configuration cannot
prove MySQL 8 utf8mb4 or bare-CREATE defaults; future version derivations,
general charset registries, comparison semantics, and table conversions
are not added. T17 independent acceptance remains the Reviewer's decision.
