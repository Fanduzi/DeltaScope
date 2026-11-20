package spec

// PGColumnTraversal binds observed column traversal blockers to one target.
type PGColumnTraversal struct {
	Schema, Table, Column                                            string
	Inherited, DirectPartitionKey, AttachedChildren, StoredGenerated bool
	Partition, ColumnExists, System                                  bool
}

// PGColumnTraversalTarget bounds lookups to the nonordinary shapes for this check.
func PGColumnTraversalTarget(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || len(s.DDL.Alter) != 1 || s.DDL.PGTableCatalogQualified ||
		s.DDL.Table == nil || s.Metadata == nil || !PG17Observed(s.Metadata.Instance) || s.Metadata.TargetTable == nil {
		return ""
	}
	table, snapshot := s.DDL.Table, s.Metadata.TargetTable
	if table.Schema == "" || table.Schema == "pg_temp" || table.Schema != s.Metadata.Schema || table.Schema != snapshot.Schema ||
		!snapshot.Exists || snapshot.Table == nil || snapshot.Table.Name != table.Name || snapshot.PGRelation.Name != table.Name ||
		(snapshot.PGRelation.Kind != "r" && snapshot.PGRelation.Kind != "p") || snapshot.PGColumns == nil || snapshot.PGColumns.Typed || snapshot.PGColumns.Ordinary {
		return ""
	}
	a := s.DDL.Alter[0]
	switch a.Action {
	case "drop_column", "rename_column", "set_data_type", "drop_expression", "add_column":
		return a.Name
	default:
		return ""
	}
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
