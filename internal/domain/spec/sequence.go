package spec

import "strings"

// SequenceOptionKind names one authored sequence option member. Values are the
// bounded vocabulary TiDB 8.5 accepts on CREATE/ALTER SEQUENCE; duplicate
// declarations keep authored order rather than collapsing into a map.
type SequenceOptionKind string

const (
	SequenceOptionIncrementBy SequenceOptionKind = "increment_by"
	SequenceOptionStartWith   SequenceOptionKind = "start_with"
	SequenceOptionMinValue    SequenceOptionKind = "minvalue"
	SequenceOptionNoMinValue  SequenceOptionKind = "no_minvalue"
	SequenceOptionMaxValue    SequenceOptionKind = "maxvalue"
	SequenceOptionNoMaxValue  SequenceOptionKind = "no_maxvalue"
	SequenceOptionCache       SequenceOptionKind = "cache"
	SequenceOptionNoCache     SequenceOptionKind = "no_cache"
	SequenceOptionCycle       SequenceOptionKind = "cycle"
	SequenceOptionNoCycle     SequenceOptionKind = "no_cycle"
	SequenceOptionRestart     SequenceOptionKind = "restart"
	SequenceOptionRestartWith SequenceOptionKind = "restart_with"
	SequenceOptionComment     SequenceOptionKind = "comment"
	SequenceOptionUnknown     SequenceOptionKind = "unknown"
)

// SequenceOption is one authored sequence option in declaration order. Value
// is nil for valueless kinds (NO MINVALUE, NOCYCLE, bare RESTART) so an
// omitted value is never conflated with an explicit zero.
type SequenceOption struct {
	Kind  SequenceOptionKind `json:"kind"`
	Value *int64             `json:"value,omitempty"`
}

// SequenceRef is one sequence identity as authored — the schema qualifier is
// preserved exactly and stays empty when the statement did not qualify the
// name. Downstream binds it to the effective schema; it is never guessed here.
type SequenceRef struct {
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name"`
}

