package queryaccess

import (
	"strings"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

type builtinSemanticDecision string

const (
	builtinSemanticAllProven builtinSemanticDecision = "all_proven"
	builtinSemanticUnproven  builtinSemanticDecision = "unproven"
)

type builtinSemanticProofResult struct {
	decision builtinSemanticDecision
}

func proveBuiltinSemantics(profile AnalysisProfile, dialect string, candidates []EffectCandidate, result domain.Result, requirements []domain.Requirement, registry *builtinSemanticRegistry) builtinSemanticProofResult {
	if profile == AnalysisProfileEmpty || (dialect != "mysql" && dialect != "tidb") || ValidateAnalysisProfile(profile, dialect) != nil {
		return builtinSemanticProofResult{decision: builtinSemanticUnproven}
	}
	manifest := registry.manifest(profile)
	if manifest == nil || len(manifest.entries) == 0 || !strictPhysicalRequirementsComplete(result, requirements, candidates) {
		return builtinSemanticProofResult{decision: builtinSemanticUnproven}
	}
	byName := make(map[string][]BuiltinSemanticEntry, len(manifest.entries))
	for _, entry := range manifest.entries {
		byName[entry.Name] = append(byName[entry.Name], entry)
	}
	if !candidateOrdinalsClosed(candidates) {
		return builtinSemanticProofResult{decision: builtinSemanticUnproven}
	}
	for _, candidate := range candidates {
		if !builtinCandidateMatches(candidate, byName[candidateName(candidate)]) {
			return builtinSemanticProofResult{decision: builtinSemanticUnproven}
		}
	}
	return builtinSemanticProofResult{decision: builtinSemanticAllProven}
}

func candidateOrdinalsClosed(candidates []EffectCandidate) bool {
	if len(candidates) == 0 {
		return false
	}
	seen := make(map[int]struct{}, len(candidates))
	for index, candidate := range candidates {
		if candidate.Ordinal < 0 {
			return false
		}
		if candidate.Ordinal != index {
			return false
		}
		if _, exists := seen[candidate.Ordinal]; exists {
			return false
		}
		seen[candidate.Ordinal] = struct{}{}
	}
	return true
}

func candidateName(candidate EffectCandidate) string {
	if len(candidate.NamePath) != 1 {
		return ""
	}
	return candidate.NamePath[0]
}

func builtinCandidateMatches(candidate EffectCandidate, entries []BuiltinSemanticEntry) bool {
	if candidate.Kind != EffectCandidateFunction || candidate.Ordinal < 0 || candidate.IsQuoted || candidate.ExplicitSchema || candidate.UnqualifiedRelation || candidate.Ambiguous || !candidate.Canonical || len(candidate.NamePath) != 1 || len(candidate.OriginalNamePath) != 1 || len(candidate.TargetTypePath) != 0 {
		return false
	}
	if candidate.NamePath[0] != strings.ToLower(candidate.NamePath[0]) || !strings.EqualFold(candidate.NamePath[0], candidate.OriginalNamePath[0]) {
		return false
	}
	for _, entry := range entries {
		if !candidateCallClassMatches(candidate, entry) {
			continue
		}
		if candidate.NamePath[0] != entry.Name {
			continue
		}
		if !candidateArityMatches(candidate, entry) {
			continue
		}
		if !candidateOperandKindsMatch(candidate, entry) {
			continue
		}
		if !candidateOperandRefsShape(candidate) {
			continue
		}
		if !candidateModifiersAllowed(candidate, entry) {
			continue
		}
		return true
	}
	return false
}

func candidateCallClassMatches(candidate EffectCandidate, entry BuiltinSemanticEntry) bool {
	switch entry.CallClass {
	case BuiltinSemanticAggregate:
		return candidate.ParserClassification == "aggregate"
	case BuiltinSemanticWindow:
		return candidate.ParserClassification == "window"
	case BuiltinSemanticScalar:
		return candidate.ParserClassification == "generic" || candidate.ParserClassification == "keyword"
	default:
		return false
	}
}

func candidateArityMatches(candidate EffectCandidate, entry BuiltinSemanticEntry) bool {
	if entry.MinArity > 0 {
		if candidate.Arity < entry.MinArity {
			return false
		}
		if entry.MaxArity > 0 && candidate.Arity > entry.MaxArity {
			return false
		}
		return true
	}
	return candidate.Arity == entry.Arity
}

func candidateOperandKindsMatch(candidate EffectCandidate, entry BuiltinSemanticEntry) bool {
	if entry.MinArity > 0 {
		if len(candidate.OperandKinds) < entry.MinArity {
			return false
		}
		if len(candidate.OperandKinds) != candidate.Arity {
			return false
		}
		if len(entry.OperandKinds) == 0 {
			return false
		}
		for i, kind := range candidate.OperandKinds {
			expected := entry.OperandKinds[len(entry.OperandKinds)-1]
			if i < len(entry.OperandKinds) {
				expected = entry.OperandKinds[i]
			}
			if kind != expected {
				return false
			}
		}
		return true
	}
	if len(candidate.OperandKinds) != len(entry.OperandKinds) {
		return false
	}
	for i, kind := range candidate.OperandKinds {
		if kind != entry.OperandKinds[i] {
			return false
		}
	}
	return true
}

func candidateModifiersAllowed(candidate EffectCandidate, entry BuiltinSemanticEntry) bool {
	if candidate.HasFilter && !entry.AllowFilter || candidate.HasDistinct && !entry.AllowDistinct || candidate.HasAggOrder && !entry.AllowAggOrder || candidate.HasWithinGroup && !entry.AllowWithinGroup || candidate.HasFrame && !entry.AllowFrame || candidate.HasNamedWindow && !entry.AllowNamedWindow || candidate.HasWindowPartition && !entry.AllowWindowPartition || candidate.HasWindowOrder && !entry.AllowWindowOrder {
		return false
	}
	if entry.RequireWindowPartition && !candidate.HasWindowPartition {
		return false
	}
	if entry.RequireWindowOrder && !candidate.HasWindowOrder {
		return false
	}
	if entry.CallClass == BuiltinSemanticAggregate && candidate.HasWindow {
		return false
	}
	if entry.CallClass == BuiltinSemanticWindow && !candidate.HasWindow {
		return false
	}
	if entry.CallClass == BuiltinSemanticScalar && (candidate.HasWindow || candidate.IsAggregate || candidate.HasFilter || candidate.HasDistinct || candidate.HasAggOrder || candidate.HasWithinGroup || candidate.HasFrame || candidate.HasNamedWindow || candidate.HasWindowPartition || candidate.HasWindowOrder) {
		return false
	}
	if !candidate.HasWindowPartition && len(candidate.WindowPartitionKinds) > 0 || !candidate.HasWindowOrder && len(candidate.WindowOrderKinds) > 0 || !candidate.HasFrame && len(candidate.WindowFrameKinds) > 0 {
		return false
	}
	if candidate.HasFrame && len(candidate.WindowFrameKinds) != 2 {
		return false
	}
	return true
}

func candidateOperandRefsShape(candidate EffectCandidate) bool {
	columnOperands := 0
	for _, kind := range candidate.OperandKinds {
		if kind == "column" {
			columnOperands++
		}
	}
	if len(candidate.OperandColumnRefs) != columnOperands {
		return false
	}
	if len(candidate.WindowPartitionKinds) != len(candidate.WindowPartitionColumnRefs) || len(candidate.WindowOrderKinds) != len(candidate.WindowOrderColumnRefs) {
		return false
	}
	for _, kind := range append(append([]string(nil), candidate.WindowPartitionKinds...), candidate.WindowOrderKinds...) {
		if kind != "column" {
			return false
		}
	}
	return true
}

func strictPhysicalRequirementsComplete(result domain.Result, requirements []domain.Requirement, candidates []EffectCandidate) bool {
	if result.Mode != domain.ModeStrict || len(result.Unresolved) > 0 || len(result.Relations) == 0 {
		return false
	}
	for _, relation := range result.Relations {
		if relation.Kind != domain.RelationTable || !relation.PermissionRequired || relation.Unbound || relation.Schema == "" || relation.Name == "" {
			return false
		}
	}
	for _, column := range result.ReferencedColumns {
		if column.Unbound || column.Schema == "" || column.Table == "" || column.Column == "" || column.Column == "*" {
			return false
		}
	}
	for _, output := range result.Outputs {
		if output.Name == "*" || strings.HasSuffix(output.Name, ".*") {
			return false
		}
	}
	required := make(map[string]struct{}, len(requirements))
	for _, requirement := range requirements {
		if requirement.Privilege == "indeterminate" {
			return false
		}
		required[requirement.Object+"\x00"+requirement.Privilege] = struct{}{}
	}
	for _, relation := range result.Relations {
		key := domain.FormatRelationKey(relation.Schema, relation.Name) + "\x00read_table"
		if _, ok := required[key]; !ok {
			return false
		}
	}
	for _, column := range result.ReferencedColumns {
		key := domain.FormatColumnKey(column.Schema, column.Table, column.Column) + "\x00read_column"
		if _, ok := required[key]; !ok {
			return false
		}
	}
	for _, candidate := range candidates {
		refs := append(append([]OperandColumnRef(nil), candidate.OperandColumnRefs...), candidate.WindowPartitionColumnRefs...)
		refs = append(refs, candidate.WindowOrderColumnRefs...)
		for _, ref := range refs {
			if ref.Schema == "" || ref.Table == "" || ref.Column == "" || ref.Column == "*" {
				return false
			}
			key := domain.FormatColumnKey(ref.Schema, ref.Table, ref.Column) + "\x00read_column"
			if _, ok := required[key]; !ok {
				return false
			}
		}
	}
	return true
}

func stringSliceEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
