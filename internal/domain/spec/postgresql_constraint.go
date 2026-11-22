package spec

// PGConstraintState contains only catalog facts needed by constraint target checks.
// Names are decoded PostgreSQL identifiers, compared exactly, never case-folded.
type PGConstraintState struct {
	Name      string
	Type      string // pg_constraint.contype, not a declaration's assumed kind
	Validated bool
	// DerivedForeignKey is a positive observation of a partition-derived FK.
	DerivedForeignKey bool `json:"-"`
	// InheritanceKnown marks an observed persistent-table CHECK or partition foreign key.
	InheritanceKnown bool `json:"-"`
	Inherited        bool `json:"-"`
	// PartitionCheckHasChildren records whether a partitioned CHECK propagates to direct children.
	PartitionCheckHasChildren      bool `json:"-"`
	PartitionCheckHasChildrenKnown bool `json:"-"`
	// CheckValidationHasChildren observes whether VALIDATE must traverse children.
	CheckValidationHasChildren   bool `json:"-"`
	CheckValidationChildrenKnown bool `json:"-"`
}

// FindPGConstraint returns a uniquely proven member or proven absence. False
// means the catalog/name/type is unobserved or ambiguous, never an absence claim.
func (s TableSnapshot) FindPGConstraint(name string) (*PGConstraintState, bool) {
	if name == "" || s.PGConstraints == nil {
		return nil, false
	}
	var found *PGConstraintState
	for i := range s.PGConstraints {
		c := &s.PGConstraints[i]
		if c.Name == "" {
			return nil, false
		}
		if c.Name != name {
			continue
		}
		if found != nil {
			return nil, false
		}
		switch c.Type {
		case "c", "f", "n", "p", "u", "t", "x":
		default:
			return nil, false
		}
		found = c
	}
	return found, true
}
