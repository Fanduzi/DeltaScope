// Package queryaccess contains the narrow PostgreSQL COUNT(integer_one) proof predicate.
// input: exact parser envelope, resolved domain result, requirements, and internal candidate facts
// output: fail-closed proof-completeness decision for one physical table
// pos: PostgreSQL-only proof gate before catalog identity promotion
// note: if this file changes, update this header and module README.md.
package queryaccess

import domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"

func countIntegerOneRequirementsComplete(result domain.Result, requirements []domain.Requirement, candidates []EffectCandidate, exactStatement bool) bool {
	if !exactStatement || result.Dialect != "postgresql" || result.Mode != domain.ModeStrict ||
		len(result.Relations) != 1 || len(result.ReferencedColumns) != 0 || len(result.Unresolved) != 0 || len(requirements) != 1 ||
		len(candidates) != 1 || !IsExactCountIntegerOneCandidate(candidates[0]) {
		return false
	}
	relation := result.Relations[0]
	if relation.Kind != domain.RelationTable || !relation.PermissionRequired || relation.Unbound || relation.Schema == "" || relation.Name == "" {
		return false
	}
	requirement := requirements[0]
	return requirement.Object == domain.FormatRelationKey(relation.Schema, relation.Name) && requirement.Privilege == "read_table"
}

func hasExactCountIntegerOneCandidate(candidates []EffectCandidate) bool {
	for _, candidate := range candidates {
		if IsExactCountIntegerOneCandidate(candidate) {
			return true
		}
	}
	return false
}
