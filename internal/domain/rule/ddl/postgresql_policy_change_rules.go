package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgPolicyChangeRule struct{ level rule.Level }

func newPGPolicyChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgPolicyChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgPolicyChangeRule) ID() string { return ruleIDPGPolicyChangeValid }

func (r pgPolicyChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationAlterPolicy && s.DDL.PGPolicyChange != nil
}

func assessPGPolicyChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGPolicyChangeLookup(s) == nil {
		return "", "alter_policy.target_identity"
	}
	decl := s.DDL.PGPolicyChange
	state := s.Metadata.PGPolicyChange
	if state == nil || state.Table != decl.Table.Table || state.Name != decl.Name || state.RenameTo != decl.RenameTo ||
		!slices.Equal(state.NamedRoles, decl.NamedPolicyRoles()) {
		return "", "alter_policy.catalog"
	}
	switch {
	case state.MissingRole:
		return "alter_policy_role_absent", ""
	case state.RelationKind == "":
		return "alter_policy_table_absent", ""
	case state.RelationKind != "r" && state.RelationKind != "p":
		return "alter_policy_wrong_relation_kind", ""
	case decl.RenameTo != "" && state.DestinationOccupied:
		return "alter_policy_rename_occupied", ""
	}
	for _, predicate := range []spec.PGPolicyPredicate{decl.Using, decl.WithCheck} {
		if !predicate.Present {
			continue
		}
		if predicate.Column != "" {
			if state.Columns == nil {
				return "", "alter_policy.columns"
			}
			if predicate.Column != state.Table.Name && !slices.Contains(state.Columns, predicate.Column) {
				return "alter_policy_column_absent", ""
			}
		}
		if !predicate.BooleanResult {
			fact = "alter_policy.predicate_type"
		}
	}
	switch {
	case state.PolicyCommand == "":
		return "alter_policy_source_absent", ""
	case (state.PolicyCommand == "r" || state.PolicyCommand == "d") && decl.WithCheck.Present:
		return "alter_policy_with_check_inapplicable", ""
	case state.PolicyCommand == "a" && decl.Using.Present:
		return "alter_policy_using_inapplicable", ""
	}
	return "", fact
}

func (r pgPolicyChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGPolicyChange(s)
	var message string
	switch reason {
	case "alter_policy_role_absent":
		message = "A PostgreSQL policy role does not exist"
	case "alter_policy_table_absent":
		message = "The PostgreSQL policy table does not exist"
	case "alter_policy_wrong_relation_kind":
		message = "PostgreSQL row-security policies require an ordinary or partitioned table"
	case "alter_policy_rename_occupied":
		message = "The PostgreSQL policy name is already occupied on this table"
	case "alter_policy_column_absent":
		message = "The PostgreSQL policy predicate references a missing column"
	case "alter_policy_source_absent":
		message = "The PostgreSQL policy does not exist on this table"
	case "alter_policy_with_check_inapplicable":
		message = "PostgreSQL SELECT and DELETE policies accept only USING"
	case "alter_policy_using_inapplicable":
		message = "PostgreSQL INSERT policies accept only WITH CHECK"
	default:
		return nil, nil
	}
	decl := s.DDL.PGPolicyChange
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": decl.Table.Table.Schema, "table": decl.Table.Table.Name, "policy": decl.Name, "reason_code": reason,
	}}}, nil
}

func (r pgPolicyChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGPolicyChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_policy_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
