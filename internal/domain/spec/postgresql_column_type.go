package spec

import "strings"

// PGColumnTypeTarget carries an exact two-part destination name and array intent.
type PGColumnTypeTarget struct {
	Schema, Name string
	Array        bool
}

// Qualified reports whether the destination can be resolved by exact catalog identity.
func (t PGColumnTypeTarget) Qualified() bool {
	return t.Schema != "" && t.Name != "" && t.Schema != "pg_temp" &&
		!strings.HasPrefix(t.Schema, "pg_temp_") && !strings.HasPrefix(t.Schema, "pg_toast_temp_")
}

// PGColumnCollation retains an authored collation clause and its exact two-part identity.
// Empty components describe an unmodeled qualification.
type PGColumnCollation struct{ Schema, Name string }

func (c PGColumnCollation) Qualified() bool {
	return (PGColumnTypeTarget{Schema: c.Schema, Name: c.Name}).Qualified()
}

// PGColumnTypeModifiers retains bounded literal facts for built-in modifiers.
// Count records every authored item. Values holds the first two when Known.
type PGColumnTypeModifiers struct {
	Count  int
	Values [2]int32
	Known  bool
}

// PGColumnTypeState records the catalog premises for a declared type.
type PGColumnTypeState struct {
	Target                                      PGColumnTypeTarget
	SchemaExists, BaseExists                    bool
	Resolved, Defined                           bool
	Pseudo                                      bool
	Collation                                   PGColumnCollation
	CollationKnown, CollationExists, Collatable bool
	AssignmentKnown, Assignable                 bool
	SourceSchema, SourceTable, SourceColumn     string
}

// PGColumnTypeSource checks the source precondition of one PG17 TYPE action.
// A known failure stops destination assessment because the source must resolve first.
func PGColumnTypeSource(s Statement) (reason string, known bool) {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 {
		return "", false
	}
	alter := s.DDL.Alter[0]
	if alter.Action != "set_data_type" {
		return "", false
	}
	if !PGOrdinaryTableBound(s) {
		if facts := BoundPGColumnTraversal(s); facts != nil {
			switch {
			case facts.StoredGenerated && alter.Options["has_using"] == "true":
				return "generated_column_using_inapplicable", true
			case facts.Inherited:
				return "column_type_inherited", true
			case facts.DirectPartitionKey:
				return "column_type_partition_key", true
			}
		}
		return "", false
	}
	member, known := s.Metadata.TargetTable.PGColumns.FindColumn(alter.Name)
	switch {
	case !known:
		return "", false
	case member == nil:
		return "column_absent", true
	case member.System:
		return "system_column_inapplicable", true
	case member.Generated == "s" && alter.Options["has_using"] == "true":
		return "generated_column_using_inapplicable", true
	}
	return "", true
}

// PGColumnTypeDestinationPremises checks the destination after source eligibility.
func PGColumnTypeDestinationPremises(s Statement) (reason, fact string) {
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
	if reason, fact := pgColumnTypeModifiersAssessment(target, s.DDL.Alter[0].PGColumnTypeModifiers); reason != "" || fact != "" {
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

func pgColumnTypeModifiersAssessment(target PGColumnTypeTarget, mods *PGColumnTypeModifiers) (string, string) {
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
