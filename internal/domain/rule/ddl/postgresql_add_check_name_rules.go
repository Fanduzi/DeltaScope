package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgAddCheckNameRule struct{ level rule.Level }

func newPGAddCheckNameRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgAddCheckNameRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgAddCheckNameRule) ID() string { return ruleIDPGAlterAddCheckNameAvailable }

func (r pgAddCheckNameRule) AppliesTo(s spec.Statement) bool {
	return pgAddCheck(s) && (s.DDL.Alter[0].Name != "" || s.DDL.Alter[0].PGCheckHasSubquery ||
		s.DDL.Alter[0].PGCheckLiteral != "" || len(s.DDL.Alter[0].PGCheckNullTest) != 0)
}

func pgNamedAddCheck(s spec.Statement) bool {
	return pgAddCheck(s) && s.DDL.Alter[0].Name != ""
}

func pgAddCheck(s spec.Statement) bool {
	if s.Dialect != spec.DialectPostgreSQL || s.Kind != spec.KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != spec.DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return false
	}
	a := s.DDL.Alter[0]
	return a.Action == "add_constraint" && a.Options["constraint_type"] == "check"
}

func assessPGAddCheckName(s spec.Statement) (reason, missing string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	if !spec.PGOrdinaryTableBound(s) {
		return "", "target_table.pg_constraint_scope"
	}
	a := s.DDL.Alter[0]
	if a.PGCheckHasSubquery {
		return "check_predicate_subquery_inapplicable", ""
	}
	switch a.PGCheckLiteral {
	case "invalid_boolean":
		return "check_predicate_invalid_boolean", ""
	case "integer":
		coercion := s.Metadata.PGCheckIntegerCoercion
		if coercion == nil || coercion.Allowed {
			return "", "check.integer_boolean_coercion"
		}
		return "check_predicate_boolean_required", ""
	}
	if parts := a.PGCheckNullTest; len(parts) != 0 {
		if len(parts) == 2 && parts[0] != s.DDL.Table.Name {
			return "check_predicate_qualifier_absent", ""
		}
		name := parts[len(parts)-1]
		member, known := s.Metadata.TargetTable.PGColumns.FindColumn(name)
		switch {
		case !known:
			return "", "target_table.pg_columns"
		case member != nil && member.System && name != "tableoid":
			return "check_predicate_system_column", ""
		case member == nil && (len(parts) == 2 || name != s.DDL.Table.Name):
			return "check_predicate_column_absent", ""
		}
	}
	if a.Name == "" {
		return "", ""
	}
	member, known := pgConstraintTargetState(s, 0)
	if !known {
		return "", "target_table.pg_constraints"
	}
	if member != nil {
		return "constraint_name_occupied", ""
	}
	return "", ""
}

func (r pgAddCheckNameRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGAddCheckName(s)
	if reason == "" {
		return nil, nil
	}
	message := "The PostgreSQL table already has a constraint with the requested CHECK name"
	switch reason {
	case "check_predicate_subquery_inapplicable":
		message = "PostgreSQL CHECK predicates cannot contain a subquery"
	case "check_predicate_invalid_boolean":
		message = "PostgreSQL cannot interpret the CHECK predicate literal as a Boolean"
	case "check_predicate_boolean_required":
		message = "The CHECK predicate must have a Boolean result"
	case "check_predicate_qualifier_absent":
		message = "The CHECK predicate qualifier does not name the target table"
	case "check_predicate_column_absent":
		message = "The CHECK predicate references a column that does not exist"
	case "check_predicate_system_column":
		message = "PostgreSQL rejects this system column in a CHECK predicate"
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Metadata: map[string]any{
			"action": "add_constraint", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name,
			"constraint": s.DDL.Alter[0].Name, "reason_code": reason,
		},
	}}, nil
}

func (r pgAddCheckNameRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, missing := assessPGAddCheckName(s); missing != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_check_constraint_name", RequiredFacts: []string{missing}}}
		}
	}
	return nil
}
