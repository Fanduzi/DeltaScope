package spec

import "strings"

// PlacementOutcome is the bounded tri-state answer for one placement
// lifecycle or binding transition after TiDB's semantics are applied:
// refused keeps the exact pre-state, applied publishes, unproven withholds.
type PlacementOutcome int

const (
	PlacementRefused PlacementOutcome = iota
	PlacementApplied
	PlacementUnproven
)

// PlacementOption is one authored CREATE/ALTER PLACEMENT POLICY member
// normalized to a bounded key name plus its parsed value. Findings expose
// the key names only — constraint bodies never appear in findings.
type PlacementOption struct {
	Key       string  `json:"key"`
	StrValue  string  `json:"str_value,omitempty"`
	UintValue *uint64 `json:"uint_value,omitempty"`
}

// PlacementBinding is one authored PLACEMENT POLICY assignment. Policy
// carries the bound policy name; Default marks the `= DEFAULT` unbind form.
type PlacementBinding struct {
	Policy  string `json:"policy,omitempty"`
	Default bool   `json:"default,omitempty"`
}

// PlacementPolicyFact is the catalog fact for one placement policy: its
// proven existence plus the bounded bound-object list (the proven inbound
// references TiDB consults for the 8241 drop refusal). nil members never
// appear in the map — presence in the map itself is the existence answer.
type PlacementPolicyFact struct {
	PrimaryRegion string   `json:"primary_region,omitempty"`
	Regions       string   `json:"regions,omitempty"`
	BoundTargets  []string `json:"bound_targets,omitempty"`
}

// PlacementTopologyFacts is the bounded, proven observation of the
// topology prerequisites behind a placement action — a different evidence
// layer from the catalog: where the policy *could physically deploy*.
// A nil Metadata.PlacementTopology means topology was never read; rules
// surface that absence as a structured evidence gap rather than implying
// verified deployability (issue #99: 拓扑未知给gap而非猜测可部署).
//
// The projection is deliberately bounded: label keys/values and scheduling
// state are provider-visible facts, never constraint bodies copied from
// authored SQL, and the whole structure stays off public JSON (json:"-").
type PlacementTopologyFacts struct {
	// StoreLabels is the proven label universe (SHOW PLACEMENT LABELS):
	// label key → observed values across stores. nil means labels were
	// never read.
	StoreLabels map[string][]string `json:"-"`
	// SchedulingState is the bound target's proven SCHEDULING_STATE
	// (PENDING|INPROGRESS|SCHEDULED) when observed. Empty means not
	// observed — and PENDING is an observed pending state, never itself
	// proof of satisfaction or failure.
	SchedulingState string `json:"-"`
}

// PlacementBindOutcome adjudicates `PLACEMENT POLICY = <name|DEFAULT>`
// against policy existence. DEFAULT is the unconditional unbind and always
// applies; a named policy applies only when existence is proven (native
// 8239 refuses a proven-absent name, an unvouched policy set stays
// unproven). MySQL is the vendor boundary — a parsed placement clause is
// refused, never silently applied.
func PlacementBindOutcome(dialect Dialect, binding PlacementBinding, policyExists *bool) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	if binding.Default {
		return PlacementApplied
	}
	if strings.TrimSpace(binding.Policy) == "" {
		return PlacementUnproven
	}
	if policyExists == nil {
		return PlacementUnproven
	}
	if !*policyExists {
		return PlacementRefused
	}
	return PlacementApplied
}

// PlacementCreateOutcome adjudicates CREATE PLACEMENT POLICY against the
// proven policy set: a proven-present name refuses the bare create and
// no-ops under IF NOT EXISTS or lands under OR REPLACE; an unvouched set
// stays unproven. or_replace semantics are modeled as a replacement land
// (bound objects keep the name — probed), never as a fresh object.
func PlacementCreateOutcome(dialect Dialect, orReplace, ifNotExists bool, policyExists *bool) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	if orReplace {
		return PlacementApplied
	}
	if policyExists == nil {
		return PlacementUnproven
	}
	if *policyExists {
		if ifNotExists {
			return PlacementApplied
		}
		return PlacementRefused
	}
	return PlacementApplied
}

