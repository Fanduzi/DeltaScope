package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgOperatorChangeRule struct{ level rule.Level }

func newPGOperatorChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgOperatorChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgOperatorChangeRule) ID() string { return ruleIDPGOperatorChangeValid }

func (r pgOperatorChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationAlterOperator && s.DDL.PGOperatorChange != nil
}

func assessPGOperatorChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGOperatorChangeLookup(s) == nil {
		return "", "alter_operator.signature"
	}
	state := s.Metadata.PGOperatorChange
	if state == nil || state.Target != *s.DDL.PGOperatorChange {
		return "", "alter_operator.catalog"
	}
	if state.Target.Action == "set_owner" {
		switch {
		case state.Target.Owner.Kind == "public":
			return "alter_operator_owner_public", ""
		case state.Target.Owner.Kind == "named" && !state.OwnerExists:
			return "alter_operator_owner_absent", ""
		case !state.OwnerExists:
			return "", "alter_operator.owner_identity"
		}
	}
	switch {
	case !state.OperandTypes || !state.OperandSchema:
		return "alter_operator_operand_unresolved", ""
	case !state.SourceSchema:
		return "alter_operator_schema_absent", ""
	case !state.SourceExists:
		return "alter_operator_source_absent", ""
	case state.Target.Action == "set_estimators":
		return assessPGOperatorEstimators(state), ""
	case state.Target.Action != "set_schema":
		return "", ""
	case !state.DestinationSchema:
		return "alter_operator_destination_schema_absent", ""
	case state.Target.Destination == state.Target.Source.Table.Schema:
		return "", ""
	case state.NamespaceIneligible:
		return "alter_operator_destination_ineligible", ""
	case state.DestinationOccupied:
		return "alter_operator_destination_occupied", ""
	}
	return "", ""
}

func assessPGOperatorEstimators(state *spec.PGOperatorChangeState) string {
	restrict, join := state.Target.Restrict.Name != "", state.Target.Join.Name != ""
	switch {
	case restrict && state.RestrictMatches == 0:
		return "alter_operator_restrict_absent"
	case restrict && !state.RestrictReturnsFloat:
		return "alter_operator_restrict_result"
	case join && state.JoinMatches == 0:
		return "alter_operator_join_absent"
	case join && state.JoinMatches > 1:
		return "alter_operator_join_ambiguous"
	case join && !state.JoinReturnsFloat:
		return "alter_operator_join_result"
	case join && state.Target.Unary:
		return "alter_operator_join_requires_binary"
	case (restrict || join) && !state.BooleanResult:
		return "alter_operator_estimator_requires_boolean"
	default:
		return ""
	}
}

func (r pgOperatorChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGOperatorChange(s)
	var message string
	switch reason {
	case "alter_operator_owner_public":
		message = "PostgreSQL does not permit PUBLIC as an operator owner"
	case "alter_operator_owner_absent":
		message = "The requested PostgreSQL operator owner does not exist"
	case "alter_operator_operand_unresolved":
		message = "A declared PostgreSQL operator operand type could not be resolved"
	case "alter_operator_schema_absent":
		message = "The PostgreSQL operator schema does not exist"
	case "alter_operator_source_absent":
		message = "The PostgreSQL operator signature does not exist"
	case "alter_operator_destination_schema_absent":
		message = "The requested PostgreSQL operator destination schema does not exist"
	case "alter_operator_destination_ineligible":
		message = "PostgreSQL disallows moving the operator into or out of this namespace"
	case "alter_operator_destination_occupied":
		message = "The PostgreSQL operator signature is occupied in the destination schema"
	case "alter_operator_restrict_absent":
		message = "The PostgreSQL restriction estimator signature does not exist"
	case "alter_operator_restrict_result":
		message = "The PostgreSQL restriction estimator must return double precision"
	case "alter_operator_join_absent":
		message = "The PostgreSQL join estimator signature does not exist"
	case "alter_operator_join_ambiguous":
		message = "Both PostgreSQL join estimator signatures exist"
	case "alter_operator_join_result":
		message = "The PostgreSQL join estimator must return double precision"
	case "alter_operator_join_requires_binary":
		message = "PostgreSQL join estimators require a binary operator"
	case "alter_operator_estimator_requires_boolean":
		message = "PostgreSQL estimators require an operator returning Boolean"
	default:
		return nil, nil
	}
	t := s.DDL.PGOperatorChange.Source.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": t.Schema, "operator": t.Name, "reason_code": reason,
	}}}, nil
}

func (r pgOperatorChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGOperatorChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_operator_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
