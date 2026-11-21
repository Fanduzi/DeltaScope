package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTypeChangeRule struct{ level rule.Level }

func newPGTypeChangeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTypeChangeRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r pgTypeChangeRule) ID() string { return ruleIDPGTypeChangeValid }

func (r pgTypeChangeRule) AppliesTo(s spec.Statement) bool {
	return s.Dialect == spec.DialectPostgreSQL && s.Kind == spec.KindDDL && s.Unsupported == nil && s.DDL != nil &&
		s.DDL.Operation == spec.DDLOperationAlterType && s.DDL.PGTypeChange != nil
}

func assessPGTypeChange(s spec.Statement) (reason, fact string) {
	if !pgConstraintTargetVersionKnown(s) {
		return "", "instance.version"
	}
	if spec.PGTypeChangeLookup(s) == nil {
		return "", "alter_type.target_identity"
	}
	state := s.Metadata.PGTypeChange
	if state == nil || state.Target != *s.DDL.PGTypeChange {
		return "", "alter_type.namespace_catalog"
	}
	if state.Target.Action == "set_owner" {
		switch {
		case state.Target.Owner.Kind == "public":
			return "alter_type_owner_public", ""
		case state.Target.Owner.Kind == "named" && !state.OwnerExists:
			return "alter_type_owner_absent", ""
		case !state.OwnerExists:
			return "", "alter_type.owner_identity"
		}
	}
	if !state.SourceExists {
		return "alter_type_source_absent", ""
	}
	if !state.Defined && state.Target.Action != "set_owner" {
		return "alter_type_source_undefined", ""
	}
	switch state.Target.Action {
	case "set_owner":
		switch {
		case state.RowType:
			return "alter_type_is_table_row_type", ""
		case state.Array:
			return "alter_type_is_array", ""
		}
	case "rename":
		switch {
		case state.RowType:
			return "alter_type_is_table_row_type", ""
		case state.Array:
			return "alter_type_is_array", ""
		case state.Composite && state.DestinationRelation:
			return "alter_type_relation_occupied", ""
		case state.DestinationType:
			return "alter_type_destination_occupied", ""
		}
	case "set_schema":
		if !state.DestinationSchema {
			return "alter_type_schema_absent", ""
		}
		if state.Array {
			return "alter_type_is_array", ""
		}
		if state.Target.Destination == state.Target.Source.Table.Schema {
			if state.RowType {
				return "alter_type_is_table_row_type", ""
			}
			return "", ""
		}
		switch {
		case state.NamespaceIneligible:
			return "alter_type_schema_ineligible", ""
		case state.DestinationType:
			return "alter_type_destination_occupied", ""
		case state.RowType:
			return "alter_type_is_table_row_type", ""
		case state.Composite && state.DestinationRelation:
			return "alter_type_relation_occupied", ""
		case state.CompanionOccupied:
			return "alter_type_companion_occupied", ""
		}
	default:
		return "", "alter_type.namespace_action"
	}
	return "", ""
}

func (r pgTypeChangeRule) Evaluate(ctx context.Context, s spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(s) {
		return nil, nil
	}
	reason, _ := assessPGTypeChange(s)
	var message string
	switch reason {
	case "alter_type_source_absent":
		message = "The requested PostgreSQL type does not exist"
	case "alter_type_source_undefined":
		message = "The requested PostgreSQL type is an undefined shell"
	case "alter_type_is_table_row_type":
		message = "PostgreSQL requires ALTER TABLE to change this relation's row type"
	case "alter_type_is_array":
		message = "PostgreSQL requires altering the element type to change this array type"
	case "alter_type_schema_absent":
		message = "The PostgreSQL destination schema does not exist"
	case "alter_type_schema_ineligible":
		message = "PostgreSQL disallows moving types into or out of this namespace"
	case "alter_type_destination_occupied":
		message = "A PostgreSQL type already occupies the destination"
	case "alter_type_relation_occupied":
		message = "A PostgreSQL relation already occupies the composite type destination"
	case "alter_type_companion_occupied":
		message = "The PostgreSQL type's associated array name is occupied in the destination schema"
	case "alter_type_owner_public":
		message = "PostgreSQL does not permit PUBLIC as the owner of a type"
	case "alter_type_owner_absent":
		message = "The requested PostgreSQL type owner does not exist"
	default:
		return nil, nil
	}
	target := s.DDL.PGTypeChange.Source.Table
	return []rule.Finding{{Level: r.level, Message: message, Metadata: map[string]any{
		"schema": target.Schema, "type": target.Name, "reason_code": reason,
	}}}, nil
}

func (r pgTypeChangeRule) EvidenceGaps(s spec.Statement) []rule.EvidenceGap {
	if r.AppliesTo(s) {
		if _, fact := assessPGTypeChange(s); fact != "" {
			return []rule.EvidenceGap{{ReasonCode: "unproven_pg_type_change", RequiredFacts: []string{fact}}}
		}
	}
	return nil
}
