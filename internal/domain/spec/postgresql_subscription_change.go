package spec

// PGSubscriptionChange carries authored local subscription premises only.
type PGSubscriptionChange struct {
	Name, Action, RenameTo string
	Owner                  PGTableOwnerTarget
	IfExists               bool
	Publications           []string
	Options                []PGReplicationOption
}

// PGReplicationOption keeps the parsed option order and bounded values.
type PGReplicationOption struct {
	Name, Text  string
	Known, Bare bool
	Integer     bool
}

// PGSubscriptionChangeState contains observed current-database facts.
type PGSubscriptionChangeState struct {
	Name, RenameTo                                string
	Owner                                         PGTableOwnerTarget
	Exists, Enabled, SlotPresent, TwoPhaseEnabled bool
	DestinationOccupied, OwnerExists              bool
	Publications                                  []string
}

// PGSubscriptionChangeLookup requests exact subscription and role facts.
func PGSubscriptionChangeLookup(s Statement) *ObjectLookupRequest {
	if s.Dialect != DialectPostgreSQL || s.Kind != KindDDL || s.Unsupported != nil || s.DDL == nil || s.DDL.PGSubscriptionChange == nil {
		return nil
	}
	d := s.DDL.PGSubscriptionChange
	if d.Name == "" || d.Owner.Kind != "" && !d.Owner.Valid() {
		return nil
	}
	return &ObjectLookupRequest{Type: "pg_subscription_change", Name: d.Name,
		Qualifiers: map[string]string{"rename_to": d.RenameTo, "role_kind": d.Owner.Kind, "role_name": d.Owner.Name}}
}
