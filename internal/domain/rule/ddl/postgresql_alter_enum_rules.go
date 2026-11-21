package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgAlterEnumRule struct{ level rule.Level }

func newPGAlterEnumRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAlterEnumRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgAlterEnumRule) ID() string { return ruleIDPGAlterEnumValid }

func (r pgAlterEnumRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationAlterType && s.DDL.PGAlterEnum != nil
}

func assessPGAlterEnum(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	decl := s.DDL.PGAlterEnum
	if !decl.Known {
		return "", "alter_type.enum_declaration"
	}
	if spec.PGTypeDeclarationLookup(s) == nil {
		return "", "alter_type.enum_namespace"
	}
	state := s.Metadata.PGEnum
	if state == nil || state.Target != decl.Target {
		return "", "alter_type.enum_catalog"
	}
	switch {
	case !state.SchemaExists:
		return "alter_enum_schema_absent", ""
	case !state.TypeExists:
		return "alter_enum_type_absent", ""
	case !state.Defined:
		return "alter_enum_type_undefined", ""
	case state.TypeKind != "e":
		return "alter_enum_wrong_type", ""
	case state.Encoding != "UTF8" || state.LabelBytes != 63:
		return "", "alter_type.enum_label_encoding"
	case len(decl.Value) > state.LabelBytes:
		return "alter_enum_label_too_long", ""
	case state.Labels == nil:
		return "", "alter_type.enum_labels"
	case decl.Old != nil && !slices.Contains(state.Labels, *decl.Old):
		return "alter_enum_old_label_absent", ""
	case slices.Contains(state.Labels, decl.Value):
		if decl.Old == nil && decl.IfNotExists {
			return "", ""
		}
		return "alter_enum_label_occupied", ""
	case decl.Neighbor != nil && !slices.Contains(state.Labels, *decl.Neighbor):
		return "alter_enum_neighbor_absent", ""
	}
	return "", ""
}

func (r pgAlterEnumRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGAlterEnum(s)
	var message string
	switch reason {
	case "alter_enum_schema_absent":
		message = "The PostgreSQL enum schema does not exist"
	case "alter_enum_type_absent":
		message = "The PostgreSQL enum type does not exist"
	case "alter_enum_type_undefined":
		message = "The PostgreSQL enum target is an undefined shell type"
	case "alter_enum_wrong_type":
		message = "The PostgreSQL type is ineligible for enum label changes"
	case "alter_enum_label_too_long":
		message = "The PostgreSQL enum label exceeds the supported byte limit"
	case "alter_enum_old_label_absent":
		message = "The PostgreSQL enum label to rename does not exist"
	case "alter_enum_label_occupied":
		message = "The requested PostgreSQL enum label already exists"
	case "alter_enum_neighbor_absent":
		message = "The PostgreSQL enum placement label does not exist"
	default:
		return nil, nil
	}
	target := s.DDL.PGAlterEnum.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "type": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgAlterEnumRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGAlterEnum(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_alter_enum", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
