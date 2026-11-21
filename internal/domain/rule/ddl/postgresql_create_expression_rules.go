package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateExpressionRule struct{ level rule.Level }

func newPGCreateExpressionRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateExpressionRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateExpressionRule) ID() string { return ruleIDPGCreateExpressionValid }

func (r pgCreateExpressionRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationCreateTable && s.DDL.PGCreateExpression != nil
}

func assessPGCreateExpression(s spec.Statement) (reason, gap string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	expression := s.DDL.PGCreateExpression
	if expression.Unbound || !expression.Target.Qualified() {
		return "", "create_table.expression_namespace"
	}
	state := s.Metadata.PGCreateDestination
	if state == nil || state.Target != expression.Target {
		return "", "create_table.expression_destination"
	}
	switch {
	case !state.SchemaExists:
		return "create_expression_schema_absent", ""
	case expression.IfNotExists && state.RelationExists:
		return "", ""
	case state.RelationExists:
		return "create_expression_destination_occupied", ""
	default:
		return expression.Reason, ""
	}
}

func (r pgCreateExpressionRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateExpression(s)
	var message string
	switch reason {
	case "create_expression_schema_absent":
		message = "The PostgreSQL table creation schema does not exist"
	case "create_expression_destination_occupied":
		message = "A PostgreSQL relation already occupies the creation target"
	case "create_default_disallowed_reference":
		message = "PostgreSQL does not allow column references or subqueries in a column default"
	case "create_check_subquery":
		message = "PostgreSQL does not allow subqueries in a CHECK constraint"
	case "create_generated_subquery":
		message = "PostgreSQL does not allow subqueries in a generated column expression"
	case "create_generated_references_generated":
		message = "A PostgreSQL generated column cannot reference a generated column"
	default:
		return nil, nil
	}
	target := s.DDL.PGCreateExpression.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "table": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateExpressionRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateExpression(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_expression", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
