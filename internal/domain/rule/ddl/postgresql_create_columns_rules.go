package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgCreateColumnsRule struct{ level rule.Level }

func newPGCreateColumnsRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgCreateColumnsRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgCreateColumnsRule) ID() string { return ruleIDPGCreateColumnsValid }

func (r pgCreateColumnsRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationCreateTable && s.DDL.PGCreateColumns != nil
}

func assessPGCreateColumns(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	decl := s.DDL.PGCreateColumns
	if !decl.Target.Qualified() {
		return "", "create_table.column_namespace"
	}
	destination := s.Metadata.PGCreateDestination
	if destination == nil || destination.Target != decl.Target {
		return "", "create_table.column_destination"
	}
	switch {
	case !destination.SchemaExists:
		return "create_column_destination_schema_absent", ""
	case decl.IfNotExists && destination.RelationExists:
		return "", ""
	case decl.Refusal == "create_column_duplicate":
		return decl.Refusal, ""
	}
	if len(s.Metadata.PGCreateColumnTypes) != len(decl.Types) {
		return "", "create_table.column_types"
	}
	pseudo := false
	for i, target := range decl.Types {
		state := s.Metadata.PGCreateColumnTypes[i]
		if state == nil || state.Target != target {
			return "", "create_table.column_types"
		}
		switch {
		case !state.SchemaExists:
			return "create_column_type_schema_absent", ""
		case !state.Resolved:
			return "create_column_type_absent", ""
		case !state.Defined:
			return "create_column_type_undefined", ""
		}
		pseudo = pseudo || state.Pseudo
	}
	if decl.Refusal != "" {
		return decl.Refusal, ""
	}
	if pseudo {
		return "create_column_type_pseudo", ""
	}
	return "", ""
}

func (r pgCreateColumnsRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGCreateColumns(s)
	var message string
	switch reason {
	case "create_column_destination_schema_absent":
		message = "The PostgreSQL table creation schema does not exist"
	case "create_column_duplicate":
		message = "A PostgreSQL CREATE TABLE column is declared more than once"
	case "create_column_system_name":
		message = "A PostgreSQL CREATE TABLE column conflicts with a system column name"
	case "create_column_type_schema_absent":
		message = "The PostgreSQL column type schema does not exist"
	case "create_column_type_absent":
		message = "The PostgreSQL column type does not exist"
	case "create_column_type_undefined":
		message = "The PostgreSQL column uses an undefined shell type"
	case "create_column_type_pseudo":
		message = "PostgreSQL cannot use this pseudo-type as a table column"
	default:
		return nil, nil
	}
	target := s.DDL.PGCreateColumns.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "table": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgCreateColumnsRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGCreateColumns(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_create_columns", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
