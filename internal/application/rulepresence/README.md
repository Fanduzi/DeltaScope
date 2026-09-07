# Application Rule Presence Module

Computes Catalog, Default Policy, Loaded, and Suppression for one rule ID.

## Files

| File | Responsibility |
|------|----------------|
| presence.go | `Of` registers the policy and names the four facts |
| presence_test.go | FK naming suppressed, where.require loaded, impact catalog-only |

## Exports

- `Presence` — `{ InCatalog, InDefaultPolicy, Loaded, SuppressionReason, SuppressionBy }`
- `Of(ruleID, policy) (Presence, error)`

## Dependencies

- Upstream: `internal/application/configstatus`
- Downstream: `internal/domain/policy`, `internal/domain/rule`, `internal/domain/rule/catalog`, `internal/domain/rule/ddl`, `internal/domain/rule/dml`

## Update Rule

- If members/interfaces/dependencies change, update this file in the same change.
