package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgColumnTypeDestinationRule struct{ level rule.Level }

func newPGColumnTypeDestinationRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgColumnTypeDestinationRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgColumnTypeDestinationRule) ID() string { return ruleIDPGAlterColumnTypeDestinationValid }

func (r pgColumnTypeDestinationRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && pgColumnIdentityAction(s, "set_data_type")
}

func assessPGColumnTypeDestination(s spec.Statement) (reason, fact string) {
	if s.Metadata == nil || !spec.PG17Observed(s.Metadata.Instance) {
		return "", "instance.version"
	}
	if exists, known := pgAlterRelation(s); s.DDL.PGTableIfExists && known && !exists {
		return "", ""
	}
	sourceFailure, known := spec.PGColumnTypeSource(s)
	if !known {
		return "", "target_table.pg_columns"
	}
	if sourceFailure != "" {
		return "", ""
	}
	target := s.DDL.Alter[0].PGColumnType
	if !target.Qualified() {
		return "", "column.type_destination_identity"
	}
	observed := s.Metadata.PGColumnType
	if observed == nil || observed.Target != target {
		return "", "column.pg_type_destination"
	}
	switch {
	case !observed.SchemaExists:
		return "destination_type_schema_absent", ""
	case !observed.BaseExists:
		return "destination_type_absent", ""
	case target.Array && !observed.Resolved:
		return "destination_array_type_absent", ""
	case !observed.Resolved:
		return "", "column.pg_type_destination"
	case !observed.Defined:
		return "destination_type_undefined", ""
	case observed.Pseudo:
		return "destination_pseudo_type_inapplicable", ""
	}
	if reason, fact := assessPGColumnTypeModifiers(target, s.DDL.Alter[0].PGColumnTypeModifiers); reason != "" || fact != "" {
		return reason, fact
	}
	if collation := s.DDL.Alter[0].PGColumnCollation; collation != nil {
		if !collation.Qualified() {
			return "", "column.collation_identity"
		}
		if !observed.CollationKnown || observed.Collation != *collation {
			return "", "column.pg_collation"
		}
		if !observed.CollationExists {
			return "destination_collation_absent", ""
		}
		if !observed.Collatable {
			return "destination_type_not_collatable", ""
		}
	}
	action, table := s.DDL.Alter[0], s.DDL.Table
	if !target.Array && action.Options["has_using"] != "true" {
		if !observed.AssignmentKnown || observed.SourceSchema != table.Schema || observed.SourceTable != table.Name || observed.SourceColumn != action.Name {
			return "", "column.type_assignment"
		}
		if !observed.Assignable {
			return "destination_type_assignment_unavailable", ""
		}
	}
	return "", ""
}

func assessPGColumnTypeModifiers(target spec.PGColumnTypeTarget, mods *spec.PGColumnTypeModifiers) (string, string) {
	if mods == nil {
		return "", ""
	}
	if target.Schema != "pg_catalog" || target.Array || !mods.Known || mods.Count == 0 {
		return "", "column.type_modifiers"
	}
	first, second := mods.Values[0], mods.Values[1]
	switch target.Name {
	case "varchar", "bpchar":
		if mods.Count != 1 || first < 1 || first > 10*1024*1024 {
			return "destination_character_type_modifier_invalid", ""
		}
	case "numeric":
		if mods.Count > 2 || first < 1 || first > 1000 || (mods.Count == 2 && (second < -1000 || second > 1000)) {
			return "destination_numeric_type_modifier_invalid", ""
		}
	default:
		return "", "column.type_modifiers"
	}
	return "", ""
}

func (r pgColumnTypeDestinationRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGColumnTypeDestination(s)
	var message string
	switch reason {
	case "destination_type_schema_absent":
		message = "The PostgreSQL destination type schema does not exist"
	case "destination_type_absent":
		message = "The qualified PostgreSQL destination type does not exist"
	case "destination_array_type_absent":
		message = "The PostgreSQL destination type has no associated array type"
	case "destination_type_undefined":
		message = "The PostgreSQL destination is an undefined shell type"
	case "destination_pseudo_type_inapplicable":
		message = "PostgreSQL does not permit this destination pseudo-type in a table column"
	case "destination_character_type_modifier_invalid":
		message = "The PostgreSQL destination character type requires one length between 1 and 10485760"
	case "destination_numeric_type_modifier_invalid":
		message = "The PostgreSQL numeric destination requires precision from 1 to 1000 and optional scale from -1000 to 1000"
	case "destination_type_assignment_unavailable":
		message = "PostgreSQL has no assignment cast from the selected column type to the destination type"
	case "destination_collation_absent":
		message = "The qualified PostgreSQL collation is unavailable for this database encoding"
	case "destination_type_not_collatable":
		message = "The PostgreSQL destination type does not accept COLLATE"
	default:
		return nil, nil
	}
	return []rule.Finding{{Level: r.level, Message: message,
		Metadata: map[string]any{"action": "set_data_type", "schema": s.DDL.Table.Schema, "table": s.DDL.Table.Name,
			"column": s.DDL.Alter[0].Name, "reason_code": reason},
	}}, nil
}

func (r pgColumnTypeDestinationRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGColumnTypeDestination(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_column_type_destination", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
