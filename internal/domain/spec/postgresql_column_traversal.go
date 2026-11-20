package spec

// PGColumnTraversal binds observed column traversal blockers to one target.
type PGColumnTraversal struct {
	Schema, Table, Column                           string
	Inherited, DirectPartitionKey, AttachedChildren bool
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
	if a.Action != "drop_column" && a.Action != "rename_column" {
		return ""
	}
	return a.Name
}
