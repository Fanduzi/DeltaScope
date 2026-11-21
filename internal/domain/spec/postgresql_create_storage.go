package spec

// PGCreateStorage retains explicit PostgreSQL table storage declarations.
type PGCreateStorage struct {
	Target             PGDestructiveTableTarget
	IfNotExists        bool
	SessionDependent   bool
	Method, Tablespace string
	Options            []PGTableReloption
}

// PGCreateStorageState binds storage catalog facts to the exact creation request.
type PGCreateStorageState struct {
	Target                     PGDestructiveTableTarget
	SchemaExists, TargetExists bool
	Method, MethodKind         string
	Tablespace                 string
	TablespaceExists, Global   bool
}

// PGCreateStorageLookup requests exact persistent table creation facts.
func PGCreateStorageLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationCreateTable || s.DDL.PGCreateStorage == nil {
		return nil
	}
	storage := s.DDL.PGCreateStorage
	if storage.SessionDependent || !storage.Target.Qualified() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_create_storage", Schema: storage.Target.Table.Schema, Name: storage.Target.Table.Name,
		Qualifiers: map[string]string{"method": storage.Method, "tablespace": storage.Tablespace}}
}
