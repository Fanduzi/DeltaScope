# Domain Rule Module

Rule contracts, registration, and finding types for audit evaluation.

## Files

| File | Responsibility |
|------|---------------|
| rule.go | Defines finding severity, finding metadata, and skip reasons for loaded-but-inapplicable rules (`dialect_mismatch`) |
| registry.go | Registers statement/global rules, enforces rule IDs, evaluates them deterministically, and reports whether a rule ID is Loaded |
| registry_test.go | Verifies registry behavior, ID enforcement, Loaded Contains, and deterministic execution |
| catalog/README.md | Documents the explanation-oriented shipped-rule catalog module |
| ddl/README.md | Documents the Tier-1 DDL rule catalog module |
| dml/README.md | Documents the Tier-1 DML rule catalog module |

## Exports

- `Level`
- `Finding`
- `FindingExplanation`
- `ExplanationMetadata`
- `Location`
- `StatementRule`
- `GlobalRule`
- `Registry`
- `NewRegistry()`
- `Contains(id string) bool`
- `RegisterStatement(rule StatementRule) error`
- `RegisterGlobal(rule GlobalRule) error`

## Dependencies
- Upstream: `internal/application/audit`, domain report aggregation, and future rule implementations
- Downstream: `internal/domain/spec`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