// SequenceDeclaration carries the authored facts of a CREATE/ALTER/DROP
// SEQUENCE statement. It is internal evidence (never serialized): names,
// guards, the ordered option list, and every drop target. Runtime facts —
// current value, next value, persisted position — are deliberately absent;
// declarations and catalog definitions never model issuance progress.
type SequenceDeclaration struct {
	// Schema is the authored qualifier on the primary target (may be empty).
	Schema string `json:"-"`
	// Name is the primary target name — same identity as DDL.ObjectName.
	Name string `json:"-"`
	// IfExists / IfNotExists record parsed existence guards.
	IfExists    bool `json:"-"`
	IfNotExists bool `json:"-"`
	// Options preserves every authored sequence option in order, including
	// duplicates — last-write-wins is a semantics decision, not an extraction
	// one.
	Options []SequenceOption `json:"-"`
	// Targets lists every identity a DROP SEQUENCE names, in source order;
	// empty on CREATE/ALTER which carry exactly the primary target.
	Targets []SequenceRef `json:"-"`
	// CommentDeclared records only that a COMMENT clause was authored — the
	// text itself is never retained in diagnostics or state.
	CommentDeclared bool `json:"-"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *SequenceDeclaration) Clone() *SequenceDeclaration {
	if d == nil {
		return nil
	}
	out := *d
	if d.Options != nil {
		out.Options = append([]SequenceOption(nil), d.Options...)
		for i := range out.Options {
			if v := out.Options[i].Value; v != nil {
				value := *v
				out.Options[i].Value = &value
			}
		}
	}
	if d.Targets != nil {
		out.Targets = append([]SequenceRef(nil), d.Targets...)
	}
	return &out
}

// TargetRefs returns every target identity the statement binds — the drop
// list when present, otherwise the primary target.
func (d *SequenceDeclaration) TargetRefs() []SequenceRef {
	if d == nil {
		return nil
	}
	if len(d.Targets) > 0 {
		return append([]SequenceRef(nil), d.Targets...)
	}
	if d.Name == "" {
		return nil
	}
	return []SequenceRef{{Schema: d.Schema, Name: d.Name}}
}

// RestartRequested reports whether the statement declares RESTART or
// RESTART WITH — a runtime issuance request, never a definition parameter.
func (d *SequenceDeclaration) RestartRequested() bool {
	for _, option := range d.Options {
		if option.Kind == SequenceOptionRestart || option.Kind == SequenceOptionRestartWith {
			return true
		}
	}
	return false
}

// ChangesDefinition reports whether the statement authors at least one
// definition parameter (anything other than a restart request) — the premise
// for runtime-state uncertainty on ALTER.
func (d *SequenceDeclaration) ChangesDefinition() bool {
	for _, option := range d.Options {
		switch option.Kind {
		case SequenceOptionRestart, SequenceOptionRestartWith:
			continue
		default:
			return true
		}
	}
	return false
}

// DeclaredCycle returns the effective authored cycle flag under
// last-option-wins order, or nil when no cycle member is declared.
func (d *SequenceDeclaration) DeclaredCycle() *bool {
	var cycle *bool
	for _, option := range d.Options {
		switch option.Kind {
		case SequenceOptionCycle:
			v := true
			cycle = &v
		case SequenceOptionNoCycle:
			v := false
			cycle = &v
		}
	}
	return cycle
}

// SequenceDefinition carries catalog-observed or batch-derived definition
// parameters for one sequence. Every numeric field is nullable: absent means
// "not proven", never "implicitly zero". It is definition state only —
// current/next values, cache consumption, and persisted position are runtime
// facts and never appear here.
type SequenceDefinition struct {
	Start           *int64 `json:"-"`
	Increment       *int64 `json:"-"`
	MinValue        *int64 `json:"-"`
	MaxValue        *int64 `json:"-"`
	CacheValue      *int64 `json:"-"`
	Cache           *bool  `json:"-"`
	Cycle           *bool  `json:"-"`
	CommentDeclared bool   `json:"-"`
}

// Clone returns a deep copy safe for ordered-state publication.
func (d *SequenceDefinition) Clone() *SequenceDefinition {
	if d == nil {
		return nil
	}
	out := *d
	clonePtr := func(in *int64) *int64 {
		if in == nil {
			return nil
		}
		v := *in
		return &v
	}
	out.Start = clonePtr(d.Start)
	out.Increment = clonePtr(d.Increment)
	out.MinValue = clonePtr(d.MinValue)
	out.MaxValue = clonePtr(d.MaxValue)
	out.CacheValue = clonePtr(d.CacheValue)
	if d.Cache != nil {
		v := *d.Cache
		out.Cache = &v
	}
	if d.Cycle != nil {
		v := *d.Cycle
		out.Cycle = &v
	}
	return &out
}

// SequenceRefEdge is one proven inbound reference to a sequence: a distinct
// (owner-kind, owner identity, slot) → (target sequence) edge. The same
// target seen through different owner-slots is a different edge; the same
// edge re-observed deduplicates. Slot carries the bounded owner-side position
// vocabulary — "default:<column>" for a table column default — and stays
// empty when the source vouched the edge without slot detail.
type SequenceRefEdge struct {
	OwnerKind    string `json:"owner_kind,omitempty"`
	OwnerSchema  string `json:"owner_schema,omitempty"`
	OwnerName    string `json:"owner_name,omitempty"`
	Slot         string `json:"slot,omitempty"`
	TargetSchema string `json:"target_schema,omitempty"`
	TargetName   string `json:"target_name,omitempty"`
}

// OwnerIdentity renders the bounded owner-slot identity for findings and
// dedup — never SQL text, expression bodies, or secret content.
func (e SequenceRefEdge) OwnerIdentity() string {
	owner := e.OwnerSchema + "." + e.OwnerName
	if e.Slot != "" {
		owner += "@" + e.Slot
	}
	return e.OwnerKind + ":" + owner
}

// Key is the normalized dedup identity — lowercased across every member.
func (e SequenceRefEdge) Key() string {
	return strings.ToLower(e.OwnerKind + ":" + e.OwnerSchema + "." + e.OwnerName + "@" + e.Slot + "->" + e.TargetSchema + "." + e.TargetName)
}

// DedupeSequenceRefEdges collapses repeated edges by normalized key while
// preserving first-observed order — provider-vouched order, then batch order.
func DedupeSequenceRefEdges(in []SequenceRefEdge) []SequenceRefEdge {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]SequenceRefEdge, 0, len(in))
	for _, edge := range in {
		key := edge.Key()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, edge)
	}
	return out
}

// CloneSequenceRefEdges deep-copies an edge slice.
func CloneSequenceRefEdges(in []SequenceRefEdge) []SequenceRefEdge {
	if in == nil {
		return nil
	}
	return append([]SequenceRefEdge(nil), in...)
}

// SequenceState is the ordered-state projection of one sequence identity as a
// statement adjudicates it: existence plus the proven definition and bounded
// inbound references. Status mirrors the MetadataStatus contract — confirmed
// and not_found are proven answers; anything else is unproven, never silently
// absent.
type SequenceState struct {
	Schema string `json:"schema,omitempty"`
	Name   string `json:"name,omitempty"`
	// Status reports the existence answer's provenance: confirmed,
	// not_found, or unavailable/ambiguous when the identity is unproven.
	Status     MetadataStatus      `json:"status,omitempty"`
	Exists     bool                `json:"exists"`
	Definition *SequenceDefinition `json:"-"`
	// References lists the proven inbound edges — provider-vouched and
	// batch-derived owner-slot/target pairs. An empty non-nil set under
	// ReferencesUnknown=false is proven-empty; nil means unvouched.
	References []SequenceRefEdge `json:"-"`
	// ReferencesUnknown marks the edge set's completeness unproven — an
	// unvouched catalog read, a batch-diverged owner, or an authored
	// reference whose landing could not be decided. Positive edges stay
	// real under it; it never downgrades a vouched-empty set.
	ReferencesUnknown bool `json:"-"`
	// ConflictKind names a non-sequence object kind proven at the same
	// namespace identity (sequences share the table namespace in TiDB);
	// empty when no conflict was proven.
	ConflictKind string `json:"conflict_kind,omitempty"`
}

// Clone returns a deep copy safe for statement-metadata publication.
func (s *SequenceState) Clone() *SequenceState {
	if s == nil {
		return nil
	}
	out := *s
	out.Definition = s.Definition.Clone()
	out.References = CloneSequenceRefEdges(s.References)
	return &out
}

// Proven reports whether the state carries a proven existence answer.
func (s *SequenceState) Proven() bool {
	if s == nil {
		return false
	}
	return s.Status == MetadataStatusConfirmed || s.Status == MetadataStatusNotFound
}
