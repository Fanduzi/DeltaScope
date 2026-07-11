package queryaccess

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	// ErrInvalidMode indicates the mode is not a recognized value.
	ErrInvalidMode = errors.New("invalid mode: must be strict or projection_only")
	// ErrInvalidAdmission indicates an invalid admission/classification combination.
	ErrInvalidAdmission = errors.New("invalid admission: admissible requires read_only classification")
	// ErrForbiddenField indicates the result contains a forbidden field.
	ErrForbiddenField = errors.New("result contains forbidden field")
)

// NormalizeMode defaults empty mode to strict.
func NormalizeMode(m Mode) Mode {
	if m == "" {
		return ModeStrict
	}
	return m
}

// ValidateMode checks whether the mode is a recognized value.
func ValidateMode(m Mode) error {
	switch m {
	case ModeStrict, ModeProjectionOnly:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidMode, m)
	}
}

// FoldReadClassification folds multi-statement read classifications into a single result.
// Rules: any not_read_only → not_read_only; any indeterminate → indeterminate; all read_only → read_only.
func FoldReadClassification(classifications []ReadClassification) ReadClassification {
	if len(classifications) == 0 {
		return Indeterminate
	}
	hasIndeterminate := false
	for _, c := range classifications {
		switch c {
		case NotReadOnly:
			return NotReadOnly
		case Indeterminate:
			hasIndeterminate = true
		}
	}
	if hasIndeterminate {
		return Indeterminate
	}
	return ReadOnly
}

// ValidateAdmission rejects invalid admission/classification combinations.
// Admissible requires read_only classification.
func ValidateAdmission(rc ReadClassification, adm Admission) error {
	if adm == Admissible && rc != ReadOnly {
		return fmt.Errorf("%w: classification is %q", ErrInvalidAdmission, rc)
	}
	return nil
}

// SortRelations sorts relation references by schema+name+alias+kind for deterministic output.
func SortRelations(refs []RelationReference) []RelationReference {
	if len(refs) == 0 {
		return refs
	}
	sorted := append([]RelationReference(nil), refs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Schema != sorted[j].Schema {
			return sorted[i].Schema < sorted[j].Schema
		}
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		if sorted[i].Alias != sorted[j].Alias {
			return sorted[i].Alias < sorted[j].Alias
		}
		return sorted[i].Kind < sorted[j].Kind
	})
	return sorted
}

// SortColumns sorts column references by schema+table+column+usages for deterministic output.
func SortColumns(refs []ColumnReference) []ColumnReference {
	if len(refs) == 0 {
		return refs
	}
	sorted := append([]ColumnReference(nil), refs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Schema != sorted[j].Schema {
			return sorted[i].Schema < sorted[j].Schema
		}
		if sorted[i].Table != sorted[j].Table {
			return sorted[i].Table < sorted[j].Table
		}
		if sorted[i].Column != sorted[j].Column {
			return sorted[i].Column < sorted[j].Column
		}
		return usageKey(sorted[i].Usages) < usageKey(sorted[j].Usages)
	})
	return sorted
}

func usageKey(usages []UsageContext) string {
	if len(usages) == 0 {
		return ""
	}
	var b strings.Builder
	for i, u := range usages {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(string(u))
	}
	return b.String()
}

// SortRequirements sorts requirements by object+privilege for deterministic output.
func SortRequirements(reqs []Requirement) []Requirement {
	if len(reqs) == 0 {
		return reqs
	}
	sorted := append([]Requirement(nil), reqs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Object != sorted[j].Object {
			return sorted[i].Object < sorted[j].Object
		}
		return sorted[i].Privilege < sorted[j].Privilege
	})
	return sorted
}

// DeduplicateUsages deduplicates usage contexts preserving first-seen order.
func DeduplicateUsages(usages []UsageContext) []UsageContext {
	if len(usages) == 0 {
		return usages
	}
	seen := make(map[UsageContext]struct{}, len(usages))
	result := make([]UsageContext, 0, len(usages))
	for _, u := range usages {
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			result = append(result, u)
		}
	}
	return result
}

// forbiddenFields is the set of JSON field names that must never appear in a Result.
var forbiddenFields = map[string]bool{
	"sql":            true,
	"raw_sql":        true,
	"normalized_sql": true,
	"severity":       true,
	"literal":        true,
	"password":       true,
	"credential":     true,
}

// ValidateResult checks the result for forbidden fields and structural invariants.
func ValidateResult(r *Result) error {
	if r == nil {
		return nil
	}
	// Check admission/classification consistency
	if err := ValidateAdmission(r.ReadClassification, r.Admission); err != nil {
		return err
	}
	// Check mode validity
	if err := ValidateMode(r.Mode); err != nil {
		return err
	}
	return nil
}

// FormatRelationKey returns a canonical "schema.name" or "name" key for a relation.
func FormatRelationKey(schema, name string) string {
	if schema == "" {
		return name
	}
	return schema + "." + name
}

// FormatColumnKey returns a canonical "schema.table.column" or "table.column" key for a column.
func FormatColumnKey(schema, table, column string) string {
	var b strings.Builder
	if schema != "" {
		b.WriteString(schema)
		b.WriteByte('.')
	}
	b.WriteString(table)
	b.WriteByte('.')
	b.WriteString(column)
	return b.String()
}

// SortOutputs sorts output columns by name for deterministic output.
func SortOutputs(outputs []OutputColumn) []OutputColumn {
	if len(outputs) == 0 {
		return outputs
	}
	sorted := append([]OutputColumn(nil), outputs...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

// SortUnresolved sorts unresolved references by reference+reason for deterministic output.
func SortUnresolved(unresolved []Unresolved) []Unresolved {
	if len(unresolved) == 0 {
		return unresolved
	}
	sorted := append([]Unresolved(nil), unresolved...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Reference != sorted[j].Reference {
			return sorted[i].Reference < sorted[j].Reference
		}
		return sorted[i].Reason < sorted[j].Reason
	})
	return sorted
}

// SortReasonCodes sorts reason codes for deterministic output.
func SortReasonCodes(codes []ReasonCode) []ReasonCode {
	if len(codes) == 0 {
		return codes
	}
	sorted := append([]ReasonCode(nil), codes...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	return sorted
}

// SortWarningCodes sorts warning codes for deterministic output.
func SortWarningCodes(codes []WarningCode) []WarningCode {
	if len(codes) == 0 {
		return codes
	}
	sorted := append([]WarningCode(nil), codes...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	return sorted
}
