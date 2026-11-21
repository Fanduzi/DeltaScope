package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgForeignTableChangeRule struct{ level rule.Level }

func newPGForeignTableChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgForeignTableChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgForeignTableChangeRule) ID() string { return ruleIDPGForeignTableChangeValid }

func (r pgForeignTableChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil &&
		s.DDL != nil && s.DDL.Operation == spec.DDLOperationAlterForeignTable && s.DDL.PGForeignTableChange != nil
}

func assessPGForeignTableChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGForeignTableChangeLookup(s) == nil {
		return "", "foreign_table.target_or_action"
	}
	d := s.DDL.PGForeignTableChange
	state := s.Metadata.PGForeignTableChange
	if state == nil || state.Target != d.Target || state.Action != d.Action || state.Column != d.Column || state.Destination != d.Destination || state.Owner != d.Owner {
		return "", "foreign_table.catalog"
	}
	switch state.RelationKind {
	case "":
		if d.IfExists {
			return "", ""
		}
		return "foreign_table_absent", ""
	case "f":
	default:
		return "foreign_table_wrong_kind", ""
	}
	switch d.Action {
	case "alter_options":
		if reason, fact = assessPGForeignOptions(d.Options, state.Options); reason != "" || fact != "" {
			return reason, fact
		}
		if len(d.Options) > 0 && state.HasValidator {
			return "", "foreign_table.validator"
		}
	case "add_column":
		if state.SystemColumn || state.ColumnExists && !d.ColumnGuard {
			return "foreign_table_column_exists", ""
		}
	case "drop_column":
		if state.SystemColumn {
			return "foreign_table_system_column", ""
		}
		if !state.ColumnExists && !d.ColumnGuard {
			return "foreign_table_column_absent", ""
		}
	case "rename_column":
		switch {
		case !state.ColumnExists:
			return "foreign_table_column_absent", ""
		case state.SystemColumn:
			return "foreign_table_system_column", ""
		case state.DestinationColumnExists:
			return "foreign_table_column_exists", ""
		}
	case "set_schema":
		switch {
		case !state.DestinationSchema:
			return "foreign_table_destination_schema_absent", ""
		case state.DestinationForbidden:
			return "foreign_table_destination_forbidden", ""
		case d.Destination == d.Target.Table.Schema:
			return "", ""
		case state.DestinationOccupied:
			return "foreign_table_destination_occupied", ""
		}
	case "rename":
		if state.DestinationOccupied {
			return "foreign_table_destination_occupied", ""
		}
	case "set_owner":
		if !state.OwnerExists {
			if d.Owner.Kind == "named" || d.Owner.Kind == "public" {
				return "foreign_table_owner_absent", ""
			}
			return "", "foreign_table.owner_identity"
		}
	}
	return "", ""
}

func (r pgForeignTableChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGForeignTableChange(s)
	messages := map[string]string{
		"foreign_table_absent":                    "The PostgreSQL foreign table does not exist",
		"foreign_table_wrong_kind":                "PostgreSQL requires a foreign table for this action",
		"foreign_table_column_exists":             "The PostgreSQL foreign table column name is occupied",
		"foreign_table_system_column":             "PostgreSQL cannot change a system column",
		"foreign_table_column_absent":             "The PostgreSQL foreign table column does not exist",
		"foreign_table_destination_schema_absent": "The PostgreSQL foreign table destination schema does not exist",
		"foreign_table_destination_forbidden":     "PostgreSQL disallows this foreign table namespace change",
		"foreign_table_destination_occupied":      "The PostgreSQL foreign table destination is occupied",
		"foreign_table_owner_absent":              "The PostgreSQL foreign table owner does not exist",
		"foreign_option_exists":                   "The PostgreSQL foreign option already exists",
		"foreign_option_absent":                   "The PostgreSQL foreign option does not exist",
	}
	message := messages[reason]
	if message == "" {
		return nil, nil
	}
	t := s.DDL.PGForeignTableChange.Target.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": t.Schema, "table": t.Name, "reason_code": reason,
	}}}, nil
}

func (r pgForeignTableChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGForeignTableChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_foreign_table_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
