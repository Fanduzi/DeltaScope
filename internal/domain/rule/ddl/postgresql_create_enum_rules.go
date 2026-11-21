package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateEnumRule struct{ level rule.Level }

func newPGCreateEnumRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateEnumRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateEnumRule) ID() string { return ruleIDPGCreateEnumValid }

func (r pgCreateEnumRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationCreateType && s.DDL.PGCreateEnum != nil
}

func assessPGCreateEnum(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGTypeDeclarationLookup(s) == nil {
		return "", "create_type.enum_namespace"
	}
	decl := s.DDL.PGCreateEnum
	state := s.Metadata.PGEnum
	if state == nil || state.Target != decl.Target {
		return "", "create_type.enum_destination"
	}
	switch {
	case !state.SchemaExists:
		return "create_enum_schema_absent", ""
	case state.NameOccupied:
		return "create_enum_type_occupied", ""
	}
	if len(decl.Labels) != 0 && (state.Encoding != "UTF8" || state.LabelBytes != 63) {
		return "", "create_type.enum_label_encoding"
	}
	for _, label := range decl.Labels {
		if len(label) > state.LabelBytes {
			return "create_enum_label_too_long", ""
		}
	}
	seen := make(map[string]bool, len(decl.Labels))
	for _, label := range decl.Labels {
		if seen[label] {
			return "create_enum_label_duplicate", ""
		}
		seen[label] = true
	}
	return "", ""
}

func (r pgCreateEnumRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateEnum(s)
	var message string
	switch reason {
	case "create_enum_schema_absent":
		message = "The PostgreSQL enum creation schema does not exist"
	case "create_enum_type_occupied":
		message = "A PostgreSQL type already occupies the enum creation target"
	case "create_enum_label_too_long":
		message = "The PostgreSQL enum label exceeds the supported byte limit"
	case "create_enum_label_duplicate":
		message = "A PostgreSQL enum label is declared more than once"
	default:
		return nil, nil
	}
	target := s.DDL.PGCreateEnum.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "type": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateEnumRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateEnum(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_enum", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
