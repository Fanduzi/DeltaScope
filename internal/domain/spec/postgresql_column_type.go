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

// PGColumnTypeState records the catalog premises for a declared type.
type PGColumnTypeState struct {
	Target                   PGColumnTypeTarget
	SchemaExists, BaseExists bool
	Resolved, Defined        bool
	Pseudo                   bool
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
