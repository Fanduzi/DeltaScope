# DeltaScope v0.514.0 Release Notes

Release date: 2026-11-16

## Summary

v0.514.0 brings PostgreSQL metadata-connected DML target-existence checks into parity with MySQL and TiDB, expands the cross-dialect DML regression corpus, and fixes a deprecated Homebrew cask directive.

DeltaScope remains static analysis: it does not execute submitted SQL, return query results, or decide authorization.

## PostgreSQL DML Target Existence (#145, T60)

- Metadata-connected PostgreSQL `INSERT`, `UPDATE`, and `DELETE` audits with one mutation target now emit a `dml.table.exists.require` **blocker** when metadata proves the target table is missing. Existing policy settings still control whether the rule is enabled and its finding level.
- A schema-qualified DML target uses its authored schema for metadata reads on every dialect. Unqualified PostgreSQL targets continue to use the request schema; this is not PostgreSQL `search_path` resolution.
- Offline PostgreSQL audits are unchanged: without a provider there is no existence fact, and this rule does not invent one. Unknown metadata is not proof of absence.
- This is an intentional audit-outcome change: affected audits can now return `reject` and trigger a caller's blocker fail threshold. The result schema and rule ID are unchanged.

## Evidence and Regression Coverage (T61–T62)

- Live PostgreSQL golden cases pin missing-target blockers and present-target controls, with post-verification that auditing did not mutate fixtures.
- MySQL `ALTER TABLESPACE ... ENGINE` metadata cases pin the existing/absent target distinction. This strengthens evidence for the already-shipped audited slice; it does not broaden parser support.
- New MySQL, TiDB, and PostgreSQL DML corpus fixtures cover mutation findings, clean controls, metadata-backed existence, and explicit unsupported/parser-error boundaries. Unsupported syntax and parser errors remain incomplete audits, not successful checks.
- The existing `TASK=all` aggregate at **`6a4f14e7`** contains **50 tasks, 3,361/3,361 cases, and 59,206 assertions**, all at one HEAD. This is the retained behavior-evidence baseline, not a claim that the aggregate was rerun at the release tag.

## Homebrew

The cask renderer now uses `postflight_steps` instead of deprecated `postflight`, retaining the same trust/quarantine guidance.

## Compatibility and Remaining Boundaries

No public API or output-shape breaking change. The new missing-target blocker can change verdicts and process exit status as described above. MySQL/TiDB DML behavior is unchanged. Five PostgreSQL parser-upgrade candidates remain pinned parser errors pending an upstream binding release (#149); generic notices and mixed-coverage inventory rows are not promoted by this release.

### Maintainer Notes

The bounded metadata contract and evidence rationale are recorded in [PostgreSQL DML table-existence parity](../decisions/2026-11-16-postgresql-dml-table-existence-parity.md) and [tablespace ENGINE metadata existence evidence](../decisions/2026-11-16-tablespace-engine-meta-existence-evidence.md).
