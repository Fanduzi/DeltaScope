package spec

// PGIndexDestinationTarget retains one ALTER INDEX rename or tablespace destination.
type PGIndexDestinationTarget struct {
	Source      PGDestructiveTableTarget
	Action      string
	Destination string
	IfExists    bool
}

// PGIndexDestinationState binds catalog facts to the authored source and destination.
type PGIndexDestinationState struct {
	Source            PGDestructiveTableTarget
	Action            string
	Destination       string
	Kind              string
	DestinationExists bool
	HasRowType        bool
	ConstraintBacked  bool
	SameTablespace    bool
	Mapped            bool
	Global            bool
}

// PGIndexDestinationLookup binds a qualified ALTER INDEX target.
func PGIndexDestinationLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterIndex || s.DDL.PGIndexDestination == nil {
		return nil
	}
	t := s.DDL.PGIndexDestination
	if !t.Source.Qualified() || t.Destination == "" || t.Action != "rename" && t.Action != "set_tablespace" {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_index_destination", Schema: t.Source.Table.Schema, Name: t.Source.Table.Name,
		Qualifiers: map[string]string{"action": t.Action, "destination": t.Destination}}
}
