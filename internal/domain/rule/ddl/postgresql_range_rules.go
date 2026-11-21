package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgRangeRule struct{ level rule.Level }

func newPGRangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgRangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgRangeRule) ID() string { return ruleIDPGRangeValid }

func (r pgRangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationCreateType && s.DDL.PGRange != nil
}

func assessPGRange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	decl := s.DDL.PGRange
	if spec.PGTypeDeclarationLookup(s) == nil {
		return "", "create_type.range_namespace"
	}
	dst := s.Metadata.PGEnum
	if dst == nil || dst.Target != decl.Target {
		return "", "create_type.range_destination"
	}
	switch {
	case !dst.SchemaExists:
		return "range_schema_absent", ""
	case dst.NameOccupied:
		return "range_name_occupied", ""
	}
	seen := make(map[string]bool, len(decl.Options))
	var subtype *spec.PGColumnTypeState
	for _, option := range decl.Options {
		if seen[option] {
			return "range_option_duplicate", ""
		}
		seen[option] = true
		switch option {
		case "subtype":
			subtype = s.Metadata.PGRangeSubtype
			if subtype == nil || subtype.Target != decl.Subtype {
				return "", "create_type.range_subtype"
			}
			if !subtype.SchemaExists || !subtype.Resolved || !subtype.Defined {
				return "range_subtype_unresolved", ""
			}
		case "collation", "subtype_opclass", "canonical", "subtype_diff", "multirange_type_name":
		default:
			return "range_option_unknown", ""
		}
	}
	switch {
	case subtype == nil:
		return "range_subtype_required", ""
	case subtype.Pseudo:
		return "range_subtype_pseudo", ""
	case seen["collation"] && !subtype.Collatable:
		return "range_subtype_not_collatable", ""
	case seen["canonical"] && (!dst.TypeExists || dst.Defined):
		return "range_canonical_requires_shell", ""
	case len(seen) != 1:
		return "", "create_type.range_options"
	}
	return "", "create_type.range_opclass"
}

func (r pgRangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGRange(s)
	var message string
	switch reason {
	case "range_schema_absent":
		message = "The PostgreSQL range creation schema does not exist"
	case "range_name_occupied":
		message = "A PostgreSQL type occupies the range creation target"
	case "range_option_duplicate":
		message = "A PostgreSQL range option was specified more than once"
	case "range_option_unknown":
		message = "PostgreSQL does not recognize this range option"
	case "range_subtype_unresolved":
		message = "The PostgreSQL range subtype could not be resolved to a defined type"
	case "range_subtype_required":
		message = "PostgreSQL requires a subtype for a range declaration"
	case "range_subtype_pseudo":
		message = "PostgreSQL does not allow a pseudo-type as a range subtype"
	case "range_subtype_not_collatable":
		message = "The PostgreSQL range subtype does not support collation"
	case "range_canonical_requires_shell":
		message = "A PostgreSQL range canonical function requires a pre-created shell type"
	default:
		return nil, nil
	}
	target := s.DDL.PGRange.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "type": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgRangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGRange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_range", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
