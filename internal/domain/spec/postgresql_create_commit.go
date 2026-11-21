package spec

// PGCreateCommit retains a PostgreSQL CREATE TABLE persistence and ON COMMIT declaration.
type PGCreateCommit struct {
	Target      PGDestructiveTableTarget
	Persistence string
	Action      string
	IfNotExists bool
}

// PGCreateDestinationState binds the destination facts for a qualified declaration.
type PGCreateDestinationState struct {
	Target         PGDestructiveTableTarget
	SchemaExists   bool
	RelationExists bool
}

// PGCreateDestinationLookup returns the literal creation namespace and relation.
func PGCreateDestinationLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationCreateTable {
		return nil
	}
	var target PGDestructiveTableTarget
	if commit := s.DDL.PGCreateCommit; commit != nil {
		target = commit.Target
	} else if columns := s.DDL.PGCreateColumns; columns != nil {
		target = columns.Target
	} else if expression := s.DDL.PGCreateExpression; expression != nil && !expression.Unbound {
		target = expression.Target
	} else if foreignKeys := s.DDL.PGCreateForeignKeys; foreignKeys != nil && !foreignKeys.Unbound {
		target = foreignKeys.Target
	} else if reason, _ := PGCreateKeyRefusal(s.DDL); reason != "" {
		target = s.DDL.PGCreateKeys.Target
	}
	if !target.Qualified() || target.Table.Schema == "pg_toast_temp" {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_create_destination", Schema: target.Table.Schema, Name: target.Table.Name}
}
