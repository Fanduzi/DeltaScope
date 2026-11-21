package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgOperatorFamilyMemberRule struct{ level rule.Level }

func newPGOperatorFamilyMemberRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgOperatorFamilyMemberRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgOperatorFamilyMemberRule) ID() string { return ruleIDPGOperatorFamilyMemberValid }

func (r pgOperatorFamilyMemberRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterOperatorFamily && s.DDL.PGOperatorFamilyMember != nil
}

func assessPGOperatorFamilyMember(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGOperatorFamilyMemberLookup(s) == nil {
		return "", "operator_family.target_identity"
	}
	state := s.Metadata.PGOperatorFamilyMember
	if state == nil || state.Target != *s.DDL.PGOperatorFamily || state.Member != *s.DDL.PGOperatorFamilyMember {
		return "", "operator_family.member_state"
	}
	switch {
	case state.MethodKind == "":
		return "operator_family_method_absent", ""
	case state.MethodKind != "i":
		return "operator_family_method_wrong_kind", ""
	case !state.SchemaExists:
		return "operator_family_schema_absent", ""
	case !state.FamilyExists:
		return "operator_family_absent", ""
	case !state.BTree:
		return "", "operator_family.method_member_semantics"
	case state.Member.Number < 1 || state.Member.Number > 5:
		return "operator_family_strategy_out_of_range", ""
	case !state.OperandTypesRead:
		return "", "operator_family.operand_types"
	case !state.OperandTypesOK:
		return "operator_family_operand_type_absent", ""
	}
	if state.Member.Drop {
		switch {
		case !state.SlotExists:
			return "operator_family_slot_absent", ""
		case state.SlotProtected:
			return "operator_family_slot_protected", ""
		}
		return "", ""
	}
	if !(spec.PGDestructiveTableTarget{Table: state.Member.Operator}).Qualified() {
		return "", "operator_family.operator_identity"
	}
	switch {
	case !state.OperatorExists:
		return "operator_family_operator_absent", ""
	case !state.OperatorBoolean:
		return "operator_family_operator_result", ""
	case state.SlotExists:
		return "operator_family_slot_exists", ""
	}
	return "", ""
}

func (r pgOperatorFamilyMemberRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGOperatorFamilyMember(s)
	if reason == "" {
		return nil, nil
	}
	target := s.DDL.PGOperatorFamily
	return []rule.Finding{{Level: r.level, Message: "PostgreSQL rejects this operator family member change", Metadata: map[string]any{
		"schema": target.Schema, "operator_family": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgOperatorFamilyMemberRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGOperatorFamilyMember(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_operator_family_member", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
