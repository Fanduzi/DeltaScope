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
	// ErrUnknownAdmission indicates an unrecognized admission value.
	ErrUnknownAdmission = errors.New("unknown admission value")
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
// Unknown admission values are rejected.
func ValidateAdmission(rc ReadClassification, adm Admission) error {
	switch adm {
	case Admissible:
		if rc != ReadOnly {
			return fmt.Errorf("%w: classification is %q", ErrInvalidAdmission, rc)
		}
		return nil
	case Rejected, IndeterminateAdmission:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownAdmission, adm)
	}
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

// DeduplicateReasonCodes removes duplicate reason codes, preserving first-seen order.
func DeduplicateReasonCodes(codes []ReasonCode) []ReasonCode {
	if len(codes) == 0 {
		return codes
	}
	seen := make(map[ReasonCode]struct{}, len(codes))
	result := make([]ReasonCode, 0, len(codes))
	for _, c := range codes {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		result = append(result, c)
	}
	return result
}

// ReasonForIdentityFailure maps a bounded identity-failure category to a
// stable reason code. Unknown categories return false so callers cannot inject
// free-text or ad-hoc strings as trusted reasons.
func ReasonForIdentityFailure(f IdentityFailure) (ReasonCode, bool) {
	switch f {
	case IdentityFailureUnavailable:
		return ReasonIdentityResolverUnavailable, true
	case IdentityFailureUnknown:
		return ReasonIdentityUnknown, true
	case IdentityFailureError:
		return ReasonIdentityLookupFailed, true
	case IdentityFailureAmbiguous:
		return ReasonIdentityAmbiguous, true
	case IdentityFailureCoercionGap:
		return ReasonIdentityCoercionGap, true
	default:
		return "", false
	}
}

// ValidIdentityStatus reports whether s is a known bounded identity status.
// Free-text and empty strings return false.
func ValidIdentityStatus(s IdentityStatus) bool {
	switch s {
	case IdentityStatusResolved,
		IdentityStatusUnknown,
		IdentityStatusAmbiguous,
		IdentityStatusCoercionGap,
		IdentityStatusLookupFailed,
		IdentityStatusUnavailable:
		return true
	default:
		return false
	}
}

// IdentityStatusIsFailClosed reports whether the status forbids pure-read
// promotion for that candidate. Only resolved is non-fail-closed; free-text
// and empty statuses are treated as fail-closed.
func IdentityStatusIsFailClosed(s IdentityStatus) bool {
	return s != IdentityStatusResolved
}

// IdentityStatusToFailure maps a non-resolved bounded status to IdentityFailure.
// Resolved and free-text statuses return false (callers must not invent failures
// from success or inject error strings).
func IdentityStatusToFailure(s IdentityStatus) (IdentityFailure, bool) {
	switch s {
	case IdentityStatusUnavailable:
		return IdentityFailureUnavailable, true
	case IdentityStatusUnknown:
		return IdentityFailureUnknown, true
	case IdentityStatusLookupFailed:
		return IdentityFailureError, true
	case IdentityStatusAmbiguous:
		return IdentityFailureAmbiguous, true
	case IdentityStatusCoercionGap:
		return IdentityFailureCoercionGap, true
	default:
		return "", false
	}
}

// ReasonForIdentityStatus maps a non-resolved bounded status to a reason code.
// Resolved and free-text statuses return false so callers cannot inject
// arbitrary strings as trusted reasons. Resolved never yields a reason code
// from this helper (trust/promotion is a later policy step).
func ReasonForIdentityStatus(s IdentityStatus) (ReasonCode, bool) {
	f, ok := IdentityStatusToFailure(s)
	if !ok {
		return "", false
	}
	return ReasonForIdentityFailure(f)
}

// NormalizeReasonCodes deduplicates and sorts reason codes for stable public output.
func NormalizeReasonCodes(codes []ReasonCode) []ReasonCode {
	return SortReasonCodes(DeduplicateReasonCodes(codes))
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
