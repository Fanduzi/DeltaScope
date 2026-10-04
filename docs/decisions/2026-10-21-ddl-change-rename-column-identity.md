# Decision: Bounded CHANGE and RENAME COLUMN identity migration

Date: 2026-10-21
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79, T05-A5 under #84)
Related decisions: `2026-10-04-ddl-modify-column-state.md`, `2026-09-23-ddl-incomplete-coverage-contract.md`, `2026-09-28-ddl-target-version-evidence-contract.md`
Related tests: `internal/application/audit/batch_state_a5_test.go`, `internal/application/audit/batch_state_a5_matrix_test.go`, `internal/domain/spec/version_rename_column_test.go`, `internal/domain/rule/ddl/column_identity_rules.go`, `testdata/ddl-golden/T05.json`, `scripts/test_ddl_golden.py`
Related docs: `docs/reference/rules.md`, `docs/reference/config.md`, `configs/deltascope.example.yaml`

## Context

The existing CHANGE and RENAME source-existence rules inspect `Alter.Name`, which is the old column name. A later statement therefore cannot tell a free destination name from a name that already belongs to another column. `CHANGE COLUMN` carries a full new definition. `RENAME COLUMN` carries only the old name and `Definition.Name`. MySQL added `RENAME COLUMN` in 8.0.3. That introduction is separate from the 8.0.28 INSTANT algorithm note. Ordered state previously invalidated the whole table after either action, so a following `MODIFY` or `CREATE INDEX` could not use the new name.

## Decision

Three new default blockers sit beside the existing source-existence rules. Their defaults do not change the level, parameters, or enablement of any existing rule.

| Rule | Responsibility |
|---|---|
| `ddl.alter.change_column.target.exists.forbid` | The CHANGE destination must not be another column |
| `ddl.alter.rename_column.target.exists.forbid` | The RENAME destination must not be another column |
| `ddl.alter.rename_column.version.require` | `RENAME COLUMN` must be legal for the resolved version (`required: true`) |

The two destination rules share one conflict check. One blocker is emitted when the new name is another column. The same identity (`old == new`, compared case-insensitively) is not a conflict with itself and does not manufacture a destination-conflict finding or gap. An unknown column set keeps the existing `unknown_table_state` gap when the new name is a different column. A confirmed-absent table adds no destination-column finding or gap. Disabling either rule removes only that rule's finding or gap. Source existence, compatibility, and the RENAME version check stay on their own contracts. The state machine still refuses to publish a known duplicate column name.

`CHANGE` reuses the ordinary MODIFY definition replacement, including primary-key `NOT NULL`, and still clears the A4 cardinality and storage counters. `RENAME` copies the source column and changes only its name. A partial old definition stays partial. RENAME does not require a non-empty type and does not invent type or length. Ordinary primary-key, secondary, and unique indexes rewrite column references only. Index names and key order stay. Both the `PrimaryKey` field and a `primary_key` constraint member are updated. Foreign keys, CHECK constraints, generated columns, and prefix, expression, or other special indexes that cannot be recomputed invalidate the known-complete target together with the loaded dependent. There is no catalog scan and no string replacement across every constraint.

| Premise | Post-state |
|---|---|
| Table present, old column present, new name free, other premises hold | Migrate in place. The old name is absent |
| CHANGE, identity known, old type or length partial | Keep the current compatibility gap. A sufficient new declaration can establish later facts |
| RENAME, old definition partial | Move the identity and keep the unknown fields |
| Destination conflict, or source confirmed absent | Invalidate the target and related loaded dependents. Do not publish both names and do not simulate a rollback |
| Table or column set unknown, or earlier contamination | Do not invent a one-column table and do not reread the provider under the new name |
| Column set withheld, and indexes, constraints, or the primary key are loaded and empty (`nil` or an empty slice, with the matching Unknown flag false) | Keep that known absence. Do not mark the empty collection unknown |
| Column set withheld, and CHANGE declares a primary key, AUTO_INCREMENT, or another extra member the ordinary template rejects | Tombstone before retaining the partial snapshot. Do not keep a false "no primary key" |
| Destination rule, old and new names are the same identity | Zero findings and zero gaps from that rule |
| Only a policy blocker, structure premises hold | Publish the conditional successor |
| `old == new` | CHANGE replaces the definition. RENAME leaves attributes unchanged and still checks source existence and version |

`RENAME COLUMN` applicability is one pure function, `RenameColumnVersionSupportFor`, shared by the rule and the state machine. The request stores the already-resolved version on the request-local batch state. The state machine does not assume `statement.Metadata.Version` is populated on the original statement it receives. Online identity stays the observed banner. A provider that returns empty instance facts leaves the resolved version empty even when the request carries `target_version`. The offline nil-provider path uses the request target. PostgreSQL enrichment is unchanged. No new public JSON field is added.

