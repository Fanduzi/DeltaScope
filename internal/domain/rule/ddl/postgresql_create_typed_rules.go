package ddl

import (
	"context"
	"slices"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateTypedRule struct{ level rule.Level }

func newPGCreateTypedRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateTypedRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateTypedRule) ID() string { return ruleIDPGCreateTypedValid }

func (r pgCreateTypedRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationCreateTable && s.DDL.PGCreateTyped != nil
}

func assessPGCreateTyped(s spec.Statement) (reason, gap string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGCreateTypedLookup(s) == nil {
		return "", "create_table.typed_namespace"
	}
	typed := s.DDL.PGCreateTyped
	state := s.Metadata.PGCreateTyped
	if state == nil || state.Target != typed.Target || state.Type != typed.Type {
		return "", "create_table.typed_catalog"
	}
	switch {
	case !state.TargetSchema:
		return "create_typed_destination_schema_absent", ""
	case typed.IfNotExists && state.TargetExists:
		return "", ""
	case !state.TypeSchema:
		return "create_typed_type_schema_absent", ""
	case !state.TypeExists:
		return "create_typed_type_absent", ""
	case !state.Defined:
		return "create_typed_type_undefined", ""
	case !state.Composite:
		return "create_typed_type_not_standalone_composite", ""
	case len(s.DDL.Columns) != 0 && state.Columns == nil:
		return "", "create_table.typed_columns"
	}
	seen := make(map[string]bool, len(s.DDL.Columns))
	for _, column := range s.DDL.Columns {
		if !slices.Contains(state.Columns, column.Name) {
			return "create_typed_option_column_absent", ""
		}
		if seen[column.Name] {
			return "create_typed_option_column_duplicate", ""
		}
		seen[column.Name] = true
	}
	return "", ""
}

func (r pgCreateTypedRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateTyped(s)
	var message string
	switch reason {
	case "create_typed_destination_schema_absent":
		message = "The PostgreSQL table creation schema does not exist"
	case "create_typed_type_schema_absent":
		message = "The PostgreSQL typed table source schema does not exist"
	case "create_typed_type_absent":
		message = "The PostgreSQL typed table source type does not exist"
	case "create_typed_type_undefined":
		message = "The PostgreSQL typed table source is an undefined shell type"
	case "create_typed_type_not_standalone_composite":
		message = "PostgreSQL requires a standalone composite type for CREATE TABLE OF"
	case "create_typed_option_column_absent":
		message = "A PostgreSQL typed table column option names a missing source column"
	case "create_typed_option_column_duplicate":
		message = "A PostgreSQL typed table column option is repeated"
	default:
		return nil, nil
	}
	target := s.DDL.PGCreateTyped.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "table": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateTypedRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateTyped(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_typed", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
