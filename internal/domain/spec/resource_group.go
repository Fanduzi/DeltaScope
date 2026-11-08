package spec

// ResourceGroupOptionKind names one authored resource-group option member.
// The vocabulary is what TiDB 8.5's DirectResourceGroupOption grammar
// produces; the parser rejects duplicates per type, so authored order is
// fidelity, not precedence. Raw-mode members (CPU / IO bandwidth) are not
// reachable through the CREATE/ALTER RESOURCE GROUP grammar in this
// version and a Tp the model cannot classify lands on unrecognized —
// never silently accepted.
type ResourceGroupOptionKind string

const (
	ResourceGroupOptionRURate       ResourceGroupOptionKind = "ru_per_sec"
	ResourceGroupOptionPriority     ResourceGroupOptionKind = "priority"
	ResourceGroupOptionBurstable    ResourceGroupOptionKind = "burstable"
	ResourceGroupOptionQueryLimit   ResourceGroupOptionKind = "query_limit"
	ResourceGroupOptionBackground   ResourceGroupOptionKind = "background"
	ResourceGroupOptionUnrecognized ResourceGroupOptionKind = "unrecognized"
)

// ResourceGroupBurstable is the three-state burst allowance. Bare
// BURSTABLE parses to moderated; OFF is the explicit disable.
type ResourceGroupBurstable string

const (
	ResourceGroupBurstableDisabled  ResourceGroupBurstable = "disabled"
	ResourceGroupBurstableModerated ResourceGroupBurstable = "moderated"
	ResourceGroupBurstableUnlimited ResourceGroupBurstable = "unlimited"
)

// Enabled reports whether the allowance lets the group exceed its
// RU_PER_SEC budget — never a performance or throughput claim.
func (b ResourceGroupBurstable) Enabled() bool {
	return b == ResourceGroupBurstableModerated || b == ResourceGroupBurstableUnlimited
}

// ResourceGroupRunawayMemberKind names one QUERY_LIMIT member. The
// bounded vocabulary TiDB 8.5 accepts; values stay typed, never raw SQL.
type ResourceGroupRunawayMemberKind string

const (
	ResourceGroupRunawayExecElapsed   ResourceGroupRunawayMemberKind = "exec_elapsed"
	ResourceGroupRunawayProcessedKeys ResourceGroupRunawayMemberKind = "processed_keys"
	ResourceGroupRunawayRU            ResourceGroupRunawayMemberKind = "ru"
	ResourceGroupRunawayAction        ResourceGroupRunawayMemberKind = "action"
	ResourceGroupRunawayWatch         ResourceGroupRunawayMemberKind = "watch"
)

// ResourceGroupRunawayMember is one QUERY_LIMIT member. TextValue carries
// bounded enum/duration text (EXEC_ELAPSED durations, action and watch
// vocabulary, the switch-group target name); UintValue carries numeric
// thresholds. A nil UintValue on a numeric member is "not declared", never
// zero.
type ResourceGroupRunawayMember struct {
	Kind      ResourceGroupRunawayMemberKind `json:"kind"`
	UintValue *uint64                        `json:"uint_value,omitempty"`
	TextValue string                         `json:"text_value,omitempty"`
}

