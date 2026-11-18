package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func pgColumnIdentityAction(s spec.Statement, action string) bool {
	return s.Unsupported == nil && s.DDL != nil && s.DDL.Operation == spec.DDLOperationAlterTable && appliesToAlterActions(s, action)
}

// pgAlterRelation answers only existence in the bound namespace. A schema hint
// cannot turn an unqualified name's local miss into search-path-wide absence.
func pgAlterRelation(s spec.Statement) (exists, known bool) {
	snapshot, ok := targetTableSnapshot(s)
	if !ok || s.DDL == nil || s.DDL.Table == nil {
		return false, false
	}
	table := s.DDL.Table
	schema := s.Metadata.Schema
	if schema == "" || schema == "pg_temp" || snapshot.Schema != schema ||
		(table.Schema != "" && table.Schema != schema) ||
		table.Name == "" || snapshot.PGRelation.Name != table.Name ||
		(snapshot.Table != nil && snapshot.Table.Name != table.Name) {
		return false, false
	}
	if snapshot.PGRelation.Kind != "" {
		return true, true
	}
	return false, table.Schema != "" && !snapshot.Exists
}

func pgGuardedAlterTableMissingFact(s spec.Statement) string {
	if !pgConstraintTargetVersionKnown(s) {
		return "instance.version"
	}
	if _, known := pgAlterRelation(s); !known {
		return "target_table.pg_relation"
	}
	return ""
}

func pgColumnIdentityTarget(s spec.Statement, index int, ruleID string) (reason, missing string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); known && !exists {
		return "", "" // The parent existence check owns the outcome.
	}
	if len(s.DDL.Alter) != 1 {
		return "", "target_table.pg_columns" // PostgreSQL can reorder subcommands by execution phase.
	}
	a := s.DDL.Alter[index]
	catalog := pgColumnTargetCatalog(s, index)
	member, known := catalog.FindColumn(a.Name)
	if !known {
		return "", "target_table.pg_columns"
	}
	if a.Action == "add_column" {
		switch {
		case member == nil:
			return "", ""
		case member.System:
			return "system_column_name_conflict", ""
		case a.PGAddColumnIfNotExists:
			return "", ""
		default:
			return "column_already_exists", ""
		}
	}
	if ruleID == ruleIDAlterRenameColumnExistsRequire {
		switch {
		case member == nil:
			return "column_absent", ""
		case member.System:
			return "system_column_inapplicable", ""
		default:
			return "", ""
		}
	}
	if member == nil || member.System {
		return "", "" // The source rule owns the refusal.
	}
	column, ok := alterColumnDefinition(a)
	if !ok || column.Name == "" {
		return "", "column.rename_destination"
	}
	destination, known := catalog.FindColumn(column.Name)
	switch {
	case !known:
		return "", "target_table.pg_columns"
	case destination == nil:
		return "", ""
	case destination.System:
		return "system_column_name_conflict", ""
	default:
		return "column_already_exists", ""
	}
}

func pgAddColumnNoop(s spec.Statement, index int) bool {
	if !pgConstraintTargetVersionKnown(s) {
		return false
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return true
	}
	if len(s.DDL.Alter) != 1 || !s.DDL.Alter[index].PGAddColumnIfNotExists {
		return false
	}
	member, known := pgColumnTargetState(s, index)
	return known && member != nil && !member.System
}

func evaluatePGColumnIdentity(ctx context.Context, s spec.Statement, action, ruleID string, level rule.Level) ([]rule.Finding, error) {
	var findings []rule.Finding
	for i, a := range s.DDL.Alter {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if a.Action != action {
			continue
		}
		reason, _ := pgColumnIdentityTarget(s, i, ruleID)
		if reason == "" {
			continue
		}
		name := a.Name
		metadata := map[string]any{"table": s.DDL.Table.Name, "action": a.Action, "exists": reason != "column_absent", "reason_code": reason}
		if ruleID == ruleIDAlterRenameColumnTargetExistsForbid {
			name = a.Column.Definition.Name
			metadata["source_column"], metadata["target_column"] = a.Name, name
		} else {
			metadata["name"] = name
		}
		message := fmt.Sprintf("column %q already exists on table %q", name, s.DDL.Table.Name)
		switch reason {
		case "column_absent":
			message = fmt.Sprintf("column %q does not exist on table %q", name, s.DDL.Table.Name)
		case "system_column_inapplicable":
			message = fmt.Sprintf("PostgreSQL does not allow renaming system column %q", name)
		case "system_column_name_conflict":
			message = fmt.Sprintf("column name %q conflicts with a PostgreSQL system column", name)
		}
		findings = append(findings, rule.Finding{
			Level: level, Message: message,
			Suggestion: "verify the exact source and destination names against the current PostgreSQL column catalog",
			Metadata:   metadata,
		})
	}
	return findings, nil
}

func pgColumnIdentityGaps(s spec.Statement, action, ruleID string) []rule.EvidenceGap {
	var gaps []rule.EvidenceGap
	seen := map[string]bool{}
	for i, a := range s.DDL.Alter {
		if a.Action != action {
			continue
		}
		_, missing := pgColumnIdentityTarget(s, i, ruleID)
		if missing != "" && !seen[missing] {
			gaps = append(gaps, rule.EvidenceGap{ReasonCode: "unproven_column_identity", RequiredFacts: []string{missing}})
			seen[missing] = true
		}
	}
	return gaps
}