// PlacementAlterOutcome adjudicates ALTER PLACEMENT POLICY: the target
// policy must provably exist (native refuses an absent name), while an
// unvouched policy set stays unproven.
func PlacementAlterOutcome(dialect Dialect, policyExists *bool) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	if policyExists == nil {
		return PlacementUnproven
	}
	if !*policyExists {
		return PlacementRefused
	}
	return PlacementApplied
}

// PlacementDropOutcome adjudicates DROP PLACEMENT POLICY: IF EXISTS on a
// proven-absent name is a no-op land, a bare drop of a proven-absent name
// refuses, and a proven in-use policy refuses (8241) while a proven
// bound-empty set lands. Any unvouched bound set stays unproven.
func PlacementDropOutcome(dialect Dialect, ifExists bool, policyExists *bool, boundTargets *[]string) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	if policyExists == nil || boundTargets == nil {
		return PlacementUnproven
	}
	if !*policyExists {
		if ifExists {
			return PlacementApplied
		}
		return PlacementRefused
	}
	if len(*boundTargets) > 0 {
		return PlacementRefused
	}
	return PlacementApplied
}

// PlacementRangeOutcome resolves the authored ALTER RANGE scope name to
// the bounded set probed on TiDB v8.5.0: `global` and `meta` land while
// `dist`, `region`, and every other spelling refuse natively (1105).
// The bind premise itself is checked by PlacementBindOutcome.
func PlacementRangeOutcome(dialect Dialect, rangeName string) PlacementOutcome {
	if dialect != DialectTiDB {
		return PlacementRefused
	}
	switch strings.ToLower(strings.TrimSpace(rangeName)) {
	case "global", "meta":
		return PlacementApplied
	case "":
		return PlacementUnproven
	default:
		return PlacementRefused
	}
}

// PlacementBindingTarget identifies one authored assignment. Indices are
// zero-based; -1 denotes the enclosing statement/no member definition.
// DEFAULT is retained. Source is direct, member_definition, or scheme_definition.
// No raw SQL, options, constraint text, or invented source offset is retained.
type PlacementBindingTarget struct {
	Binding         PlacementBinding
	Action          string
	ActionIndex     int
	DefinitionIndex int
	Source          string
	TargetKind      string
	Schema          string
	Table           string
	Partition       string
	Target          string
}

// PlacementBindingFact is one collected binding's adjudicated outcome.
type PlacementBindingFact struct {
	Target  PlacementBindingTarget
	Exists  *bool
	Outcome PlacementOutcome
}

// PlacementPolicyPremise contains only the current lifecycle target's
// proof, without turning a partial catalog into a complete one.
// BoundTargets != nil is either a proven-empty COMPLETE set, or a nonempty
// set of proven references sufficient to refuse DROP. It is not a public
// catalog enumeration or a claim that a nonempty list is exhaustive.
type PlacementPolicyPremise struct {
	Name         string
	Exists       *bool
	BoundTargets *[]string
}

// PlacementFacts is an owned per-statement pre-state projection. Outcome is
// the aggregate binding premise. MutationOutcome is the enclosing atomic
// CREATE/partition operation's static outcome, populated by the existing
// transition; topology satisfaction is deliberately not part of either.
type PlacementFacts struct {
	Bindings        []PlacementBindingFact
	Outcome         PlacementOutcome
	Policy          *PlacementPolicyPremise
	MutationOutcome *PlacementOutcome
	RequiredFacts   []string
}

