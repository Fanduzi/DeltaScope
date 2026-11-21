package spec

// PGForeignOption keeps an authored option name and action. Values stay out of catalog requests.
type PGForeignOption struct {
	Name, Action string
}

// PGWrapperFunction retains a handler or validator selection in authored order.
type PGWrapperFunction struct {
	Kind   string
	Target Table
	Clear  bool
}

// PGWrapperChange retains an exact foreign-data wrapper action.
type PGWrapperChange struct {
	Name, RenameTo string
	Owner          PGTableOwnerTarget
	Functions      []PGWrapperFunction
	Options        []PGForeignOption
}

// PGWrapperFunctionState records lookup results without returning catalog OIDs.
type PGWrapperFunctionState struct {
	Kind            string
	Target          Table
	Exists, Handler bool
}

// PGWrapperChangeState binds the declaration to its observed catalog facts.
type PGWrapperChangeState struct {
	Name, RenameTo              string
	Owner                       PGTableOwnerTarget
	Exists, DestinationOccupied bool
	OwnerExists, OwnerSuperuser bool
	HasValidator                bool
	Options                     []string
	Functions                   []PGWrapperFunctionState
}

// PGWrapperChangeLookup requests wrapper facts for a recognized ALTER action.
func PGWrapperChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil ||
		s.DDL.Operation != DDLOperationAlterForeignDataWrapper || s.DDL.PGWrapperChange == nil {
		return nil
	}
	target := s.DDL.PGWrapperChange
	if target.Name == "" || target.Owner.Kind != "" && !target.Owner.Valid() {
		return nil
	}
	functions := make([]PGWrapperFunction, len(target.Functions))
	copy(functions, target.Functions)
	return &ObjectLookupRequest{Type: "pg_wrapper_change", Name: target.Name, PGWrapperFunctions: functions,
		Qualifiers: map[string]string{"rename_to": target.RenameTo, "role_kind": target.Owner.Kind, "role_name": target.Owner.Name}}
}