// ResourceGroupRunawayDeclaration is one authored QUERY_LIMIT clause.
// Reset marks QUERY_LIMIT = NULL (or the equivalent empty list), which
// removes a runaway rule rather than declaring one.
type ResourceGroupRunawayDeclaration struct {
	Reset   bool                         `json:"reset,omitempty"`
	Members []ResourceGroupRunawayMember `json:"members,omitempty"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *ResourceGroupRunawayDeclaration) Clone() *ResourceGroupRunawayDeclaration {
	if d == nil {
		return nil
	}
	out := *d
	if d.Members != nil {
		out.Members = append([]ResourceGroupRunawayMember(nil), d.Members...)
		for i := range out.Members {
			if v := out.Members[i].UintValue; v != nil {
				value := *v
				out.Members[i].UintValue = &value
			}
		}
	}
	return &out
}

// Action returns the declared ACTION member's bounded name (kill,
// cooldown, dryrun, switch_group), empty when undeclared.
func (d *ResourceGroupRunawayDeclaration) Action() string {
	if d == nil {
		return ""
	}
	for _, member := range d.Members {
		if member.Kind == ResourceGroupRunawayAction {
			return member.TextValue
		}
	}
	return ""
}

// ResourceGroupBackgroundDeclaration is one authored BACKGROUND clause.
// BACKGROUND is recognized in the TiDB 8.5 grammar but outside the T22
// audit surface — the declaration preserves only member presence and the
// reset marker so coverage can bound it; member values are never kept.
type ResourceGroupBackgroundDeclaration struct {
	Reset   bool `json:"reset,omitempty"`
	Members int  `json:"members,omitempty"`
}

// ResourceGroupOption is one authored resource-group option in
// declaration order. UintValue/Unlimited carry the RU rate; TextValue
// carries bounded enum text (priority name); Burstable carries the
// three-state allowance; Runaway/Background carry the clause payloads.
// A member the parser produced but the model cannot classify stays
// Kind=unrecognized — recognized evidence, never silent acceptance.
type ResourceGroupOption struct {
	Kind       ResourceGroupOptionKind             `json:"kind"`
	UintValue  *uint64                             `json:"uint_value,omitempty"`
	Unlimited  bool                                `json:"unlimited,omitempty"`
	TextValue  string                              `json:"text_value,omitempty"`
	Burstable  ResourceGroupBurstable              `json:"burstable,omitempty"`
	Runaway    *ResourceGroupRunawayDeclaration    `json:"runaway,omitempty"`
	Background *ResourceGroupBackgroundDeclaration `json:"background,omitempty"`
}

// Clone returns a deep copy.
func (o ResourceGroupOption) Clone() ResourceGroupOption {
	out := o
	if o.UintValue != nil {
		v := *o.UintValue
		out.UintValue = &v
	}
	out.Runaway = o.Runaway.Clone()
	if o.Background != nil {
		bg := *o.Background
		out.Background = &bg
	}
	return out
}

// ResourceGroupDeclaration carries the authored facts of a
// CREATE/ALTER/DROP RESOURCE GROUP statement. It is internal evidence
// (never serialized): name, guards, and the ordered option list. Catalog
// definitions, the resource-control capability premise, and workload
// effects live in separate layers.
type ResourceGroupDeclaration struct {
	// Name is the statement's group identity — resource groups are
	// globally namespaced in TiDB, never schema-qualified.
	Name string `json:"-"`
	// IfExists / IfNotExists record parsed existence guards.
	IfExists    bool `json:"-"`
	IfNotExists bool `json:"-"`
	// Options preserves every authored option in order.
	Options []ResourceGroupOption `json:"-"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *ResourceGroupDeclaration) Clone() *ResourceGroupDeclaration {
	if d == nil {
		return nil
	}
	out := *d
	if d.Options != nil {
		out.Options = make([]ResourceGroupOption, len(d.Options))
		for i := range d.Options {
			out.Options[i] = d.Options[i].Clone()
		}
	}
	return &out
}

