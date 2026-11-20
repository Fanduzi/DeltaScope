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
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 || !PGOrdinaryTableBound(s) {
		return "", false
	}
	alter := s.DDL.Alter[0]
	if alter.Action != "set_data_type" {
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
