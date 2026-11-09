package spec

// mysql_resource_group.go models the MySQL CPU resource-group family —
// a statement surface the locked TiDB grammar cannot produce, recovered
// by the parser adapter as typed declarations (issue #103 T24). It is
// deliberately separate from the TiDB RU model: TYPE/VCPU/
// THREAD_PRIORITY/ENABLE|DISABLE/FORCE have no RU counterpart, and the
// families never share a declaration so a boundary stays provable.

// MySQLResourceGroupOptionKind names one authored MySQL resource-group
// option member. MySQL's grammar gives each option a single positional
// slot — the recognizer never produces duplicates, so authored order is
// fidelity, never precedence.
type MySQLResourceGroupOptionKind string

const (
	MySQLResourceGroupOptionType           MySQLResourceGroupOptionKind = "type"
	MySQLResourceGroupOptionVCPU           MySQLResourceGroupOptionKind = "vcpu"
	MySQLResourceGroupOptionThreadPriority MySQLResourceGroupOptionKind = "thread_priority"
	MySQLResourceGroupOptionEnabled        MySQLResourceGroupOptionKind = "enabled"
	MySQLResourceGroupOptionForce          MySQLResourceGroupOptionKind = "force"
)

// MySQLResourceGroupType is the authored group class — fixed at CREATE
// and never alterable. SYSTEM groups hold background threads and accept
// negative priorities; USER groups hold session threads.
type MySQLResourceGroupType string

const (
	MySQLResourceGroupTypeSystem MySQLResourceGroupType = "system"
	MySQLResourceGroupTypeUser   MySQLResourceGroupType = "user"
)

// MySQLResourceGroupVCPUSpec is one authored VCPU member — a single CPU
// id (Lo == Hi) or an inclusive range Lo-Hi. The adapter validates
// Lo <= Hi at lexical time; whether the ids fit the host's CPU count is
// an instance fact the audit never guesses.
type MySQLResourceGroupVCPUSpec struct {
	Lo uint32 `json:"lo"`
	Hi uint32 `json:"hi"`
}

// MySQLResourceGroupOption is one authored MySQL resource-group option
// in declaration order. Exactly one payload member is populated per
// recognized kind; a member the recognizer cannot classify stays
// unclaimed — the statement falls back to the base grammar's
// diagnostics, never a silently accepted half-parse.
type MySQLResourceGroupOption struct {
	Kind MySQLResourceGroupOptionKind `json:"kind"`
	// GroupType carries the CREATE-time TYPE member.
	GroupType MySQLResourceGroupType `json:"group_type,omitempty"`
	// VCPUSpecs carries the authored VCPU list in order.
	VCPUSpecs []MySQLResourceGroupVCPUSpec `json:"vcpu_specs,omitempty"`
	// IntValue carries THREAD_PRIORITY — signed; the authored-type
	// bounds (system: -20..0, user: 0..19) are rule adjudications, not
	// lexical facts.
	IntValue *int64 `json:"int_value,omitempty"`
	// Enabled carries ENABLE (true) / DISABLE (false).
	Enabled *bool `json:"enabled,omitempty"`
}

// Clone returns a deep copy.
func (o MySQLResourceGroupOption) Clone() MySQLResourceGroupOption {
	out := o
	if o.VCPUSpecs != nil {
		out.VCPUSpecs = append([]MySQLResourceGroupVCPUSpec(nil), o.VCPUSpecs...)
	}
	if o.IntValue != nil {
		v := *o.IntValue
		out.IntValue = &v
	}
	if o.Enabled != nil {
		v := *o.Enabled
		out.Enabled = &v
	}
	return out
}

