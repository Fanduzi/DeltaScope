package spec

// PGPartitionConstraintState binds fresh partition members to a qualified relation.
// Nil lists leave the corresponding directory unknown.
type PGPartitionConstraintState struct {
	Target        Table
	Columns       []PGColumnState
	SystemColumns []PGColumnState
	PartitionKeys []PGPartitionConstraintKey
}

// FindColumn binds an exact member of the partitioned table's observed directory.
func (s *PGPartitionConstraintState) FindColumn(name string) (*PGColumnState, bool) {
	return findPGColumn(s.Columns, s.SystemColumns, name)
}

// PGPartitionConstraintKey carries only eligibility facts, with no catalog IDs or expressions.
type PGPartitionConstraintKey struct {
	Name         string `json:"name"`
	Expression   bool   `json:"expression"`
	CollationOK  bool   `json:"collation_ok"`
	EqualityRead bool   `json:"equality_read"`
	EqualityOK   bool   `json:"equality_ok"`
}

// PGPartitionConstraintLookup identifies one direct primary or unique constraint on a partitioned table.
func PGPartitionConstraintLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 ||
		s.Metadata == nil || s.Metadata.TargetTable == nil {
		return nil
	}
	action := s.DDL.Alter[0]
	if action.Action != "add_constraint" || action.Options["using_index"] != "false" ||
		(action.Options["constraint_type"] != "primary_key" && action.Options["constraint_type"] != "unique") {
		return nil
	}
	identity := PGDestructiveTableTarget{Table: *s.DDL.Table, CatalogQualified: s.DDL.PGTableCatalogQualified}
	if !identity.Qualified() {
		return nil
	}
	observed := s.Metadata.TargetTable
	if !observed.Exists || observed.Schema != identity.Table.Schema || observed.PGRelation.Name != identity.Table.Name || observed.PGRelation.Kind != "p" {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_partition_constraint", Schema: identity.Table.Schema, Name: identity.Table.Name}
}