// CollectPlacementBindings enumerates every authored placement assignment a
// statement carries, in structured source order: the statement-level
// binding, CREATE member definitions, each alter action's direct binding
// (expanded once per named partition target), alter member definitions and
// alter partition-scheme definitions. DEFAULT bindings are retained.
// Subpartition children carry no structured Placement and are never
// collected (TiDB discards the whole subpartition axis).
func CollectPlacementBindings(ddl *DDL) []PlacementBindingTarget {
	if ddl == nil {
		return nil
	}
	var out []PlacementBindingTarget
	base := PlacementBindingTarget{
		Action: string(ddl.Operation), ActionIndex: -1, DefinitionIndex: -1,
		Source: "direct", TargetKind: "table",
	}
	if ddl.Table != nil {
		base.Schema, base.Table, base.Target = ddl.Table.Schema, ddl.Table.Name, ddl.Table.Name
	}
	switch ddl.Operation {
	case DDLOperationCreateSchema, DDLOperationAlterSchema:
		base.TargetKind, base.Schema, base.Target = "schema", ddl.ObjectName, ddl.ObjectName
	case DDLOperationAlterRangePlacement:
		base.TargetKind, base.Target = "range", "range:"+strings.ToLower(ddl.PlacementRange)
	}
	appendBinding := func(target PlacementBindingTarget, binding *PlacementBinding) {
		if binding == nil {
			return
		}
		target.Binding = *binding
		out = append(out, target)
	}
	appendDefs := func(target PlacementBindingTarget, defs []PartitionDef, source string) {
		for i, def := range defs {
			target.Source, target.DefinitionIndex = source, i
			target.TargetKind, target.Partition = "partition", def.Name
			appendBinding(target, def.Placement)
		}
	}
	appendBinding(base, ddl.PlacementBinding)
	appendDefs(base, ddl.PartitionDefs, "member_definition")
	for i, alter := range ddl.Alter {
		target := base
		target.Action, target.ActionIndex = alter.Action, i
		if len(alter.Partitions) == 0 {
			appendBinding(target, alter.Placement)
		} else {
			for _, part := range alter.Partitions {
				member := target
				member.TargetKind, member.Partition = "partition", part
				appendBinding(member, alter.Placement)
			}
		}
		appendDefs(target, alter.PartitionDefs, "member_definition")
		if alter.PartitionScheme != nil {
			appendDefs(target, alter.PartitionScheme.Defs, "scheme_definition")
		}
	}
	return out
}

// AssessPlacementBindings adjudicates each collected target against the
// shared tri-state outcome. lookup answers a policy's existence (nil means
// unvouched); results are cached per lower-cased name. Refusal dominates
// the aggregate, then unproven.
func AssessPlacementBindings(dialect Dialect, targets []PlacementBindingTarget,
	lookup func(string) *bool) *PlacementFacts {
	facts := &PlacementFacts{Outcome: PlacementApplied}
	cache := map[string]*bool{}
	for _, target := range targets {
		key := strings.ToLower(strings.TrimSpace(target.Binding.Policy))
		var exists *bool
		if !target.Binding.Default && lookup != nil {
			var seen bool
			exists, seen = cache[key]
			if !seen {
				if answer := lookup(key); answer != nil {
					copied := *answer
					exists = &copied
				}
				cache[key] = exists
			}
		}
		outcome := PlacementBindOutcome(dialect, target.Binding, exists)
		facts.Bindings = append(facts.Bindings, PlacementBindingFact{Target: target, Exists: exists, Outcome: outcome})
		switch outcome {
		case PlacementRefused:
			facts.Outcome = PlacementRefused
		case PlacementUnproven:
			if facts.Outcome != PlacementRefused {
				facts.Outcome = PlacementUnproven
			}
		}
	}
	return facts
}

// HasSubpartitionDeclaration reports whether any structured declaration
// carries a subpartitioning axis — the CREATE tail, member definitions,
// alter member defs, or an alter PARTITION BY scheme including its defs.
// Consumers adjudicate the execution boundary by dialect, version and
// statement form; this predicate only preserves the authored marker.
func HasSubpartitionDeclaration(ddl *DDL) bool {
	if ddl == nil {
		return false
	}
	if ddl.Subpartitioned {
		return true
	}
	for i := range ddl.PartitionDefs {
		if ddl.PartitionDefs[i].Subpartitioned {
			return true
		}
	}
	for i := range ddl.Alter {
		for j := range ddl.Alter[i].PartitionDefs {
			if ddl.Alter[i].PartitionDefs[j].Subpartitioned {
				return true
			}
		}
		if scheme := ddl.Alter[i].PartitionScheme; scheme != nil {
			if scheme.Subpartitioned {
				return true
			}
			for j := range scheme.Defs {
				if scheme.Defs[j].Subpartitioned {
					return true
				}
			}
		}
	}
	return false
}
