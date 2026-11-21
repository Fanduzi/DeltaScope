package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgOperatorFamilyTargetRule struct{ level rule.Level }

func newPGOperatorFamilyTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgOperatorFamilyTargetRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgOperatorFamilyTargetRule) ID() string { return ruleIDPGOperatorFamilyTargetValid }

func (r pgOperatorFamilyTargetRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateOperatorFamily && s.DDL.PGOperatorFamily != nil
}

func assessPGOperatorFamilyTarget(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGOperatorFamilyLookup(s) == nil {
		return "", "operator_family.target_identity"
	}
	state := s.Metadata.PGOperatorFamily
	if state == nil || state.Target != *s.DDL.PGOperatorFamily {
		return "", "operator_family.catalog_state"
	}
	switch {
	case !state.SchemaExists:
		return "operator_family_schema_absent", ""
	case state.MethodKind == "":
		return "operator_family_method_absent", ""
	case state.MethodKind != "i":
		return "operator_family_method_wrong_kind", ""
	case state.Exists:
		return "operator_family_already_exists", ""
	default:
		return "", ""
	}
}

func (r pgOperatorFamilyTargetRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGOperatorFamilyTarget(s)
	if reason == "" {
		return nil, nil
	}
	target := s.DDL.PGOperatorFamily
	return []rule.Finding{{Level: r.level, Message: "PostgreSQL rejects this operator family creation target", Metadata: map[string]any{
		"schema": target.Schema, "operator_family": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgOperatorFamilyTargetRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGOperatorFamilyTarget(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_operator_family_target", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