| Version fact | Check | Precise post-state |
|---|---|---|
| MySQL 8.0.3+ inside 8.0, MySQL 8.4, TiDB 8.5 | Passes | Publish only when the other premises hold |
| MySQL 5.7, or MySQL 8.0.0 through 8.0.2 | One incompatibility blocker | Do not publish |
| Version missing | `missing_target_version` gap, fact `target.version` | Do not publish |
| Legal but outside the validated series | `target_version_out_of_validated_range` gap, fact `target.version.validated_range` | Do not publish |

A known incompatible RENAME version is a determined negative result of this implemented applicability check. The statement coverage stays `complete` and the verdict is `reject`. The check does not forge a parser error. Semantics the tool still does not understand stay on the #82 incomplete contract and do not become a closeable finding. A missing version stays on the #83 unverified contract and does not add a finding. `CHANGE` does not inherit this version gap. Setting `required: false` or disabling the version rule removes that rule's finding and gap. It does not relax the state gate. The incompatible finding metadata is `action: rename_column`, `product`, the canonical `target_version`, and `minimum_supported_version: 8.0.3`. The raw banner is not copied.

The golden isolation profile `t05-a5-column-identity-isolated` enables the previous eight ordered-state blockers plus these three rules, eleven blockers in total. Every other catalog rule, including action forbids and notices, stays off in that profile only.

## Rationale

Checking only the old name would report a missing source and still treat a colliding destination as free. Sharing one conflict implementation keeps CHANGE and RENAME on the same name rule while their payloads stay different: CHANGE has a definition to replace, and RENAME does not. Publishing a duplicate name when the conflict rule is disabled would make policy and state disagree. Treating 8.0.0 as "8.0 supports RENAME" would accept three patch releases that predate the syntax. Recording that incompatibility as incomplete coverage or as a parser failure would hide a result the tool has already decided.

## Public Contract

Consumers can rely on the three rule IDs, their default blocker level, and `required: true` for the version rule. Destination-conflict metadata carries `table`, `action`, `source_column`, `target_column`, and `exists: true`. It does not carry the column definition, the SQL text, or credentials. A supported rename publishes the new name in place and makes the old name absent for later statements in the same MySQL or TiDB request. An unsupported or unproven rename does not publish that shape. CLI, HTTP, MCP, and the SDK use the same request-local state. PostgreSQL keeps its previous per-statement contract.

## Deferred / Out Of Scope

This slice does not complete every CHANGE form. DROP COLUMN precise post-state, cross-schema column move, multi-action success, foreign-key rewrite, catalog scan, prefix and expression index rewriting, runtime limits, and storage engines stay unimplemented. Cross-family conversion, decimal precision, temporal implicit defaults, and generated or identity columns stay outside the ordinary template. No 8.0.2 or 8.0.3 database is added. The 8.0.3 boundary is the release note plus the pure function and real `target_version` audits. This slice does not complete #84 or the milestone. A1 through A4 stay accepted.

## Verification Evidence

Application tests pin the four-statement CHANGE path, the destination conflict, source absence, `old == new`, partial and nil column sets, ordinary index and primary-key rebinding, unsafe dependents, version matrix, disabled and `required: false` gates, policy-only publication, request isolation, cancellation, and provider errors. The T05-A5-R1 regressions pin three corrections on that same contract: a known-empty nil collection agrees with an empty slice, a declaration-level extra member invalidates before the partial snapshot is kept, and the same identity emits no destination gap. The PostgreSQL build tag keeps the three legacy request shapes. Representative SDK, CLI, HTTP, and MCP checks cover success, reject, gap, and error. Corpus fixtures cover the destination rules on MySQL and TiDB. The version finding is MySQL-only because TiDB 8.5 accepts the syntax. The T05 manifest keeps its previous 204 cases and adds 27 under `t05-a5-column-identity-isolated` (231 total). The four fixed anchors remain MySQL 5.7.44, 8.0.46, 8.4.10, and TiDB 8.5.0. Their raw logs live with this slice's evidence commit.

## Consequences

Later DROP COLUMN or multi-action work must tombstone identities it cannot recompute, including a CHANGE or RENAME mixed with another action. A new product series needs an explicit validated-range decision before RENAME may publish there. Closing the version rule in a user policy does not authorize the state machine to publish the rename.

## Links

- Parent contract: https://github.com/Fanduzi/DeltaScope/issues/79
- Ordered state: https://github.com/Fanduzi/DeltaScope/issues/84
- MySQL 8.0.3 release notes: https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-3.html
- WL#10761: https://dev.mysql.com/worklog/task/?id=10761
- MySQL 5.7 ALTER TABLE: https://docs.oracle.com/cd/E17952_01/mysql-5.7-en/alter-table.html
- MySQL 8.4 ALTER TABLE: https://dev.mysql.com/doc/refman/8.4/en/alter-table.html
- TiDB release-8.5 ALTER TABLE: https://raw.githubusercontent.com/pingcap/docs/release-8.5/sql-statements/sql-statement-alter-table.md
- Predecessor: `docs/decisions/2026-10-04-ddl-modify-column-state.md`
