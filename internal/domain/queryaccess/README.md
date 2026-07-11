# Domain Query Access Module

Transport-neutral domain types for query access analysis, including read classification, admission decisions, relation/column references, and permission requirements.

## Files

| File | Responsibility |
|------|---------------|
| doc.go | Declares the queryaccess package boundary |
| model.go | Defines core domain types: Mode, ReadClassification, Admission, RelationKind, UsageContext, ReasonCode, WarningCode, and Result |
| normalize.go | Pure deterministic helpers for mode normalization, classification folding, admission validation, sorting, deduplication, and result validation |
| model_test.go | Verifies JSON round-trip, omitted empty fields, forbidden field absence, and constant correctness |
| normalize_test.go | Verifies each normalize function independently with edge cases |

## Exports

- `Mode`
  - `ModeStrict`
  - `ModeProjectionOnly`
- `ReadClassification`
  - `ReadOnly`
  - `NotReadOnly`
  - `Indeterminate`
- `Admission`
  - `Admissible`
  - `Rejected`
  - `IndeterminateAdmission`
- `RelationKind`
  - `RelationTable`
  - `RelationView`
  - `RelationCTE`
  - `RelationDerived`
- `UsageContext`
  - `UsageProjection`
  - `UsageFilter`
  - `UsageJoin`
  - `UsageGrouping`
  - `UsageHaving`
  - `UsageOrdering`
  - `UsageWindow`
- `ReasonCode`
  - `ReasonParseFailure`
  - `ReasonUnsupportedDialect`
  - `ReasonWriteOperation`
  - `ReasonMultiStatement`
  - `ReasonSchemaUnavailable`
  - `ReasonAmbiguousReference`
- `WarningCode`
  - `WarningAmbiguousColumn`
  - `WarningMissingSchema`
  - `WarningDeprecatedSyntax`
  - `WarningInferenceRisk`
- `RelationReference`
- `ColumnReference`
- `OutputColumn`
- `Requirement`
- `Unresolved`
- `Result`
- `NormalizeMode()`
- `ValidateMode()`
- `FoldReadClassification()`
- `ValidateAdmission()`
- `SortRelations()`
- `SortColumns()`
- `SortRequirements()`
- `DeduplicateUsages()`
- `ValidateResult()`
- `FormatRelationKey()`
- `FormatColumnKey()`

## Notes

- All types use `json` struct tags with `omitempty` for optional fields.
- `Result` intentionally excludes raw SQL, severity, literal, password, and credential fields.
- Sorting functions return new slices; they do not mutate input.
- `FoldReadClassification` priority: `not_read_only` > `indeterminate` > `read_only`.
- `ValidateAdmission` rejects `admissible` + non-`read_only` combinations.

## Dependencies
- Upstream: `internal/application/queryaccess`
- Downstream: none inside the domain core

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
