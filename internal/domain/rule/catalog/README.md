# Rule Catalog Module

Explanation-oriented metadata for shipped DeltaScope rules, with discoverability fields for CLI rule listing and explanation.

## Files

| File | Responsibility |
|------|---------------|
| catalog.go | Builds stable catalog entries from shipped defaults, explanation templates, and discoverability metadata for CLI discovery |
| catalog_test.go | Verifies catalog completeness, lookup stability, and metadata-aware flags |
| catalog_discoverability_test.go | Verifies rule discoverability contract: completeness, field validity, deterministic ordering, dialect/category coverage, drift prevention |

## Exports

- `MetadataKind`
- `MetadataNotes`
- `Entry`
- `All()`
- `Lookup(ruleID)`
- `Search(query)`

## Entry Fields

| Field | Required | Description |
|-------|----------|-------------|
| RuleID | Yes | Stable rule identifier used in findings and config |
| ConfigKey | Yes | Config key controlling the rule (equals RuleID) |
| Summary | Yes | One-line human-readable description |
| Description | Yes | Detailed description with scope and metadata mode |
| StatementKinds | Yes | SQL statement kinds: `ddl` or `dml` |
| DefaultEnabled | Yes | Default enabled state from shipped policy |
| DefaultLevel | Yes | One of `blocker`, `warning`, `notice` |
| DefaultParams | Yes | Default parameter map from shipped policy |
| MetadataAware | Yes | Whether the rule uses live metadata |
| TriggerExample | Yes | SQL that triggers the rule |
| ValidExample | Yes | SQL that passes the rule |
| ConfigExample | Yes | YAML config snippet for the rule |
| RemediationHint | Yes | How to fix the finding |
| Why | Yes | Why the rule exists |
| Risk | Yes | What can go wrong |
| Suggestion | Yes | Recommended action |
| ConfigHints | Yes | Config keys controlling the rule |
| MetadataNotes | No | How metadata affects this rule's explanation |
| Dialects | Yes | Dialect scopes: `common`, `mysql`, `tidb`, `postgresql` |
| Category | Yes | Stable grouping (e.g., `table`, `alter_table`, `index`, `dml_safety`) |
| Tags | Yes | Searchable tags: kind + dialect + category + suffix pattern |
| Source | Yes | Provenance: always `policy` for current entries |

## Derivation Strategy

Catalog entries derive from the shipped default policy (`domainpolicy.Default()`):
- **Enabled/Level/Params**: read directly from default policy
- **Dialects**: derived from rule ID prefix (`ddl.pg.*` → `postgresql`, `ddl.tidb.*` → `tidb`, `.merge.mysql.` → `mysql`, `.merge.tidb.` → `tidb`, else → `common`)
- **Category**: derived from rule ID segments (e.g., `ddl.table.*` → `table`, `ddl.alter.*` → `alter_table`, `dml.*` → `dml_safety`)
- **Tags**: synthesized from kind + dialect + category + action suffix
- **Source**: always `policy` since all entries derive from default policy
- **Explanation fields** (Why/Risk/Suggestion/Summary): generated from rule ID pattern heuristics

No supplemental hand-maintained metadata is used. All enrichment derives from the rule ID and default policy, preventing drift.

## Dependencies
- Upstream: `internal/interfaces/cli`, `internal/interfaces/http`, `internal/interfaces/mcp`, future documentation tooling
- Downstream: `internal/domain/policy`, `internal/domain/rule`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
- If a new rule ID pattern is added that `dialectsForRule` or `categoryForRule` does not cover, add a mapping and the `TestCategoryForRuleCoversAllRules` drift test will catch the gap.
