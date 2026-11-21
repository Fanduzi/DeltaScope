package spec

import "strconv"

// PGCompositeDeclaration retains exact CREATE TYPE or single-action ALTER TYPE facts.
type PGCompositeDeclaration struct {
	Target        PGDestructiveTableTarget
	Action        string
	Attributes    []string
	Name, NewName string
	IfExists      bool
}

// PGCompositeState binds namespace, type and relation facts to an exact target.
type PGCompositeState struct {
	Target                     PGDestructiveTableTarget
	SchemaExists, TypeOccupied bool
	RelationKind               string
	Attributes                 []string
}

// PGCompositeLookup returns the exact type creation or attribute mutation target.
func PGCompositeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.PGComposite == nil {
		return nil
	}
	decl := s.DDL.PGComposite
	if !decl.Target.Qualified() || decl.Target.Table.Schema == "pg_toast_temp" || decl.Action == "" {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_composite_type", Schema: decl.Target.Table.Schema, Name: decl.Target.Table.Name,
		Qualifiers: map[string]string{"attributes": strconv.FormatBool(decl.Action != "create")}}
}
