package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgOperatorFamilyChangeRule struct{ level rule.Level }

func newPGOperatorFamilyChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgOperatorFamilyChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgOperatorFamilyChangeRule) ID() string { return ruleIDPGOperatorFamilyChangeValid }

func (r pgOperatorFamilyChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterOperatorFamily && s.DDL.PGOperatorFamilyChange != nil
}

func assessPGOperatorFamilyChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGOperatorFamilyChangeLookup(s) == nil {
		return "", "operator_family.target_identity"
	}
	state := s.Metadata.PGOperatorFamilyChange
	if state == nil || state.Target != *s.DDL.PGOperatorFamilyChange {
		return "", "operator_family.change_state"
	}
	if state.Target.Action == "set_owner" {
		switch {
		case state.Target.Owner.Kind == "public":
			return "operator_family_change_owner_public", ""
		case state.Target.Owner.Kind == "named" && !state.OwnerExists:
			return "operator_family_change_owner_absent", ""
		case !state.OwnerExists:
			return "", "operator_family.owner_identity"
		}
	}
	switch {
	case state.MethodKind == "":
		return "operator_family_method_absent", ""
	case state.MethodKind != "i":
		return "operator_family_method_wrong_kind", ""
	case !state.SourceSchemaExists:
		return "operator_family_schema_absent", ""
	case !state.SourceExists:
		return "operator_family_absent", ""
	}
	switch state.Target.Action {
	case "set_owner":
		return "", ""
	case "set_schema":
		switch {
		case !state.DestinationExists:
			return "operator_family_destination_schema_absent", ""
		case state.SameNamespace:
			return "", ""
		case state.NamespaceUnsupported:
			return "operator_family_destination_schema_inapplicable", ""
		}
	case "rename":
	default:
		return "", "operator_family.change_action"
	}
	if state.DestinationOccupied {
		return "operator_family_destination_occupied", ""
	}
	return "", ""
}

func (r pgOperatorFamilyChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGOperatorFamilyChange(s)
	if reason == "" {
		return nil, nil
	}
	target := s.DDL.PGOperatorFamilyChange.Source
	return []rule.Finding{{Level: r.level, Message: "PostgreSQL rejects this operator family source, destination, or owner", Metadata: map[string]any{
		"schema": target.Schema, "operator_family": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgOperatorFamilyChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGOperatorFamilyChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_operator_family_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
