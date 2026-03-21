# Domain Policy Module

Policy model for rule configuration and future audit settings.

## Files

| File | Responsibility |
|------|---------------|
| policy.go | Defines policy and per-rule configuration |
| defaults.go | Defines the built-in rule policy, including create-table identifier governance, expanded column/type-family breadth rules, primary-key semantics, indexes, alter restrictions, shipped semantic alter rules, metadata-backed existence rules, object-lifecycle rules for view/drop/truncate, table options/object shape, and the Tier-1 DML rule set |
| policy_test.go | Verifies flexible per-rule parameter modeling |

## Exports

- `RulePolicy`
- `Policy`
- `Default()`

## Notes

- The default alter policy keeps `ddl.alter.change_column.forbid` enabled as the stricter coarse guard.
- `ddl.alter.change_column.target_type_family.allowlist` remains enabled as a follow-on semantic guard for teams that intentionally relax the coarse forbid later.
- The default metadata-aware alter policy now also enables source-aware compatibility checks for `modify column` and `change column`; those rules only fire when the statement carries a live table snapshot.
- The default alter policy also enables explicit nullability/default/auto_increment change forbids; the `change_column` variants act as follow-on guards when the coarse `ddl.alter.change_column.forbid` gate is intentionally relaxed.
- The default policy also enables shipped alter-added index prefix checks for unique, secondary, and fulltext indexes.
- The default create-table policy now enables identifier pattern and reserved-keyword checks for table, column, and secondary-index names.
- The default create-table type-family policy keeps `timestamp` forbidden, keeps `char` length capped, and enforces charset/collation allowlists plus pair-coherence checks.
- Blob/text, json, and bit forbids are shipped in the default template but remain relaxed by default via `forbid: false` until a team intentionally tightens them.
- The default create-table index policy now also enables left-prefix and unique-overlap redundant-index findings on top of exact duplicate detection.
- The default create-table option policy now requires `ROW_FORMAT=DYNAMIC` when row format is specified and requires explicit `AUTO_INCREMENT` seeds to stay at `1`.
- The default metadata-backed DDL policy enables existence checks for create/alter table plus add/drop/rename column/index and drop-primary-key operations.
- Those existence rules advertise `requires_metadata: true` in the shipped policy/template so operators can see that offline runs will skip them when no live table snapshot is attached.
- The default lifecycle policy now also blocks `create view`, `drop table`, and `truncate table`, and ships adaptive-hash cautions plus metadata-backed existence checks for drop/truncate operations.

## Dependencies
- Upstream: application policy loading and future config adapters
- Downstream: `internal/domain/rule`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
