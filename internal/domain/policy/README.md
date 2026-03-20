# Domain Policy Module

Policy model for rule configuration and future audit settings.

## Files

| File | Responsibility |
|------|---------------|
| policy.go | Defines policy and per-rule configuration |
| defaults.go | Defines the built-in rule policy, including expanded DDL batches for columns, primary-key semantics, indexes, alter restrictions, first semantic alter rules, table options/object shape, and the Tier-1 DML rule set |
| policy_test.go | Verifies flexible per-rule parameter modeling |

## Exports

- `RulePolicy`
- `Policy`
- `Default()`

## Dependencies
- Upstream: application policy loading and future config adapters
- Downstream: `internal/domain/rule`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
