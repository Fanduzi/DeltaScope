package spec

// PGColumnTraversal binds observed column traversal blockers to one target.
type PGColumnTraversal struct {
	Schema, Table, Column                                                    string
	Inherited, DirectPartitionKey, AttachedChildren, StoredGenerated         bool
	Partition, ColumnExists, System                                          bool
	ParentNotNull, NullableDescendant                                        bool
	ValidatedNotNullCheck, IdentityChangeBlocked                             bool `json:"-"`
	RenameDestination                                                        string
	RenameDescendantsKnown, RenameDescendantConflict, TypeDescendantConflict bool
	RenameDestinationKind                                                    int // -1 system column, 0 absent, 1 user column.
}

// PGColumnRenameDestination returns the exact decoded destination for a rename action.
func PGColumnRenameDestination(a Alter) string {
	if a.Action == "rename_column" && a.Column != nil && a.Column.Definition != nil {
		return a.Column.Definition.Name
	}
	return ""
}

// PGColumnTraversalTarget binds a traversal or nullability-proof lookup.
func PGColumnTraversalTarget(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 || s.DDL.PGTableCatalogQualified ||
		s.DDL.Table == nil || s.Metadata == nil || !PG17Observed(s.Metadata.Instance) || s.Metadata.TargetTable == nil {
		return ""
	}
	table, snapshot := s.DDL.Table, s.Metadata.TargetTable
	if table.Schema == "" || table.Schema == "pg_temp" || table.Schema != s.Metadata.Schema || table.Schema != snapshot.Schema ||
		!snapshot.Exists || snapshot.Table == nil || snapshot.Table.Name != table.Name || snapshot.PGRelation.Name != table.Name ||
		(snapshot.PGRelation.Kind != "r" && snapshot.PGRelation.Kind != "p") || snapshot.PGColumns == nil || snapshot.PGColumns.Typed {
		return ""
	}
	a := s.DDL.Alter[0]
	if snapshot.PGColumns.Ordinary {
		if a.Action == "set_not_null" && PGOrdinaryTableBound(s) {
			if c, known := snapshot.PGColumns.FindColumn(a.Name); known && c != nil && !c.System && !c.NotNull && c.TypeWidth == 32 {
				return a.Name
			}
		}
		return ""
	}
	switch a.Action {
	case "drop_column", "rename_column", "set_data_type", "drop_expression", "add_column", "drop_not_null", "set_not_null":
		return a.Name
	case "add_identity", "drop_identity", "set_generated":
		if f := a.PGColumnTransition; f != nil && !f.Unmodeled && (snapshot.PGRelation.Kind == "r" || s.DDL.PGTableOnly) {
			return a.Name
		}
	}
	return ""
}

// BoundPGColumnTraversal returns the observation for the exact current target.
func BoundPGColumnTraversal(s Statement) *PGColumnTraversal {
	name := PGColumnTraversalTarget(s)
	if name == "" {
		return nil
	}
	facts, table := s.Metadata.PGColumnTraversal, s.DDL.Table
	if facts == nil || facts.Schema != table.Schema || facts.Table != table.Name || facts.Column != name {
		return nil
	}
	return facts
}
