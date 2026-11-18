package spec

// PGColumnTransition preserves authored existing-column generation changes.
// Legacy action/options projections lose guards, duplicates and 64-bit values.
type PGColumnTransition struct {
	IfExists  bool
	Mode      string // first a/d member; ModeCount must prove uniqueness
	ModeCount int
	Options   []SequenceOption // source order, never last-wins
	Unmodeled bool
}

// PGIdentitySequenceState contains only dependency-bound stable catalog facts.
// TypeWidth is the sequence's actual type, not the owning column's type.
type PGIdentitySequenceState struct {
	TypeWidth  int
	Definition SequenceDefinition
}

// PGColumnState is one catalog member, with decoded, case-sensitive identity.
// TypeWidth is 16/32/64 for a builtin integer, -1 for another proven type,
// and 0 for unobserved type identity. No expressions or issuance state are kept.
type PGColumnState struct {
	Name             string
	Identity         string // pg_attribute.attidentity: empty, a, d
	Generated        string // pg_attribute.attgenerated: empty, s on PG17
	TypeWidth        int
	NotNull          bool
	HasDefault       bool
	PrimaryKey       bool
	IdentitySequence *PGIdentitySequenceState
}

// PGColumnCatalog distinguishes unknown (nil) from a complete empty directory.
// Ordinary proves a persistent, non-inheritance/non-partition ordinary table.
type PGColumnCatalog struct {
	Ordinary bool
	Columns  []PGColumnState // nil is unobserved, not an empty set
}

// FindColumn returns one uniquely proven member or proven absence.
func (c *PGColumnCatalog) FindColumn(name string) (*PGColumnState, bool) {
	if c == nil || !c.Ordinary || c.Columns == nil || name == "" {
		return nil, false
	}
	var found *PGColumnState
	for i := range c.Columns {
		m := &c.Columns[i]
		if m.Name == "" {
			return nil, false
		}
		if m.Name != name {
			continue
		}
		if found != nil || (m.Identity != "" && m.Identity != "a" && m.Identity != "d") || (m.Generated != "" && m.Generated != "s") || (m.Identity != "" && m.Generated != "") {
			return nil, false
		}
		found = m
	}
	return found, true
}

// Clone preserves unknown/known-empty distinctions and owns numeric facts.
func (c *PGColumnCatalog) Clone() *PGColumnCatalog {
	if c == nil {
		return nil
	}
	out := *c
	if c.Columns != nil {
		out.Columns = make([]PGColumnState, len(c.Columns))
		copy(out.Columns, c.Columns)
		for i := range out.Columns {
			if s := c.Columns[i].IdentitySequence; s != nil {
				seq := *s
				seq.Definition = *s.Definition.Clone()
				out.Columns[i].IdentitySequence = &seq
			}
		}
	}
	return &out
}