// declaredOption returns the last authored option of a kind — the
// effective declaration when the grammar's dup guard did not fire.
func (d *ResourceGroupDeclaration) declaredOption(kind ResourceGroupOptionKind) (ResourceGroupOption, bool) {
	var found ResourceGroupOption
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

// DeclaredRUPerSec returns the authored RU backfilling rate. ok=false
// means undeclared; ok=true with nil value means UNLIMITED.
func (d *ResourceGroupDeclaration) DeclaredRUPerSec() (*uint64, bool) {
	option, ok := d.declaredOption(ResourceGroupOptionRURate)
	if !ok {
		return nil, false
	}
	if option.Unlimited {
		return nil, true
	}
	return option.UintValue, true
}

// DeclaredPriority returns the authored priority name (LOW/MEDIUM/HIGH),
// or "" when undeclared.
func (d *ResourceGroupDeclaration) DeclaredPriority() string {
	option, ok := d.declaredOption(ResourceGroupOptionPriority)
	if !ok {
		return ""
	}
	return option.TextValue
}

// DeclaredBurstable returns the authored burst allowance, or ok=false
// when undeclared.
func (d *ResourceGroupDeclaration) DeclaredBurstable() (ResourceGroupBurstable, bool) {
	option, ok := d.declaredOption(ResourceGroupOptionBurstable)
	return option.Burstable, ok
}

// DeclaredRunaway returns the authored QUERY_LIMIT clause, or nil when
// undeclared.
func (d *ResourceGroupDeclaration) DeclaredRunaway() *ResourceGroupRunawayDeclaration {
	option, ok := d.declaredOption(ResourceGroupOptionQueryLimit)
	if !ok {
		return nil
	}
	return option.Runaway
}

// UnmodeledOptionKinds lists recognized option kinds outside the audited
// surface (background, unrecognized) in declaration order — coverage turns
// each into a bounded unaudited aspect instead of silent acceptance.
func (d *ResourceGroupDeclaration) UnmodeledOptionKinds() []ResourceGroupOptionKind {
	if d == nil {
		return nil
	}
	var out []ResourceGroupOptionKind
	for _, option := range d.Options {
		switch option.Kind {
		case ResourceGroupOptionBackground, ResourceGroupOptionUnrecognized:
			out = append(out, option.Kind)
		}
	}
	return out
}

// ResourceGroupDefinition carries catalog-observed configuration for one
// resource group — the information_schema.resource_groups row shape.
// Every member is nullable: absent means unobserved, never a guessed
// default or a conflated zero.
type ResourceGroupDefinition struct {
	RUPerSec    *uint64 `json:"-"`
	RUUnlimited bool    `json:"-"`
	Priority    string  `json:"-"`
	Burstable   *bool   `json:"-"`
	// QueryLimit carries the catalog's QUERY_LIMIT column text when the
	// read returned it; nil means unobserved, never proven-reset.
	QueryLimit *string `json:"-"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *ResourceGroupDefinition) Clone() *ResourceGroupDefinition {
	if d == nil {
		return nil
	}
	out := *d
	if d.RUPerSec != nil {
		v := *d.RUPerSec
		out.RUPerSec = &v
	}
	if d.Burstable != nil {
		v := *d.Burstable
		out.Burstable = &v
	}
	if d.QueryLimit != nil {
		v := *d.QueryLimit
		out.QueryLimit = &v
	}
	return &out
}

// ResourceGroupState is the ordered-state projection of one resource
// group as a statement adjudicates it: existence plus the proven
// definition. Status mirrors the MetadataStatus contract — confirmed and
// not_found are proven answers; anything else is unproven, never
// silently absent.
type ResourceGroupState struct {
	Name   string         `json:"name,omitempty"`
	Status MetadataStatus `json:"status,omitempty"`
	Exists bool           `json:"exists"`
	// Reserved marks the builtin default group — a fixed-version fact
	// provable without a catalog read (cannot be dropped, always exists).
	Reserved   bool                     `json:"reserved,omitempty"`
	Definition *ResourceGroupDefinition `json:"-"`
}

// Clone returns a deep copy safe for statement-metadata publication.
func (s *ResourceGroupState) Clone() *ResourceGroupState {
	if s == nil {
		return nil
	}
	out := *s
	out.Definition = s.Definition.Clone()
	return &out
}

// Proven reports whether the state carries a proven existence answer.
func (s *ResourceGroupState) Proven() bool {
	if s == nil {
		return false
	}
	return s.Status == MetadataStatusConfirmed || s.Status == MetadataStatusNotFound
}
