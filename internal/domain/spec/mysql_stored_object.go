package spec

// MySQLStoredObjectTarget describes the catalog identity needed for a trigger or event check.
type MySQLStoredObjectTarget struct {
	Kind    string
	Name    Table
	Related Table
}

// MySQLStoredObjectState contains visible object identities and trigger ordering facts.
type MySQLStoredObjectState struct {
	Target       MySQLStoredObjectTarget
	SourceSeen   bool
	RelatedSeen  bool
	SiblingTable Table
	Timing       TriggerTiming
	Event        TriggerEvent
}

// MySQLStoredObjectLookup requests qualified identities without interpreting the object body.
func MySQLStoredObjectLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectMySQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil {
		return nil
	}
	var target MySQLStoredObjectTarget
	switch {
	case s.DDL.Trigger != nil:
		d := s.DDL.Trigger
		if d.IfExists {
			return nil
		}
		target.Kind, target.Name = "trigger", d.CatalogName
		if d.Order != nil {
			if d.CatalogTable.Schema == "" || d.CatalogTable.Schema != d.CatalogName.Schema {
				return nil
			}
			target.Related = Table{Schema: target.Name.Schema, Name: d.Order.CatalogName}
		}
	case s.DDL.Event != nil:
		d := s.DDL.Event
		if d.IfExists || d.IfNotExists {
			return nil
		}
		target.Kind, target.Name, target.Related = "event", d.CatalogName, d.CatalogRenameTo
		if d.RenameTo != nil && target.Related.Schema == "" {
			return nil
		}
	}
	if target.Name.Schema == "" || target.Name.Name == "" {
		return nil
	}
	return &ObjectLookupRequest{Type: "mysql_stored_object", Schema: target.Name.Schema, Name: target.Name.Name,
		Qualifiers: map[string]string{"kind": target.Kind, "related_schema": target.Related.Schema, "related_name": target.Related.Name}}
}
