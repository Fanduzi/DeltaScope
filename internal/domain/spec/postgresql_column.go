package spec

// PGColumnTransition preserves authored existing-column generation changes.
// Legacy action/options projections lose guards, duplicates and 64-bit values.
type PGColumnTransition struct {
	IfExists                      bool
	Mode                          string // first a/d member; ModeCount must prove uniqueness
	ModeCount                     int
	Options                       []SequenceOption // source order, never last-wins
	Unmodeled                     bool
	DefaultHasDisallowedReference bool // Raw column reference or subquery in SET DEFAULT.
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
	Name        string
	Identity    string // pg_attribute.attidentity: empty, a, d
	Generated   string // pg_attribute.attgenerated: empty, s on PG17
	TypeWidth   int
	TypeStorage string // observed pg_type.typstorage: p/e/m/x; empty is unknown
	System      bool   // observed negative pg_attribute.attnum, not a name inference
	NotNull     bool
	HasDefault  bool
	PrimaryKey  bool
	// ReplicaIdentityKey records membership in the key slots of a live attached index.
	ReplicaIdentityKey      bool `json:"-"`
	ReplicaIdentityKeyKnown bool `json:"-"`
	IdentitySequence        *PGIdentitySequenceState
}

// PGColumnCatalog keeps user and observed system directories separate.
// Ordinary proves a persistent, non-inheritance/non-partition ordinary table.
type PGColumnCatalog struct {
	Ordinary      bool
	Typed         bool            // Observed pg_class.reloftype is nonzero.
	Columns       []PGColumnState // nil is unobserved; non-nil empty proves no user columns
	SystemColumns []PGColumnState // nil is unobserved, never proof that system targets are absent
}

// FindColumn returns one uniquely proven member or proven absence.
func (c *PGColumnCatalog) FindColumn(name string) (*PGColumnState, bool) {
	if c == nil || !c.Ordinary {
		return nil, false
	}
	return findPGColumn(c.Columns, c.SystemColumns, name)
}

func findPGColumn(columns, systemColumns []PGColumnState, name string) (*PGColumnState, bool) {
	if columns == nil || name == "" {
		return nil, false
	}
	var found *PGColumnState
	for namespace, members := range [][]PGColumnState{columns, systemColumns} {
		for i := range members {
			m := &members[i]
			if m.Name == "" || m.System != (namespace == 1) || pgSystemColumnName(m.Name) != (namespace == 1) {
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
	}
	if found == nil && pgSystemColumnName(name) {
		return nil, false // User-directory absence never proves a system attribute absent.
	}
	return found, true
}

func pgSystemColumnName(name string) bool {
	switch name {
	case "ctid", "xmin", "cmin", "xmax", "cmax", "tableoid":
		return true
	default:
		return false
	}
}

// Clone preserves unknown/known-empty distinctions and owns numeric facts.
func (c *PGColumnCatalog) Clone() *PGColumnCatalog {
	if c == nil {
		return nil
	}
	out := *c
	out.Columns = clonePGColumnMembers(c.Columns)
	out.SystemColumns = clonePGColumnMembers(c.SystemColumns)
	return &out
}

func clonePGColumnMembers(members []PGColumnState) []PGColumnState {
	if members == nil {
		return nil
	}
	out := make([]PGColumnState, len(members))
	copy(out, members)
	for i := range out {
		if s := members[i].IdentitySequence; s != nil {
			seq := *s
			seq.Definition = *s.Definition.Clone()
			out[i].IdentitySequence = &seq
		}
	}
	return out
}

// PGOrdinaryTableBound checks the fresh, qualified ordinary persistent non-typed target.
func PGOrdinaryTableBound(s Statement) bool {
	if !PGExactTableBound(s) {
		return false
	}
	catalog := s.Metadata.TargetTable.PGColumns
	return catalog != nil && catalog.Ordinary && !catalog.Typed
}

// PGExactTableBound checks the identity and freshness of the live relation.
// Callers must establish the table shape their rule requires.
func PGExactTableBound(s Statement) bool {
	if s.DDL == nil || s.DDL.Table == nil || s.DDL.PGTableCatalogQualified || s.Metadata == nil || s.Metadata.TargetTable == nil {
		return false
	}
	table, snapshot := s.DDL.Table, s.Metadata.TargetTable
	return table.Name != "" && table.Schema != "" && table.Schema != "pg_temp" &&
		s.Metadata.Schema == table.Schema && snapshot.Schema == table.Schema && snapshot.Exists &&
		snapshot.Table != nil && snapshot.Table.Name == table.Name &&
		snapshot.PGRelation.Name == table.Name && snapshot.PGRelation.Kind == "r"
}
