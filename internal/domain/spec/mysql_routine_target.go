package spec

// MySQLRoutineTarget binds a visible stored routine to a qualified declaration.
type MySQLRoutineTarget struct {
	Name Table
	Kind RoutineKind
}

// MySQLRoutineLookup requests visible stored routine identity only.
func MySQLRoutineLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectMySQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.Routine == nil {
		return nil
	}
	d := s.DDL.Routine
	if d.IfExists || d.IfNotExists || s.DDL.Operation == DDLOperationCreateFunction && d.ReturnsType == "" {
		return nil
	}
	if d.CatalogTarget.Schema == "" || d.CatalogTarget.Name == "" || (d.Kind != RoutineKindProcedure && d.Kind != RoutineKindFunction) {
		return nil
	}
	for _, c := range d.CatalogTarget.Name {
		if c > 127 {
			return nil
		}
	}
	return &ObjectLookupRequest{Type: "mysql_routine", Schema: d.CatalogTarget.Schema, Name: d.CatalogTarget.Name,
		Qualifiers: map[string]string{"kind": string(d.Kind)}}
}
