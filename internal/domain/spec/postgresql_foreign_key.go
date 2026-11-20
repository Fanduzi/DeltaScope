package spec

// PGForeignKeySelection retains the two authored column lists without joining
// names on commas. An empty referenced list means REFERENCES omitted its columns.
type PGForeignKeySelection struct {
	Columns, ReferencedColumns []string
	DeleteSetColumns           []string
	Reference                  Table
	ReferenceCatalogQualified  bool
}

// ReferenceQualified reports whether the referenced relation can be bound by schema and name.
func (f PGForeignKeySelection) ReferenceQualified() bool {
	return !f.ReferenceCatalogQualified && f.Reference.Schema != "" && f.Reference.Schema != "pg_temp" && f.Reference.Name != ""
}

// PGForeignKeyTarget returns the single table-level ADD FOREIGN KEY declaration.
func PGForeignKeyTarget(s Statement) *PGForeignKeySelection {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return nil
	}
	a := s.DDL.Alter[0]
	if a.Action != "add_constraint" || a.Options["constraint_type"] != "foreign_key" {
		return nil
	}
	return a.PGForeignKey
}
