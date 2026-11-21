package spec

// PGSequenceChange preserves the exact qualified sequence target and ordered options.
type PGSequenceChange struct {
	Target      PGDestructiveTableTarget
	Persistence string
	IfExists    bool
	IfNotExists bool
	RenameTo    string
	MoveTo      string
	Options     []PGSequenceChangeOption
}

// PGSequenceChangeOption keeps each option occurrence and its typed argument.
type PGSequenceChangeOption struct {
	Name     string
	Value    *int64
	Argument []string
	Width    int
	Known    bool
}

// PGSequenceChangeState binds catalog definition facts without reading issuance state.
type PGSequenceChangeState struct {
	Target               PGDestructiveTableTarget
	SchemaExists         bool
	Kind                 string
	Width                int
	Definition           *SequenceDefinition
	Ownership            *PGSequenceOwnershipState
	RenameTo, MoveTo     string
	DestinationSchema    bool
	DestinationOccupied  bool
	DestinationForbidden bool
	Owned                bool
}

// PGSequenceOwnershipState records eligibility for an explicit OWNED BY target.
type PGSequenceOwnershipState struct {
	Table, Sequence  Table
	Column           string
	Kind             string
	OwnerMatches     bool
	ColumnExists     bool
	IdentitySequence bool
}

// Ownership returns the first explicit OWNED BY target without resolving names.
func (d *PGSequenceChange) Ownership() []string {
	for _, option := range d.Options {
		if option.Name == "owned_by" {
			return option.Argument
		}
	}
	return nil
}

// PGSequenceChangeLookup requests definition facts for an exact sequence identity.
func PGSequenceChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.PGSequenceChange == nil {
		return nil
	}
	if s.DDL.Operation != DDLOperationCreateSequence && s.DDL.Operation != DDLOperationAlterSequence {
		return nil
	}
	decl := s.DDL.PGSequenceChange
	if !decl.Target.Qualified() || decl.Target.Table.Schema == "pg_toast_temp" || decl.MoveTo == "pg_temp" {
		return nil
	}
	lookup := &ObjectLookupRequest{Type: "pg_sequence_change", Schema: decl.Target.Table.Schema, Name: decl.Target.Table.Name,
		Qualifiers: map[string]string{"rename_to": decl.RenameTo, "move_to": decl.MoveTo}}
	owned := decl.Ownership()
	if len(owned) == 1 && owned[0] == "none" || len(owned) == 3 && (PGDestructiveTableTarget{Table: Table{Schema: owned[0], Name: owned[1]}}).Qualified() && owned[0] != "pg_toast_temp" {
		if s.DDL.Operation == DDLOperationCreateSequence {
			lookup.Qualifiers["create"] = "true"
		}
		if len(owned) == 3 {
			lookup.Qualifiers["owner_schema"] = owned[0]
			lookup.Qualifiers["owner_table"] = owned[1]
			lookup.Qualifiers["owner_column"] = owned[2]
		} else {
			lookup.Qualifiers["owner_none"] = "true"
		}
	}
	return lookup
}
