package spec

// PGReplicaIdentityIndex is a private attachment observation for one index
// candidate and its exact table. A zero target is unobserved. An empty Kind
// on an observed target means the candidate relation is absent.
type PGReplicaIdentityIndex struct {
	Schema, Table, Name                    string
	Kind                                   string
	IndexKnown, OnTable                    bool
	CanUnique, CanUniqueKnown              bool
	Unique, Immediate, Expression, Partial bool
	// KeyAttributesValid is false for a system or nullable key attribute.
	// Incomplete key positions or attributes leave KeyAttributesKnown false.
	KeyAttributesValid, KeyAttributesKnown bool
}

// PGReplicaIdentityIndexTarget returns the single authored index selection.
// PostgreSQL can reorder actions, so another action invalidates target proof.
func PGReplicaIdentityIndexTarget(s Statement) string {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterTable || s.DDL.Table == nil || len(s.DDL.Alter) != 1 {
		return ""
	}
	a := s.DDL.Alter[0]
	if a.Action == "replica_identity" && a.Options["identity"] == "using_index" {
		return a.Name
	}
	return ""
}

// PGReplicaIdentityTableBound checks the target binding before the index read.
func PGReplicaIdentityTableBound(s Statement) bool {
	if s.DDL == nil || s.DDL.Table == nil || s.DDL.PGTableCatalogQualified || s.Metadata == nil || s.Metadata.TargetTable == nil {
		return false
	}
	table, snapshot := s.DDL.Table, s.Metadata.TargetTable
	return table.Name != "" && table.Schema != "" && table.Schema != "pg_temp" &&
		s.Metadata.Schema == table.Schema && snapshot.Schema == table.Schema && snapshot.Exists &&
		snapshot.Table != nil && snapshot.Table.Name == table.Name &&
		snapshot.PGRelation.Name == table.Name && snapshot.PGRelation.Kind == "r" &&
		snapshot.PGColumns != nil && snapshot.PGColumns.Ordinary && !snapshot.PGColumns.Typed
}