// MySQLResourceGroupDeclaration carries the authored facts of a MySQL
// CREATE/ALTER/DROP RESOURCE GROUP statement. It is internal evidence
// (never serialized): name and the ordered option list. MySQL's grammar
// has no IF EXISTS / IF NOT EXISTS guards — their absence is a grammar
// fact, and a TiDB-form guard parsed under the mysql dialect stays the
// vendor boundary the T22 contract owns.
type MySQLResourceGroupDeclaration struct {
	// Name is the statement's group identity — resource groups are
	// globally namespaced, never schema-qualified, and case-insensitive
	// under MySQL.
	Name string `json:"-"`
	// Options preserves every authored option in order.
	Options []MySQLResourceGroupOption `json:"-"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *MySQLResourceGroupDeclaration) Clone() *MySQLResourceGroupDeclaration {
	if d == nil {
		return nil
	}
	out := *d
	if d.Options != nil {
		out.Options = make([]MySQLResourceGroupOption, len(d.Options))
		for i := range d.Options {
			out.Options[i] = d.Options[i].Clone()
		}
	}
	return &out
}

// declaredOption returns the authored option of a kind — each kind
// occupies a single positional grammar slot so at most one can exist.
func (d *MySQLResourceGroupDeclaration) declaredOption(kind MySQLResourceGroupOptionKind) (MySQLResourceGroupOption, bool) {
	var found MySQLResourceGroupOption
	ok := false
	if d == nil {
		return found, false
	}
	for _, option := range d.Options {
		if option.Kind == kind {
			found = option
			ok = true
		}
	}
	return found, ok
}

// DeclaredType returns the authored TYPE member (CREATE only), or
// ok=false when undeclared.
func (d *MySQLResourceGroupDeclaration) DeclaredType() (MySQLResourceGroupType, bool) {
	option, ok := d.declaredOption(MySQLResourceGroupOptionType)
	return option.GroupType, ok
}

// DeclaredVCPU returns the authored VCPU spec list, or ok=false when
// undeclared (an ALTER omitting VCPU keeps the proven affinity).
func (d *MySQLResourceGroupDeclaration) DeclaredVCPU() ([]MySQLResourceGroupVCPUSpec, bool) {
	option, ok := d.declaredOption(MySQLResourceGroupOptionVCPU)
	return option.VCPUSpecs, ok
}

// DeclaredThreadPriority returns the authored THREAD_PRIORITY value, or
// ok=false when undeclared.
func (d *MySQLResourceGroupDeclaration) DeclaredThreadPriority() (*int64, bool) {
	option, ok := d.declaredOption(MySQLResourceGroupOptionThreadPriority)
	return option.IntValue, ok
}

// DeclaredEnabled returns the authored ENABLE/DISABLE switch, or
// ok=false when undeclared.
func (d *MySQLResourceGroupDeclaration) DeclaredEnabled() (*bool, bool) {
	option, ok := d.declaredOption(MySQLResourceGroupOptionEnabled)
	return option.Enabled, ok
}

// DeclaredForce reports whether the statement authored FORCE.
func (d *MySQLResourceGroupDeclaration) DeclaredForce() bool {
	_, ok := d.declaredOption(MySQLResourceGroupOptionForce)
	return ok
}

// MySQLResourceGroupDefinition carries catalog-observed configuration
// for one MySQL resource group — the information_schema.RESOURCE_GROUPS
// row shape. Every member is nullable: absent means unobserved, never a
// guessed default or a conflated zero.
type MySQLResourceGroupDefinition struct {
	GroupType      MySQLResourceGroupType `json:"-"`
	Enabled        *bool                  `json:"-"`
	ThreadPriority *int64                 `json:"-"`
	// VCPUIDs carries the catalog's VCPU_IDS column text when observed —
	// bounded evidence; range semantics are not re-adjudicated.
	VCPUIDs *string `json:"-"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *MySQLResourceGroupDefinition) Clone() *MySQLResourceGroupDefinition {
	if d == nil {
		return nil
	}
	out := *d
	if d.Enabled != nil {
		v := *d.Enabled
		out.Enabled = &v
	}
	if d.ThreadPriority != nil {
		v := *d.ThreadPriority
		out.ThreadPriority = &v
	}
	if d.VCPUIDs != nil {
		v := *d.VCPUIDs
		out.VCPUIDs = &v
	}
	return &out
}

// MySQLResourceGroupState is the ordered-state projection of one MySQL
// resource group as a statement adjudicates it: existence plus the
// proven definition. Status mirrors the MetadataStatus contract —
// confirmed and not_found are proven answers; anything else is unproven.
type MySQLResourceGroupState struct {
	Name   string         `json:"name,omitempty"`
	Status MetadataStatus `json:"status,omitempty"`
	Exists bool           `json:"exists"`
	// Reserved marks a builtin group (usr_default / sys_default) — a
	// fixed-version fact provable without a catalog read: reserved groups
	// always exist and can be neither altered nor dropped.
	Reserved   bool                          `json:"reserved,omitempty"`
	Definition *MySQLResourceGroupDefinition `json:"-"`
}

// Clone returns a deep copy safe for statement-metadata publication.
func (s *MySQLResourceGroupState) Clone() *MySQLResourceGroupState {
	if s == nil {
		return nil
	}
	out := *s
	out.Definition = s.Definition.Clone()
	return &out
}

// Proven reports whether the state carries a proven existence answer.
func (s *MySQLResourceGroupState) Proven() bool {
	if s == nil {
		return false
	}
	return s.Status == MetadataStatusConfirmed || s.Status == MetadataStatusNotFound
}
